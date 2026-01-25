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

## Task 5: Source File Watcher Implementation (2026-01-25)

### fsnotify API Patterns

**Watcher Creation:**
- `fsnotify.NewWatcher()` creates a new file system watcher
- Returns `(*Watcher, error)` - must check error
- Watcher has `Events` and `Errors` channels for event loop

**Adding Watches:**
- `watcher.Add(path)` adds a directory or file to watch
- Must add directories, not glob patterns directly
- For glob patterns, extract base directory and filter events

**Event Loop Pattern:**
```go
for {
    select {
    case <-ctx.Done():
        return
    case event, ok := <-watcher.Events:
        if !ok { return }
        if event.Op&fsnotify.Write == fsnotify.Write { ... }
    case err, ok := <-watcher.Errors:
        if !ok { return }
        log.Error().Err(err).Msg("Watcher error")
    }
}
```

**Event Operations:**
- `fsnotify.Write` - file modified
- `fsnotify.Create` - file created
- `fsnotify.Remove` - file deleted
- `fsnotify.Rename` - file renamed
- `fsnotify.Chmod` - permissions changed
- Use bitwise AND to check: `event.Op&fsnotify.Write == fsnotify.Write`

**Cleanup:**
- `watcher.Close()` closes the watcher and channels
- Must be called to free resources
- Channels will be closed after Close()

### Debouncing Implementation

**Timer-Based Debouncing:**
- Use `time.AfterFunc(duration, func())` for delayed execution
- Store timers in map keyed by function name
- Stop existing timer before creating new one
- Prevents rapid successive builds from multiple file changes

**Pattern:**
```go
type SourceWatcher struct {
    debounceTimers   map[string]*time.Timer
    debounceDuration time.Duration
    mu               sync.Mutex
}

func (sw *SourceWatcher) debounceBuild(fn *FunctionDef) {
    sw.mu.Lock()
    defer sw.mu.Unlock()
    
    if timer, exists := sw.debounceTimers[fn.Name]; exists {
        timer.Stop()
    }
    
    sw.debounceTimers[fn.Name] = time.AfterFunc(sw.debounceDuration, func() {
        sw.executeBuild(fn)
    })
}
```

**Default Duration:**
- 100ms default debounce duration
- Configurable via `SetDebounceDuration()`
- Balances responsiveness vs. build frequency

### Build Execution Patterns

**Command Execution:**
- Use `exec.CommandContext(ctx, command, args...)` for cancellable execution
- Set `cmd.Dir` to function directory for correct working directory
- Use `cmd.CombinedOutput()` to capture both stdout and stderr

**Timeout Handling:**
- Create context with timeout: `context.WithTimeout(context.Background(), 5*time.Minute)`
- Defer `cancel()` to clean up resources
- Timeout propagates through CommandContext

**Error Handling:**
- Log build failures but continue watching (graceful degradation)
- Capture output even on failure for debugging
- Non-zero exit code indicates build failure

**Pattern:**
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

cmd := exec.CommandContext(ctx, manifest.Build.Command, manifest.Build.Args...)
cmd.Dir = funcDir

output, err := cmd.CombinedOutput()
if err != nil {
    log.Error().Err(err).Str("output", string(output)).Msg("Build failed")
    return
}
log.Info().Str("output", string(output)).Msg("Build succeeded")
```

### Glob Pattern Matching

**gobwas/glob Library:**
- `glob.Compile(pattern, separator)` compiles glob pattern
- Use `/` as separator for cross-platform compatibility
- Returns `Glob` interface with `Match(string) bool` method

**Pattern Extraction:**
- Extract base directory from glob pattern (first non-wildcard component)
- Watch base directory, filter events by pattern
- Supports `*`, `**`, `?`, `[...]` wildcards

**Relative Path Matching:**
- Convert absolute file path to relative path from function directory
- Match relative path against glob patterns
- Allows patterns like `src/**/*.js` to work correctly

**Pattern:**
```go
matcher, err := glob.Compile(pattern, '/')
if err != nil {
    log.Warn().Err(err).Str("pattern", pattern).Msg("Invalid glob pattern")
    continue
}

