# Universal Subprocess Runtime - Replace WASM with Polyglot Functions

## Context

### Original Request
Replace the WASM-based function runtime (Extism) with a universal subprocess runtime that supports any language via stdin/stdout JSON protocol. No CGO, pure Go implementation.

### Interview Summary
**Key Discussions**:
- WASM complexity was too high: cryptic build errors, limited npm support, Wizer initialization issues
- Subprocess approach enables true polyglot: any language that reads stdin/writes stdout works
- No CGO requirement - pure Go using `os/exec`
- One-shot execution model (process pooling is future scope)

**Research Findings**:
- Current WASM implementation: ~900 lines (wasm.go, wasm_test.go, parts of executor.go, watcher.go)
- Subprocess runtime: ~200 lines estimated
- Extism dependency to remove: `github.com/extism/go-sdk v1.7.1`

### Metis Review
**Identified Gaps** (addressed):
- Protocol definition: Added explicit JSON request/response format
- Error handling: Defined stderr capture, exit code handling
- Edge cases: Large payloads (pipes), runtime not found, timeouts
- Acceptance criteria: Added specific verification commands

---

## Work Objectives

### Core Objective
Replace WASM runtime with a subprocess-based universal runtime that executes functions in any language via JSON stdin/stdout protocol.

### Concrete Deliverables
- `internal/functions/runtime.go` - New subprocess runtime (~200 lines)
- `internal/functions/runtime_test.go` - Tests for subprocess execution
- Updated `internal/functions/types.go` - New runtime constants
- Updated `internal/functions/manifest.go` - Validate new runtimes
- Updated `internal/functions/discovery.go` - Detect new entry files
- Updated `internal/functions/executor.go` - Use subprocess runtime
- Simplified `internal/functions/watcher.go` - Remove WASMWatcher
- Example functions: Deno, Node, Python
- Removed: `wasm.go`, `wasm_test.go`, Extism dependency

### Definition of Done
- [ ] `make lint` passes with zero issues
- [ ] `make test` passes with all tests green
- [ ] Deno function example works: `curl http://localhost:8090/api/functions/hello-deno`
- [ ] Node function example works: `curl http://localhost:8090/api/functions/hello-node`
- [ ] Python function example works: `curl http://localhost:8090/api/functions/hello-python`
- [ ] No WASM references remain in codebase (except docs noting migration)

### Must Have
- Subprocess execution via `os/exec`
- JSON stdin/stdout protocol
- Support for Deno, Node, Bun, Python, Go runtimes
- Timeout handling via context
- Stderr capture for debugging
- Hot reload via source file watching

### Must NOT Have (Guardrails)
- No CGO or external C dependencies
- No process pooling (future scope)
- No container/sandboxing (rely on runtime's own security)
- No streaming protocol (single request/response)
- No API surface changes beyond runtime selection
- No new CLI commands

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go testing)
- **User wants tests**: YES (tests after - verify behavior)
- **Framework**: `go test`

### Test Commands
```bash
# All tests
make test

# Functions package only
go test -v ./internal/functions/...

# Specific runtime tests
go test -v -run TestSubprocessRuntime ./internal/functions/...
```

### Manual Verification
```bash
# Start server
./build/alyx dev

# Test Deno function
curl -X POST http://localhost:8090/api/functions/hello-deno \
  -H "Content-Type: application/json" \
  -d '{"name": "World"}'

# Test Node function
curl -X POST http://localhost:8090/api/functions/hello-node \
  -H "Content-Type: application/json" \
  -d '{"name": "World"}'

# Test Python function
curl -X POST http://localhost:8090/api/functions/hello-python \
  -H "Content-Type: application/json" \
  -d '{"name": "World"}'
```

---

## Task Flow

```
1 (Remove WASM) → 2 (Update Types) → 3 (Implement Runtime) → 4 (Update Executor)
                                                                    ↓
                    7 (Examples) ← 6 (Update Watcher) ← 5 (Update Discovery)
                        ↓
                    8 (Update Manifest) → 9 (Cleanup) → 10 (Verify)
```

