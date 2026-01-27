# Node.js Native TypeScript Support

## Context

### Original Request
Add native TypeScript support to the Node.js function runtime, leveraging Node.js v22/v23's native type-stripping capabilities.

### Interview Summary
**Key Discussions**:
- **Node.js version**: User chose Node 23 for native TypeScript support (no flags needed for basic type-stripping)
- **TypeScript features**: User wants full TypeScript support including `enum` and `namespace` via `--experimental-transform-types`
- **Testing**: User confirmed tests should be included

**Research Findings**:
- Node.js v23.6.0 has type-stripping unflagged and enabled by default
- `--experimental-transform-types` enables full TypeScript including enum/namespace
- Current entry file candidates do NOT include `.ts` extensions
- Watcher already recognizes `.ts` files (no changes needed there)

### Metis Review
**Identified Gaps** (addressed):
- Entry file precedence when both `.js` and `.ts` exist: **Resolved** - JS takes precedence (backward compatibility)
- Node version pinning: **Resolved** - Use `node:23-alpine` (latest stable Node 23)
- Mixed module types: **Resolved** - Follow existing `.mjs`/`.cjs` pattern for `.mts`/`.cts`

**Guardrails Applied**:
- No changes to user function API, config format, or CLI flags
- No transpilation tooling beyond native Node support
- Only update entry discovery; do not alter watcher or deploy pipeline
- Changes limited to Node runtime; do not affect Python/Go runtimes

---

## Work Objectives

### Core Objective
Enable TypeScript functions (`.ts`, `.mts`, `.cts`) to be discovered and executed natively in the Node.js function runtime using Node 23's built-in TypeScript support.

### Concrete Deliverables
- Updated `runtimes/node/Dockerfile` with Node 23 and transform-types flag
- Updated `runtimes/node/executor.js` to recognize TypeScript entry files
- Updated `internal/functions/discovery.go` to discover TypeScript entry files
- Tests verifying TypeScript function discovery and execution

### Definition of Done
- [x] `make lint` passes with zero issues
- [x] `make test` passes with all tests green
- [x] A TypeScript function with `index.ts` is discovered and can be invoked
- [x] TypeScript-specific features (enum, namespace) work correctly

### Must Have
- Support for `.ts`, `.mts`, `.cts` entry files
- Node 23 base image with `--experimental-transform-types` flag
- Backward compatibility: existing JS functions continue to work
- Tests for TypeScript file discovery

### Must NOT Have (Guardrails)
- No `tsconfig.json` support or path alias resolution
- No source maps or runtime type-checking
- No `.tsx` support
- No new transpilation tooling (ts-node, esbuild, etc.)
- No changes to function packaging or directory layout expectations
- No changes to Python or Go runtimes

---

## Verification Strategy (MANDATORY)

### Test Decision
- **Infrastructure exists**: YES (Go test framework via `make test`)
- **User wants tests**: YES (TDD for discovery, integration for execution)
- **Framework**: Go testing + Docker for runtime verification

### Test Coverage Required
1. **Discovery tests** (`internal/functions/discovery_test.go`):
   - Test that `.ts` files are discovered as RuntimeNode
   - Test entry file precedence: `.js` before `.ts`
   - Test `.mts` and `.cts` recognition

2. **Integration verification**:
   - Verify Docker image builds successfully
   - Verify TypeScript function executes in container

---

## Task Flow

```
Task 0 (Dockerfile) → Task 1 (executor.js) → Task 2 (discovery.go) → Task 3 (tests)
```

All tasks are sequential due to dependencies.

## Parallelization

| Task | Depends On | Reason |
|------|------------|--------|
| 0 | None | Base Docker changes |
| 1 | 0 | Executor changes need compatible image |
| 2 | None | Can be done in parallel with 0/1 but logical flow is sequential |
| 3 | 0, 1, 2 | Tests verify all changes together |

---

## TODOs

