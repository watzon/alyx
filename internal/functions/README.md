# Functions Package

This package provides serverless function execution for Alyx using a subprocess-based runtime.

## Architecture

**Subprocess-based execution** - Functions run as subprocesses communicating via JSON stdin/stdout protocol.

### Supported Runtimes

- **Deno** - TypeScript/JavaScript with Deno runtime
- **Node.js** - JavaScript/TypeScript with Node runtime  
- **Bun** - TypeScript/JavaScript with Bun runtime
- **Python** - Python 3 functions
- **Go** - Go functions via `go run`

### JSON Protocol

Functions communicate via JSON over stdin/stdout:

**Input (stdin)**:
```json
{
  "request_id": "req_123",
  "function": "hello",
  "input": {"name": "World"},
  "context": {
    "auth": {"id": "user_123", "email": "user@example.com"},
    "env": {"API_KEY": "secret"},
    "alyx_url": "http://localhost:8090",
    "internal_token": "token_xyz"
  }
}
```

**Output (stdout)**:
```json
{
  "request_id": "req_123",
  "success": true,
  "output": {"message": "Hello, World!"}
}
```

### Components

- `runtime.go` - Subprocess runtime implementation
- `executor.go` - Function service and execution orchestration
- `manifest.go` - Function manifest parsing (YAML)
- `discovery.go` - Function discovery and registration
- `watcher.go` - Source file watching for hot reload
- `types.go` - Core types (FunctionRequest, FunctionResponse, Runtime)
- `token.go` - Internal API token management

### Example Functions

See `examples/functions-demo/functions/` for working examples:
- `hello-deno/` - Deno TypeScript example
- `hello-node/` - Node.js JavaScript example
- `hello-python/` - Python 3 example

## Migration from WASM

This package previously used WebAssembly (WASM) via Extism for function execution. The subprocess approach provides:
- **Simpler deployment** - No WASM compilation required
- **Better compatibility** - Native runtime support for each language
- **Easier debugging** - Standard stdin/stdout, familiar tooling
- **No CGO** - Pure Go implementation using `os/exec`

The `Executor` interface remains unchanged, making the subprocess runtime a drop-in replacement.
