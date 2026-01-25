// Package functions provides serverless function execution via containers.
package functions

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

var errNotAFunction = errors.New("not a function file")

// FunctionDef represents a discovered function and its metadata.
type FunctionDef struct {
	// Name is the function name (derived from filename).
	Name string `json:"name"`
	// Runtime is the function runtime (node, python, go).
	Runtime Runtime `json:"runtime"`
	// Path is the absolute path to the function file.
	Path string `json:"path"`
	// Timeout overrides the default timeout (optional).
	Timeout int `json:"timeout,omitempty"`
	// Memory overrides the default memory limit in MB (optional).
	Memory int `json:"memory,omitempty"`
	// Env contains environment variables for this function.
	Env map[string]string `json:"env,omitempty"`
	// HasManifest indicates if a YAML manifest was found.
	HasManifest bool `json:"has_manifest"`
	// Routes contains HTTP route configurations from manifest.
	Routes []RouteConfig `json:"routes,omitempty"`
	// Hooks contains hook configurations from manifest.
	Hooks []HookConfig `json:"hooks,omitempty"`
	// Schedules contains schedule configurations from manifest.
	Schedules []ScheduleConfig `json:"schedules,omitempty"`
}

// FunctionManifest represents a function's YAML manifest file (legacy format).
// Deprecated: Use Manifest for new manifests with hooks/schedules/routes support.
type FunctionManifest struct {
	Name         string            `yaml:"name"`
	Runtime      string            `yaml:"runtime"`
	Timeout      string            `yaml:"timeout"`
	Memory       string            `yaml:"memory"`
	Env          map[string]string `yaml:"env"`
	Dependencies []string          `yaml:"dependencies"`
}

// Registrar defines the interface for auto-registering manifest components.
type Registrar interface {
	RegisterHooks(ctx context.Context, functionID string, hooks []HookConfig) error
	RegisterSchedules(ctx context.Context, functionID string, schedules []ScheduleConfig) error
	RegisterWebhooks(ctx context.Context, functionID string, hooks []HookConfig) error
}

// Registry manages discovered functions.
type Registry struct {
	functionsDir string
	functions    map[string]*FunctionDef
	registrar    Registrar
	mu           sync.RWMutex
}

// NewRegistry creates a new function registry.
func NewRegistry(functionsDir string) *Registry {
	return &Registry{
		functionsDir: functionsDir,
		functions:    make(map[string]*FunctionDef),
	}
}

// SetRegistrar sets the registrar for auto-registration of manifest components.
func (r *Registry) SetRegistrar(registrar Registrar) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registrar = registrar
}

// Discover scans the functions directory and discovers all functions.
func (r *Registry) Discover() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear existing functions
	r.functions = make(map[string]*FunctionDef)

	// Check if functions directory exists
	if _, err := os.Stat(r.functionsDir); os.IsNotExist(err) {
		log.Warn().Str("path", r.functionsDir).Msg("Functions directory does not exist")
		return nil
	}

	entries, err := os.ReadDir(r.functionsDir)
	if err != nil {
		return fmt.Errorf("reading functions directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()

		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "node_modules" {
			continue
		}

		funcDef, err := r.parseFunctionDirectory(name)
		if errors.Is(err, errNotAFunction) {
			continue
		}
		if err != nil {
			log.Warn().Err(err).Str("file", name).Msg("Failed to parse function")
			continue
		}

		r.functions[funcDef.Name] = funcDef
		log.Debug().
			Str("name", funcDef.Name).
			Str("runtime", string(funcDef.Runtime)).
			Bool("has_manifest", funcDef.HasManifest).
			Msg("Discovered function")
	}

	log.Info().Int("count", len(r.functions)).Msg("Functions discovered")
	return nil
}

func (r *Registry) parseFunctionDirectory(dirName string) (*FunctionDef, error) {
	dirPath := filepath.Join(r.functionsDir, dirName)

	entryFile, runtime := r.findEntryFile(dirPath)
	if entryFile == "" {
		return nil, errNotAFunction
	}

	funcDef := &FunctionDef{
		Name:    dirName,
		Runtime: runtime,
		Path:    entryFile,
		Env:     make(map[string]string),
	}

	manifestPath := filepath.Join(dirPath, "manifest.yaml")
	if _, err := os.Stat(manifestPath); err == nil {
		if err := r.loadManifest(funcDef, manifestPath); err != nil {
			return nil, fmt.Errorf("loading manifest: %w", err)
		}
		funcDef.HasManifest = true
	}

	return funcDef, nil
}

