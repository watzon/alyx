package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/config"
)

// ServiceConfig contains configuration for the function service.
type ServiceConfig struct {
	// FunctionsDir is the directory containing function definitions.
	FunctionsDir string
	// Config is the functions configuration from alyx.yaml.
	Config *config.FunctionsConfig
	// ServerPort is the port the Alyx server is running on.
	ServerPort int
}

// Service manages function execution using WASM runtime.
type Service struct {
	runtime       *WASMRuntime
	registry      *Registry
	sourceWatcher *SourceWatcher
	wasmWatcher   *WASMWatcher
	tokenStore    *InternalTokenStore
	functionsDir  string
	config        *config.FunctionsConfig
	serverPort    int
}

// NewService creates a new function service with WASM runtime.
func NewService(cfg *ServiceConfig) (*Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("service config is required")
	}

	// Create WASM runtime
	wasmConfig := &WASMConfig{
		MemoryLimitMB:  256,
		TimeoutSeconds: 30,
		EnableWASI:     true,
		AlyxURL:        fmt.Sprintf("http://localhost:%d", cfg.ServerPort),
	}
	runtime := NewWASMRuntime(wasmConfig)

	// Create function registry
	registry := NewRegistry(cfg.FunctionsDir)

	// Discover functions
	if err := registry.Discover(); err != nil {
		return nil, fmt.Errorf("discovering functions: %w", err)
	}

	// Create token store
	const tokenTTL = 5 * time.Minute
	tokenStore := NewInternalTokenStore(tokenTTL)

	// Create source watcher
	sourceWatcher, err := NewSourceWatcher(registry)
	if err != nil {
		return nil, fmt.Errorf("creating source watcher: %w", err)
	}

	// Create WASM watcher
	wasmWatcher, err := NewWASMWatcher(runtime, registry)
	if err != nil {
		return nil, fmt.Errorf("creating WASM watcher: %w", err)
	}

	return &Service{
		runtime:       runtime,
		registry:      registry,
		sourceWatcher: sourceWatcher,
		wasmWatcher:   wasmWatcher,
		tokenStore:    tokenStore,
		functionsDir:  cfg.FunctionsDir,
		config:        cfg.Config,
		serverPort:    cfg.ServerPort,
	}, nil
}

// Start starts the function service and watchers.
func (s *Service) Start(ctx context.Context) error {
	// Load all WASM plugins
	functions := s.registry.List()
	for _, fn := range functions {
		funcDir := filepath.Dir(fn.Path)
		wasmPath := filepath.Join(funcDir, "plugin.wasm")

		// Check if WASM file exists
		if err := s.runtime.LoadPlugin(fn.Name, wasmPath); err != nil {
			log.Warn().
				Err(err).
				Str("function", fn.Name).
				Str("path", wasmPath).
				Msg("Failed to load WASM plugin, will retry on build")
			continue
		}

		log.Debug().
			Str("function", fn.Name).
			Str("path", wasmPath).
			Msg("Loaded WASM plugin")
	}

	// Start watchers (always enabled for hot reload)
	if true {
		if err := s.sourceWatcher.Start(); err != nil {
			return fmt.Errorf("starting source watcher: %w", err)
		}
		log.Info().Msg("Source watcher started")

		if err := s.wasmWatcher.Start(); err != nil {
			return fmt.Errorf("starting WASM watcher: %w", err)
		}
		log.Info().Msg("WASM watcher started")
	}

	return nil
}

