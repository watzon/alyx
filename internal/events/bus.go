package events

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/database"
)

// EventHandler is a function that handles an event.
type EventHandler func(ctx context.Context, event *Event) error

// EventBus manages event publishing and subscription.
type EventBus struct {
	db          *database.DB
	store       *Store
	subscribers map[string][]EventHandler // key: "type:source:action"
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	retention   time.Duration
}

// EventBusConfig holds configuration for EventBus.
type EventBusConfig struct {
	// Retention is how long to keep completed/failed events (default: 7 days).
	Retention time.Duration
	// ProcessInterval is how often to poll for pending events (default: 1 second).
	ProcessInterval time.Duration
	// CleanupInterval is how often to cleanup old events (default: 1 hour).
	CleanupInterval time.Duration
}

// NewEventBus creates a new event bus.
func NewEventBus(db *database.DB, config *EventBusConfig) *EventBus {
	if config == nil {
		config = &EventBusConfig{}
	}
	if config.Retention == 0 {
		config.Retention = 7 * 24 * time.Hour
	}
	if config.ProcessInterval == 0 {
		config.ProcessInterval = 1 * time.Second
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 1 * time.Hour
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &EventBus{
		db:          db,
		store:       NewStore(db),
		subscribers: make(map[string][]EventHandler),
		ctx:         ctx,
		cancel:      cancel,
		retention:   config.Retention,
	}
}

// Start begins background processing.
func (bus *EventBus) Start(ctx context.Context, config *EventBusConfig) {
	if config == nil {
		config = &EventBusConfig{}
	}
	if config.ProcessInterval == 0 {
		config.ProcessInterval = 1 * time.Second
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 1 * time.Hour
	}

	bus.wg.Add(2)
	go bus.processLoop(bus.ctx, config.ProcessInterval)
	go bus.cleanupLoop(bus.ctx, config.CleanupInterval)
}

// Stop gracefully shuts down the event bus.
func (bus *EventBus) Stop() {
	bus.cancel()
	bus.wg.Wait()
}

// Publish publishes an event to the queue.
func (bus *EventBus) Publish(ctx context.Context, event *Event) error {
	if err := bus.store.Create(ctx, event); err != nil {
		return fmt.Errorf("creating event: %w", err)
	}

	log.Debug().
		Str("event_id", event.ID).
		Str("type", string(event.Type)).
		Str("source", event.Source).
		Str("action", event.Action).
		Msg("Event published")

	return nil
}

// Subscribe registers a handler for events matching the pattern.
// Use "*" for source or action to match all.
func (bus *EventBus) Subscribe(eventType EventType, source, action string, handler EventHandler) {
	key := bus.makeKey(eventType, source, action)

	bus.mu.Lock()
	defer bus.mu.Unlock()

	bus.subscribers[key] = append(bus.subscribers[key], handler)

	log.Debug().
		Str("type", string(eventType)).
		Str("source", source).
		Str("action", action).
		Msg("Handler subscribed")
}

// ProcessPending processes pending events.
func (bus *EventBus) ProcessPending(ctx context.Context) error {
	events, err := bus.store.GetPending(ctx, 100)
	if err != nil {
		return fmt.Errorf("getting pending events: %w", err)
	}

	for _, event := range events {
		if err := bus.processEvent(ctx, event); err != nil {
			log.Error().
				Err(err).
				Str("event_id", event.ID).
				Msg("Failed to process event")
		}
	}

	return nil
}

// ProcessScheduled processes events with process_at in the past.
func (bus *EventBus) ProcessScheduled(ctx context.Context) error {
	events, err := bus.store.GetScheduled(ctx, 100)
	if err != nil {
		return fmt.Errorf("getting scheduled events: %w", err)
	}

	for _, event := range events {
		if err := bus.processEvent(ctx, event); err != nil {
			log.Error().
				Err(err).
				Str("event_id", event.ID).
				Msg("Failed to process scheduled event")
		}
	}

	return nil
}

// processEvent processes a single event.
func (bus *EventBus) processEvent(ctx context.Context, event *Event) error {
	// Update status to processing
	if err := bus.store.UpdateStatus(ctx, event.ID, "processing"); err != nil {
		return fmt.Errorf("updating event status to processing: %w", err)
	}

	// Find matching handlers
	handlers := bus.findHandlers(event)

	if len(handlers) == 0 {
		log.Debug().
			Str("event_id", event.ID).
			Str("type", string(event.Type)).
			Str("source", event.Source).
			Str("action", event.Action).
			Msg("No handlers found for event")

		// Mark as completed even if no handlers
		if err := bus.store.UpdateStatus(ctx, event.ID, "completed"); err != nil {
			return fmt.Errorf("updating event status to completed: %w", err)
		}
		return nil
	}

	// Execute handlers
	var handlerErr error
	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			log.Error().
				Err(err).
				Str("event_id", event.ID).
				Msg("Handler failed")
			handlerErr = err
			// Continue executing other handlers
		}
	}

	// Update status based on handler results
	status := "completed"
	if handlerErr != nil {
		status = "failed"
	}

	if err := bus.store.UpdateStatus(ctx, event.ID, status); err != nil {
		return fmt.Errorf("updating event status to %s: %w", status, err)
	}

	log.Debug().
		Str("event_id", event.ID).
		Str("status", status).
		Int("handlers", len(handlers)).
		Msg("Event processed")

	return handlerErr
}

// findHandlers finds all handlers matching the event.
func (bus *EventBus) findHandlers(event *Event) []EventHandler {
	bus.mu.RLock()
	defer bus.mu.RUnlock()

	var handlers []EventHandler

	// Try exact match
	key := bus.makeKey(event.Type, event.Source, event.Action)
	if h, ok := bus.subscribers[key]; ok {
		handlers = append(handlers, h...)
	}

	// Try wildcard source
	key = bus.makeKey(event.Type, "*", event.Action)
	if h, ok := bus.subscribers[key]; ok {
		handlers = append(handlers, h...)
	}

	// Try wildcard action
	key = bus.makeKey(event.Type, event.Source, "*")
	if h, ok := bus.subscribers[key]; ok {
		handlers = append(handlers, h...)
	}

	// Try wildcard source and action
	key = bus.makeKey(event.Type, "*", "*")
	if h, ok := bus.subscribers[key]; ok {
		handlers = append(handlers, h...)
	}

	return handlers
}

// makeKey creates a subscription key.
func (bus *EventBus) makeKey(eventType EventType, source, action string) string {
	return fmt.Sprintf("%s:%s:%s", eventType, source, action)
}

// processLoop periodically processes pending and scheduled events.
func (bus *EventBus) processLoop(ctx context.Context, interval time.Duration) {
	defer bus.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := bus.ProcessPending(ctx); err != nil {
				log.Error().Err(err).Msg("Failed to process pending events")
			}
			if err := bus.ProcessScheduled(ctx); err != nil {
				log.Error().Err(err).Msg("Failed to process scheduled events")
			}
		}
	}
}

// cleanupLoop periodically removes old events.
func (bus *EventBus) cleanupLoop(ctx context.Context, interval time.Duration) {
	defer bus.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := bus.store.DeleteOlderThan(ctx, bus.retention); err != nil {
				log.Error().Err(err).Msg("Failed to cleanup old events")
			}
		}
	}
}
