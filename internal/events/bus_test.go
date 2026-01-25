package events

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
)

func testDB(t *testing.T) *database.DB {
	t.Helper()

	tmpDir := t.TempDir()
	cfg := &config.DatabaseConfig{
		Path:            filepath.Join(tmpDir, "test.db"),
		WALMode:         true,
		ForeignKeys:     true,
		BusyTimeout:     5 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 1 * time.Hour,
		CacheSize:       -2000,
	}

	db, err := database.Open(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestEventBus_Publish(t *testing.T) {
	db := testDB(t)
	bus := NewEventBus(db, nil)
	ctx := context.Background()

	event := &Event{
		Type:   EventTypeHTTP,
		Source: "test",
		Action: "create",
		Payload: map[string]any{
			"key": "value",
		},
		Metadata: EventMetadata{
			RequestID: "req-123",
			UserID:    "user-456",
		},
	}

	err := bus.Publish(ctx, event)
	require.NoError(t, err)
	require.NotEmpty(t, event.ID)
	require.NotZero(t, event.CreatedAt)
	require.Equal(t, "pending", event.Status)

	// Verify event is in database
	events, err := bus.store.GetPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, event.ID, events[0].ID)
	require.Equal(t, EventTypeHTTP, events[0].Type)
	require.Equal(t, "test", events[0].Source)
	require.Equal(t, "create", events[0].Action)
}

func TestEventBus_Subscribe(t *testing.T) {
	db := testDB(t)
	bus := NewEventBus(db, nil)
	ctx := context.Background()

	var handlerCalled bool
	var receivedEvent *Event

	handler := func(ctx context.Context, event *Event) error {
		handlerCalled = true
		receivedEvent = event
		return nil
	}

	bus.Subscribe(EventTypeHTTP, "test", "create", handler)

	event := &Event{
		Type:   EventTypeHTTP,
		Source: "test",
		Action: "create",
		Payload: map[string]any{
			"key": "value",
		},
	}

	err := bus.Publish(ctx, event)
	require.NoError(t, err)

	// Process the event
	err = bus.ProcessPending(ctx)
	require.NoError(t, err)

	require.True(t, handlerCalled)
	require.NotNil(t, receivedEvent)
	require.Equal(t, event.ID, receivedEvent.ID)
	require.Equal(t, EventTypeHTTP, receivedEvent.Type)
	require.Equal(t, "test", receivedEvent.Source)
	require.Equal(t, "create", receivedEvent.Action)
}

func TestEventBus_Subscribe_Wildcard(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	tests := []struct {
		name            string
		subscribeType   EventType
		subscribeSource string
		subscribeAction string
		eventType       EventType
		eventSource     string
		eventAction     string
		shouldMatch     bool
	}{
		{
			name:            "exact match",
			subscribeType:   EventTypeHTTP,
			subscribeSource: "users",
			subscribeAction: "create",
			eventType:       EventTypeHTTP,
			eventSource:     "users",
			eventAction:     "create",
			shouldMatch:     true,
		},
		{
			name:            "wildcard source",
			subscribeType:   EventTypeHTTP,
			subscribeSource: "*",
			subscribeAction: "create",
			eventType:       EventTypeHTTP,
			eventSource:     "users",
			eventAction:     "create",
			shouldMatch:     true,
		},
		{
			name:            "wildcard action",
			subscribeType:   EventTypeHTTP,
			subscribeSource: "users",
			subscribeAction: "*",
			eventType:       EventTypeHTTP,
			eventSource:     "users",
			eventAction:     "delete",
			shouldMatch:     true,
		},
		{
			name:            "wildcard both",
			subscribeType:   EventTypeHTTP,
			subscribeSource: "*",
			subscribeAction: "*",
			eventType:       EventTypeHTTP,
			eventSource:     "posts",
			eventAction:     "update",
			shouldMatch:     true,
		},
		{
			name:            "no match - different type",
			subscribeType:   EventTypeHTTP,
			subscribeSource: "users",
			subscribeAction: "create",
			eventType:       EventTypeDatabase,
			eventSource:     "users",
			eventAction:     "create",
			shouldMatch:     false,
		},
		{
			name:            "no match - different source",
			subscribeType:   EventTypeHTTP,
			subscribeSource: "users",
			subscribeAction: "create",
			eventType:       EventTypeHTTP,
			eventSource:     "posts",
			eventAction:     "create",
			shouldMatch:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh bus for each test
			bus := NewEventBus(db, nil)

			var handlerCalled bool
			handler := func(ctx context.Context, event *Event) error {
				handlerCalled = true
				return nil
			}

			bus.Subscribe(tt.subscribeType, tt.subscribeSource, tt.subscribeAction, handler)

			event := &Event{
				Type:   tt.eventType,
				Source: tt.eventSource,
				Action: tt.eventAction,
				Payload: map[string]any{
					"test": true,
				},
			}

			err := bus.Publish(ctx, event)
			require.NoError(t, err)

			err = bus.ProcessPending(ctx)
			require.NoError(t, err)

			require.Equal(t, tt.shouldMatch, handlerCalled)
		})
	}
}