## Parallelization

| Group | Tasks | Reason |
|-------|-------|--------|
| A | 2, 8 | Independent type/validation changes |
| B | 7a, 7b, 7c | Independent example functions |

| Task | Depends On | Reason |
|------|------------|--------|
| 3 | 2 | Runtime needs type definitions |
| 4 | 3 | Executor needs runtime |
| 5 | 2 | Discovery needs runtime types |
| 6 | 4 | Watcher uses executor patterns |
| 7 | 4, 5 | Examples need working runtime |
| 9 | 1-8 | Cleanup after all code changes |
| 10 | 9 | Final verification |

---

## TODOs

- [x] 1. Remove WASM Implementation

  **What to do**:
  - Delete `internal/functions/wasm.go` entirely
  - Delete `internal/functions/wasm_test.go` entirely
  - Remove `github.com/extism/go-sdk` from `go.mod`
  - Run `go mod tidy` to clean up dependencies

  **Must NOT do**:
  - Don't delete other files yet (they reference WASM, will be updated)

  **Parallelizable**: NO (must be first - other tasks reference these)

  **References**:
  - `internal/functions/wasm.go` - File to delete (300 lines)
  - `internal/functions/wasm_test.go` - File to delete
  - `go.mod:7` - Extism dependency line to remove

  **Acceptance Criteria**:
  - [ ] `wasm.go` deleted
  - [ ] `wasm_test.go` deleted  
  - [ ] `go mod tidy` completes without error
  - [ ] `grep -r "extism" . --include="*.go"` returns no results

  **Commit**: YES
  - Message: `refactor(functions): remove WASM/Extism implementation`
  - Files: `internal/functions/wasm.go`, `internal/functions/wasm_test.go`, `go.mod`, `go.sum`

---

- [x] 2. Update Runtime Type Definitions

  **What to do**:
  - Remove `RuntimeWasm` constant from `types.go`
  - Add new runtime constants: `RuntimeDeno`, `RuntimeBun`
  - Keep `RuntimeNode`, `RuntimePython`, `RuntimeGo`
  - Add `RuntimeConfig` struct for runtime metadata
  - Add `defaultRuntimes` map with command/args for each runtime

  **Must NOT do**:
  - Don't change FunctionRequest/FunctionResponse (keep API stable)
  - Don't add new fields to existing types

  **Parallelizable**: YES (with task 8)

  **References**:
  - `internal/functions/types.go:12-21` - Current Runtime constants
  - `internal/functions/types.go:23-33` - FunctionRequest (keep unchanged)

  **Acceptance Criteria**:
  - [ ] `RuntimeWasm` removed
  - [ ] `RuntimeDeno`, `RuntimeBun` added
  - [ ] `RuntimeConfig` struct defined with Command, Args, Extensions
  - [ ] `go build ./...` succeeds (may have errors until task 3-6 complete)

  **Commit**: NO (groups with 3)

---

- [x] 3. Implement Subprocess Runtime

  **What to do**:
  - Create `internal/functions/runtime.go`
  - Implement `SubprocessRuntime` struct with config
  - Implement `Call(ctx, name, entrypoint, input) (output, error)` method
  - Use `os/exec.CommandContext` for timeout support
  - Pipe input JSON to stdin, read output JSON from stdout
  - Capture stderr separately for logging
  - Handle non-zero exit codes as errors
  - Check runtime binary exists with clear error message

  **Must NOT do**:
  - No process pooling or caching
  - No streaming (single request/response)
  - No CGO or external dependencies

  **Parallelizable**: NO (depends on 2)

  **References**:
  - `internal/functions/types.go` - RuntimeConfig and types (after task 2)
  - Go stdlib: `os/exec`, `context`, `encoding/json`, `bytes`

  **Acceptance Criteria**:
  - [ ] `runtime.go` created with ~200 lines
  - [ ] `SubprocessRuntime.Call()` executes subprocess
  - [ ] JSON piped via stdin, read from stdout
  - [ ] Stderr captured separately
  - [ ] Context timeout works
  - [ ] Runtime not found returns clear error

  **Commit**: YES
  - Message: `feat(functions): implement subprocess runtime for polyglot functions`
  - Files: `internal/functions/types.go`, `internal/functions/runtime.go`

