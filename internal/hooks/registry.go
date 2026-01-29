package hooks

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/database"
)

type HookTriggerFunc func(ctx context.Context, hook *Hook, payload any) error

type Registry struct {
	store     *Store
	validator *Validator
	triggers  map[string][]HookTriggerFunc
	hooksByID map[string]*Hook
	mu        sync.RWMutex
}

func NewRegistry(db *database.DB, funcChecker FunctionChecker) *Registry {
	return &Registry{
		store:     NewStore(db),
		validator: NewValidator(funcChecker),
		triggers:  make(map[string][]HookTriggerFunc),
		hooksByID: make(map[string]*Hook),
	}
}

func (r *Registry) LoadFromDatabase(ctx context.Context) error {
	hooks, err := r.store.List(ctx)
	if err != nil {
		return fmt.Errorf("loading hooks from database: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, hook := range hooks {
		if hook.Enabled {
			r.registerHookUnsafe(hook)
		}
		r.hooksByID[hook.ID] = hook
	}

	log.Info().Int("count", len(hooks)).Msg("Hooks loaded from database")
	return nil
}

func (r *Registry) Create(ctx context.Context, hook *Hook) error {
	if err := r.validator.ValidateHook(hook); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := r.store.Create(ctx, hook); err != nil {
		return fmt.Errorf("creating hook in database: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if hook.Enabled {
		r.registerHookUnsafe(hook)
	}
	r.hooksByID[hook.ID] = hook

	log.Info().Str("id", hook.ID).Str("type", string(hook.Type)).Str("source", hook.Source).Msg("Hook created")
	return nil
}

func (r *Registry) Update(ctx context.Context, id string, updates *Hook) error {
	if err := r.validator.ValidateHook(updates); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	updates.ID = id

	if err := r.store.Update(ctx, updates); err != nil {
		return fmt.Errorf("updating hook in database: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if oldHook, exists := r.hooksByID[id]; exists {
		r.unregisterHookUnsafe(oldHook)
	}

	if updates.Enabled {
		r.registerHookUnsafe(updates)
	}
	r.hooksByID[id] = updates

	log.Info().Str("id", id).Msg("Hook updated and hot-reloaded")
	return nil
}

func (r *Registry) Delete(ctx context.Context, id string) error {
	if err := r.store.Delete(ctx, id); err != nil {
		return fmt.Errorf("deleting hook from database: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if hook, exists := r.hooksByID[id]; exists {
		r.unregisterHookUnsafe(hook)
		delete(r.hooksByID, id)
	}

	log.Info().Str("id", id).Msg("Hook deleted")
	return nil
}

func (r *Registry) Enable(ctx context.Context, id string) error {
	hook, err := r.store.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("getting hook: %w", err)
	}

	if hook.Enabled {
		return nil
	}

	hook.Enabled = true
	if err := r.store.Update(ctx, hook); err != nil {
		return fmt.Errorf("updating hook: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.registerHookUnsafe(hook)
	r.hooksByID[id] = hook

	log.Info().Str("id", id).Msg("Hook enabled")
	return nil
}

func (r *Registry) Disable(ctx context.Context, id string) error {
	hook, err := r.store.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("getting hook: %w", err)
	}

	if !hook.Enabled {
		return nil
	}

	hook.Enabled = false
	if err := r.store.Update(ctx, hook); err != nil {
		return fmt.Errorf("updating hook: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.unregisterHookUnsafe(hook)
	r.hooksByID[id] = hook

	log.Info().Str("id", id).Msg("Hook disabled")
	return nil
}

func (r *Registry) Get(ctx context.Context, id string) (*Hook, error) {
	return r.store.Get(ctx, id)
}

func (r *Registry) List(ctx context.Context) ([]*Hook, error) {
	return r.store.List(ctx)
}

func (r *Registry) RegisterTrigger(key string, trigger HookTriggerFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.triggers[key] = append(r.triggers[key], trigger)
}

func (r *Registry) Trigger(ctx context.Context, hookType HookType, source string, action string, payload any) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := r.buildKey(hookType, source, action)
	triggers, exists := r.triggers[key]
	if !exists || len(triggers) == 0 {
		return nil
	}

	for _, trigger := range triggers {
		hooks, err := r.store.ListBySourceAction(ctx, hookType, source, action)
		if err != nil {
			log.Error().Err(err).Str("type", string(hookType)).Str("source", source).Msg("Failed to get hooks for trigger")
			continue
		}

		for _, hook := range hooks {
			if err := trigger(ctx, hook, payload); err != nil {
				log.Error().Err(err).Str("hook_id", hook.ID).Msg("Hook trigger failed")
			}
		}
	}

	return nil
}

func (r *Registry) registerHookUnsafe(hook *Hook) {
	key := r.buildKey(hook.Type, hook.Source, hook.Action)
	log.Debug().Str("key", key).Str("hook_id", hook.ID).Msg("Registering hook")
}

func (r *Registry) unregisterHookUnsafe(hook *Hook) {
	key := r.buildKey(hook.Type, hook.Source, hook.Action)
	log.Debug().Str("key", key).Str("hook_id", hook.ID).Msg("Unregistering hook")
}

func (r *Registry) buildKey(hookType HookType, source string, action string) string {
	if action == "" {
		return fmt.Sprintf("%s:%s", hookType, source)
	}
	return fmt.Sprintf("%s:%s:%s", hookType, source, action)
}
