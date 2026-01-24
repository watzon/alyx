package realtime

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

// Broker manages WebSocket clients and subscriptions.
type Broker struct {
	db     *database.DB
	schema *schema.Schema

	clients       map[string]*Client
	subscriptions map[string]*Subscription
	index         *SubscriptionIndex
	detector      *ChangeDetector

	mu       sync.RWMutex
	done     chan struct{}
	changeCh chan *Change
}

// BrokerConfig holds configuration for the broker.
type BrokerConfig struct {
	PollInterval   int64
	MaxConnections int
	BufferSize     int
}

// NewBroker creates a new subscription broker.
func NewBroker(db *database.DB, s *schema.Schema, cfg *BrokerConfig) *Broker {
	if cfg == nil {
		cfg = &BrokerConfig{
			PollInterval:   50,
			MaxConnections: 1000,
			BufferSize:     1000,
		}
	}

	b := &Broker{
		db:            db,
		schema:        s,
		clients:       make(map[string]*Client),
		subscriptions: make(map[string]*Subscription),
		index:         NewSubscriptionIndex(),
		done:          make(chan struct{}),
		changeCh:      make(chan *Change, cfg.BufferSize),
	}

	b.detector = NewChangeDetector(db, cfg.PollInterval, b.changeCh)
	return b
}

// Start begins processing changes and broadcasting to subscribers.
func (b *Broker) Start(ctx context.Context) error {
	go b.detector.Start(ctx)
	go b.processChanges(ctx)
	return nil
}

// Stop gracefully shuts down the broker.
func (b *Broker) Stop() {
	close(b.done)
	b.detector.Stop()

	b.mu.Lock()
	clients := make([]*Client, 0, len(b.clients))
	for _, client := range b.clients {
		clients = append(clients, client)
	}
	b.clients = make(map[string]*Client)
	b.subscriptions = make(map[string]*Subscription)
	b.mu.Unlock()

	for _, client := range clients {
		client.CloseWithoutUnsubscribe()
	}
}

// RegisterClient adds a new client to the broker.
func (b *Broker) RegisterClient(client *Client) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.clients[client.ID] = client
	log.Debug().Str("client_id", client.ID).Int("total_clients", len(b.clients)).Msg("Client connected")
}

// UnregisterClient removes a client from the broker.
func (b *Broker) UnregisterClient(clientID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	client, ok := b.clients[clientID]
	if !ok {
		return
	}

	for _, sub := range client.Subscriptions() {
		delete(b.subscriptions, sub.ID)
		b.index.Remove(sub)
	}

	delete(b.clients, clientID)
	log.Debug().Str("client_id", clientID).Int("total_clients", len(b.clients)).Msg("Client disconnected")
}

// Subscribe creates a new subscription for a client.
func (b *Broker) Subscribe(client *Client, sub *Subscription) (*SubscriptionSnapshot, error) {
	col, ok := b.schema.Collections[sub.Collection]
	if !ok {
		return nil, ErrCollectionNotFound
	}

	if err := client.AddSubscription(sub); err != nil {
		return nil, err
	}

	b.mu.Lock()
	b.subscriptions[sub.ID] = sub
	b.index.Add(sub)
	b.mu.Unlock()

	snapshot, err := b.executeSubscriptionQuery(sub, col)
	if err != nil {
		b.mu.Lock()
		delete(b.subscriptions, sub.ID)
		b.index.Remove(sub)
		b.mu.Unlock()
		client.RemoveSubscription(sub.ID)
		return nil, err
	}

	return snapshot, nil
}

// Unsubscribe removes a subscription.
func (b *Broker) Unsubscribe(subID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub, ok := b.subscriptions[subID]
	if !ok {
		return
	}

	delete(b.subscriptions, subID)
	b.index.Remove(sub)
}

// SubscriptionSnapshot holds the initial data for a subscription.
type SubscriptionSnapshot struct {
	Docs  []any
	Total int64
}

func (b *Broker) executeSubscriptionQuery(sub *Subscription, col *schema.Collection) (*SubscriptionSnapshot, error) {
	collection := database.NewCollection(b.db, col)

	opts := &database.QueryOptions{
		Limit:  sub.Limit,
		Expand: sub.Expand,
	}

	for field, filter := range sub.Filter {
		dbFilters := convertFilter(field, filter)
		opts.Filters = append(opts.Filters, dbFilters...)
	}

	for _, s := range sub.Sort {
		field, order := database.ParseSortString(s)
		opts.Sorts = append(opts.Sorts, &database.Sort{Field: field, Order: order})
	}

	result, err := collection.Find(context.Background(), opts)
	if err != nil {
		return nil, err
	}

	sub.DocIDs = make(map[string]struct{})
	docs := make([]any, 0, len(result.Docs))
	pk := col.PrimaryKeyField()

	for _, doc := range result.Docs {
		if pk != nil {
			if id, ok := doc[pk.Name]; ok {
				sub.DocIDs[toString(id)] = struct{}{}
			}
		}
		docs = append(docs, doc)
	}

	return &SubscriptionSnapshot{
		Docs:  docs,
		Total: result.Total,
	}, nil
}

