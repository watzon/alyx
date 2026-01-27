# Production Build Support - Environment-Aware Function Execution

## Context

### Original Request
Extend the subprocess runtime to support compiled/bundled functions in production while maintaining interpreted execution in development mode. This enables:
- Fast iteration in development (no build step)
- Optimized performance in production (compiled binaries)
- Proper dependency management (npm, pip, go mod)

### Interview Summary
**Key Discussions**:
- Dependencies should work out of the box (already do since cmd.Dir is set)
- Development mode uses interpreters: `deno run`, `node`, `python3`, `go run`
- Production mode uses compiled artifacts from BuildConfig.Output
- BuildConfig already exists in manifest - we just need to use it

**Research Findings**:
- `DevConfig.Enabled` exists and is used to detect dev mode
- `BuildConfig` has Command, Args, Watch, Output fields
- FunctionDef has Path field that could point to source OR compiled output
- Service is initialized with FunctionsConfig which doesn't have dev mode info

### Metis Review
**Identified Gaps** (addressed):
- Need to pass dev mode flag to Service/Registry
- Need runtime selection logic based on dev mode and BuildConfig presence
- Need to handle cases: no build (always interpret), build (dev=interpret, prod=run output)
- Need to support compiled binaries as a "runtime" (just execute directly)

---

## Work Objectives

### Core Objective
Enable environment-aware function execution where development mode runs source files via interpreters, and production mode runs compiled/bundled artifacts when BuildConfig is present.

### Concrete Deliverables
- Updated `FunctionsConfig` with dev mode awareness
- Updated `ServiceConfig` to include dev mode flag
- Updated `FunctionDef` with separate source/output paths
- New `RuntimeBinary` constant for pre-compiled executables
- Updated `SubprocessRuntime.Call()` to handle binary execution
- Updated discovery to track both source and output paths
- Updated executor to choose entrypoint based on mode
- Example function with build configuration

### Definition of Done
- [x] `make lint` passes with zero issues
- [x] `make test` passes with all tests green
- [x] Dev mode runs source files via interpreter
- [x] Production mode runs BuildConfig.Output when present
- [x] Functions without BuildConfig work in both modes (interpreter)
- [x] Example Node.js function with TypeScript build

### Must Have
- Dev mode detection passed to function service
- BuildConfig.Output used as production entrypoint
- Fallback to source interpretation when no build config
- Support for compiled binaries (no interpreter needed)
- Hot reload only in dev mode

### Must NOT Have (Guardrails)
- No automatic build triggering (that's what the watcher does)
- No Docker/container support (subprocess only)
- No breaking changes to existing functions without BuildConfig
- No changes to JSON stdin/stdout protocol

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go testing)
- **User wants tests**: YES (tests after implementation)
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
# Build alyx
make build

# Dev mode - should use interpreter
cd examples/functions-demo
../../build/alyx dev

# Test function (should see "from Node.js" in output)
curl -X POST http://localhost:8090/api/functions/hello-node \
  -H "Content-Type: application/json" \
  -d '{"name": "World"}'
```

---

## Task Flow

```
1 (Update Config) → 2 (Update Types) → 3 (Update Discovery) → 4 (Update Runtime)
                                                                    ↓
                           7 (Example) ← 6 (Update Watcher) ← 5 (Update Executor)
                               ↓
                           8 (Verify)