- [x] 0. Update Node.js Runtime Dockerfile

  **What to do**:
  - Change base image from `node:20-alpine` to `node:23-alpine`
  - Update CMD to include `--experimental-transform-types` flag
  - Update engines requirement in package.json to `>=23.0.0`

  **Must NOT do**:
  - Add any build steps or transpilation
  - Install additional npm packages

  **Parallelizable**: NO (foundational change)

  **References**:

  **Pattern References**:
  - `runtimes/node/Dockerfile:4` - Current base image declaration
  - `runtimes/node/Dockerfile:42` - Current CMD instruction
  - `runtimes/node/package.json:12` - Current engines constraint

  **Documentation References**:
  - Node.js TypeScript support: https://nodejs.org/api/typescript.html
  - `--experimental-transform-types` flag documentation

  **WHY Each Reference Matters**:
  - Dockerfile:4 shows where to change the base image tag
  - Dockerfile:42 shows where to add the transform-types flag
  - package.json:12 ensures engines field matches new Node version

  **Acceptance Criteria**:

  **Manual Execution Verification:**
  - [ ] Build Docker image: `docker build -t alyx-node-ts-test runtimes/node/`
  - [ ] Expected: Build completes successfully
  - [ ] Run container: `docker run --rm alyx-node-ts-test node --version`
  - [ ] Expected output contains: `v23.` (Node 23.x)

  **Commit**: YES
  - Message: `feat(runtime): upgrade Node runtime to v23 with TypeScript support`
  - Files: `runtimes/node/Dockerfile`, `runtimes/node/package.json`
  - Pre-commit: `docker build -t alyx-node-test runtimes/node/`

---

- [x] 1. Update Node.js Executor to Recognize TypeScript Files

  **What to do**:
  - Add `.ts`, `.mts`, `.cts` to the `entryFiles` array in `findFunctionEntry`
  - Add `.ts`, `.mts`, `.cts` to the `directFiles` patterns
  - Update `handleListFunctions` filter to include TypeScript extensions
  - Maintain precedence: `.js` before `.ts`, `.mjs` before `.mts`, `.cjs` before `.cts`

  **Must NOT do**:
  - Add any TypeScript compilation logic
  - Modify the function loading mechanism beyond file discovery
  - Change how modules are imported (Node handles it natively)

  **Parallelizable**: NO (depends on Task 0 for testing)

  **References**:

  **Pattern References**:
  - `runtimes/node/executor.js:56-78` - `findFunctionEntry` function with entry file candidates
  - `runtimes/node/executor.js:163-172` - `handleListFunctions` file filter

  **WHY Each Reference Matters**:
  - executor.js:56-78 shows the exact arrays to extend with TS extensions
  - executor.js:163-172 shows the filter that needs TS extensions added

  **Acceptance Criteria**:

  **Code Review Verification:**
  - [ ] `entryFiles` array includes: `"index.ts"`, `"index.mts"`, `"index.cts"`
  - [ ] `directFiles` array includes: `` `${name}.ts` ``, `` `${name}.mts` ``, `` `${name}.cts` ``
  - [ ] Precedence order: JS files listed before TS equivalents
  - [ ] `handleListFunctions` filter includes `.ts`, `.mts`, `.cts` extensions

  **Manual Execution Verification:**
  - [ ] Create test function: `mkdir -p /tmp/test-func && echo 'export default () => ({ message: "hello from ts" })' > /tmp/test-func/index.ts`
  - [ ] Start executor with FUNCTIONS_DIR=/tmp/test-func
  - [ ] Call `/functions` endpoint
  - [ ] Expected: `test-func` appears in list

  **Commit**: YES
  - Message: `feat(runtime): add TypeScript entry file discovery to Node executor`
  - Files: `runtimes/node/executor.js`
  - Pre-commit: N/A (manual verification)

---