func (b *Broker) processChanges(ctx context.Context) {
	for {
		select {
		case change := <-b.changeCh:
			b.broadcastChange(change)
		case <-b.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (b *Broker) broadcastChange(change *Change) {
	b.mu.RLock()
	candidates := b.index.GetCandidates(change.Collection)
	b.mu.RUnlock()

	col, ok := b.schema.Collections[change.Collection]
	if !ok {
		return
	}

	for _, sub := range candidates {
		if sub.State != SubscriptionStateActive {
			continue
		}

		client := b.getClient(sub.ClientID)
		if client == nil {
			continue
		}

		delta, err := b.calculateDelta(sub, col, change)
		if err != nil {
			log.Error().Err(err).
				Str("subscription_id", sub.ID).
				Str("collection", change.Collection).
				Msg("Failed to calculate delta")
			continue
		}

		if delta == nil || delta.IsEmpty() {
			continue
		}

		b.sendDelta(client, sub, delta)
	}
}

func (b *Broker) calculateDelta(sub *Subscription, col *schema.Collection, change *Change) (*Changes, error) {
	collection := database.NewCollection(b.db, col)
	pk := col.PrimaryKeyField()
	if pk == nil {
		return nil, nil
	}

	delta := &Changes{}

	switch change.Operation {
	case OperationInsert:
		doc, err := collection.FindOne(context.Background(), change.DocID)
		if err != nil {
			if err == database.ErrNotFound {
				return nil, nil
			}
			return nil, err
		}

		if b.matchesFilter(doc, sub.Filter) {
			delta.Inserts = append(delta.Inserts, doc)
			sub.DocIDs[change.DocID] = struct{}{}
		}

	case OperationUpdate:
		_, wasInSet := sub.DocIDs[change.DocID]

		doc, err := collection.FindOne(context.Background(), change.DocID)
		if err != nil {
			if err == database.ErrNotFound {
				if wasInSet {
					delta.Deletes = append(delta.Deletes, change.DocID)
					delete(sub.DocIDs, change.DocID)
				}
				return delta, nil
			}
			return nil, err
		}

		matchesNow := b.matchesFilter(doc, sub.Filter)

		if wasInSet && matchesNow {
			delta.Updates = append(delta.Updates, doc)
		} else if !wasInSet && matchesNow {
			delta.Inserts = append(delta.Inserts, doc)
			sub.DocIDs[change.DocID] = struct{}{}
		} else if wasInSet && !matchesNow {
			delta.Deletes = append(delta.Deletes, change.DocID)
			delete(sub.DocIDs, change.DocID)
		}

	case OperationDelete:
		if _, wasInSet := sub.DocIDs[change.DocID]; wasInSet {
			delta.Deletes = append(delta.Deletes, change.DocID)
			delete(sub.DocIDs, change.DocID)
		}
	}

	return delta, nil
}

func (b *Broker) matchesFilter(doc database.Row, filters map[string]Filter) bool {
	if len(filters) == 0 {
		return true
	}

	for field, filter := range filters {
		val, exists := doc[field]
		if !exists {
			return false
		}

		if !matchValue(val, filter) {
			return false
		}
	}

	return true
}

func matchValue(val any, filter Filter) bool {
	if filter.Eq != nil && !equals(val, filter.Eq) {
		return false
	}
	if filter.Ne != nil && equals(val, filter.Ne) {
		return false
	}
	if filter.In != nil && !inArray(val, filter.In) {
		return false
	}
	return true
}

func equals(a, b any) bool {
	return toString(a) == toString(b)
}

func inArray(val any, arr []any) bool {
	for _, v := range arr {
		if equals(val, v) {
			return true
		}
	}
	return false
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func (b *Broker) sendDelta(client *Client, sub *Subscription, delta *Changes) {
	payload, _ := json.Marshal(&DeltaPayload{
		SubscriptionID: sub.ID,
		Changes:        *delta,
	})

	_ = client.Send(&Message{
		Type:    MessageTypeDelta,
		Payload: payload,
	})
}

func (b *Broker) getClient(clientID string) *Client {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.clients[clientID]
}

// ClientCount returns the number of connected clients.
func (b *Broker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// SubscriptionCount returns the total number of active subscriptions.
func (b *Broker) SubscriptionCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscriptions)
}

func convertFilter(field string, filter Filter) []*database.Filter {
	var filters []*database.Filter

	if filter.Eq != nil {
		filters = append(filters, &database.Filter{Field: field, Op: database.OpEq, Value: filter.Eq})
	}
	if filter.Ne != nil {
		filters = append(filters, &database.Filter{Field: field, Op: database.OpNe, Value: filter.Ne})
	}
	if filter.Gt != nil {
		filters = append(filters, &database.Filter{Field: field, Op: database.OpGt, Value: filter.Gt})
	}
	if filter.Gte != nil {
		filters = append(filters, &database.Filter{Field: field, Op: database.OpGte, Value: filter.Gte})
	}
	if filter.Lt != nil {
		filters = append(filters, &database.Filter{Field: field, Op: database.OpLt, Value: filter.Lt})
	}
	if filter.Lte != nil {
		filters = append(filters, &database.Filter{Field: field, Op: database.OpLte, Value: filter.Lte})
	}
	if filter.Like != nil {
		filters = append(filters, &database.Filter{Field: field, Op: database.OpLike, Value: filter.Like})
	}
	if filter.In != nil {
		filters = append(filters, &database.Filter{Field: field, Op: database.OpIn, Value: filter.In})
	}
	if filter.Contains != nil {
		filters = append(filters, &database.Filter{Field: field, Op: database.OpContains, Value: filter.Contains})
	}

	return filters
}

// IsEmpty returns true if there are no changes.
func (c *Changes) IsEmpty() bool {
	return len(c.Inserts) == 0 && len(c.Updates) == 0 && len(c.Deletes) == 0
}
