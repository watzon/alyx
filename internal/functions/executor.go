package functions

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/config"
)

const internalTokenTTL = 5 * time.Minute

// Service manages function execution.
type Service struct {
	registry        *Registry
	poolManager     *PoolManager
	cfg             *config.FunctionsConfig
	hostNetwork     string
	serverPort      int
	tokenStore      *InternalTokenStore
	executionLogger ExecutionLogger
}

// ExecutionLogger defines the interface for logging function executions.
type ExecutionLogger interface {
	WrapExecution(
		ctx context.Context,
		functionID string,
		requestID string,
		triggerType string,
		triggerID string,
		input map[string]any,
		execute func() (*FunctionResponse, error),
	) (*FunctionResponse, error)
}

// ServiceConfig holds configuration for the function service.
type ServiceConfig struct {
	FunctionsDir string
	Config       *config.FunctionsConfig
	ServerPort   int
}

// NewService creates a new function service.
func NewService(cfg *ServiceConfig) (*Service, error) {
	registry := NewRegistry(cfg.FunctionsDir)

	// Discover functions
	if err := registry.Discover(); err != nil {
		return nil, fmt.Errorf("discovering functions: %w", err)
	}

	absFunctionsDir, err := filepath.Abs(cfg.FunctionsDir)
	if err != nil {
		absFunctionsDir = cfg.FunctionsDir
	}

	runtimeConfigs := make(map[Runtime]*PoolConfig)
	for name, poolCfg := range cfg.Config.Pools {
		runtime := Runtime(name)
		runtimeConfigs[runtime] = &PoolConfig{
			MinWarm:          poolCfg.MinWarm,
			MaxInstances:     poolCfg.MaxInstances,
			IdleTimeout:      poolCfg.IdleTimeout,
			Image:            poolCfg.Image,
			MemoryLimit:      cfg.Config.MemoryLimit,
			CPULimit:         cfg.Config.CPULimit,
			ExecutionTimeout: cfg.Config.Timeout,
			FunctionsDir:     absFunctionsDir,
		}
	}

	// Create pool manager
	poolManager, err := NewPoolManager(&PoolManagerConfig{
		ContainerRuntime: cfg.Config.Runtime,
		StartPort:        19000,
		RuntimeConfigs:   runtimeConfigs,
	})
	if err != nil {
		return nil, fmt.Errorf("creating pool manager: %w", err)
	}

	tokenStore := NewInternalTokenStore(internalTokenTTL)

	return &Service{
		registry:        registry,
		poolManager:     poolManager,
		cfg:             cfg.Config,
		serverPort:      cfg.ServerPort,
		tokenStore:      tokenStore,
		executionLogger: nil,
	}, nil
}

// Start starts the function service and initializes container pools.
func (s *Service) Start(ctx context.Context) error {
	if err := s.poolManager.Start(ctx); err != nil {
		return fmt.Errorf("starting pool manager: %w", err)
	}
	s.hostNetwork = s.poolManager.GetHostNetwork()
	return nil
}

// SetExecutionLogger sets the execution logger for the service.
func (s *Service) SetExecutionLogger(logger ExecutionLogger) {
	s.executionLogger = logger
}

// Invoke executes a function by name.
func (s *Service) Invoke(ctx context.Context, name string, input map[string]any, auth *AuthContext) (*FunctionResponse, error) {
	return s.InvokeWithTrigger(ctx, name, input, auth, "http", "")
}

// InvokeWithTrigger executes a function with trigger information for logging.
func (s *Service) InvokeWithTrigger(ctx context.Context, name string, input map[string]any, auth *AuthContext, triggerType, triggerID string) (*FunctionResponse, error) {
	// Look up function
	funcDef, ok := s.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("function not found: %s", name)
	}

	// Build function request
	req := &FunctionRequest{
		RequestID: uuid.New().String(),
		Function:  name,
		Input:     input,
		Context: &FunctionContext{
			Auth:          auth,
			Env:           s.buildEnv(funcDef),
			AlyxURL:       s.buildAlyxURL(),
			InternalToken: s.generateInternalToken(),
		},
	}

	// Get timeout for this function
	timeout := s.cfg.Timeout
	if funcDef.Timeout > 0 {
		timeout = time.Duration(funcDef.Timeout) * time.Second
	}

	// Create timeout context
	invokeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Debug().
		Str("function", name).
		Str("request_id", req.RequestID).
		Str("runtime", string(funcDef.Runtime)).
		Str("alyx_url", req.Context.AlyxURL).
		Msg("Invoking function")

	if s.executionLogger != nil {
		return s.executionLogger.WrapExecution(
			invokeCtx,
			name,
			req.RequestID,
			triggerType,
			triggerID,
			input,
			func() (*FunctionResponse, error) {
				return s.poolManager.Invoke(invokeCtx, funcDef.Runtime, req)
			},
		)
	}

	resp, err := s.poolManager.Invoke(invokeCtx, funcDef.Runtime, req)
	if err != nil {
		return nil, fmt.Errorf("invoking function: %w", err)
	}

	if resp.Success {
		log.Debug().
			Str("function", name).
			Str("request_id", req.RequestID).
			Int64("duration_ms", resp.DurationMs).
			Msg("Function completed successfully")
	} else {
		log.Warn().
			Str("function", name).
			Str("request_id", req.RequestID).
			Str("error_code", resp.Error.Code).
			Str("error_message", resp.Error.Message).
			Msg("Function returned error")
	}

	return resp, nil
}

// buildEnv builds environment variables for a function.
func (s *Service) buildEnv(funcDef *FunctionDef) map[string]string {
	env := make(map[string]string)

	// Add global env vars
	for k, v := range s.cfg.Env {
		env[k] = v
	}

	// Add function-specific env vars (override globals)
	for k, v := range funcDef.Env {
		env[k] = v
	}

	return env
}

// buildAlyxURL builds the URL for containers to reach the Alyx server.
func (s *Service) buildAlyxURL() string {
	return fmt.Sprintf("http://%s:%d", s.hostNetwork, s.serverPort)
}

// generateInternalToken generates a short-lived token for internal API calls.
func (s *Service) generateInternalToken() string {
	return s.tokenStore.Generate()
}

// TokenStore returns the internal token store.
func (s *Service) TokenStore() *InternalTokenStore {
	return s.tokenStore
}

// GetFunction returns a function definition by name.
func (s *Service) GetFunction(name string) (*FunctionDef, bool) {
	return s.registry.Get(name)
}

// ListFunctions returns all discovered functions.
func (s *Service) ListFunctions() []*FunctionDef {
	return s.registry.List()
}

// ReloadFunctions rediscovers all functions.
func (s *Service) ReloadFunctions() error {
	return s.registry.Reload()
}

// Stats returns pool statistics.
func (s *Service) Stats() map[Runtime]PoolStats {
	return s.poolManager.Stats()
}

// Close shuts down the function service.
func (s *Service) Close() error {
	return s.poolManager.Close()
}

// FunctionsDir returns the functions directory path.
func (s *Service) FunctionsDir() string {
	return s.registry.FunctionsDir()
}

// AbsFunctionsDir returns the absolute path to the functions directory.
func (s *Service) AbsFunctionsDir() string {
	abs, err := filepath.Abs(s.registry.FunctionsDir())
	if err != nil {
		return s.registry.FunctionsDir()
	}
	return abs
}
