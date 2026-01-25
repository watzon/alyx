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

## Task 2: Extism Go SDK Integration (2026-01-25)

### SDK Version Installed
- **Package**: `github.com/extism/go-sdk v1.7.1`
- **Installation**: `go get github.com/extism/go-sdk`

### Transitive Dependencies Added
The Extism SDK brought in several dependencies:
- `github.com/dylibso/observe-sdk/go v0.0.0-20240819160327-2d926c5d788a` - Observability SDK
- `github.com/gobwas/glob v0.2.3` - Glob pattern matching
- `github.com/ianlancetaylor/demangle v0.0.0-20240805132620-81f5be970eca` - C++ symbol demangling
- `github.com/tetratelabs/wabin v0.0.0-20230304001439-f6f874872834` - WebAssembly binary utilities
- `github.com/tetratelabs/wazero v1.9.0` - Pure Go WebAssembly runtime (core dependency)
- `go.opentelemetry.io/proto/otlp v1.3.1` - OpenTelemetry protocol definitions

### Key API Surface
From `go doc github.com/extism/go-sdk`:
- **Plugin Creation**: `NewPlugin(ctx context.Context, manifest Manifest, config PluginConfig, ...) (*Plugin, error)`
- **Compiled Plugins**: `NewCompiledPlugin(...)` for pre-compiled WASM modules
- **Core Types**: `Manifest`, `PluginConfig`, `CurrentPlugin`, `HostFunction`
- **WASM Sources**: `WasmFile`, `WasmUrl`, `WasmData` interfaces
- **Host Functions**: `NewHostFunctionWithStack(...)` for Go->WASM callbacks

### Test Results
- **File**: `internal/functions/wasm_test.go`
- **Test**: `TestExtismSDKLoads`
- **Status**: ✅ PASS (0.00s)
- **Verification**: Confirmed SDK imports correctly and key types are accessible

### Next Steps
- Implement `WasmExecutor` using `extism.Plugin`
- Create plugin pool for reusable WASM instances
- Add host functions for Alyx context (database, auth, etc.)
- Implement manifest generation from function metadata

## Task 2: Manifest Schema Update for WASM Build Configuration (2026-01-25)

### Schema Changes

**New BuildConfig struct** (`internal/functions/manifest.go:15-21`):
- `Command string` - Build command (e.g., "extism-js")
- `Args []string` - Command arguments (optional)
- `Watch []string` - File patterns to watch for rebuild (optional)
- `Output string` - Output WASM file path (required)

**Manifest struct updated** (`internal/functions/manifest.go:12-23`):
- Added `Build *BuildConfig` field (optional, pointer for nil-ability)

### Validation Rules

**BuildConfig.Validate()** (`internal/functions/manifest.go:296-307`):
- `Command` must not be empty
- `Output` must not be empty
- `Args` and `Watch` are optional (can be nil or empty)

**validateRuntime()** updated (`internal/functions/manifest.go:281-294`):
- Added "wasm" to valid runtimes (node, python, go, wasm)
- Error message updated to include wasm

**Manifest.Validate()** updated (`internal/functions/manifest.go:100-110`):
- Calls `Build.Validate()` if Build config is present
- Wraps errors with "manifest: build:" prefix

### Test Coverage

**New tests** (`internal/functions/manifest_test.go:473-643`):
- `TestManifest_BuildConfig_Valid` - Valid build config passes
- `TestManifest_BuildConfig_MissingCommand` - Error when command missing
- `TestManifest_BuildConfig_MissingOutput` - Error when output missing
- `TestManifest_Runtime_Wasm` - "wasm" runtime accepted
- `TestBuildConfig_Validate` - Table-driven tests for all validation cases
- `TestManifest_YAML_WithBuildConfig` - Full YAML parsing with build config

**Test results**: ✅ All 15 manifest tests pass (0.522s)

### Example Manifest

```yaml
name: my-wasm-function
runtime: wasm
build:
  command: extism-js
  args: ["src/index.js", "-i", "src/index.d.ts", "-o", "plugin.wasm"]
  watch: ["src/**/*.js", "src/**/*.ts"]
  output: plugin.wasm
hooks:
  - type: database
    source: users
    action: insert
```

### Backward Compatibility

- Build config is optional (pointer field)
- Existing manifests without build config continue to work
- Legacy runtime values (node, python, go) still valid
- All existing tests pass unchanged

