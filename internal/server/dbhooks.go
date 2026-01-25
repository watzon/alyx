package server

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/functions"
)

type DatabaseHookTrigger struct {
	funcService *functions.Service
	hooks       map[string][]DatabaseHook // collection -> hooks
	mu          sync.RWMutex
}

type DatabaseHook struct {
	FunctionName string
	Action       string // insert, update, delete
	Mode         string // sync, async
}

func NewDatabaseHookTrigger(funcService *functions.Service) *DatabaseHookTrigger {
	t := &DatabaseHookTrigger{
		funcService: funcService,
		hooks:       make(map[string][]DatabaseHook),
	}

	t.loadHooksFromFunctions()

	return t
}

func (t *DatabaseHookTrigger) loadHooksFromFunctions() {
	if t.funcService == nil {
		return
	}

	for _, fn := range t.funcService.ListFunctions() {
		for _, hook := range fn.Hooks {
			if hook.Type != "database" {
				continue
			}

			dbHook := DatabaseHook{
				FunctionName: fn.Name,
				Action:       hook.Action,
				Mode:         hook.Mode,
			}
			if dbHook.Mode == "" {
				dbHook.Mode = "async"
			}

			t.hooks[hook.Source] = append(t.hooks[hook.Source], dbHook)

			log.Info().
				Str("function", fn.Name).
				Str("collection", hook.Source).
				Str("action", hook.Action).
				Str("mode", dbHook.Mode).
				Msg("Database hook registered")
		}
	}
}

func (t *DatabaseHookTrigger) OnInsert(ctx context.Context, collection string, document map[string]any) error {
	return t.executeHooks(ctx, collection, "insert", map[string]any{
		"document":   document,
		"collection": collection,
		"action":     "insert",
	})
}

func (t *DatabaseHookTrigger) OnUpdate(ctx context.Context, collection string, document, previousDocument map[string]any) error {
	return t.executeHooks(ctx, collection, "update", map[string]any{
		"document":   document,
		"previous":   previousDocument,
		"collection": collection,
		"action":     "update",
	})
}

func (t *DatabaseHookTrigger) OnDelete(ctx context.Context, collection string, document map[string]any) error {
	return t.executeHooks(ctx, collection, "delete", map[string]any{
		"document":   document,
		"collection": collection,
		"action":     "delete",
	})
}

func (t *DatabaseHookTrigger) executeHooks(ctx context.Context, collection, action string, input map[string]any) error {
	t.mu.RLock()
	hooks := t.hooks[collection]
	t.mu.RUnlock()

	for _, hook := range hooks {
		if hook.Action != action && hook.Action != "*" {
			continue
		}

		log.Debug().
			Str("function", hook.FunctionName).
			Str("collection", collection).
			Str("action", action).
			Str("mode", hook.Mode).
			Msg("Executing database hook")

		if hook.Mode == "sync" {
			resp, err := t.funcService.Invoke(ctx, hook.FunctionName, input, nil)
			if err != nil {
				log.Error().Err(err).Str("function", hook.FunctionName).Msg("Sync hook failed")
				return err
			}
			if !resp.Success {
				log.Warn().
					Str("function", hook.FunctionName).
					Str("error_code", resp.Error.Code).
					Str("error_message", resp.Error.Message).
					Msg("Sync hook returned error")
			}
		} else {
			go func(hookCopy DatabaseHook) {
				resp, err := t.funcService.Invoke(context.Background(), hookCopy.FunctionName, input, nil)
				if err != nil {
					log.Error().Err(err).Str("function", hookCopy.FunctionName).Msg("Async hook failed")
					return
				}
				if !resp.Success {
					log.Warn().
						Str("function", hookCopy.FunctionName).
						Str("error_code", resp.Error.Code).
						Str("error_message", resp.Error.Message).
						Msg("Async hook returned error")
				} else {
					log.Debug().
						Str("function", hookCopy.FunctionName).
						Int64("duration_ms", resp.DurationMs).
						Msg("Async hook completed")
				}
			}(hook)
		}
	}

	return nil
}

func (t *DatabaseHookTrigger) Reload() {
	t.mu.Lock()
	t.hooks = make(map[string][]DatabaseHook)
	t.mu.Unlock()

	t.loadHooksFromFunctions()
}
