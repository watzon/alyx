# Learnings - Universal Subprocess Runtime

## Conventions & Patterns
<!-- Append discoveries about code patterns, naming conventions, architectural decisions -->


## Task 1: Remove WASM Implementation (2026-01-25)

### What Was Removed
- **Files deleted**: `internal/functions/wasm.go` (300 lines), `internal/functions/wasm_test.go` (466 lines)
- **Dependency removed**: `github.com/extism/go-sdk v1.7.1` and all transitive dependencies
- **Transitive deps cleaned**: `dylibso/observe-sdk`, `tetratelabs/wabin`, `tetratelabs/wazero`, `ianlancetaylor/demangle`

### Key Components Removed
1. **WASMRuntime**: Plugin management with Extism SDK
2. **WASMConfig**: Memory limits, timeouts, WASI support
3. **Host Functions**: `alyx_http_request` for HTTP access from WASM
4. **Plugin Lifecycle**: Load, unload, reload, call operations
5. **Concurrency**: Mutex-protected plugin map for thread-safe access

### Remaining References (Expected)
- `manifest.go` and `manifest_test.go` contain `"extism-js"` as test data for the `Command` field
- These are YAML parsing tests and will be updated in later tasks when manifest structure changes
- No actual code dependencies on Extism remain

### Verification
- ✅ `go mod tidy` completed successfully
- ✅ All Extism dependencies removed from `go.mod` and `go.sum`
- ✅ No extism imports in `internal/functions/*.go` (except test data strings)
- ✅ Files confirmed deleted via `ls internal/functions/`

### Next Steps
This unblocks all other refactoring work. The `Executor` interface remains unchanged, allowing the subprocess executor to be a drop-in replacement.

## Task 2: Update Runtime Type Definitions (2026-01-25)

### Changes Made
- **Removed**: `RuntimeWasm` constant
- **Added**: `RuntimeDeno = "deno"` and `RuntimeBun = "bun"` constants
- **Added**: `RuntimeConfig` struct with Command, Args, Extensions fields
- **Added**: `defaultRuntimes` map with configurations for all 5 runtimes

### RuntimeConfig Structure
```go
type RuntimeConfig struct {
    Command    string      // Executable name (e.g., "deno", "node")
    Args       []string    // Default arguments (e.g., ["run", "--allow-all"])
    Extensions []string    // File extensions (e.g., [".ts", ".tsx"])
}
```

### Default Runtime Configurations
| Runtime | Command | Args | Extensions |
|---------|---------|------|------------|
| Deno | `deno` | `["run", "--allow-all"]` | `[".ts", ".tsx"]` |
| Node | `node` | `[]` | `[".js", ".mjs"]` |
| Bun | `bun` | `["run"]` | `[".ts", ".tsx", ".js"]` |
| Python | `python3` | `[]` | `[".py"]` |
| Go | `go` | `["run"]` | `[".go"]` |

### Design Decisions
1. **Deno permissions**: Used `--allow-all` for simplicity (can be restricted per-function later)
2. **Bun multi-extension**: Supports both TypeScript and JavaScript natively
3. **Node ESM**: Included `.mjs` for ES module support
4. **Go runtime**: Uses `go run` for direct source execution (no pre-compilation)

### Expected Build Errors (Verified)
- `executor.go:28,45,51`: Undefined WASMRuntime, WASMConfig, NewWASMRuntime
- `watcher.go:334,346`: Undefined WASMRuntime
- `discovery.go:175-177`: Undefined RuntimeWasm

These will be resolved in Tasks 3-6.

### API Stability
- ✅ FunctionRequest/FunctionResponse unchanged (API stable)
- ✅ Executor interface unchanged (drop-in replacement pattern maintained)

### Next Steps
This unblocks Task 3 (Implement Subprocess Runtime) which will use `defaultRuntimes` map for runtime detection and execution.

## Task 2: Update Manifest Validation (2026-01-25)

### Changes Made
1. **Updated `validateRuntime()` in `manifest.go`**:
   - Added `"deno"` to valid runtimes
   - Added `"bun"` to valid runtimes
   - Removed `"wasm"` from valid runtimes
   - Updated error message to reflect new valid runtimes

2. **Updated test cases in `manifest_test.go`**:
   - `TestManifest_BuildConfig_Valid`: Changed from `wasm` runtime with `extism-js` to `node` runtime with `tsc` (TypeScript compilation)
   - `TestManifest_BuildConfig_MissingCommand`: Changed from `wasm` to `node` runtime
   - `TestManifest_BuildConfig_MissingOutput`: Changed from `wasm` to `node` runtime
   - `TestManifest_Runtime_Wasm`: Replaced with two new tests:
     - `TestManifest_Runtime_Deno`: Validates `deno` as a valid runtime
     - `TestManifest_Runtime_Bun`: Validates `bun` as a valid runtime
   - `TestManifest_YAML_WithBuildConfig`: Changed from WASM example to Node.js TypeScript compilation example

### Key Findings
- **BuildConfig remains optional**: The validation logic checks `if m.Build != nil` before validating, meaning BuildConfig is not required for any runtime
- **BuildConfig is runtime-agnostic**: No runtime-specific logic ties BuildConfig to WASM or any other runtime
- **Use case preserved**: BuildConfig is still useful for TypeScript compilation (`tsc`), bundling, or any pre-processing step

