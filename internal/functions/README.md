# Functions Package

This package provides serverless function execution for Alyx.

## Architecture Migration

**Container-based execution has been removed in favor of WASM.**

The previous Docker/Podman container-based execution system has been replaced with WebAssembly (WASM) for improved performance, security, and portability.

### Removed Components

- `container.go` - Docker container management
- `pool.go` - Container pool management  
- `executor.go` - Container-based executor implementation
- Container-specific types: `Container`, `ContainerState`, `PoolConfig`, `ContainerManager`

### Retained Components

- `manifest.go` - Function manifest parsing (YAML)
- `discovery.go` - Function discovery and registration
- `types.go` - Core types (FunctionRequest, FunctionResponse, Executor interface)
- `token.go` - Internal API token management

### Migration Path

The `Executor` interface remains unchanged, allowing the WASM executor to be a drop-in replacement for the container-based implementation.

Functions will be compiled to WASM modules and executed in a sandboxed WASM runtime instead of Docker containers.
