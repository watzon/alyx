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
