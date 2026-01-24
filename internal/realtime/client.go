package realtime

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	writeTimeout     = 10 * time.Second
	pingInterval     = 30 * time.Second
	pongTimeout      = 60 * time.Second
	maxMessageSize   = 512 * 1024
	maxSubscriptions = 100
	sendBufferSize   = 256
)

// Client represents a connected WebSocket client.
type Client struct {
	ID            string
	conn          *websocket.Conn
	broker        *Broker
	subscriptions map[string]*Subscription
	mu            sync.RWMutex
	sendCh        chan []byte
	done          chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewClient creates a new WebSocket client.
func NewClient(conn *websocket.Conn, broker *Broker) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		ID:            uuid.New().String(),
		conn:          conn,
		broker:        broker,
		subscriptions: make(map[string]*Subscription),
		sendCh:        make(chan []byte, sendBufferSize),
		done:          make(chan struct{}),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Run starts the client's read and write loops.
func (c *Client) Run() {
	go c.writePump()
	go c.pingPump()
	c.readPump()
}

// Close terminates the client connection and cleans up resources.
func (c *Client) Close() {
	c.mu.Lock()
	select {
	case <-c.done:
		c.mu.Unlock()
		return
	default:
		close(c.done)
	}
	c.mu.Unlock()

	c.cancel()

	c.mu.Lock()
	for _, sub := range c.subscriptions {
		c.broker.Unsubscribe(sub.ID)
	}
	c.subscriptions = make(map[string]*Subscription)
	c.mu.Unlock()

	c.conn.Close(websocket.StatusNormalClosure, "closing")
}

// CloseWithoutUnsubscribe terminates the connection without broker cleanup.
// Used during broker shutdown to avoid deadlock.
func (c *Client) CloseWithoutUnsubscribe() {
	c.mu.Lock()
	select {
	case <-c.done:
		c.mu.Unlock()
		return
	default:
		close(c.done)
	}
	c.subscriptions = make(map[string]*Subscription)
	c.mu.Unlock()

	c.cancel()
	c.conn.Close(websocket.StatusGoingAway, "server shutting down")
}

// Send queues a message to be sent to the client.
func (c *Client) Send(msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case c.sendCh <- data:
		return nil
	case <-c.done:
		return context.Canceled
	default:
		log.Warn().Str("client_id", c.ID).Msg("Client send buffer full, dropping message")
		return nil
	}
}

// SendError sends an error message to the client.
func (c *Client) SendError(msgID string, code ErrorCode, message string) error {
	payload, _ := json.Marshal(&ErrorPayload{
		Code:    string(code),
		Message: message,
	})

	return c.Send(&Message{
		ID:      msgID,
		Type:    MessageTypeError,
		Payload: payload,
	})
}

// AddSubscription registers a subscription for this client.
func (c *Client) AddSubscription(sub *Subscription) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.subscriptions) >= maxSubscriptions {
		return ErrSubscriptionLimit
	}

	c.subscriptions[sub.ID] = sub
	return nil
}

// RemoveSubscription removes a subscription from this client.
func (c *Client) RemoveSubscription(subID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subscriptions, subID)
}

// GetSubscription returns a subscription by ID.
func (c *Client) GetSubscription(subID string) *Subscription {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.subscriptions[subID]
}

// Subscriptions returns all subscriptions for this client.
func (c *Client) Subscriptions() []*Subscription {
	c.mu.RLock()
	defer c.mu.RUnlock()

	subs := make([]*Subscription, 0, len(c.subscriptions))
	for _, sub := range c.subscriptions {
		subs = append(subs, sub)
	}
	return subs
}

func (c *Client) readPump() {
	defer c.Close()

	c.conn.SetReadLimit(maxMessageSize)

	for {
		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure {
				log.Debug().Err(err).Str("client_id", c.ID).Msg("WebSocket read error")
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			_ = c.SendError("", ErrorCodeInvalidMessage, "Invalid JSON message")
			continue
		}

		c.handleMessage(&msg)
	}
}

func (c *Client) writePump() {
	for {
		select {
		case data := <-c.sendCh:
			ctx, cancel := context.WithTimeout(c.ctx, writeTimeout)
			err := c.conn.Write(ctx, websocket.MessageText, data)
			cancel()
			if err != nil {
				log.Debug().Err(err).Str("client_id", c.ID).Msg("WebSocket write error")
				return
			}
		case <-c.done:
			return
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Client) pingPump() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(c.ctx, pongTimeout)
			err := c.conn.Ping(ctx)
			cancel()
			if err != nil {
				log.Debug().Err(err).Str("client_id", c.ID).Msg("Ping failed")
				c.Close()
				return
			}
		case <-c.done:
			return
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Client) handleMessage(msg *Message) {
	switch msg.Type {
	case MessageTypeSubscribe:
		c.handleSubscribe(msg)
	case MessageTypeUnsubscribe:
		c.handleUnsubscribe(msg)
	case MessageTypePing:
		c.handlePing(msg)
	default:
		_ = c.SendError(msg.ID, ErrorCodeInvalidMessage, "Unknown message type")
	}
}

func (c *Client) handleSubscribe(msg *Message) {
	var payload SubscribePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		_ = c.SendError(msg.ID, ErrorCodeInvalidPayload, "Invalid subscribe payload")
		return
	}

	if payload.Collection == "" {
		_ = c.SendError(msg.ID, ErrorCodeInvalidPayload, "Collection is required")
		return
	}

	sub := NewSubscription(c.ID, &payload)
	sub.ID = uuid.New().String()

	snapshot, err := c.broker.Subscribe(c, sub)
	if err != nil {
		log.Error().Err(err).
			Str("client_id", c.ID).
			Str("collection", payload.Collection).
			Msg("Failed to create subscription")
		_ = c.SendError(msg.ID, ErrorCodeInternalError, err.Error())
		return
	}

	snapshotPayload, _ := json.Marshal(&SnapshotPayload{
		SubscriptionID: sub.ID,
		Docs:           snapshot.Docs,
		Total:          snapshot.Total,
	})

	_ = c.Send(&Message{
		ID:      msg.ID,
		Type:    MessageTypeSnapshot,
		Payload: snapshotPayload,
	})
}

func (c *Client) handleUnsubscribe(msg *Message) {
	var payload UnsubscribePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		_ = c.SendError(msg.ID, ErrorCodeInvalidPayload, "Invalid unsubscribe payload")
		return
	}

	if payload.SubscriptionID == "" {
		_ = c.SendError(msg.ID, ErrorCodeInvalidPayload, "Subscription ID is required")
		return
	}

	c.broker.Unsubscribe(payload.SubscriptionID)
	c.RemoveSubscription(payload.SubscriptionID)
}

func (c *Client) handlePing(msg *Message) {
	_ = c.Send(&Message{
		ID:   msg.ID,
		Type: MessageTypePong,
	})
}
