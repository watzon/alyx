package hooks

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/database"
)

// Registry manages hook registrations.
type Registry struct {
	db    *database.DB
	store *Store
	cache map[string]*Hook // key: hook ID
	mu    sync.RWMutex
}

// NewRegistry creates a new hook registry.
func NewRegistry(db *database.DB) (*Registry, error) {
	r := &Registry{
		db:    db,
		store: NewStore(db),
		cache: make(map[string]*Hook),
	}

	// Load all hooks into cache
	if err := r.loadCache(context.Background()); err != nil {
		return nil, fmt.Errorf("loading cache: %w", err)
	}

	return r, nil
}

// Register registers a new hook.
func (r *Registry) Register(ctx context.Context, hook *Hook) error {
	// Create in database
	if err := r.store.Create(ctx, hook); err != nil {
		return fmt.Errorf("creating hook: %w", err)
	}

	// Add to cache
	r.mu.Lock()
	r.cache[hook.ID] = hook
	r.mu.Unlock()

	log.Debug().
		Str("id", hook.ID).
		Str("name", hook.Name).
		Str("function_id", hook.FunctionID).
		Str("event_type", hook.EventType).
		Str("event_source", hook.EventSource).
		Str("event_action", hook.EventAction).
		Msg("Hook registered")

	return nil
}

// Unregister removes a hook.
func (r *Registry) Unregister(ctx context.Context, hookID string) error {
	// Delete from database
	if err := r.store.Delete(ctx, hookID); err != nil {
		return fmt.Errorf("deleting hook: %w", err)
	}

	// Remove from cache
	r.mu.Lock()
	delete(r.cache, hookID)
	r.mu.Unlock()

	log.Debug().Str("id", hookID).Msg("Hook unregistered")

	return nil
}

// FindByEvent finds hooks matching the event pattern.
// Returns hooks sorted by priority (descending).
func (r *Registry) FindByEvent(ctx context.Context, eventType, source, action string) ([]*Hook, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []*Hook

	// Iterate through cache and match hooks
	for _, hook := range r.cache {
		if !hook.Enabled {
			continue
		}

		// Match event type (exact match required)
		if hook.EventType != eventType {
			continue
		}

		// Match source (exact or wildcard)
		if hook.EventSource != "*" && hook.EventSource != source {
			continue
		}

		// Match action (exact or wildcard)
		if hook.EventAction != "*" && hook.EventAction != action {
			continue
		}

		matches = append(matches, hook)
	}

	// Sort by priority (higher values first), then by created_at (earlier first)
	sortHooksByPriority(matches)

	return matches, nil
}

// FindByFunction finds all hooks for a function.
func (r *Registry) FindByFunction(ctx context.Context, functionID string) ([]*Hook, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []*Hook

	for _, hook := range r.cache {
		if hook.FunctionID == functionID {
			matches = append(matches, hook)
		}
	}

	// Sort by priority (higher values first), then by created_at (earlier first)
	sortHooksByPriority(matches)

	return matches, nil
}

// Get retrieves a hook by ID.
func (r *Registry) Get(ctx context.Context, hookID string) (*Hook, error) {
	r.mu.RLock()
	hook, ok := r.cache[hookID]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("hook not found: %s", hookID)
	}

	return hook, nil
}

// List retrieves all hooks.
func (r *Registry) List(ctx context.Context) ([]*Hook, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Hook, 0, len(r.cache))
	for _, hook := range r.cache {
		result = append(result, hook)
	}

	// Sort by priority (higher values first), then by created_at (earlier first)
	sortHooksByPriority(result)

	return result, nil
}

// loadCache loads all hooks into memory cache.
func (r *Registry) loadCache(ctx context.Context) error {
	hooks, err := r.store.List(ctx)
	if err != nil {
		return fmt.Errorf("listing hooks: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.cache = make(map[string]*Hook)
	for _, hook := range hooks {
		r.cache[hook.ID] = hook
	}

	log.Info().Int("count", len(r.cache)).Msg("Hooks loaded into cache")

	return nil
}

// invalidateCache clears the cache.
func (r *Registry) invalidateCache() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cache = make(map[string]*Hook)

	log.Debug().Msg("Hook cache invalidated")
}

// Reload reloads all hooks from the database.
func (r *Registry) Reload(ctx context.Context) error {
	if err := r.loadCache(ctx); err != nil {
		return fmt.Errorf("reloading cache: %w", err)
	}
	return nil
}

// sortHooksByPriority sorts hooks by priority (descending) then by created_at (ascending).
func sortHooksByPriority(hooks []*Hook) {
	// Simple bubble sort for small lists
	// For larger lists, consider using sort.Slice
	for i := 0; i < len(hooks); i++ {
		for j := i + 1; j < len(hooks); j++ {
			// Higher priority comes first
			if hooks[i].Priority < hooks[j].Priority {
				hooks[i], hooks[j] = hooks[j], hooks[i]
			} else if hooks[i].Priority == hooks[j].Priority {
				// Same priority: earlier created_at comes first
				if hooks[i].CreatedAt.After(hooks[j].CreatedAt) {
					hooks[i], hooks[j] = hooks[j], hooks[i]
				}
			}
		}
	}
}