- [x] 2. Update Go Discovery to Recognize TypeScript Entry Files

  **What to do**:
  - Add TypeScript entry file candidates to `findEntryFile` function in `discovery.go`
  - Add entries: `{"index.ts", RuntimeNode}`, `{"index.mts", RuntimeNode}`, `{"index.cts", RuntimeNode}`
  - Maintain precedence: JS files before TS files
  - Update `detectRuntime` function to handle `.ts`, `.mts`, `.cts` extensions

  **Must NOT do**:
  - Change the Runtime type or add new runtime variants
  - Modify manifest parsing or validation
  - Change how functions are registered

  **Parallelizable**: NO (tests depend on this)

  **References**:

  **Pattern References**:
  - `internal/functions/discovery.go:161-182` - `findEntryFile` function with candidates slice
  - `internal/functions/discovery.go:264-275` - `detectRuntime` function

  **WHY Each Reference Matters**:
  - discovery.go:161-182 shows the exact struct slice to extend
  - discovery.go:264-275 shows the extension-to-runtime mapping to update

  **Acceptance Criteria**:

  **Code Review Verification:**
  - [ ] `candidates` slice includes TypeScript entries with `RuntimeNode`
  - [ ] Order maintains JS precedence over TS
  - [ ] `detectRuntime` handles `.ts`, `.mts`, `.cts` returning `RuntimeNode`

  **Test Verification:**
  - [ ] `go test -v -run TestDiscovery ./internal/functions/...` passes
  - [ ] New test cases verify TypeScript discovery

  **Commit**: YES
  - Message: `feat(functions): add TypeScript entry file discovery`
  - Files: `internal/functions/discovery.go`
  - Pre-commit: `go test ./internal/functions/...`

---

- [x] 3. Add Tests for TypeScript Discovery and Execution

  **What to do**:
  - Add test cases to `discovery_test.go` for TypeScript file discovery
  - Test cases:
    1. Function with only `index.ts` is discovered as RuntimeNode
    2. Function with both `index.js` and `index.ts` - JS takes precedence
    3. Function with `index.mts` is discovered as RuntimeNode
    4. Function with `index.cts` is discovered as RuntimeNode
  - Update `detectRuntime` tests if they exist

  **Must NOT do**:
  - Add integration tests requiring Docker (keep unit tests fast)
  - Test actual TypeScript execution (that's runtime-level)
  - Add test dependencies

  **Parallelizable**: NO (depends on Task 2)

  **References**:

  **Pattern References**:
  - `internal/functions/discovery_test.go` - Existing discovery tests
  - `internal/functions/discovery.go:161-182` - Function under test

  **Test References**:
  - `internal/functions/discovery_test.go` - Follow existing test patterns and helper functions

  **WHY Each Reference Matters**:
  - discovery_test.go shows existing test patterns to follow
  - Ensures new tests are consistent with codebase style

  **Acceptance Criteria**:

  **Test Verification:**
  - [ ] `make test` passes with all tests green
  - [ ] `go test -v -run TestTypescript ./internal/functions/...` shows new tests passing
  - [ ] Test coverage includes:
    - `.ts` only discovery
    - `.ts` + `.js` precedence
    - `.mts` discovery
    - `.cts` discovery

  **Commit**: YES
  - Message: `test(functions): add TypeScript discovery tests`
  - Files: `internal/functions/discovery_test.go`
  - Pre-commit: `make test`

---

## Commit Strategy

| After Task | Message | Files | Verification |
|------------|---------|-------|--------------|
| 0 | `feat(runtime): upgrade Node runtime to v23 with TypeScript support` | Dockerfile, package.json | Docker build |
| 1 | `feat(runtime): add TypeScript entry file discovery to Node executor` | executor.js | Manual test |
| 2 | `feat(functions): add TypeScript entry file discovery` | discovery.go | go test |
| 3 | `test(functions): add TypeScript discovery tests` | discovery_test.go | make test |

---

## Success Criteria

### Verification Commands
```bash
make lint          # Expected: zero issues
make test          # Expected: all tests pass
docker build -t alyx-node-test runtimes/node/  # Expected: successful build
```

### Final Checklist
- [x] All "Must Have" present (TS discovery, Node 23, tests)
- [x] All "Must NOT Have" absent (no tsconfig, no transpilation tools)
- [x] All tests pass
- [x] Lint clean
