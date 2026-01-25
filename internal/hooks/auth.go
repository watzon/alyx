package hooks

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/auth"
	"github.com/watzon/alyx/internal/events"
)

// AuthHookTrigger manages auth event hooks.
type AuthHookTrigger struct {
	registry *Registry
	bus      *events.EventBus
}

// NewAuthHookTrigger creates a new auth hook trigger.
func NewAuthHookTrigger(registry *Registry, bus *events.EventBus) *AuthHookTrigger {
	return &AuthHookTrigger{
		registry: registry,
		bus:      bus,
	}
}

// OnSignup triggers hooks for user signup.
// Supports sync hooks that can reject registration by returning an error.
func (t *AuthHookTrigger) OnSignup(ctx context.Context, user *auth.User, metadata map[string]any) error {
	return t.executeHooks(ctx, "signup", user, metadata)
}

// OnLogin triggers hooks for user login.
func (t *AuthHookTrigger) OnLogin(ctx context.Context, user *auth.User, metadata map[string]any) error {
	return t.executeHooks(ctx, "login", user, metadata)
}

// OnLogout triggers hooks for user logout.
func (t *AuthHookTrigger) OnLogout(ctx context.Context, user *auth.User, metadata map[string]any) error {
	return t.executeHooks(ctx, "logout", user, metadata)
}

// OnPasswordReset triggers hooks for password reset.
func (t *AuthHookTrigger) OnPasswordReset(ctx context.Context, user *auth.User, metadata map[string]any) error {
	return t.executeHooks(ctx, "password_reset", user, metadata)
}

// OnEmailVerify triggers hooks for email verification.
func (t *AuthHookTrigger) OnEmailVerify(ctx context.Context, user *auth.User, metadata map[string]any) error {
	return t.executeHooks(ctx, "email_verify", user, metadata)
}

// executeHooks executes hooks for an auth event.
// For sync hooks, returns error if any hook fails (allows rejection).
// For async hooks, publishes events to the event bus.
func (t *AuthHookTrigger) executeHooks(ctx context.Context, action string, user *auth.User, metadata map[string]any) error {
	// Find matching hooks
	hooks, err := t.registry.FindByEvent(ctx, "auth", "user", action)
	if err != nil {
		return fmt.Errorf("finding hooks: %w", err)
	}

	if len(hooks) == 0 {
		log.Debug().
			Str("action", action).
			Str("user_id", user.ID).
			Msg("No auth hooks found")
		return nil
	}

	// Build event payload
	payload := map[string]any{
		"user": map[string]any{
			"id":         user.ID,
			"email":      user.Email,
			"verified":   user.Verified,
			"role":       user.Role,
			"created_at": user.CreatedAt.Format(time.RFC3339),
		},
		"action":   action,
		"metadata": metadata,
	}

	// Execute sync hooks first (can reject)
	for _, hook := range hooks {
		if hook.Mode != HookModeSync {
			continue
		}

		log.Debug().
			Str("hook_id", hook.ID).
			Str("hook_name", hook.Name).
			Str("action", action).
			Str("user_id", user.ID).
			Msg("Executing sync auth hook")

		// For sync hooks, we would execute the function directly
		// This is a placeholder - actual function execution will be implemented later
		// For now, sync hooks are logged but not executed
		log.Warn().
			Str("hook_id", hook.ID).
			Msg("Sync hook execution not yet implemented - hook will be skipped")
	}

	// Publish async hooks to event bus
	for _, hook := range hooks {
		if hook.Mode != HookModeAsync {
			continue
		}

		event := &events.Event{
			ID:     uuid.New().String(),
			Type:   events.EventTypeAuth,
			Source: "user",
			Action: action,
			Payload: map[string]any{
				"hook_id":  hook.ID,
				"user":     payload["user"],
				"action":   action,
				"metadata": metadata,
			},
			Metadata: events.EventMetadata{
				Extra: map[string]any{
					"hook_id":     hook.ID,
					"function_id": hook.FunctionID,
					"user_id":     user.ID,
					"action":      action,
				},
			},
			Status:    "pending",
			CreatedAt: time.Now().UTC(),
		}

		if err := t.bus.Publish(ctx, event); err != nil {
			log.Error().
				Err(err).
				Str("hook_id", hook.ID).
				Str("action", action).
				Str("user_id", user.ID).
				Msg("Failed to publish auth event")
			// Continue with other hooks
			continue
		}

		log.Debug().
			Str("event_id", event.ID).
			Str("hook_id", hook.ID).
			Str("action", action).
			Str("user_id", user.ID).
			Msg("Auth event published")
	}

	return nil
}
