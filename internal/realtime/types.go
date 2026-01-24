// Package realtime provides WebSocket-based real-time subscriptions.
package realtime

import (
	"encoding/json"
	"time"
)

// MessageType represents the type of WebSocket message.
type MessageType string

const (
	MessageTypeSubscribe   MessageType = "subscribe"
	MessageTypeUnsubscribe MessageType = "unsubscribe"
	MessageTypePing        MessageType = "ping"

	MessageTypeConnected MessageType = "connected"
	MessageTypeSnapshot  MessageType = "snapshot"
	MessageTypeDelta     MessageType = "delta"
	MessageTypeError     MessageType = "error"
	MessageTypePong      MessageType = "pong"
)

// Operation represents a database change operation.
type Operation string

const (
	OperationInsert Operation = "INSERT"
	OperationUpdate Operation = "UPDATE"
	OperationDelete Operation = "DELETE"
)

// Message is the base WebSocket message structure.
type Message struct {
	ID      string          `json:"id,omitempty"`
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// SubscribePayload is the payload for subscribe messages.
type SubscribePayload struct {
	Collection string            `json:"collection"`
	Filter     map[string]Filter `json:"filter,omitempty"`
	Sort       []string          `json:"sort,omitempty"`
	Limit      int               `json:"limit,omitempty"`
	Expand     []string          `json:"expand,omitempty"`
}

// Filter represents a filter condition.
type Filter struct {
	Eq       any   `json:"$eq,omitempty"`
	Ne       any   `json:"$ne,omitempty"`
	Gt       any   `json:"$gt,omitempty"`
	Gte      any   `json:"$gte,omitempty"`
	Lt       any   `json:"$lt,omitempty"`
	Lte      any   `json:"$lte,omitempty"`
	Like     any   `json:"$like,omitempty"`
	In       []any `json:"$in,omitempty"`
	Contains any   `json:"$contains,omitempty"`
}

// UnsubscribePayload is the payload for unsubscribe messages.
type UnsubscribePayload struct {
	SubscriptionID string `json:"subscription_id"`
}

// ConnectedPayload is the payload for connected messages.
type ConnectedPayload struct {
	ClientID string `json:"client_id"`
}

// SnapshotPayload is the payload for snapshot messages.
type SnapshotPayload struct {
	SubscriptionID string `json:"subscription_id"`
	Docs           []any  `json:"docs"`
	Total          int64  `json:"total"`
}

// DeltaPayload is the payload for delta messages.
type DeltaPayload struct {
	SubscriptionID string  `json:"subscription_id"`
	Changes        Changes `json:"changes"`
}

// Changes represents the set of changes in a delta.
type Changes struct {
	Inserts []any    `json:"inserts,omitempty"`
	Updates []any    `json:"updates,omitempty"`
	Deletes []string `json:"deletes,omitempty"`
}

// ErrorPayload is the payload for error messages.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Change represents a single database change event.
type Change struct {
	ID            int64     `json:"id"`
	Collection    string    `json:"collection"`
	Operation     Operation `json:"operation"`
	DocID         string    `json:"doc_id"`
	ChangedFields []string  `json:"changed_fields,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

// SubscriptionState represents the current state of a subscription.
type SubscriptionState string

const (
	SubscriptionStateActive   SubscriptionState = "active"
	SubscriptionStatePaused   SubscriptionState = "paused"
	SubscriptionStateCanceled SubscriptionState = "canceled"
)

// Subscription represents an active subscription.
type Subscription struct {
	ID           string            `json:"id"`
	ClientID     string            `json:"client_id"`
	Collection   string            `json:"collection"`
	Filter       map[string]Filter `json:"filter,omitempty"`
	Sort         []string          `json:"sort,omitempty"`
	Limit        int               `json:"limit,omitempty"`
	Expand       []string          `json:"expand,omitempty"`
	State        SubscriptionState `json:"state"`
	CreatedAt    time.Time         `json:"created_at"`
	LastSyncedAt time.Time         `json:"last_synced_at"`

	// AuthContext stores the authenticated user context for rule evaluation.
	AuthContext map[string]any `json:"-"`

	DocIDs map[string]struct{} `json:"-"`
}

// NewSubscription creates a new subscription from a subscribe payload.
func NewSubscription(clientID string, payload *SubscribePayload, authContext map[string]any) *Subscription {
	limit := payload.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	return &Subscription{
		ClientID:    clientID,
		Collection:  payload.Collection,
		Filter:      payload.Filter,
		Sort:        payload.Sort,
		Limit:       limit,
		Expand:      payload.Expand,
		State:       SubscriptionStateActive,
		CreatedAt:   time.Now(),
		AuthContext: authContext,
		DocIDs:      make(map[string]struct{}),
	}
}

// MatchesChange returns true if the change potentially affects this subscription.
func (s *Subscription) MatchesChange(change *Change) bool {
	return s.Collection == change.Collection
}

// ClientState represents the state of a connected client.
type ClientState string

const (
	ClientStateConnected    ClientState = "connected"
	ClientStateDisconnected ClientState = "disconnected"
)

// ErrorCode represents an error code for WebSocket errors.
type ErrorCode string

const (
	ErrorCodeInvalidMessage     ErrorCode = "INVALID_MESSAGE"
	ErrorCodeInvalidPayload     ErrorCode = "INVALID_PAYLOAD"
	ErrorCodeCollectionNotFound ErrorCode = "COLLECTION_NOT_FOUND"
	ErrorCodeInvalidFilter      ErrorCode = "INVALID_FILTER"
	ErrorCodeSubscriptionLimit  ErrorCode = "SUBSCRIPTION_LIMIT_REACHED"
	ErrorCodeInternalError      ErrorCode = "INTERNAL_ERROR"
)