relPath, _ := filepath.Rel(funcDir, filePath)
if matcher.Match(relPath) {
    // File matches pattern
}
```

### Registry Integration

**Function Discovery:**
- Use `registry.List()` to get all discovered functions
- Each function has `Path` field pointing to entry file
- Use `filepath.Dir(fn.Path)` to get function directory

**Manifest Loading:**
- Load manifest from `<funcDir>/manifest.yaml`
- Parse with `yaml.Unmarshal()` into `Manifest` struct
- Validate with `manifest.Validate()` before use

**Build Config Access:**
- Check `manifest.Build != nil` before accessing
- `Build.Command` - build command to execute
- `Build.Args` - command arguments
- `Build.Watch` - glob patterns to watch
- `Build.Output` - output file path

### Thread Safety

**Mutex Protection:**
- Use `sync.Mutex` to protect `debounceTimers` map
- Lock before accessing/modifying map
- Defer unlock for exception safety

**Goroutine Management:**
- Use `sync.WaitGroup` to track event loop goroutine
- `wg.Add(1)` before starting goroutine
- `defer wg.Done()` in goroutine
- `wg.Wait()` in Stop() to ensure clean shutdown

**Context Cancellation:**
- Create context with `context.WithCancel()`
- Store cancel function in struct
- Call `cancel()` in Stop() to signal shutdown
- Check `<-ctx.Done()` in event loop

### Test Patterns

**Test Helpers:**
- `testRegistry(t)` creates temporary registry with cleanup
- `createTestFunction(t, registry, name, buildConfig)` creates test function with manifest
- `manifestToYAML(t, manifest)` converts manifest to YAML string
- Use `t.TempDir()` for isolated test directories

**Timing in Tests:**
- Use `time.Sleep()` to wait for async operations
- 100ms for file system events to propagate
- 200ms for debounced builds to execute
- Tests are timing-sensitive but reliable

**Table-Driven Tests:**
- `TestSourceWatcher_GlobPattern` uses table-driven approach
- Each test case has patterns, files, and expected behavior
- Subtests with `t.Run()` for isolation

### Gotchas

1. **Directory Creation**: Must create parent directories before adding watch patterns
2. **Glob vs. Watch**: fsnotify watches directories, not glob patterns - must filter events
3. **Relative Paths**: Must convert absolute paths to relative for glob matching
4. **Timer Cleanup**: Must stop timers in Stop() to prevent goroutine leaks
5. **Channel Closure**: Check `ok` when receiving from channels (closed on watcher.Close())
6. **YAML Quoting**: Must quote strings in YAML to avoid parsing errors with special characters

### Files Created

- `internal/functions/watcher.go` (350 lines) - SourceWatcher implementation
- `internal/functions/watcher_test.go` (410 lines) - Comprehensive test suite

### Dependencies Added

- `github.com/fsnotify/fsnotify` - File system notifications
- `github.com/gobwas/glob` - Glob pattern matching (already present from Extism)

### Test Coverage

- ✅ File change detection
- ✅ Debouncing (multiple rapid changes → single build)
- ✅ Build success (command execution with output capture)
- ✅ Build failure (graceful error handling, continues watching)
- ✅ Glob pattern matching (single file, nested, exclusion, multiple patterns)
- ✅ Multiple functions (independent watching)
- ✅ No build config (skips watching)
- ✅ Cleanup (Stop() cleans up timers)

### Next Steps

- Task 6: Integrate watcher with server startup
- Task 7: Add watcher reload on manifest changes
- Task 8: Implement .wasm file watching for hot reload

## Task 6: WASM File Watcher with Hot Reload (2026-01-25)

### WASMRuntime.Reload() Implementation

**Method Signature:**
```go
func (w *WASMRuntime) Reload(name string, wasmPath string) error
```

**Reload Strategy:**
1. Acquire write lock (full mutex, not RWMutex)
2. Close existing plugin if present
3. Delete from plugins map
4. Load new plugin using `loadPluginLocked()`
5. Log reload success

**Key Design Decision:**
- Extracted `loadPluginLocked()` internal method to avoid duplicate code
- Reload works even if plugin doesn't exist (creates new)
- Failed reload leaves plugin unloaded (no rollback to old version)
- Thread-safe with mutex protection

### WASMWatcher Architecture

**Structure:**
```go
type WASMWatcher struct {
    runtime          *WASMRuntime
    registry         *Registry
    watcher          *fsnotify.Watcher
    debounceTimers   map[string]*time.Timer
    debounceDuration time.Duration
    mu               sync.Mutex
    ctx              context.Context
    cancel           context.CancelFunc
    wg               sync.WaitGroup
}
```

**Debounce Duration:**
- Default: 200ms (vs. 100ms for SourceWatcher)
- Rationale: Longer delay allows builds to complete before triggering reload
- Prevents reload attempts on incomplete WASM files

### Coordination Between Watchers

**Flow:**
1. SourceWatcher detects source change
2. SourceWatcher triggers build (debounced 100ms)
3. Build writes new .wasm file
4. WASMWatcher detects .wasm change
5. WASMWatcher triggers reload (debounced 200ms)

**Timing:**
- Source debounce: 100ms (fast response to edits)
- WASM debounce: 200ms (wait for build completion)
- Total latency: ~300ms from source change to reload

### File Watching Patterns

**WASM File Detection:**
- Watch directory containing .wasm file (not file directly)
- Filter events by `.wasm` extension
- Match against expected output path from manifest

**Directory Structure:**
```
functions/
  my-function/
    src/
      index.js          <- SourceWatcher
    manifest.yaml
    plugin.wasm         <- WASMWatcher