// Invoke invokes a function with the given input and auth context.
func (s *Service) Invoke(ctx context.Context, functionName string, input map[string]any, authCtx *AuthContext) (*FunctionResponse, error) {
	startTime := time.Now()
	requestID := uuid.New().String()

	// Get function definition
	fn, ok := s.registry.Get(functionName)
	if !ok {
		return nil, fmt.Errorf("function %s not found", functionName)
	}

	// Generate internal token for API access
	token := s.tokenStore.Generate()

	// Build function context
	funcCtx := &FunctionContext{
		Auth:          authCtx,
		Env:           fn.Env,
		AlyxURL:       fmt.Sprintf("http://localhost:%d", s.serverPort),
		InternalToken: token,
	}

	// Build function request
	req := &FunctionRequest{
		RequestID: requestID,
		Function:  functionName,
		Input:     input,
		Context:   funcCtx,
	}

	// Marshal request to JSON
	inputBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Call WASM function
	outputBytes, err := s.runtime.Call(functionName, "handle", inputBytes)
	if err != nil {
		duration := time.Since(startTime)
		return &FunctionResponse{
			RequestID:  requestID,
			Success:    false,
			Error:      &FunctionError{Code: "EXECUTION_ERROR", Message: err.Error()},
			DurationMs: duration.Milliseconds(),
		}, fmt.Errorf("calling WASM function: %w", err)
	}

	// Parse response
	var resp FunctionResponse
	if err := json.Unmarshal(outputBytes, &resp); err != nil {
		duration := time.Since(startTime)
		return &FunctionResponse{
			RequestID:  requestID,
			Success:    false,
			Error:      &FunctionError{Code: "INVALID_RESPONSE", Message: "Failed to parse function response"},
			DurationMs: duration.Milliseconds(),
		}, fmt.Errorf("parsing function response: %w", err)
	}

	// Set duration
	resp.DurationMs = time.Since(startTime).Milliseconds()

	return &resp, nil
}

// GetFunction returns a function definition by name.
func (s *Service) GetFunction(name string) (*FunctionDef, bool) {
	return s.registry.Get(name)
}

// ListFunctions returns all registered functions.
func (s *Service) ListFunctions() []*FunctionDef {
	return s.registry.List()
}

// ReloadFunctions rediscovers functions and reloads the registry.
func (s *Service) ReloadFunctions() error {
	// Clear registry
	s.registry = NewRegistry(s.functionsDir)

	// Rediscover functions
	if err := s.registry.Discover(); err != nil {
		return fmt.Errorf("discovering functions: %w", err)
	}

	// Reload WASM plugins
	functions := s.registry.List()
	for _, fn := range functions {
		funcDir := filepath.Dir(fn.Path)
		wasmPath := filepath.Join(funcDir, "plugin.wasm")

		// Reload plugin (unloads old, loads new)
		if err := s.runtime.Reload(fn.Name, wasmPath); err != nil {
			log.Warn().
				Err(err).
				Str("function", fn.Name).
				Msg("Failed to reload WASM plugin")
			continue
		}
	}

	log.Info().Int("count", len(functions)).Msg("Functions reloaded")

	return nil
}

// Stats returns runtime statistics (placeholder for compatibility).
func (s *Service) Stats() map[Runtime]PoolStats {
	// Return empty stats for now - WASM runtime doesn't use pools
	return make(map[Runtime]PoolStats)
}

// PoolStats represents pool statistics (placeholder for compatibility).
type PoolStats struct {
	Ready int `json:"ready"`
	Busy  int `json:"busy"`
	Total int `json:"total"`
}

// Close shuts down the service and releases resources.
func (s *Service) Close() error {
	// Stop watchers
	if s.sourceWatcher != nil {
		if err := s.sourceWatcher.Stop(); err != nil {
			log.Warn().Err(err).Msg("Failed to stop source watcher")
		}
	}
	if s.wasmWatcher != nil {
		if err := s.wasmWatcher.Stop(); err != nil {
			log.Warn().Err(err).Msg("Failed to stop WASM watcher")
		}
	}

	// Close WASM runtime
	if err := s.runtime.Close(); err != nil {
		return fmt.Errorf("closing WASM runtime: %w", err)
	}

	return nil
}

// TokenStore returns the internal token store.
func (s *Service) TokenStore() *InternalTokenStore {
	return s.tokenStore
}