---

- [x] 4. Update Service/Executor to Use Subprocess Runtime

  **What to do**:
  - Rewrite `internal/functions/executor.go`
  - Replace `WASMRuntime` with `SubprocessRuntime`
  - Update `NewService()` to create subprocess runtime
  - Update `Start()` to skip WASM plugin loading
  - Update `Invoke()` to call subprocess runtime
  - Remove `wasmWatcher` field and references
  - Simplify `ReloadFunctions()` (no plugin reload needed)

  **Must NOT do**:
  - Don't change the Service interface methods
  - Don't change how functions are registered/discovered

  **Parallelizable**: NO (depends on 3)

  **References**:
  - `internal/functions/executor.go` - Current implementation using WASMRuntime
  - `internal/functions/runtime.go` - New SubprocessRuntime (from task 3)

  **Acceptance Criteria**:
  - [ ] `executor.go` uses SubprocessRuntime
  - [ ] No references to WASMRuntime, wasmWatcher
  - [ ] `NewService()` creates subprocess runtime
  - [ ] `Invoke()` calls subprocess runtime
  - [ ] `go build ./internal/functions/...` succeeds

  **Commit**: YES
  - Message: `refactor(functions): update executor to use subprocess runtime`
  - Files: `internal/functions/executor.go`

---

- [x] 5. Update Discovery for New Runtimes

  **What to do**:
  - Update `findEntryFile()` in `discovery.go`
  - Add Deno entry files: `mod.ts`, `main.ts`
  - Add Bun entry files: `index.tsx`
  - Remove `.wasm` file candidates
  - Update `detectRuntime()` for new extensions
  - Ensure runtime detection priority: manifest > file extension

  **Must NOT do**:
  - Don't change function directory structure expectations
  - Don't change manifest loading logic

  **Parallelizable**: NO (depends on 2)

  **References**:
  - `internal/functions/discovery.go:161-188` - `findEntryFile()` function
  - `internal/functions/discovery.go:269-281` - `detectRuntime()` function

  **Acceptance Criteria**:
  - [ ] Deno files detected (`.ts` → RuntimeDeno if deno.json exists)
  - [ ] Bun files detected (check for bunfig.toml or bun.lockb)
  - [ ] No `.wasm` candidates
  - [ ] `go test ./internal/functions/... -run TestDiscovery` passes

  **Commit**: YES
  - Message: `feat(functions): update discovery for Deno, Node, Bun, Python runtimes`
  - Files: `internal/functions/discovery.go`

---

- [x] 6. Simplify Watcher (Remove WASMWatcher)

  **What to do**:
  - Remove `WASMWatcher` struct and all its methods from `watcher.go`
  - Keep `SourceWatcher` (still useful for hot reload)
  - Update SourceWatcher to work without build step (optional)
  - If manifest has no `build` config, just watch entry file for changes

  **Must NOT do**:
  - Don't remove source watching entirely
  - Don't change debounce logic

  **Parallelizable**: NO (depends on 4)

  **References**:
  - `internal/functions/watcher.go:332-543` - WASMWatcher to remove
  - `internal/functions/watcher.go:24-330` - SourceWatcher to keep

  **Acceptance Criteria**:
  - [ ] `WASMWatcher` struct and methods removed
  - [ ] `SourceWatcher` still works
  - [ ] `watcher.go` reduced by ~200 lines
  - [ ] `go test ./internal/functions/... -run TestWatcher` passes

  **Commit**: YES
  - Message: `refactor(functions): remove WASMWatcher, simplify to source watching only`
  - Files: `internal/functions/watcher.go`, `internal/functions/watcher_test.go`

---

