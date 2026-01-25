# Function TypeScript Type Generation

## Context

### Original Request
When the node executor is enabled, auto-generate a `.d.ts` file in development mode that gets put in the functions directory and can be referenced by function files. Support JSDoc comments in `.js` files to reference the types as well.

### Interview Summary
**Key Discussions**:
- File location: Per-function `alyx.d.ts` placed in each function directory (e.g., `functions/my_function/alyx.d.ts`)
- Schema awareness: Generate typed database collection methods based on `schema.yaml`
- Generation triggers: On dev server startup + schema changes + new function file detection
- JS type hints: Design for JSDoc `@type {import('./alyx').FunctionContext}` pattern
- tsconfig.json: Auto-generate per-function for full TypeScript IDE support
- Git strategy: Commit generated files (no .gitignore modifications)
- TS/JS parity: Same treatment for both JavaScript and TypeScript function directories

**Research Findings**:
- Existing codegen system at `internal/codegen/` generates TypeScript client SDKs
- Dev mode has `DevConfig.AutoGenerate` and `DevConfig.GenerateLanguages` already
- DevWatcher in `internal/cli/dev.go` watches functions directory with `handleFunctionChange` callback
- SDK at `runtimes/node/sdk/index.js` exposes: `defineFunction`, `executeFunction`, `createDbClient`, `createLogger`
- Function context includes: `auth`, `env`, `db` (proxy), `log` (logger), `alyx` (fetch client)

### Metis Review
**Identified Gaps** (addressed in guardrails and edge cases):
- Stale file cleanup on function removal: Add cleanup logic
- Nested function directories: Support `functions/foo/bar/index.js` pattern
- Generated file header: Mark files as auto-generated with warning not to edit
- Deterministic output: Same input = same output (sorted keys, stable generation)
- Error surfacing: Log errors via zerolog, don't crash dev server
- Config respect: Honor existing `AutoGenerate` config flag

---

## Work Objectives

### Core Objective
Auto-generate per-function TypeScript type definitions (`alyx.d.ts`) and tsconfig files (`tsconfig.json`) in development mode to provide LSP/editor hints for serverless function authors, including schema-aware database collection types.

### Concrete Deliverables
1. `internal/codegen/function_types.go` - New generator for per-function type files
2. Modified `internal/cli/dev.go` - Integration to trigger generation on startup and file changes
3. Modified `internal/functions/discovery.go` - Hook for getting discovered function paths
4. Per-function generated files: `alyx.d.ts`, `tsconfig.json`

### Definition of Done
- [ ] `alyx dev` generates `alyx.d.ts` and `tsconfig.json` in each function directory on startup
- [ ] Schema changes regenerate all function type files
- [ ] New function directories get type files when `index.js`/`index.ts` is detected
- [ ] Function deletion removes corresponding type files (cleanup)
- [ ] JSDoc `@type {import('./alyx').FunctionContext}` provides autocomplete in VS Code
- [ ] TypeScript functions can `import { defineFunction, FunctionContext } from './alyx'`
- [ ] `make test` passes with new tests
- [ ] `make lint` passes with no new issues

### Must Have
- Schema-aware `db` types with collection-specific methods
- FunctionContext interface matching runtime SDK
- AuthContext type for `context.auth`
- Logger type for `context.log`
- `defineFunction` generic helper type
- Generated file headers marking as auto-generated

### Must NOT Have (Guardrails)
- No generation in production mode (dev mode only)
- No changes to runtime Node.js SDK behavior
- No global/project-wide file writes (only within function directories)
- No new CLI commands (integrate with existing `alyx dev`)
- No TypeScript compilation or bundling
- No modifications to `.gitignore`
- No new config options beyond using existing `AutoGenerate` flag

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (`go test -v -race ./...`)
- **User wants tests**: YES (tests after implementation)
- **Framework**: Go testing with table-driven tests

### Test Approach
After implementation, add tests to `internal/codegen/function_types_test.go`:
- Test type generation for various schema configurations
- Test deterministic output (same schema = same output)
- Test edge cases (empty schema, nested functions, special characters)

