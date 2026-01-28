package functions

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/watzon/alyx/internal/schema"
)

// FunctionDef represents a function and its metadata.
type FunctionDef struct {
	Name        string            `json:"name"`
	Runtime     Runtime           `json:"runtime"`
	Path        string            `json:"path"`
	OutputPath  string            `json:"output_path,omitempty"`
	Description string            `json:"description,omitempty"`
	SampleInput any               `json:"sample_input,omitempty"`
	HasBuild    bool              `json:"has_build"`
	Build       *BuildConfig      `json:"build,omitempty"`
	Timeout     int               `json:"timeout,omitempty"`
	Memory      int               `json:"memory,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Routes      []RouteConfig     `json:"routes,omitempty"`
	Hooks       []HookConfig      `json:"hooks,omitempty"`
	Schedules   []ScheduleConfig  `json:"schedules,omitempty"`
}

// GetEntrypoint returns the appropriate entrypoint path based on dev mode.
func (f *FunctionDef) GetEntrypoint(devMode bool) string {
	if devMode {
		return f.Path
	}
	if f.HasBuild && f.OutputPath != "" {
		return f.OutputPath
	}
	return f.Path
}

// Registrar defines the interface for auto-registering manifest components.
type Registrar interface {
	RegisterHooks(ctx context.Context, functionID string, hooks []HookConfig) error
	RegisterSchedules(ctx context.Context, functionID string, schedules []ScheduleConfig) error
	RegisterWebhooks(ctx context.Context, functionID string, hooks []HookConfig) error
}

// Registry manages functions loaded from schema.
type Registry struct {
	functionsDir string
	functions    map[string]*FunctionDef
	registrar    Registrar
	mu           sync.RWMutex
}

// NewRegistryFromSchema creates a Registry from schema functions.
func NewRegistryFromSchema(s *schema.Schema, functionsDir string, registrar Registrar) (*Registry, error) {
	defs, err := SchemaToFunctionDefs(s, functionsDir)
	if err != nil {
		return nil, err
	}

	registry := &Registry{
		functionsDir: functionsDir,
		functions:    make(map[string]*FunctionDef),
		registrar:    registrar,
	}

	for _, def := range defs {
		registry.functions[def.Name] = def
	}

	if registrar != nil {
		ctx := context.Background()
		for _, def := range defs {
			if err := registry.autoRegister(ctx, def); err != nil {
				log.Warn().Err(err).Str("function", def.Name).Msg("Failed to auto-register manifest components")
			}
		}
	}

	log.Info().Int("count", len(registry.functions)).Msg("Functions loaded from schema")
	return registry, nil
}

// SetRegistrar sets the registrar for auto-registration of manifest components.
func (r *Registry) SetRegistrar(registrar Registrar) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registrar = registrar
}

func (r *Registry) autoRegister(ctx context.Context, funcDef *FunctionDef) error {
	functionID := funcDef.Name

	if len(funcDef.Hooks) > 0 {
		if err := r.registrar.RegisterHooks(ctx, functionID, funcDef.Hooks); err != nil {
			return err
		}
		log.Debug().Str("function", functionID).Int("count", len(funcDef.Hooks)).Msg("Auto-registered hooks")
	}

	if len(funcDef.Schedules) > 0 {
		if err := r.registrar.RegisterSchedules(ctx, functionID, funcDef.Schedules); err != nil {
			return err
		}
		log.Debug().Str("function", functionID).Int("count", len(funcDef.Schedules)).Msg("Auto-registered schedules")
	}

	const hookTypeWebhook = "webhook"
	webhookHooks := make([]HookConfig, 0)
	for _, hook := range funcDef.Hooks {
		if hook.Type == hookTypeWebhook {
			webhookHooks = append(webhookHooks, hook)
		}
	}

	if len(webhookHooks) > 0 {
		if err := r.registrar.RegisterWebhooks(ctx, functionID, webhookHooks); err != nil {
			return err
		}
		log.Debug().Str("function", functionID).Int("count", len(webhookHooks)).Msg("Auto-registered webhooks")
	}

	return nil
}

// Get returns a function definition by name.
func (r *Registry) Get(name string) (*FunctionDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fn, ok := r.functions[name]
	return fn, ok
}

// List returns all functions.
func (r *Registry) List() []*FunctionDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*FunctionDef, 0, len(r.functions))
	for _, fn := range r.functions {
		result = append(result, fn)
	}
	return result
}

// Count returns the number of functions.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.functions)
}

// GetByRuntime returns all functions for a specific runtime.
func (r *Registry) GetByRuntime(runtime Runtime) []*FunctionDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*FunctionDef, 0)
	for _, fn := range r.functions {
		if fn.Runtime == runtime {
			result = append(result, fn)
		}
	}
	return result
}

// FunctionsDir returns the configured functions directory.
func (r *Registry) FunctionsDir() string {
	return r.functionsDir
}