- [x] 7a. Create Deno Function Example

  **What to do**:
  - Create `examples/functions-demo/functions/hello-deno/`
  - Create `index.ts` using Deno's stdin/stdout
  - Create `manifest.yaml` with `runtime: deno`
  - No build step needed (Deno runs TS directly)

  **Must NOT do**:
  - Don't use npm packages (keep simple)
  - Don't require build tools

  **Parallelizable**: YES (with 7b, 7c)

  **References**:
  - Deno stdin: `Deno.stdin.readable`
  - Deno stdout: `console.log()` or `Deno.stdout`

  **Acceptance Criteria**:
  - [ ] `index.ts` reads JSON from stdin, writes JSON to stdout
  - [ ] `manifest.yaml` valid with `runtime: deno`
  - [ ] `deno run index.ts < input.json` works standalone
  - [ ] Function responds with greeting message

  **Commit**: NO (groups with 7b, 7c)

---

- [x] 7b. Create Node Function Example

  **What to do**:
  - Create `examples/functions-demo/functions/hello-node/`
  - Create `index.js` using Node's stdin/stdout
  - Create `manifest.yaml` with `runtime: node`
  - No build step needed

  **Must NOT do**:
  - Don't use external npm packages
  - Don't use TypeScript (keep simple)

  **Parallelizable**: YES (with 7a, 7c)

  **References**:
  - Node stdin: `process.stdin`, `readline`
  - Node stdout: `console.log()`, `process.stdout.write()`

  **Acceptance Criteria**:
  - [ ] `index.js` reads JSON from stdin, writes JSON to stdout
  - [ ] `manifest.yaml` valid with `runtime: node`
  - [ ] `node index.js < input.json` works standalone
  - [ ] Function responds with greeting message

  **Commit**: NO (groups with 7a, 7c)

---

- [x] 7c. Create Python Function Example

  **What to do**:
  - Create `examples/functions-demo/functions/hello-python/`
  - Create `index.py` using Python's stdin/stdout
  - Create `manifest.yaml` with `runtime: python`
  - No build step needed

  **Must NOT do**:
  - Don't use external pip packages
  - Don't use Python 2 syntax

  **Parallelizable**: YES (with 7a, 7b)

  **References**:
  - Python stdin: `sys.stdin`, `json.load()`
  - Python stdout: `print()`, `json.dumps()`

  **Acceptance Criteria**:
  - [ ] `index.py` reads JSON from stdin, writes JSON to stdout
  - [ ] `manifest.yaml` valid with `runtime: python`
  - [ ] `python3 index.py < input.json` works standalone
  - [ ] Function responds with greeting message

  **Commit**: YES
  - Message: `feat(examples): add Deno, Node, Python function examples`
  - Files: `examples/functions-demo/functions/hello-deno/*`, `examples/functions-demo/functions/hello-node/*`, `examples/functions-demo/functions/hello-python/*`

---

- [x] 8. Update Manifest Validation

  **What to do**:
  - Update `validateRuntime()` in `manifest.go`
  - Add `deno`, `bun` as valid runtimes
  - Remove `wasm` from valid runtimes
  - Update `BuildConfig` to be optional (not WASM-specific)

  **Must NOT do**:
  - Don't change manifest YAML structure
  - Don't remove build config entirely (still useful for TypeScript compilation)

  **Parallelizable**: YES (with task 2)

  **References**:
  - `internal/functions/manifest.go:283-296` - `validateRuntime()` function
  - `internal/functions/manifest.go:25-31` - BuildConfig struct

  **Acceptance Criteria**:
  - [ ] `deno`, `bun` are valid runtimes
  - [ ] `wasm` removed from valid runtimes
  - [ ] `go test ./internal/functions/... -run TestManifest` passes

  **Commit**: YES
  - Message: `feat(functions): update manifest validation for new runtimes`
  - Files: `internal/functions/manifest.go`, `internal/functions/manifest_test.go`

---