---

## Task Flow

```
Task 1 (FunctionTypesGenerator) 
    ↓
Task 2 (Discovery integration)
    ↓
Task 3 (Dev mode integration) 
    ↓
Task 4 (Cleanup logic)
    ↓
Task 5 (Tests)
```

## Parallelization

| Task | Depends On | Reason |
|------|------------|--------|
| 1 | None | Core generator can be built standalone |
| 2 | 1 | Needs generator to exist |
| 3 | 1, 2 | Needs generator and discovery |
| 4 | 3 | Cleanup is part of dev integration |
| 5 | 1, 2, 3, 4 | Tests verify all components |

---

## TODOs

- [ ] 1. Create FunctionTypesGenerator

  **What to do**:
  - Create `internal/codegen/function_types.go`
  - Implement `FunctionTypesGenerator` struct with methods:
    - `Generate(s *schema.Schema, functionDirs []string) error` - main entry point
    - `generateAlyxDTS(s *schema.Schema) string` - generate alyx.d.ts content
    - `generateTSConfig() string` - generate tsconfig.json content
  - Generate types matching SDK runtime context:
    - `AuthContext` interface (id, email, role, verified, metadata)
    - `Logger` interface (debug, info, warn, error methods)
    - `AlyxClient` interface (url, token, fetch method)
    - `FunctionContext` interface (auth, env, db, log, alyx)
    - `defineFunction` generic function type
  - Generate schema-aware database types:
    - Collection interface with typed find/findOne/create/update/delete
    - Per-collection types from schema (User, Post, etc.)
    - CreateInput and UpdateInput types per collection
    - DbClient type as intersection of all collections
  - Include header comment: `// Generated by Alyx - DO NOT EDIT`

  **Must NOT do**:
  - Don't generate to a central location (per-function only)
  - Don't include any runtime code (types only)
  - Don't modify existing codegen files

  **Parallelizable**: NO (foundation task)

  **References**:

  **Pattern References** (existing code to follow):
  - `internal/codegen/typescript.go:53-73` - `generateTypes()` pattern for building type strings
  - `internal/codegen/typescript.go:75-111` - `generateCollectionInterface()` for collection type generation
  - `internal/codegen/typescript.go:113-164` - Input type generation pattern (CreateInput, UpdateInput)
  - `internal/codegen/generator.go:132-160` - Helper functions (toPascalCase, toCamelCase, splitWords)

  **API/Type References** (contracts to implement against):
  - `internal/schema/types.go` - Schema, Collection, Field types for reading schema
  - `runtimes/node/sdk/index.js:143-161` - FunctionContext shape (auth, env, db, log, alyx)
  - `runtimes/node/sdk/index.js:12-89` - DbClient proxy interface (find, findOne, create, update, delete)
  - `runtimes/node/sdk/index.js:96-112` - Logger interface (debug, info, warn, error)

  **Test References**:
  - `internal/codegen/typescript_test.go` (if exists) - Testing patterns for code generators

  **WHY Each Reference Matters**:
  - `typescript.go` shows how to build TypeScript type strings in Go, handle schema iteration, and generate deterministic output
  - `sdk/index.js` is the source of truth for what runtime types actually look like - types must match this
  - `generator.go` helpers ensure consistent naming conventions across all codegen

  **Acceptance Criteria**:

  **Manual Execution Verification:**
  - [ ] Create test schema and run generator manually:
    ```go
    // In test file or main
    gen := NewFunctionTypesGenerator()
    content := gen.generateAlyxDTS(testSchema)
    fmt.Println(content)
    ```
  - [ ] Verify output contains:
    - `export interface AuthContext { id: string; email: string; ... }`
    - `export interface FunctionContext { auth: AuthContext | null; ... }`
    - `export interface DbClient { [collectionName]: CollectionClient<Type>; ... }`
    - `export function defineFunction<TInput, TOutput>(...): FunctionDefinition`
  - [ ] Verify tsconfig.json output is valid JSON with:
    - `"strict": true`
    - `"moduleResolution": "node"` or `"bundler"`
    - `"types": []` (no ambient types)

  **Commit**: NO (groups with task 5)

