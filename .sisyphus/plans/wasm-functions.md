# WASM-Based Serverless Functions

## Context

### Original Request
Replace the existing container-based serverless functions with a WASM-based approach using Extism. The goals are:
- Simpler deployment (single binary, no Docker required)
- Maintain library ecosystem access (npm via bundling)
- More opinionated, less complex than multi-runtime containers
- Future polyglot support (start with JS/TS, expand later)

### Interview Summary
**Key Discussions**:
- User chose Extism (WASM) over Sobek (embedded JS) for polyglot future potential
- Build process: manifest-defined build steps with two-watcher hot reload
- All hooks (database, webhooks, schedules) will use WASM - uniform system
- Plugin ↔ Alyx communication via HTTP only (uses generated Alyx SDK)
- Remove existing container-based code entirely
- Templates via external GitHub repo (out of scope for this plan)

**Research Findings**:
- Extism JS-PDK uses QuickJS + esbuild for bundling npm dependencies
- No Node.js APIs available (fs, net, crypto) - functions use Alyx SDK instead
- Extism Go SDK available for host integration
- Two-watcher system needed: source → build → .wasm → reload

### Metis Review
**Identified Gaps** (addressed in plan):
- Security constraints (memory limits, timeouts, sandbox) - defined in TODO 3
- Migration path for existing functions - documented in TODO 1
- Supported languages for v1 - scoped to JavaScript/TypeScript only
- Watcher debounce/backoff - addressed in TODO 6

---

## Work Objectives

### Core Objective
Replace container-based serverless functions with Extism WASM runtime, enabling single-binary deployment while maintaining library ecosystem access through build-time bundling.

### Concrete Deliverables
- New `internal/functions/` package with Extism-based WASM runtime
- Updated manifest schema with `build` configuration
- Two-watcher hot reload system (source → build, wasm → reload)
- Updated server integration
- Example JavaScript/TypeScript function with npm dependency
- Migration documentation

### Definition of Done
- [x] `make test` passes with 100% of new tests passing
- [x] `make lint` passes with zero issues  
- [x] Example function can be built and invoked successfully
- [x] Functions can call Alyx REST API from within WASM
- [x] Hot reload works: edit source → auto-build → auto-reload

### Must Have
- Extism Go SDK integration
- Manifest-defined build commands
- Source file watcher triggering builds
- WASM file watcher triggering reloads
- Security constraints (memory limits, execution timeouts)
- HTTP-based communication with Alyx server

### Must NOT Have (Guardrails)
- **No container/Docker code** - removed entirely
- **No Sobek/embedded JS** - WASM only
- **No direct Node.js API emulation** - use Alyx SDK
- **No template repository setup** - out of scope
- **No new CLI commands** - server-side only for now
- **No support for languages other than JS/TS in v1** - future scope

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (existing Go test infrastructure)
- **User wants tests**: Hybrid (Go tests for core, manual for E2E)
- **Framework**: Standard Go testing + manual verification

### Hybrid Approach
- **Go unit tests**: Manifest parsing, watcher logic, Extism runtime wrapper
- **Manual E2E**: Actual function compilation and execution

---

## Task Flow

```
1. Remove container code
       ↓
2. Add Extism dependency
       ↓
3. Core WASM runtime ──→ 4. Updated manifest schema
       ↓                         ↓
5. Source watcher ←───────────────┘
       ↓
6. WASM watcher + hot reload
       ↓
7. Server integration
       ↓
8. Example function + verification
```

## Parallelization

| Group | Tasks | Reason |
|-------|-------|--------|
| A | 3, 4 | Independent: runtime vs schema |

| Task | Depends On | Reason |
|------|------------|--------|
| 2 | 1 | Clean slate before new deps |
| 3 | 2 | Needs Extism SDK |
| 5 | 3, 4 | Needs runtime + manifest |
| 6 | 5 | Builds on source watcher |
| 7 | 6 | Needs complete watcher system |
| 8 | 7 | Needs full integration |

---

## TODOs