- [ ] 9. Cleanup Old WASM Examples and References

  **What to do**:
  - Delete `examples/wasm-demo/` directory
  - Delete `examples/functions/hello-go/` if it uses WASM PDK
  - Update `internal/functions/README.md` to document new runtime approach
  - Search for any remaining WASM references and remove

  **Must NOT do**:
  - Don't delete the new examples (task 7)
  - Don't remove build config from manifests (still useful)

  **Parallelizable**: NO (after all code changes)

  **References**:
  - `examples/wasm-demo/` - Directory to delete
  - `examples/functions/hello-go/` - Check if uses extism-pdk
  - `internal/functions/README.md` - Update documentation

  **Acceptance Criteria**:
  - [ ] `examples/wasm-demo/` deleted
  - [ ] No `extism` or `wasm` references in code (except migration docs)
  - [ ] README updated with subprocess runtime docs
  - [ ] `grep -r "wasm\|extism" --include="*.go" .` returns no results

  **Commit**: YES
  - Message: `chore: remove WASM examples and update documentation`
  - Files: `examples/wasm-demo/` (deleted), `internal/functions/README.md`

---

- [ ] 10. Final Verification and Tests

  **What to do**:
  - Run `make lint` - must pass with zero issues
  - Run `make test` - must pass all tests
  - Run `make build` - must compile
  - Start dev server and test all example functions
  - Verify hot reload still works

  **Must NOT do**:
  - Don't skip any verification step
  - Don't ignore lint warnings

  **Parallelizable**: NO (final task)

  **References**:
  - `Makefile` - Build and test commands
  - Example functions from task 7

  **Acceptance Criteria**:
  - [ ] `make lint` exits 0
  - [ ] `make test` exits 0 with all tests passing
  - [ ] `make build` produces binary
  - [ ] Deno function: `curl http://localhost:8090/api/functions/hello-deno` returns JSON
  - [ ] Node function: `curl http://localhost:8090/api/functions/hello-node` returns JSON
  - [ ] Python function: `curl http://localhost:8090/api/functions/hello-python` returns JSON
  - [ ] Source file change triggers function reload (hot reload works)

  **Commit**: NO (verification only)

---

## Commit Strategy

| After Task | Message | Files | Verification |
|------------|---------|-------|--------------|
| 1 | `refactor(functions): remove WASM/Extism implementation` | wasm.go, wasm_test.go, go.mod, go.sum | `go mod tidy` |
| 3 | `feat(functions): implement subprocess runtime for polyglot functions` | types.go, runtime.go | `go build ./internal/functions` |
| 4 | `refactor(functions): update executor to use subprocess runtime` | executor.go | `go build ./internal/functions` |
| 5 | `feat(functions): update discovery for Deno, Node, Bun, Python runtimes` | discovery.go | `go test -run TestDiscovery` |
| 6 | `refactor(functions): remove WASMWatcher, simplify to source watching only` | watcher.go, watcher_test.go | `go test -run TestWatcher` |
| 7 | `feat(examples): add Deno, Node, Python function examples` | examples/functions-demo/* | manual test |
| 8 | `feat(functions): update manifest validation for new runtimes` | manifest.go, manifest_test.go | `go test -run TestManifest` |
| 9 | `chore: remove WASM examples and update documentation` | examples/wasm-demo/, README.md | `grep -r wasm` |

---

## Success Criteria

### Verification Commands
```bash
# Build
make build  # Expected: binary at build/alyx

# Lint
make lint   # Expected: exit 0, no issues

# Test
make test   # Expected: exit 0, all pass

# Integration test
./build/alyx dev &
sleep 3
curl -s http://localhost:8090/api/functions/hello-deno | jq .
curl -s http://localhost:8090/api/functions/hello-node | jq .
curl -s http://localhost:8090/api/functions/hello-python | jq .
```

### Final Checklist
- [ ] All "Must Have" present (subprocess runtime, JSON protocol, 5 runtimes, timeout, stderr capture, hot reload)
- [ ] All "Must NOT Have" absent (no CGO, no pooling, no containers, no streaming, no API changes)
- [ ] All tests pass
- [ ] All examples work
- [ ] No WASM/Extism references remain