---

- [ ] 2. Add function path discovery helper

  **What to do**:
  - Add method to `internal/functions/discovery.go` or create accessor in `Registry`:
    - `GetFunctionDirectories() []string` - returns absolute paths to all discovered function directories
  - The method should return directories that contain valid entry points (index.js, index.ts, etc.)
  - Support nested function directories (e.g., `functions/api/users/index.js`)

  **Must NOT do**:
  - Don't change function discovery logic itself
  - Don't modify how functions are loaded or executed

  **Parallelizable**: NO (depends on task 1 conceptually for interface)

  **References**:

  **Pattern References**:
  - `internal/functions/discovery.go:*` - Existing Registry struct and discovery logic
  - `internal/functions/discovery.go` - `findFunctionEntry()` pattern for locating entry files

  **API/Type References**:
  - `internal/functions/types.go` - FunctionInfo and related types

  **WHY Each Reference Matters**:
  - Discovery already knows which directories contain functions - we just need to expose that information
  - Understanding the existing patterns ensures the new method integrates cleanly

  **Acceptance Criteria**:

  **Manual Execution Verification:**
  - [ ] Call `registry.GetFunctionDirectories()` and verify:
    - Returns `[]string` of absolute paths
    - Each path contains a valid function entry file
    - Paths are deduplicated and sorted
  - [ ] Test with nested structure:
    ```
    functions/
      hello/index.js       → returns /abs/path/functions/hello
      api/users/index.ts   → returns /abs/path/functions/api/users
    ```

  **Commit**: NO (groups with task 5)

---

- [ ] 3. Integrate type generation with dev mode

  **What to do**:
  - Modify `internal/cli/dev.go` to call type generation:
    - On startup after function discovery
    - When `handleSchemaChange()` is called (after `regenerateClients`)
    - When `handleFunctionChange()` detects new function directories
  - Add new function `generateFunctionTypes(s *schema.Schema, functionsPath string, registry *functions.Registry)`
  - Only generate if `cfg.Functions.Enabled` is true
  - Use existing `cfg.Dev.AutoGenerate` flag to control whether this runs
  - Log generation events: `log.Info().Str("path", funcDir).Msg("Generated function types")`

  **Must NOT do**:
  - Don't generate in production mode
  - Don't block server startup on generation failures (log and continue)
  - Don't regenerate unchanged functions (check if schema changed)

  **Parallelizable**: NO (depends on tasks 1 and 2)

  **References**:

  **Pattern References**:
  - `internal/cli/dev.go:264-305` - `regenerateClients()` pattern for triggering codegen
  - `internal/cli/dev.go:189-212` - `setupDevWatcher()` for file watch integration
  - `internal/cli/dev.go:307-319` - `handleFunctionChange()` for function change events

  **API/Type References**:
  - `internal/config/config.go:296-303` - DevConfig struct (AutoGenerate, etc.)
  - `internal/server/server.go` - Server.FunctionsRegistry() accessor (if exists)

  **WHY Each Reference Matters**:
  - `regenerateClients()` is the existing pattern for triggering codegen on schema change - follow same pattern
  - `handleFunctionChange()` is where we hook into new function detection
  - DevConfig tells us whether auto-generation is enabled

  **Acceptance Criteria**:

  **Manual Execution Verification:**
  - [ ] Start dev server: `alyx dev`
  - [ ] Verify log output: `Generated function types` for each function
  - [ ] Check `functions/hello/alyx.d.ts` exists and contains types
  - [ ] Check `functions/hello/tsconfig.json` exists and is valid JSON
  - [ ] Modify `schema.yaml` (add a field)
  - [ ] Verify types regenerate (log message appears)
  - [ ] Create new function `functions/test/index.js`
  - [ ] Verify `functions/test/alyx.d.ts` is generated

  **Commit**: NO (groups with task 5)

---