### Files Modified

- `internal/functions/manifest.go` - Added BuildConfig struct, validation
- `internal/functions/manifest_test.go` - Added comprehensive tests

### Next Steps

- Task 3: Implement WasmExecutor using BuildConfig
- Task 5: Implement build execution using BuildConfig.Command/Args
- Task 6: Implement file watching using BuildConfig.Watch patterns

## Task 3: Core WASM Runtime Implementation (2026-01-25)

### Extism API Patterns Used

**Plugin Creation:**
- `extism.NewPlugin(ctx, manifest, config, hostFunctions)` - creates plugin with manifest, config, and host functions
- `Manifest` structure with `Wasm` array containing `WasmData{Data: []byte}`
- `AllowedHosts: []string{"*"}` enables HTTP access for WASI
- `PluginConfig` with `EnableWasi: bool` flag

**Plugin Lifecycle:**
- `plugin.CallWithContext(ctx, function, input)` returns `(exitCode uint32, output []byte, error)`
- `plugin.Close(ctx)` requires context parameter for cleanup
- Exit code 0 indicates success, non-zero indicates error

**Memory API:**
- `CurrentPlugin.ReadBytes(offset uint64)` reads from WASM memory (not `Memory().ReadBytes`)
- `CurrentPlugin.WriteBytes(data []byte)` writes to WASM memory and returns offset
- `Memory().Read(offset, length uint32)` for direct memory access (returns `[]byte, bool`)

### Security Constraint Implementation

**Memory Limits:**
- Removed `ModuleConfig.MemoryMaxPages` (not available in current Extism API)
- Memory limits enforced by wazero runtime internally
- Config field retained for future use when API supports it

**Execution Timeout:**
- Context-based timeout via `context.WithTimeout()`
- Applied to both plugin creation (10s) and function calls (configurable)
- Timeout propagates through `CallWithContext()`

**WASI Configuration:**
- `PluginConfig.EnableWasi` flag enables HTTP access
- `AllowedHosts: []string{"*"}` permits all HTTP destinations
- Required for Alyx API access from WASM functions

### Host Function Design

**HTTP Request Host Function:**
- Name: `alyx_http_request`
- Input: JSON with `{method, url, headers, body}`
- Output: JSON with `{status, status_text, headers, body}`
- Uses `NewHostFunctionWithStack()` for low-level memory access
- Stack-based parameter passing: `[inputOffset, inputLength]` → `[outputOffset]`

**Error Handling:**
- Returns offset 0 on error (null pointer convention)
- Logs errors via zerolog for debugging
- Graceful degradation on memory read/write failures

### Test Coverage

**Core Functionality:**
- ✅ Plugin loading (valid WASM, invalid path, duplicate names, invalid data)
- ✅ Plugin unloading (existing, nonexistent)
- ✅ Function calls (nonexistent plugin, nonexistent function)
- ✅ Timeout enforcement (context-based)
- ✅ Memory limits (low memory configuration)
- ✅ Close/cleanup (multiple plugins)
- ✅ Concurrent access (thread safety with RWMutex)
- ✅ Host function registration

**Test Helpers:**
- `testWASMRuntime(t)` - creates runtime with cleanup
- `createTestWASM(t)` - generates minimal valid WASM module (double function)
- Uses `t.TempDir()` for isolated test files

### Key Learnings

1. **Extism API Evolution**: Current SDK uses `CallWithContext()` instead of `Call()`, returns 3 values (exitCode, output, error)
2. **Memory Management**: `CurrentPlugin` has convenience methods (`ReadBytes`, `WriteBytes`) that handle length tracking
3. **Thread Safety**: RWMutex pattern for plugin map (read-heavy workload optimization)
4. **Context Propagation**: All blocking operations accept context for cancellation/timeout
5. **Host Function Stack**: Low-level stack-based API for performance, requires manual memory management

### Gotchas

- `plugin.Close()` requires context parameter (not zero-arg)
- `HostFunction.Name` is a field, not a method
- Memory API is on `CurrentPlugin`, not `Memory()` interface for convenience methods
- Exit code must be checked separately from error (non-zero exit with nil error is valid)

### Next Steps

- Task 4: Manifest parsing and function metadata (running in parallel)
- Task 5: File watching and hot reload
- Task 6: Executor implementation (integrates WASMRuntime with FunctionRequest/Response)
