package functions

import (
	"context"
	"fmt"
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

// Service manages function execution using subprocess runtime.
type Service struct {
	runtimes      map[Runtime]*SubprocessRuntime
	registry      *Registry
	sourceWatcher *SourceWatcher
	tokenStore    *InternalTokenStore
	functionsDir  string
	config        *config.FunctionsConfig
	serverPort    int
}

// NewService creates a new function service with subprocess runtime.
func NewService(cfg *ServiceConfig) (*Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("service config is required")
	}

	// Create function registry
	registry := NewRegistry(cfg.FunctionsDir)

	// Discover functions
	if err := registry.Discover(); err != nil {
		return nil, fmt.Errorf("discovering functions: %w", err)
	}

	// Create subprocess runtimes for each runtime type
	runtimes := make(map[Runtime]*SubprocessRuntime)
	for runtime := range defaultRuntimes {
		rt, err := NewSubprocessRuntime(runtime)
		if err != nil {
			log.Warn().
				Err(err).
				Str("runtime", string(runtime)).
				Msg("Runtime not available, functions using this runtime will fail")
			continue
		}
		runtimes[runtime] = rt
	}

	// Create token store
	const tokenTTL = 5 * time.Minute
	tokenStore := NewInternalTokenStore(tokenTTL)

	// Create source watcher
	sourceWatcher, err := NewSourceWatcher(registry)
	if err != nil {
		return nil, fmt.Errorf("creating source watcher: %w", err)
	}

	return &Service{
		runtimes:      runtimes,
		registry:      registry,
		sourceWatcher: sourceWatcher,
		tokenStore:    tokenStore,
		functionsDir:  cfg.FunctionsDir,
		config:        cfg.Config,
		serverPort:    cfg.ServerPort,
	}, nil
}

// Start starts the function service and watchers.
func (s *Service) Start(ctx context.Context) error {
	// Start source watcher for hot reload
	if err := s.sourceWatcher.Start(); err != nil {
		return fmt.Errorf("starting source watcher: %w", err)
	}
	log.Info().Msg("Source watcher started")

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

	// Get runtime for function
	runtime, ok := s.runtimes[fn.Runtime]
	if !ok {
		duration := time.Since(startTime)
		return &FunctionResponse{
			RequestID:  requestID,
			Success:    false,
			Error:      &FunctionError{Code: "RUNTIME_NOT_AVAILABLE", Message: fmt.Sprintf("Runtime %s not available", fn.Runtime)},
			DurationMs: duration.Milliseconds(),
		}, fmt.Errorf("runtime %s not available", fn.Runtime)
	}

	// Call subprocess function
	resp, err := runtime.Call(ctx, functionName, fn.Path, req)
	if err != nil {
		duration := time.Since(startTime)
		return &FunctionResponse{
			RequestID:  requestID,
			Success:    false,
			Error:      &FunctionError{Code: "EXECUTION_ERROR", Message: err.Error()},
			DurationMs: duration.Milliseconds(),
		}, fmt.Errorf("calling function: %w", err)
	}

	resp.DurationMs = time.Since(startTime).Milliseconds()

	return resp, nil
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
	s.registry = NewRegistry(s.functionsDir)

	if err := s.registry.Discover(); err != nil {
		return fmt.Errorf("discovering functions: %w", err)
	}

	functions := s.registry.List()
	log.Info().Int("count", len(functions)).Msg("Functions reloaded")

	return nil
}

// Stats returns runtime statistics (placeholder for compatibility).
func (s *Service) Stats() map[Runtime]PoolStats {
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
	if s.sourceWatcher != nil {
		if err := s.sourceWatcher.Stop(); err != nil {
			log.Warn().Err(err).Msg("Failed to stop source watcher")
		}
	}

	return nil
}

// TokenStore returns the internal token store.
func (s *Service) TokenStore() *InternalTokenStore {
	return s.tokenStore
}