```

## Parallelization

| Group | Tasks | Reason |
|-------|-------|--------|
| A | 1, 2 | Independent config/type changes |

| Task | Depends On | Reason |
|------|------------|--------|
| 3 | 2 | Discovery needs updated FunctionDef |
| 4 | 2 | Runtime needs RuntimeBinary constant |
| 5 | 3, 4 | Executor needs discovery and runtime |
| 6 | 5 | Watcher uses executor patterns |
| 7 | 5 | Example needs working runtime |
| 8 | 7 | Final verification |

---

## TODOs

- [x] 1. Update Configuration for Dev Mode Awareness

  **What to do**:
  - Update `ServiceConfig` in `executor.go` to include `DevMode bool`
  - Pass dev mode from CLI/server to function service
  - Update `NewService()` to store dev mode flag

  **Must NOT do**:
  - Don't change FunctionsConfig in config.go (use existing DevConfig)
  - Don't add new CLI flags (use existing dev command detection)

  **Parallelizable**: YES (with task 2)

  **References**:
  - `internal/functions/executor.go:14-22` - ServiceConfig struct
  - `internal/config/config.go:235-260` - FunctionsConfig struct
  - `internal/server/handlers/admin.go:762-764` - isDevMode() pattern

  **Acceptance Criteria**:
  - [ ] `ServiceConfig` has `DevMode bool` field
  - [ ] `Service` stores and exposes dev mode flag
  - [ ] `go build ./internal/functions/...` succeeds

  **Commit**: NO (groups with 2)

---

- [x] 2. Update Types for Build Output Tracking

  **What to do**:
  - Add `RuntimeBinary Runtime = "binary"` constant for compiled executables
  - Update `FunctionDef` to include `OutputPath string` field
  - Update `FunctionDef` to include `HasBuild bool` field
  - Add `GetEntrypoint(devMode bool) string` method to FunctionDef

  **Must NOT do**:
  - Don't change existing runtime constants
  - Don't modify RuntimeConfig for binary (it's just direct execution)

  **Parallelizable**: YES (with task 1)

  **References**:
  - `internal/functions/types.go:12-23` - Runtime constants
  - `internal/functions/discovery.go:19-41` - FunctionDef struct

  **Acceptance Criteria**:
  - [ ] `RuntimeBinary` constant added
  - [ ] `FunctionDef.OutputPath` field added
  - [ ] `FunctionDef.HasBuild` field added
  - [ ] `FunctionDef.GetEntrypoint(devMode)` returns correct path

  **Commit**: YES
  - Message: `feat(functions): add build output tracking for production mode`
  - Files: `internal/functions/types.go`, `internal/functions/discovery.go`

---

- [x] 3. Update Discovery to Track Build Output

  **What to do**:
  - When loading manifest with BuildConfig, set `OutputPath` to BuildConfig.Output
  - Set `HasBuild = true` when BuildConfig is present and valid
  - Resolve OutputPath relative to function directory
  - Check if output file exists (warning if not, not error)

  **Must NOT do**:
  - Don't require output file to exist (it might not be built yet)
  - Don't trigger builds during discovery

  **Parallelizable**: NO (depends on 2)

  **References**:
  - `internal/functions/discovery.go:82-160` - Discover() method
  - `internal/functions/manifest.go:25-31` - BuildConfig struct

  **Acceptance Criteria**:
  - [ ] Functions with BuildConfig have OutputPath set
  - [ ] Functions with BuildConfig have HasBuild = true
  - [ ] OutputPath is absolute path
  - [ ] `go test ./internal/functions/... -run TestDiscovery` passes

  **Commit**: YES
  - Message: `feat(functions): track build output path during discovery`
  - Files: `internal/functions/discovery.go`

---

- [x] 4. Update Runtime for Binary Execution

  **What to do**:
  - Add binary execution support to SubprocessRuntime or create separate handler
  - When runtime is `RuntimeBinary` or entrypoint is executable, run directly
  - No interpreter, no args - just execute the file with JSON stdin/stdout
  - Check file is executable before running

  **Must NOT do**:
  - Don't change JSON protocol
  - Don't add special handling for specific languages

  **Parallelizable**: NO (depends on 2)

  **References**:
  - `internal/functions/runtime.go` - SubprocessRuntime implementation
  - `internal/functions/types.go:32-58` - defaultRuntimes map

  **Acceptance Criteria**:
  - [ ] Binary files can be executed directly
  - [ ] JSON stdin/stdout protocol works for binaries
  - [ ] Clear error if binary doesn't exist or isn't executable
  - [ ] `go build ./internal/functions/...` succeeds

  **Commit**: YES
  - Message: `feat(functions): support direct binary execution for production mode`
  - Files: `internal/functions/runtime.go`

---

- [x] 5. Update Executor for Mode-Aware Entrypoint Selection

  **What to do**:
  - Update `Invoke()` to use `fn.GetEntrypoint(s.devMode)` instead of `fn.Path`
  - In dev mode: always use source path (fn.Path) with interpreter
  - In prod mode: use output path if HasBuild, otherwise source path
  - Select appropriate runtime: binary for compiled, normal for source

  **Must NOT do**:
  - Don't change function response format
  - Don't add mode to function request

  **Parallelizable**: NO (depends on 3, 4)

  **References**:
  - `internal/functions/executor.go:95-152` - Invoke() method
  - `internal/functions/executor.go:126-138` - runtime selection

  **Acceptance Criteria**:
  - [ ] Dev mode uses source path with interpreter
  - [ ] Prod mode uses output path when HasBuild
  - [ ] Prod mode falls back to source for functions without build
  - [ ] `go build ./internal/functions/...` succeeds

  **Commit**: YES
  - Message: `feat(functions): implement mode-aware entrypoint selection`
  - Files: `internal/functions/executor.go`

---

- [x] 6. Update Watcher for Dev-Only Hot Reload

  **What to do**:
  - Pass dev mode to SourceWatcher
  - Only start file watching in dev mode
  - In prod mode, skip watcher initialization entirely

  **Must NOT do**:
  - Don't remove watcher code (still needed for dev)
  - Don't change watch patterns

  **Parallelizable**: NO (depends on 5)

  **References**:
  - `internal/functions/watcher.go` - SourceWatcher implementation
  - `internal/functions/executor.go:67-71` - watcher initialization

  **Acceptance Criteria**:
  - [ ] Dev mode starts file watcher
  - [ ] Prod mode skips file watcher
  - [ ] No errors when watcher disabled
  - [ ] `go test ./internal/functions/... -run TestWatcher` passes

  **Commit**: YES
  - Message: `feat(functions): make hot reload dev-mode only`
  - Files: `internal/functions/watcher.go`, `internal/functions/executor.go`

---

- [x] 7. Create Example Function with Build Configuration

  **What to do**:
  - Create `examples/functions-demo/functions/hello-typescript/`
  - Create TypeScript source file with proper types
  - Create manifest.yaml with build config (using esbuild or tsc)
  - Create package.json with build script
  - Add README explaining the build workflow

  **Must NOT do**:
  - Don't include node_modules (add to .gitignore)
  - Don't pre-build (let users run npm install && npm run build)

  **Parallelizable**: NO (depends on 5)

  **References**:
  - `examples/functions-demo/functions/hello-node/` - existing example
  - `internal/functions/manifest.go:25-31` - BuildConfig structure

  **Acceptance Criteria**:
  - [ ] TypeScript source file compiles
  - [ ] manifest.yaml has valid build config
  - [ ] `npm run build` creates output file
  - [ ] Function works in both dev and prod mode

  **Commit**: YES
  - Message: `feat(examples): add TypeScript function with build configuration`
  - Files: `examples/functions-demo/functions/hello-typescript/*`

---

- [x] 8. Final Verification

  **What to do**:
  - Run `make lint` - must pass with zero issues
  - Run `make test` - must pass all tests
  - Run `make build` - must compile
  - Test dev mode: source files executed via interpreter
  - Test function with build config in both modes
  - Verify hot reload only works in dev mode

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
  - [ ] Dev mode runs interpreter
  - [ ] Prod mode runs compiled output
  - [ ] Hot reload only in dev mode

  **Commit**: NO (verification only)

---

## Commit Strategy

| After Task | Message | Files | Verification |
|------------|---------|-------|--------------|
| 2 | `feat(functions): add build output tracking for production mode` | types.go, discovery.go | `go build` |
| 3 | `feat(functions): track build output path during discovery` | discovery.go | `go test -run TestDiscovery` |
| 4 | `feat(functions): support direct binary execution for production mode` | runtime.go | `go build` |
| 5 | `feat(functions): implement mode-aware entrypoint selection` | executor.go | `go build` |
| 6 | `feat(functions): make hot reload dev-mode only` | watcher.go, executor.go | `go test -run TestWatcher` |
| 7 | `feat(examples): add TypeScript function with build configuration` | examples/* | manual test |

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

# Dev mode test
cd examples/functions-demo
../../build/alyx dev &
curl -s http://localhost:8090/api/functions/hello-typescript | jq .
# Expected: Uses interpreter, "from TypeScript/Node.js"

# Prod mode test (after building function)
cd examples/functions-demo/functions/hello-typescript
npm install && npm run build
cd ../..
ALYX_DEV_ENABLED=false ../../build/alyx serve &
curl -s http://localhost:8090/api/functions/hello-typescript | jq .
# Expected: Uses compiled output
```

### Final Checklist
- [x] All "Must Have" present (dev mode detection, build output usage, fallback to source, binary execution)
- [x] All "Must NOT Have" absent (no auto-build, no containers, no breaking changes)
- [x] All tests pass
- [x] All examples work
- [x] Hot reload only in dev mode