- [ ] 4. Add cleanup logic for removed functions

  **What to do**:
  - When a function directory is removed, delete its `alyx.d.ts` and `tsconfig.json`
  - Implement in `handleFunctionChange()` when event type is DELETE
  - Only delete files that match generated file pattern (have the header comment)
  - Log cleanup: `log.Info().Str("path", funcDir).Msg("Cleaned up function types")`

  **Must NOT do**:
  - Don't delete files that weren't generated by Alyx (check header)
  - Don't delete the function directory itself
  - Don't crash on missing files (idempotent cleanup)

  **Parallelizable**: NO (depends on task 3)

  **References**:

  **Pattern References**:
  - `internal/cli/dev.go:307-319` - `handleFunctionChange()` and EventType handling
  - `internal/cli/watcher.go` (if exists) - EventType enum (Create, Modify, Delete)

  **WHY Each Reference Matters**:
  - Need to understand how delete events are surfaced to implement cleanup

  **Acceptance Criteria**:

  **Manual Execution Verification:**
  - [ ] Create function: `mkdir -p functions/temp && echo 'export default () => {}' > functions/temp/index.js`
  - [ ] Verify types generated: `ls functions/temp/alyx.d.ts`
  - [ ] Delete function: `rm -rf functions/temp`
  - [ ] Verify log: `Cleaned up function types`
  - [ ] Verify files gone (not orphaned)

  **Commit**: NO (groups with task 5)

---

- [ ] 5. Add tests for function types generator

  **What to do**:
  - Create `internal/codegen/function_types_test.go`
  - Test cases:
    - Basic schema with single collection generates correct types
    - Multiple collections generate all types
    - Empty schema generates base types (no collections)
    - Field types map correctly (string, int, bool, json, datetime)
    - Nullable fields generate `| null` union
    - References generate expanded types
    - tsconfig.json is valid JSON
    - Output is deterministic (same schema = same output)
  - Use table-driven tests following project patterns

  **Must NOT do**:
  - Don't test dev mode integration (that's integration testing)
  - Don't test file writing (test generation logic only)

  **Parallelizable**: NO (final task)

  **References**:

  **Test References**:
  - `internal/database/database_test.go` - Table-driven test patterns in this codebase
  - `internal/schema/parser_test.go` - Schema-related test patterns

  **WHY Each Reference Matters**:
  - Following existing test patterns ensures consistency and makes PR review easier

  **Acceptance Criteria**:

  **Test Execution:**
  - [ ] `go test -v ./internal/codegen/... -run TestFunctionTypes` → all PASS
  - [ ] `make test` → all tests pass (no regressions)
  - [ ] `make lint` → no new lint issues

  **Commit**: YES
  - Message: `feat(codegen): add per-function TypeScript type generation for dev mode`
  - Files: 
    - `internal/codegen/function_types.go`
    - `internal/codegen/function_types_test.go`
    - `internal/functions/discovery.go` (minor changes)
    - `internal/cli/dev.go` (integration)
  - Pre-commit: `make lint && make test`

---

## Commit Strategy

| After Task | Message | Files | Verification |
|------------|---------|-------|--------------|
| 5 | `feat(codegen): add per-function TypeScript type generation for dev mode` | function_types.go, function_types_test.go, discovery.go, dev.go | `make lint && make test` |

---

## Success Criteria

### Verification Commands
```bash
# Start dev server with functions
alyx dev

# Check generated files exist
ls functions/*/alyx.d.ts
ls functions/*/tsconfig.json

# Verify types content
head -20 functions/hello/alyx.d.ts
# Expected: // Generated by Alyx - DO NOT EDIT
# Expected: export interface AuthContext { ... }

# Run tests
make test  # Expected: PASS

# Run linter
make lint  # Expected: 0 issues
```

### Final Checklist
- [ ] All "Must Have" present (schema-aware types, FunctionContext, Logger, etc.)
- [ ] All "Must NOT Have" absent (no production generation, no runtime changes)
- [ ] All tests pass
- [ ] Lint clean
- [ ] VS Code shows autocomplete for `context.db.users.find()` in JS/TS functions
