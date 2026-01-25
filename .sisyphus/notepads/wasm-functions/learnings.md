# Learnings - WASM Functions Implementation

## Conventions & Patterns
_Accumulated knowledge from task execution_

---
# Container Removal - Task 1

## Types Removed from internal/functions/types.go

- `ContainerState` (type + 6 constants: Creating, Ready, Busy, Error, Stopping, Stopped)
- `Container` struct (ID, Runtime, State, CreatedAt, LastUsedAt, Port)
- `PoolConfig` struct (MinWarm, MaxInstances, IdleTimeout, Image, MemoryLimit, CPULimit, ExecutionTimeout, FunctionsDir)
- `ContainerManager` interface (Create, Start, Stop, Remove, Invoke, HealthCheck, List, Close)

## Files Deleted

- `internal/functions/container.go` (465 lines) - DockerManager implementation
- `internal/functions/pool.go` (406 lines) - Pool and PoolManager implementation
- `internal/functions/executor.go` (263 lines) - Service implementation using containers

## Retained Interface

- `Executor` interface (Execute, Close) - Will be implemented by WASM executor

## Dependencies Found (Build Errors)

### internal/webhooks/handler.go
- Line 18: `service *functions.Service`
- Line 22: `func NewHandler(store *Store, service *functions.Service)`
- **Impact**: Needs Service type which was in executor.go

### internal/server/server.go
- Line 30: `funcService *functions.Service`
- Line 88: `functions.NewService(&functions.ServiceConfig{...})`
- Line 187: `func (s *Server) FuncService() *functions.Service`
- **Impact**: Creates and stores Service instance

### internal/server/handlers/functions.go
- Line 15: `service *functions.Service`
- Line 19: `func NewFunctionHandlers(service *functions.Service)`
- Lines 32-33: Uses `functions.FunctionError`, `functions.LogEntry`
- Lines 62-64: Uses `functions.AuthContext`
- **Impact**: Depends on Service for function invocation

### internal/server/handlers/health.go
- Line 17: `funcService *functions.Service`
- Line 21: `func NewHealthHandlers(..., funcService *functions.Service, ...)`
- **Impact**: Uses Service for health checks

## Unexpected Dependencies

None - all dependencies were expected based on the plan.

## Config Package Impact

The `config.PoolConfig` type in `internal/config/config.go` still exists and references container concepts:
- Line 256: `Pools map[string]PoolConfig`
- Line 263-273: `type PoolConfig struct` with container-specific fields

This will need updating in a future task when config is migrated to WASM settings.

## Build Status

`go build ./...` produces only 2 errors (both expected):
- `internal/webhooks/handler.go:18:21: undefined: functions.Service`
- `internal/webhooks/handler.go:22:50: undefined: functions.Service`

All errors are in expected locations. No unexpected compilation failures.