- [x] 1. Remove container-based function code

  **What to do**:
  - Delete container/Docker-related files from `internal/functions/`
  - Keep: `manifest.go`, `manifest_test.go`, `discovery.go`, `discovery_test.go`, `types.go`, `token.go`, `token_test.go`
  - Remove: `container.go`, `pool.go`, `executor.go` (container-specific parts)
  - Update `types.go` to remove container-specific types (ContainerState, Container, PoolConfig, ContainerManager)
  - Update references in server/handlers that use removed types
  - Document migration notes in code comments

  **Must NOT do**:
  - Don't remove manifest/discovery infrastructure - reuse it
  - Don't remove token store - still needed for internal API auth

  **Parallelizable**: NO (must complete before adding new code)

  **References**:
  
  **Files to examine**:
  - `internal/functions/container.go` - DELETE entirely (Docker container management)
  - `internal/functions/pool.go` - DELETE entirely (container pool management)
  - `internal/functions/executor.go` - REPLACE (keep Service interface, swap implementation)
  - `internal/functions/types.go:111-137` - Remove Container, ContainerState, PoolConfig, ContainerManager types
  
  **Dependent files** (update after removal):
  - `internal/server/server.go:88` - Creates function service (update construction)
  - `internal/server/handlers/functions.go` - Uses function service
  - `internal/server/handlers/health.go` - References funcService for health checks
  - `internal/webhooks/handler.go` - Uses function service for webhook execution

  **Acceptance Criteria**:
  - [ ] `internal/functions/container.go` deleted
  - [ ] `internal/functions/pool.go` deleted  
  - [ ] `types.go` no longer contains Container*, PoolConfig, ContainerManager
  - [ ] `make build` succeeds (may have errors, that's expected)
  - [ ] Document in `internal/functions/README.md`: "Container-based execution removed in favor of WASM"

  **Commit**: YES
  - Message: `refactor(functions): remove container-based execution infrastructure`
  - Files: `internal/functions/*.go`
  - Pre-commit: `go build ./...` (expect some errors from dependents)

---

- [x] 2. Add Extism Go SDK dependency

  **What to do**:
  - Add `github.com/extism/go-sdk` to go.mod
  - Run `go mod tidy`
  - Create minimal smoke test to verify SDK loads

  **Must NOT do**:
  - Don't implement full runtime yet - just add dependency

  **Parallelizable**: NO (depends on 1)

  **References**:
  
  **External documentation**:
  - Extism Go SDK: https://github.com/extism/go-sdk
  - Go SDK docs: https://extism.org/docs/integrate-into-your-codebase/go-host-sdk
  
  **Installation command**:
  ```bash
  go get github.com/extism/go-sdk
  ```

  **Acceptance Criteria**:
  - [ ] `go.mod` contains `github.com/extism/go-sdk`
  - [ ] `go mod tidy` completes without errors
  - [ ] Create `internal/functions/wasm_test.go` with basic SDK import test:
    ```go
    func TestExtismSDKLoads(t *testing.T) {
        // Just verify the SDK can be imported
        _ = extism.NewPlugin
    }
    ```
  - [ ] `go test ./internal/functions/... -run TestExtismSDKLoads` passes

  **Commit**: YES
  - Message: `deps: add extism go-sdk for WASM function runtime`
  - Files: `go.mod`, `go.sum`, `internal/functions/wasm_test.go`
  - Pre-commit: `go mod tidy && go test ./internal/functions/...`

---

- [x] 3. Implement core WASM runtime

  **What to do**:
  - Create `internal/functions/wasm.go` with Extism plugin management
  - Implement `WASMRuntime` struct with:
    - `LoadPlugin(wasmPath string) error` - load .wasm file
    - `UnloadPlugin(name string) error` - unload plugin
    - `Call(name string, function string, input []byte) ([]byte, error)` - invoke function
    - `Close() error` - cleanup all plugins
  - Implement security constraints:
    - Memory limit (configurable, default 256MB)
    - Execution timeout (configurable, default 30s)
    - WASI enabled for HTTP access
  - Wire up HTTP host functions for Alyx API access

  **Must NOT do**:
  - Don't implement file watching yet
  - Don't integrate with server yet
  - Don't add multiple language support

  **Parallelizable**: YES (with task 4)

  **References**:
  
  **Extism Go SDK patterns**:
  - Plugin creation: https://github.com/extism/go-sdk#creating-a-plugin
  - Host functions: https://github.com/extism/go-sdk#host-functions
  - WASI configuration: https://github.com/extism/go-sdk#wasi
  
  **Existing code to reference**:
  - `internal/functions/executor.go:114-191` - Current Invoke pattern (adapt for WASM)
  - `internal/functions/types.go:22-31` - FunctionRequest structure (reuse)
  - `internal/functions/types.go:55-68` - FunctionResponse structure (reuse)

  **Acceptance Criteria**:
  
  **Go unit tests**:
  - [ ] `TestWASMRuntime_LoadPlugin` - loads a valid .wasm file
  - [ ] `TestWASMRuntime_LoadPlugin_InvalidPath` - returns error for missing file
  - [ ] `TestWASMRuntime_Call` - invokes function and returns output
  - [ ] `TestWASMRuntime_Call_Timeout` - enforces execution timeout
  - [ ] `TestWASMRuntime_MemoryLimit` - enforces memory constraints
  - [ ] `go test ./internal/functions/... -run TestWASMRuntime` passes

  **Manual verification**:
  - [ ] Create test .wasm file (use prebuilt from Extism examples)
  - [ ] Verify `LoadPlugin` + `Call` works in isolation

  **Commit**: YES
  - Message: `feat(functions): implement Extism WASM runtime core`
  - Files: `internal/functions/wasm.go`, `internal/functions/wasm_test.go`
  - Pre-commit: `make test`

---

- [x] 4. Update manifest schema for WASM builds

  **What to do**:
  - Update `internal/functions/manifest.go` to add build configuration:
    ```go
    type BuildConfig struct {
        Command string   `yaml:"command"`      // e.g., "extism-js"
        Args    []string `yaml:"args"`         // e.g., ["src/index.js", "-o", "plugin.wasm"]
        Watch   []string `yaml:"watch"`        // e.g., ["src/**/*.js"]
        Output  string   `yaml:"output"`       // e.g., "plugin.wasm"
    }
    ```
  - Update `Manifest` struct to include `Build *BuildConfig`
  - Update `validateRuntime()` to accept "wasm" as valid runtime
  - Add `BuildConfig.Validate()` method
  - Update tests

  **Must NOT do**:
  - Don't remove existing manifest fields (backward compat during migration)
  - Don't implement build execution yet

  **Parallelizable**: YES (with task 3)

  **References**:
  
  **Existing manifest code**:
  - `internal/functions/manifest.go:12-22` - Current Manifest struct
  - `internal/functions/manifest.go:56-99` - Manifest.Validate()
  - `internal/functions/manifest.go:266-278` - validateRuntime()
  
  **New manifest example**:
  ```yaml
  name: my-function
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

  **Acceptance Criteria**:
  
  **Go unit tests**:
  - [ ] `TestManifest_BuildConfig_Valid` - valid build config passes validation
  - [ ] `TestManifest_BuildConfig_MissingCommand` - error when command missing
  - [ ] `TestManifest_BuildConfig_MissingOutput` - error when output missing
  - [ ] `TestManifest_Runtime_Wasm` - "wasm" is accepted as valid runtime
  - [ ] All existing manifest tests still pass
  - [ ] `go test ./internal/functions/... -run TestManifest` passes

  **Commit**: YES
  - Message: `feat(functions): add build configuration to manifest schema`
  - Files: `internal/functions/manifest.go`, `internal/functions/manifest_test.go`
  - Pre-commit: `make test`

---

- [x] 5. Implement source file watcher with build trigger

  **What to do**:
  - Create `internal/functions/watcher.go` with:
    - `SourceWatcher` struct that watches files matching `build.watch` patterns
    - On file change: execute `build.command` with `build.args`
    - Debounce rapid changes (100ms default)
    - Log build output (stdout/stderr)
    - Handle build failures gracefully (log error, don't crash)
  - Use `fsnotify` for file watching (already a common Go library)
  - Integrate with Registry to know which functions to watch

  **Must NOT do**:
  - Don't watch .wasm files yet (that's task 6)
  - Don't reload plugins yet (that's task 6)

  **Parallelizable**: NO (depends on 3 and 4)

  **References**:
  
  **fsnotify library**:
  - https://github.com/fsnotify/fsnotify
  - Basic usage: `watcher.Add(path)` then `range watcher.Events`
  
  **Existing discovery code**:
  - `internal/functions/discovery.go:84-133` - Discover() iterates function dirs
  - `internal/functions/discovery.go:150-156` - Loads manifest per function
  
  **Build execution pattern**:
  ```go
  cmd := exec.CommandContext(ctx, build.Command, build.Args...)
  cmd.Dir = functionDir
  output, err := cmd.CombinedOutput()
  ```

  **Acceptance Criteria**:
  
  **Go unit tests**:
  - [ ] `TestSourceWatcher_DetectsChanges` - detects file modification
  - [ ] `TestSourceWatcher_Debounce` - multiple rapid changes trigger single build
  - [ ] `TestSourceWatcher_BuildSuccess` - executes build command on change
  - [ ] `TestSourceWatcher_BuildFailure` - logs error but continues watching
  - [ ] `TestSourceWatcher_GlobPattern` - matches files per `watch` patterns
  - [ ] `go test ./internal/functions/... -run TestSourceWatcher` passes

  **Manual verification**:
  - [ ] Create test function with manifest build config
  - [ ] Start watcher, modify source file
  - [ ] Verify build command executes (check logs)

  **Commit**: YES
  - Message: `feat(functions): implement source file watcher with build trigger`
  - Files: `internal/functions/watcher.go`, `internal/functions/watcher_test.go`, `go.mod` (if adding fsnotify)
  - Pre-commit: `make test`

---

- [x] 6. Implement WASM watcher with hot reload

  **What to do**:
  - Extend `watcher.go` or create `internal/functions/reload.go`:
    - `WASMWatcher` watches `.wasm` files (output from build)
    - On .wasm change: trigger plugin reload in WASMRuntime
    - Debounce to handle rapid rebuilds (200ms)
  - Integrate with WASMRuntime:
    - `WASMRuntime.Reload(name string) error` - unload + reload plugin
  - Coordinate watchers:
    - Source change → Build → WASM change → Reload
    - Ensure build completes before WASM watcher triggers

  **Must NOT do**:
  - Don't restart entire server - just reload affected plugin
  - Don't watch source files here (that's task 5)

  **Parallelizable**: NO (depends on 5)

  **References**:
  
  **Reload pattern**:
  ```go
  func (r *WASMRuntime) Reload(name string) error {
      r.mu.Lock()
      defer r.mu.Unlock()
      
      if plugin, ok := r.plugins[name]; ok {
          plugin.Close()
      }
      
      return r.loadPluginLocked(name)
  }
  ```
  
  **Existing reload pattern**:
  - `internal/functions/discovery.go:380-382` - Registry.Reload() pattern

  **Acceptance Criteria**:
  
  **Go unit tests**:
  - [ ] `TestWASMWatcher_DetectsChanges` - detects .wasm file modification
  - [ ] `TestWASMWatcher_TriggersReload` - calls WASMRuntime.Reload on change
  - [ ] `TestWASMRuntime_Reload` - unloads old plugin, loads new
  - [ ] `TestWASMRuntime_Reload_WhileRunning` - handles reload during execution
  - [ ] `go test ./internal/functions/... -run TestWASMWatcher` passes

  **Manual verification**:
  - [ ] Start runtime with loaded plugin
  - [ ] Manually replace .wasm file
  - [ ] Verify plugin reloads (check logs)
  - [ ] Verify function still callable after reload

  **Commit**: YES
  - Message: `feat(functions): implement WASM file watcher with hot reload`
  - Files: `internal/functions/reload.go` or `internal/functions/watcher.go`, tests
  - Pre-commit: `make test`

---

- [x] 7. Update server integration

  **What to do**:
  - Update `internal/functions/executor.go`:
    - Replace `Service` to use `WASMRuntime` instead of container pools
    - Keep existing interface where possible
    - Update `Invoke()` to call WASM plugins
  - Update `internal/server/server.go`:
    - Initialize WASMRuntime instead of container pool manager
    - Start source + WASM watchers in dev mode
    - Wire up hot reload
  - Update handlers to work with new runtime
  - Ensure token store still works for internal API auth

  **Must NOT do**:
  - Don't change external API (function invocation endpoints)
  - Don't change webhook/hook interfaces

  **Parallelizable**: NO (depends on 6)

  **References**:
  
  **Current server integration**:
  - `internal/server/server.go:88-96` - Creates function service
  - `internal/server/server.go:187-189` - FuncService() accessor
  - `internal/server/router.go:176` - Uses funcSvc for routing
  
  **Handler integration**:
  - `internal/server/handlers/functions.go:19-156` - Function handlers
  - `internal/webhooks/handler.go:18-22` - Webhook handler uses func service

  **Acceptance Criteria**:
  
  **Go unit tests**:
  - [ ] Existing server tests still pass
  - [ ] `TestServer_FunctionInvoke` - can invoke WASM function via HTTP
  - [ ] `make test` passes

  **Manual verification**:
  - [ ] Start server with example WASM function
  - [ ] `curl -X POST http://localhost:8090/api/functions/example-func -d '{"input": "test"}'`
  - [ ] Response contains expected output
  - [ ] Modify function source, verify hot reload works

  **Commit**: YES
  - Message: `feat(functions): integrate WASM runtime with server`
  - Files: `internal/functions/executor.go`, `internal/server/server.go`, `internal/server/handlers/*.go`
  - Pre-commit: `make lint && make test`

---

- [x] 8. Create example function and verify E2E

  **What to do**:
  - Create `examples/functions/hello-wasm/`:
    - `manifest.yaml` with build config for extism-js
    - `src/index.js` - simple function using npm package
    - `src/index.d.ts` - type definitions
    - `package.json` with example npm dependency
    - `README.md` with setup instructions
  - Document:
    - How to install extism-js CLI
    - How to set up a new function
    - How build + watch works
    - How to call Alyx API from function

  **Must NOT do**:
  - Don't create templates repo (out of scope)
  - Don't add CLI scaffolding command (future work)

  **Parallelizable**: NO (depends on 7)

  **References**:
  
  **Extism JS-PDK example structure**:
  - https://github.com/extism/js-pdk#using-with-esbuild
  
  **Example manifest**:
  ```yaml
  name: hello-wasm
  runtime: wasm
  build:
    command: npm
    args: ["run", "build"]
    watch: ["src/**/*.js", "src/**/*.ts"]
    output: plugin.wasm
  timeout: 30s
  memory: 256mb
  ```

  **Acceptance Criteria**:
  
  **Manual E2E verification**:
  - [ ] `cd examples/functions/hello-wasm && npm install`
  - [ ] `npm run build` produces `plugin.wasm`
  - [ ] Start Alyx server
  - [ ] `curl http://localhost:8090/api/functions/hello-wasm` returns expected output
  - [ ] Modify `src/index.js`, watch triggers rebuild
  - [ ] Re-call function, get updated output (hot reload works)
  
  **Documentation**:
  - [ ] `examples/functions/hello-wasm/README.md` exists
  - [ ] README explains prerequisites (extism-js, node)
  - [ ] README shows how to build and test

  **Commit**: YES
  - Message: `docs(functions): add example WASM function with npm dependency`
  - Files: `examples/functions/hello-wasm/*`
  - Pre-commit: N/A (documentation/example)

---

## Commit Strategy

| After Task | Message | Files | Verification |
|------------|---------|-------|--------------|
| 1 | `refactor(functions): remove container-based execution infrastructure` | internal/functions/*.go | `go build ./...` |
| 2 | `deps: add extism go-sdk for WASM function runtime` | go.mod, go.sum | `go mod tidy` |
| 3 | `feat(functions): implement Extism WASM runtime core` | internal/functions/wasm*.go | `make test` |
| 4 | `feat(functions): add build configuration to manifest schema` | internal/functions/manifest*.go | `make test` |
| 5 | `feat(functions): implement source file watcher with build trigger` | internal/functions/watcher*.go | `make test` |
| 6 | `feat(functions): implement WASM file watcher with hot reload` | internal/functions/reload*.go | `make test` |
| 7 | `feat(functions): integrate WASM runtime with server` | internal/server/*.go, internal/functions/*.go | `make lint && make test` |
| 8 | `docs(functions): add example WASM function with npm dependency` | examples/functions/* | N/A |

---

## Success Criteria

### Verification Commands
```bash
# All tests pass
make test  # Expected: PASS

# No lint errors
make lint  # Expected: 0 issues

# Build succeeds
make build  # Expected: binary at build/alyx

# Example function works (manual)
cd examples/functions/hello-wasm
npm install && npm run build
# Start server in another terminal
curl http://localhost:8090/api/functions/hello-wasm
# Expected: {"output": "Hello from WASM!"}
```

### Final Checklist
- [x] All container/Docker code removed
- [x] Extism WASM runtime working
- [x] Manifest supports build configuration
- [x] Two-watcher hot reload working (source → build → wasm → reload)
- [x] Server integration complete
- [x] Example function documented and working
- [x] `make lint && make test` passes