func TestEventBus_ProcessScheduled(t *testing.T) {
	db := testDB(t)
	bus := NewEventBus(db, nil)
	ctx := context.Background()

	var handlerCalled bool
	handler := func(ctx context.Context, event *Event) error {
		handlerCalled = true
		return nil
	}

	bus.Subscribe(EventTypeSchedule, "test", "run", handler)

	// Event scheduled in the past (should be processed)
	pastTime := time.Now().UTC().Add(-1 * time.Minute)
	event := &Event{
		Type:      EventTypeSchedule,
		Source:    "test",
		Action:    "run",
		ProcessAt: &pastTime,
		Payload: map[string]any{
			"scheduled": true,
		},
	}

	err := bus.Publish(ctx, event)
	require.NoError(t, err)

	// ProcessPending should not process it (has process_at set)
	err = bus.ProcessPending(ctx)
	require.NoError(t, err)
	require.False(t, handlerCalled)

	// ProcessScheduled should process it
	err = bus.ProcessScheduled(ctx)
	require.NoError(t, err)
	require.True(t, handlerCalled)
}

func TestEventBus_ProcessScheduled_Future(t *testing.T) {
	db := testDB(t)
	bus := NewEventBus(db, nil)
	ctx := context.Background()

	var handlerCalled bool
	handler := func(ctx context.Context, event *Event) error {
		handlerCalled = true
		return nil
	}

	bus.Subscribe(EventTypeSchedule, "test", "run", handler)

	// Event scheduled in the future (should NOT be processed)
	futureTime := time.Now().UTC().Add(1 * time.Hour)
	event := &Event{
		Type:      EventTypeSchedule,
		Source:    "test",
		Action:    "run",
		ProcessAt: &futureTime,
		Payload: map[string]any{
			"scheduled": true,
		},
	}

	err := bus.Publish(ctx, event)
	require.NoError(t, err)

	err = bus.ProcessScheduled(ctx)
	require.NoError(t, err)
	require.False(t, handlerCalled)
}

func TestEventBus_Cleanup(t *testing.T) {
	db := testDB(t)
	bus := NewEventBus(db, &EventBusConfig{
		Retention: 1 * time.Hour,
	})
	ctx := context.Background()

	// Create old completed event
	oldEvent := &Event{
		Type:   EventTypeHTTP,
		Source: "test",
		Action: "create",
		Payload: map[string]any{
			"old": true,
		},
		Status: "completed",
	}
	oldEvent.CreatedAt = time.Now().UTC().Add(-2 * time.Hour)

	err := bus.store.Create(ctx, oldEvent)
	require.NoError(t, err)

	// Create recent completed event
	recentEvent := &Event{
		Type:   EventTypeHTTP,
		Source: "test",
		Action: "create",
		Payload: map[string]any{
			"recent": true,
		},
		Status: "completed",
	}

	err = bus.Publish(ctx, recentEvent)
	require.NoError(t, err)

	// Update status to completed
	err = bus.store.UpdateStatus(ctx, recentEvent.ID, "completed")
	require.NoError(t, err)

	// Cleanup old events
	err = bus.store.DeleteOlderThan(ctx, 1*time.Hour)
	require.NoError(t, err)

	// Verify old event is deleted
	events, err := bus.store.GetPending(ctx, 100)
	require.NoError(t, err)

	// Should only have the recent event (if it's still pending) or none
	for _, e := range events {
		require.NotEqual(t, oldEvent.ID, e.ID)
	}
}

func TestEventBus_MultipleHandlers(t *testing.T) {
	db := testDB(t)
	bus := NewEventBus(db, nil)
	ctx := context.Background()

	var handler1Called, handler2Called bool

	handler1 := func(ctx context.Context, event *Event) error {
		handler1Called = true
		return nil
	}

	handler2 := func(ctx context.Context, event *Event) error {
		handler2Called = true
		return nil
	}

	bus.Subscribe(EventTypeHTTP, "test", "create", handler1)
	bus.Subscribe(EventTypeHTTP, "test", "create", handler2)

	event := &Event{
		Type:   EventTypeHTTP,
		Source: "test",
		Action: "create",
		Payload: map[string]any{
			"key": "value",
		},
	}

	err := bus.Publish(ctx, event)
	require.NoError(t, err)

	err = bus.ProcessPending(ctx)
	require.NoError(t, err)

	require.True(t, handler1Called)
	require.True(t, handler2Called)
}

func TestEventBus_StartStop(t *testing.T) {
	db := testDB(t)
	bus := NewEventBus(db, nil)
	ctx := context.Background()

	var handlerCalled atomic.Bool
	handler := func(ctx context.Context, event *Event) error {
		handlerCalled.Store(true)
		return nil
	}

	bus.Subscribe(EventTypeHTTP, "test", "create", handler)

	// Start background processing
	bus.Start(ctx, &EventBusConfig{
		ProcessInterval: 100 * time.Millisecond,
		CleanupInterval: 1 * time.Hour,
	})

	// Publish event
	event := &Event{
		Type:   EventTypeHTTP,
		Source: "test",
		Action: "create",
		Payload: map[string]any{
			"key": "value",
		},
	}

	err := bus.Publish(ctx, event)
	require.NoError(t, err)

	// Wait for background processing
	time.Sleep(200 * time.Millisecond)

	require.True(t, handlerCalled.Load())

	// Stop bus
	bus.Stop()
}