```

### Error Handling

**Reload Failures:**
- Log error but continue watching
- Plugin remains unloaded after failed reload
- No automatic retry (waits for next file change)

**Invalid WASM:**
- Extism validates WASM on load
- Returns error for invalid format
- Test: `TestWASMWatcher_DetectsChanges` shows graceful failure

### Thread Safety

**Mutex Strategy:**
- WASMWatcher uses `sync.Mutex` for debounceTimers map
- WASMRuntime uses `sync.RWMutex` for plugins map
- Reload acquires write lock (blocks all reads during reload)

**Concurrent Reload:**
- Test: `TestWASMRuntime_Reload/reload_while_plugin_is_in_use`
- Mutex ensures safe reload even during concurrent reads
- No race conditions detected

### Test Coverage

**WASMRuntime.Reload() Tests:**
- ✅ Reload existing plugin (verifies new instance created)
- ✅ Reload nonexistent plugin (creates new)
- ✅ Reload with invalid WASM (leaves plugin unloaded)
- ✅ Reload while plugin in use (thread safety)

**WASMWatcher Tests:**
- ✅ Detects .wasm file changes
- ✅ Triggers reload on change
- ✅ Debouncing (5 rapid changes → 1 reload)
- ✅ Multiple functions (independent watching)
- ✅ No build config (skips watching)
- ✅ Cleanup (Stop() cleans up timers)

### Integration Points

**Registry Integration:**
- Uses `registry.List()` to discover functions
- Loads manifest to get build output path
- Skips functions without build config

**WASMRuntime Integration:**
- Calls `runtime.Reload(name, wasmPath)` on change
- No direct plugin access (encapsulation)
- Runtime handles all plugin lifecycle

### Gotchas

1. **File Extension Check**: Must check `.wasm` extension to avoid triggering on temp files
2. **Debounce Timing**: 200ms chosen empirically - too short causes reload on incomplete builds
3. **Directory Watching**: fsnotify watches directories, not individual files
4. **Manifest Reloading**: Each event reloads manifest (could cache, but simple approach works)
5. **No Rollback**: Failed reload leaves plugin unloaded (intentional - fail fast)

### Performance Considerations

**Memory:**
- One fsnotify.Watcher per WASMWatcher instance
- One timer per function in debounceTimers map
- Timers cleaned up on Stop()

**CPU:**
- Minimal overhead (event-driven)
- Debouncing prevents excessive reloads
- Manifest parsing only on events

### Files Modified

- `internal/functions/wasm.go` - Added Reload() method, extracted loadPluginLocked()
- `internal/functions/watcher.go` - Added WASMWatcher struct and methods
- `internal/functions/wasm_test.go` - Added TestWASMRuntime_Reload tests
- `internal/functions/watcher_test.go` - Added WASMWatcher tests, testWASMWatcher helper

### Next Steps

- Task 7: Integrate watchers with server startup
- Task 8: Add coordination between SourceWatcher and WASMWatcher
- Task 9: Add metrics/logging for reload performance
