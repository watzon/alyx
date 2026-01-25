package events

import "time"

// EventType represents the type of event.
type EventType string

const (
	// EventTypeHTTP represents an HTTP request event.
	EventTypeHTTP EventType = "http"
	// EventTypeWebhook represents a webhook event.
	EventTypeWebhook EventType = "webhook"
	// EventTypeDatabase represents a database operation event.
	EventTypeDatabase EventType = "database"
	// EventTypeAuth represents an authentication event.
	EventTypeAuth EventType = "auth"
	// EventTypeSchedule represents a scheduled event.
	EventTypeSchedule EventType = "schedule"
	// EventTypeCustom represents a custom event.
	EventTypeCustom EventType = "custom"
)

// Event represents an event in the event bus queue.
type Event struct {
	ID          string        // Unique event ID
	Type        EventType     // Event type (http, webhook, database, etc.)
	Source      string        // Event source (collection name, auth action, etc.)
	Action      string        // Specific action (insert, update, delete, etc.)
	Payload     any           // Event payload (JSON-serializable)
	Metadata    EventMetadata // Additional metadata
	CreatedAt   time.Time     // When event was created
	ProcessAt   *time.Time    // When to process (nil = immediate)
	ProcessedAt *time.Time    // When event was processed
	Status      string        // Event status (pending, processing, completed, failed)
}

// EventMetadata contains additional context for an event.
type EventMetadata struct {
	RequestID string         // Request ID for tracing
	UserID    string         // User who triggered the event
	IP        string         // IP address
	UserAgent string         // User agent
	Extra     map[string]any // Additional metadata
}