### Verification Status
- ✅ `manifest.go` updated with new runtime validation
- ✅ `manifest_test.go` updated with new test cases
- ✅ BuildConfig remains optional and runtime-agnostic
- ⚠️ Cannot run tests yet due to WASM references in other files (`executor.go`, `watcher.go`, `discovery.go`)
- ⏳ Tests will pass once Task 3 (cleanup WASM references) is complete

### Dependencies
- **Blocks**: None (changes are isolated to manifest validation)
- **Blocked by**: Task 3 must complete before tests can run successfully
- **Parallel safe**: This task can be completed independently

### Next Steps
Task 3 will need to clean up WASM references in:
- `internal/functions/executor.go` (WASMRuntime, WASMConfig, NewWASMRuntime)
- `internal/functions/watcher.go` (WASMWatcher, NewWASMWatcher)
- `internal/functions/discovery.go` (RuntimeWasm constant)
- `internal/functions/watcher_test.go` (testWASMRuntime, testWASMWatcher)

## Task 3: Implement Subprocess Runtime (2026-01-25)

### Implementation Overview
Created `internal/functions/runtime.go` with ~100 lines implementing subprocess-based function execution.

### Key Components

#### SubprocessRuntime Struct
```go
type SubprocessRuntime struct {
    runtime Runtime        // Runtime type (deno, node, bun, python, go)
    config  RuntimeConfig  // Command, args, extensions
}
```

#### NewSubprocessRuntime Constructor
- Validates runtime exists in `defaultRuntimes` map
- Uses `exec.LookPath()` to check if runtime binary is installed
- Returns clear error if binary not found: `"runtime binary not found: deno (install deno to use this runtime)"`
- No CGO dependencies, pure Go stdlib

#### Call Method Signature
```go
func (r *SubprocessRuntime) Call(ctx context.Context, name, entrypoint string, req *FunctionRequest) (*FunctionResponse, error)
```

### JSON Protocol Implementation

**Input (stdin)**:
- Marshals `FunctionRequest` struct to JSON
- Pipes to subprocess stdin via `bytes.NewReader()`
- Includes: request_id, function name, input data, context (auth, env, alyx_url, internal_token)

**Output (stdout)**:
- Reads subprocess stdout into `bytes.Buffer`
- Unmarshals JSON into `FunctionResponse` struct
- Includes: request_id, success flag, output/error, logs, duration_ms

**Error Handling (stderr)**:
- Captures stderr separately into `bytes.Buffer`
- Included in error messages for debugging
- Non-zero exit codes return error with stderr content

### Context & Timeout Handling
- Uses `exec.CommandContext(ctx, ...)` for automatic timeout support
- Checks `ctx.Err()` to distinguish timeout from other errors
- Returns clear error: `"function hello timed out: context deadline exceeded"`

### Command Construction
```go
args := append(r.config.Args, entrypoint)
cmd := exec.CommandContext(ctx, r.config.Command, args...)
```

**Examples**:
- Deno: `deno run --allow-all /path/to/function.ts`
- Node: `node /path/to/function.js`
- Bun: `bun run /path/to/function.ts`
- Python: `python3 /path/to/function.py`
- Go: `go run /path/to/function.go`

### Error Classification

1. **Runtime not found**: `exec.LookPath()` fails → clear installation message
2. **Context timeout**: `ctx.Err() != nil` → timeout error with context
3. **Non-zero exit**: `exec.ExitError` → includes exit code and stderr
4. **Invalid JSON output**: `json.Unmarshal()` fails → includes stdout/stderr for debugging
5. **Other exec errors**: Generic execution error with wrapping

### Edge Cases Handled

- **Large payloads**: Pipes handle streaming automatically (no size limits)
- **Invalid JSON**: Clear error with stdout/stderr included
- **Missing binary**: Checked at construction time, not execution time
- **Timeout**: Handled via context cancellation
- **Stderr logging**: Captured separately, included in errors but not mixed with stdout

### Design Decisions

1. **No process pooling**: Each call spawns fresh process (future optimization)
2. **No streaming protocol**: Single request/response only (matches current API)
3. **Synchronous execution**: Blocks until subprocess completes
4. **Pure stdlib**: No external dependencies beyond `os/exec` and `encoding/json`
5. **FunctionRequest input**: Changed from `map[string]any` to `*FunctionRequest` for full protocol support

### Helper Methods
```go
func (r *SubprocessRuntime) Runtime() Runtime
func (r *SubprocessRuntime) Config() RuntimeConfig
```

### Verification Status
- ✅ `runtime.go` created (~100 lines)
- ✅ `SubprocessRuntime` struct implemented
- ✅ `NewSubprocessRuntime()` with binary existence check
- ✅ `Call()` method with JSON stdin/stdout protocol
- ✅ Context timeout support via `CommandContext`
- ✅ Stderr captured separately
- ✅ Non-zero exit codes handled as errors
- ⚠️ Expected build errors in `executor.go`, `watcher.go`, `discovery.go` (WASM references)

### Next Steps
Task 4 will update `executor.go` to use `SubprocessRuntime` instead of `WASMRuntime`, which will resolve the build errors.

### API Compatibility
- ✅ Executor interface unchanged (drop-in replacement pattern maintained)
- ✅ FunctionRequest/FunctionResponse types used directly
- ✅ No breaking changes to public API
