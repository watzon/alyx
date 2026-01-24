// Package functions provides serverless function execution via containers.
package functions

import (
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
}

// FunctionManifest represents a function's YAML manifest file.
type FunctionManifest struct {
	Name         string            `yaml:"name"`
	Runtime      string            `yaml:"runtime"`
	Timeout      string            `yaml:"timeout"`
	Memory       string            `yaml:"memory"`
	Env          map[string]string `yaml:"env"`
	Dependencies []string          `yaml:"dependencies"`
}

// Registry manages discovered functions.
type Registry struct {
	functionsDir string
	functions    map[string]*FunctionDef
	mu           sync.RWMutex
}

// NewRegistry creates a new function registry.
func NewRegistry(functionsDir string) *Registry {
	return &Registry{
		functionsDir: functionsDir,
		functions:    make(map[string]*FunctionDef),
	}
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
		if entry.IsDir() {
			// Skip directories (could be _shared or node_modules)
			continue
		}

		name := entry.Name()

		// Skip hidden files and shared modules
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}

		funcDef, err := r.parseFunction(name)
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

func (r *Registry) parseFunction(filename string) (*FunctionDef, error) {
	ext := filepath.Ext(filename)
	baseName := strings.TrimSuffix(filename, ext)

	runtime := detectRuntime(ext)
	if runtime == "" {
		return nil, errNotAFunction
	}

	funcPath := filepath.Join(r.functionsDir, filename)

	funcDef := &FunctionDef{
		Name:    baseName,
		Runtime: runtime,
		Path:    funcPath,
		Env:     make(map[string]string),
	}

	// Try to load manifest file
	manifestPath := filepath.Join(r.functionsDir, baseName+".yaml")
	if _, err := os.Stat(manifestPath); err == nil {
		if err := r.loadManifest(funcDef, manifestPath); err != nil {
			return nil, fmt.Errorf("loading manifest: %w", err)
		}
		funcDef.HasManifest = true
	}

	return funcDef, nil
}

// loadManifest loads a function manifest file.
func (r *Registry) loadManifest(funcDef *FunctionDef, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var manifest FunctionManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("parsing manifest: %w", err)
	}

	// Apply manifest overrides
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
