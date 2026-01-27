# Draft: Universal Subprocess Runtime

## Requirements (confirmed)

- **Replace WASM entirely**: Strip out Extism/WASM, go with subprocess approach
- **Polyglot support**: Support any language that can read stdin/write stdout (Deno, Node, Bun, Python, Go, Rust, Ruby, etc.)
- **No CGO**: Pure Go implementation using `os/exec`
- **Simple DX**: Users just need the runtime binary installed (like needing Node.js installed)
- **Hot reload**: Keep source watching, but simplify (no WASM watcher needed)

## Technical Decisions

- **Execution model**: Subprocess via `os/exec.Command`
- **I/O protocol**: JSON via stdin → function → JSON via stdout
- **Runtime detection**: Auto-detect from entry file or explicit in manifest
- **Error handling**: stderr for logs/errors, exit code for success/failure
- **Timeout**: Context-based timeout on exec.Command

## Research Findings

### Current WASM Implementation (to remove)
- `wasm.go` (300 lines) - Extism runtime, plugin management, host functions
- `wasm_test.go` - WASM-specific tests
- `executor.go` - Service that wraps WASMRuntime
- `watcher.go` - Contains WASMWatcher (can simplify to just SourceWatcher)
- `types.go` - Has RuntimeWasm constant
- `manifest.go` - Has BuildConfig for WASM builds (repurpose for any build step)
- `discovery.go` - Looks for .wasm files (update to look for entry files)

### Files to Keep/Modify
- `types.go` - Keep FunctionRequest/Response, update Runtime constants
- `manifest.go` - Keep, update valid runtimes
- `discovery.go` - Keep, update entry file detection
- `token.go` - Keep (internal API tokens)
- `watcher.go` - Keep SourceWatcher, remove WASMWatcher

### Files to Delete
- `wasm.go` - Entire file
- `wasm_test.go` - Entire file

### Files to Create
- `runtime.go` - New universal subprocess runtime
- `runtime_test.go` - Tests for subprocess execution

### Dependencies to Remove
- `github.com/extism/go-sdk v1.7.1`

## Scope Boundaries

### INCLUDE
- Remove all WASM code
- Implement subprocess runtime
- Support Deno, Node, Bun, Python, Go as initial runtimes
- Update manifest validation for new runtimes
- Update discovery for new entry files
- Update examples
- Update documentation
- Hot reload via source watcher (simplified)

### EXCLUDE
- Process pooling (future optimization)
- Warm instances (future optimization)
- Auto-download of runtimes (future feature)
- Embedded runtime binaries (future feature)

## Metis Gap Analysis (Addressed)

### Questions Addressed
1. **Protocol**: Single JSON object per invocation (no streaming). Request → Response.
2. **Lifecycle**: One-shot execution per request (process pooling is future scope)
3. **Security**: Minimal - rely on runtime's own security (Deno permissions, etc.). No added sandboxing.
4. **Env/Working Dir**: Set `ALYX_*` env vars, working dir = function directory
5. **Concurrency**: Yes, multiple concurrent calls allowed (each spawns separate process)

### Guardrails (from Metis)
- No API surface changes beyond runtime selection
- No deployment/build pipeline changes beyond Extism removal
- No caching/pooling (future scope)
- No new dependencies beyond stdlib

### Edge Cases to Handle
- **Large payloads**: Use pipe, not buffer (Go handles this well)
- **Non-JSON stdout**: Treat as error, capture for debugging
- **Stderr**: Capture separately for logs, don't mix with response
- **Runtime not found**: Clear error message with install instructions
- **Timeout**: Context-based cancellation, configurable per function

### Acceptance Criteria (from Metis)
- Given manifest with runtime X, function runs and returns JSON
- Invalid JSON input → standardized error response
- Timeout/exit codes surfaced consistently
- Example functions for Deno/Node/Python work end-to-end

## Open Questions

- None - ready for plan generation

## Runtime Registry Design

```go
type RuntimeConfig struct {
    Name       string   // e.g., "deno", "node", "python"
    Command    string   // e.g., "deno", "node", "python3"
    Args       []string // e.g., ["run", "--allow-net"]
    Extensions []string // e.g., [".ts", ".js"]
}

var defaultRuntimes = map[Runtime]RuntimeConfig{
    RuntimeDeno:   {Name: "deno", Command: "deno", Args: []string{"run", "-A"}, Extensions: []string{".ts", ".js"}},
    RuntimeNode:   {Name: "node", Command: "node", Args: []string{}, Extensions: []string{".js", ".mjs", ".cjs"}},
    RuntimeBun:    {Name: "bun", Command: "bun", Args: []string{"run"}, Extensions: []string{".ts", ".js"}},
    RuntimePython: {Name: "python", Command: "python3", Args: []string{}, Extensions: []string{".py"}},
    RuntimeGo:     {Name: "go", Command: "go", Args: []string{"run"}, Extensions: []string{".go"}},
}
```

## Function Interface (stdin/stdout)

**Input (stdin):**
```json
{
  "request_id": "uuid",
  "function_name": "hello",
  "input": {"name": "World"},
  "context": {
    "auth": {"id": "user123", "email": "..."},
    "env": {"API_KEY": "..."},
    "alyx_url": "http://localhost:8090",
    "internal_token": "token..."
  }
}
```

**Output (stdout):**
```json
{
  "success": true,
  "output": {"message": "Hello, World!"},
  "logs": [{"level": "info", "message": "..."}]
}
```

**Or on error:**
```json
{
  "success": false,
  "error": {"code": "VALIDATION_ERROR", "message": "..."}
}
```