func (r *Registry) findEntryFile(dirPath string) (string, Runtime) {
	candidates := []struct {
		name    string
		runtime Runtime
	}{
		{"index.js", RuntimeNode},
		{"index.mjs", RuntimeNode},
		{"index.cjs", RuntimeNode},
		{"index.py", RuntimePython},
		{"main.go", RuntimeGo},
		{"index.go", RuntimeGo},
	}

	for _, c := range candidates {
		path := filepath.Join(dirPath, c.name)
		if _, err := os.Stat(path); err == nil {
			return path, c.runtime
		}
	}

	return "", ""
}

// loadManifest loads a function manifest file.
func (r *Registry) loadManifest(funcDef *FunctionDef, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parsing manifest: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return fmt.Errorf("validating manifest: %w", err)
	}

	if manifest.Runtime != "" {
		funcDef.Runtime = Runtime(manifest.Runtime)
	}
	if manifest.Timeout != "" {
		funcDef.Timeout = parseTimeoutSeconds(manifest.Timeout)
	}
	if manifest.Memory != "" {
		funcDef.Memory = parseMemoryMB(manifest.Memory)
	}
	if manifest.Env != nil {
		for k, v := range manifest.Env {
			funcDef.Env[k] = expandEnv(v)
		}
	}

	funcDef.Routes = manifest.Routes
	funcDef.Hooks = manifest.Hooks
	funcDef.Schedules = manifest.Schedules

	if r.registrar != nil {
		if err := r.autoRegister(context.Background(), funcDef); err != nil {
			log.Warn().Err(err).Str("function", funcDef.Name).Msg("Failed to auto-register manifest components")
		}
	}

	return nil
}

func (r *Registry) autoRegister(ctx context.Context, funcDef *FunctionDef) error {
	functionID := funcDef.Name

	if len(funcDef.Hooks) > 0 {
		if err := r.registrar.RegisterHooks(ctx, functionID, funcDef.Hooks); err != nil {
			return fmt.Errorf("registering hooks: %w", err)
		}
		log.Debug().Str("function", functionID).Int("count", len(funcDef.Hooks)).Msg("Auto-registered hooks")
	}

	if len(funcDef.Schedules) > 0 {
		if err := r.registrar.RegisterSchedules(ctx, functionID, funcDef.Schedules); err != nil {
			return fmt.Errorf("registering schedules: %w", err)
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
			return fmt.Errorf("registering webhooks: %w", err)
		}
		log.Debug().Str("function", functionID).Int("count", len(webhookHooks)).Msg("Auto-registered webhooks")
	}

	return nil
}

// detectRuntime detects the runtime from file extension.
func detectRuntime(ext string) Runtime {
	switch ext {
	case ".js", ".mjs", ".cjs":
		return RuntimeNode
	case ".py":
		return RuntimePython
	case ".go":
		return RuntimeGo
	default:
		return ""
	}
}

const (
	secondsPerMinute = 60
	mbPerGB          = 1024
)

func parseTimeoutSeconds(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	var value int

	switch {
	case strings.HasSuffix(s, "m"):
		s = strings.TrimSuffix(s, "m")
		if _, err := fmt.Sscanf(s, "%d", &value); err == nil {
			return value * secondsPerMinute
		}
	case strings.HasSuffix(s, "s"):
		s = strings.TrimSuffix(s, "s")
		if _, err := fmt.Sscanf(s, "%d", &value); err == nil {
			return value
		}
	default:
		if _, err := fmt.Sscanf(s, "%d", &value); err == nil {
			return value
		}
	}

	return 0
}

func parseMemoryMB(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0
	}

	var value int

	switch {
	case strings.HasSuffix(s, "gb"):
		s = strings.TrimSuffix(s, "gb")
		if _, err := fmt.Sscanf(s, "%d", &value); err == nil {
			return value * mbPerGB
		}
	case strings.HasSuffix(s, "mb"):
		s = strings.TrimSuffix(s, "mb")
		if _, err := fmt.Sscanf(s, "%d", &value); err == nil {
			return value
		}
	case strings.HasSuffix(s, "m"):
		s = strings.TrimSuffix(s, "m")
		if _, err := fmt.Sscanf(s, "%d", &value); err == nil {
			return value
		}
	default:
		if _, err := fmt.Sscanf(s, "%d", &value); err == nil {
			return value
		}
	}

	return 0
}

// expandEnv expands environment variable references in a string.
func expandEnv(s string) string {
	return os.ExpandEnv(s)
}

// Get returns a function definition by name.
func (r *Registry) Get(name string) (*FunctionDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fn, ok := r.functions[name]
	return fn, ok
}

// List returns all discovered functions.
func (r *Registry) List() []*FunctionDef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*FunctionDef, 0, len(r.functions))
	for _, fn := range r.functions {
		result = append(result, fn)
	}
	return result
}

// Count returns the number of discovered functions.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.functions)
}

// Reload rediscovers all functions.
func (r *Registry) Reload() error {
	return r.Discover()
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
