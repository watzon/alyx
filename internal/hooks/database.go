package hooks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/events"
)

// DatabaseHookTrigger manages database event hooks.
type DatabaseHookTrigger struct {
	registry  *Registry
	bus       *events.EventBus
	mu        sync.RWMutex
	executing map[string]bool // Track executing functions for cycle detection
}

// NewDatabaseHookTrigger creates a new database hook trigger.
func NewDatabaseHookTrigger(registry *Registry, bus *events.EventBus) *DatabaseHookTrigger {
	return &DatabaseHookTrigger{
		registry:  registry,
		bus:       bus,
		executing: make(map[string]bool),
	}
}

// OnInsert triggers hooks for INSERT operations.
func (t *DatabaseHookTrigger) OnInsert(ctx context.Context, collection string, document map[string]any) error {
	payload := map[string]any{
		"document":   document,
		"action":     "insert",
		"collection": collection,
	}

	return t.executeHooks(ctx, string(events.EventTypeDatabase), collection, "insert", payload)
}

// OnUpdate triggers hooks for UPDATE operations.
func (t *DatabaseHookTrigger) OnUpdate(ctx context.Context, collection string, document, previousDocument map[string]any) error {
	changedFields := calculateChangedFields(document, previousDocument)

	payload := map[string]any{
		"document":          document,
		"previous_document": previousDocument,
		"action":            "update",
		"collection":        collection,
		"changed_fields":    changedFields,
	}

	return t.executeHooks(ctx, string(events.EventTypeDatabase), collection, "update", payload)
}

// OnDelete triggers hooks for DELETE operations.
func (t *DatabaseHookTrigger) OnDelete(ctx context.Context, collection string, document map[string]any) error {
	payload := map[string]any{
		"document":   document,
		"action":     "delete",
		"collection": collection,
	}

	return t.executeHooks(ctx, string(events.EventTypeDatabase), collection, "delete", payload)
}

// executeHooks executes hooks for an event.
func (t *DatabaseHookTrigger) executeHooks(ctx context.Context, eventType, source, action string, payload any) error {
	// Find matching hooks
	hooks, err := t.registry.FindByEvent(ctx, eventType, source, action)
	if err != nil {
		return fmt.Errorf("finding hooks: %w", err)
	}

	if len(hooks) == 0 {
		log.Debug().
			Str("event_type", eventType).
			Str("source", source).
			Str("action", action).
			Msg("No hooks found for database event")
		return nil
	}

	log.Debug().
		Str("event_type", eventType).
		Str("source", source).
		Str("action", action).
		Int("hook_count", len(hooks)).
		Msg("Executing database hooks")

	// Execute hooks based on mode
	for _, hook := range hooks {
		// Check for cycle
		if t.checkCycle(hook.FunctionID) {
			log.Warn().
				Str("hook_id", hook.ID).
				Str("function_id", hook.FunctionID).
				Msg("Cycle detected - skipping hook execution")
			continue
		}

		if hook.Mode == HookModeSync {
			// Execute synchronously
			if err := t.executeSyncHook(ctx, hook, payload); err != nil {
				// Check on_failure behavior
				if hook.Config.OnFailure == "reject" {
					return fmt.Errorf("sync hook %s failed: %w", hook.Name, err)
				}
				// Continue on failure
				log.Error().
					Err(err).
					Str("hook_id", hook.ID).
					Str("hook_name", hook.Name).
					Msg("Sync hook failed but continuing")
			}
		} else {
			// Execute asynchronously via event bus
			if err := t.executeAsyncHook(ctx, hook, eventType, source, action, payload); err != nil {
				log.Error().
					Err(err).
					Str("hook_id", hook.ID).
					Str("hook_name", hook.Name).
					Msg("Failed to publish async hook event")
			}
		}
	}

	return nil
}

// executeSyncHook executes a synchronous hook with timeout.
func (t *DatabaseHookTrigger) executeSyncHook(ctx context.Context, hook *Hook, payload any) error {
	// Set timeout (default 5s)
	timeout := hook.Config.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Mark function as executing
	t.mu.Lock()
	t.executing[hook.FunctionID] = true
	t.mu.Unlock()

	// Ensure cleanup
	defer func() {
		t.mu.Lock()
		delete(t.executing, hook.FunctionID)
		t.mu.Unlock()
	}()

	// TODO: Execute function via function runtime
	// For now, just log
	log.Debug().
		Str("hook_id", hook.ID).
		Str("function_id", hook.FunctionID).
		Str("mode", string(hook.Mode)).
		Msg("Executing sync hook (function runtime not yet implemented)")

	// Simulate execution
	select {
	case <-execCtx.Done():
		if execCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("hook execution timeout after %v", timeout)
		}
		return execCtx.Err()
	default:
		// Success
		return nil
	}
}

// executeAsyncHook publishes an event to the event bus for async execution.
func (t *DatabaseHookTrigger) executeAsyncHook(ctx context.Context, hook *Hook, eventType, source, action string, payload any) error {
	event := &events.Event{
		ID:      uuid.New().String(),
		Type:    events.EventType(eventType),
		Source:  source,
		Action:  action,
		Payload: payload,
		Metadata: events.EventMetadata{
			Extra: map[string]any{
				"hook_id":     hook.ID,
				"function_id": hook.FunctionID,
			},
		},
		CreatedAt: time.Now().UTC(),
		Status:    "pending",
	}

	if err := t.bus.Publish(ctx, event); err != nil {
		return fmt.Errorf("publishing event: %w", err)
	}

	log.Debug().
		Str("hook_id", hook.ID).
		Str("function_id", hook.FunctionID).
		Str("event_id", event.ID).
		Msg("Async hook event published")

	return nil
}

// checkCycle checks if function is already executing (cycle detection).
func (t *DatabaseHookTrigger) checkCycle(functionID string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.executing[functionID]
}

// calculateChangedFields compares two documents and returns changed field names.
func calculateChangedFields(current, previous map[string]any) []string {
	var changed []string

	// Check all fields in current document
	for key, currentVal := range current {
		previousVal, exists := previous[key]
		if !exists || !deepEqual(currentVal, previousVal) {
			changed = append(changed, key)
		}
	}

	// Check for deleted fields
	for key := range previous {
		if _, exists := current[key]; !exists {
			changed = append(changed, key)
		}
	}

	return changed
}

// deepEqual performs a simple deep equality check.
// For production, consider using reflect.DeepEqual or a more robust comparison.
func deepEqual(a, b any) bool {
	// Simple comparison - works for primitives and nil
	// For complex types (maps, slices), this is a shallow comparison
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
