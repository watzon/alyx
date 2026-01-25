# Draft: Function TypeScript Type Generation

## Requirements (confirmed)
- Auto-generate `.d.ts` file when node executor is enabled
- File should be placed in the functions directory
- Should only generate in development mode
- Types should provide LSP/editor hints for function authors
- Should also support JSDoc references in `.js` files

## Technical Discoveries

### Current Architecture
- Node executor runs in container (`runtimes/node/executor.js`)
- SDK located at `runtimes/node/sdk/index.js`
- Functions discovered from `functions/` directory (configurable via `config.Functions.Path`)
- Dev mode triggers via `alyx dev` command
- Existing codegen system in `internal/codegen/` for client SDKs
- Dev mode already has `AutoGenerate` config option for client SDKs

### Runtime API (from SDK)
The SDK exposes these types to user functions:
1. `defineFunction(config)` - function definition helper
2. `FunctionContext` - passed as second argument to handler
   - `auth: User | null`
   - `env: Record<string, string>`
   - `db: CollectionProxy` (dynamic - based on schema collections)
   - `log: Logger` (debug, info, warn, error)
   - `alyx: { url, token, fetch() }` - internal API client

### Example SDK (existing)
Located at `examples/full-stack-demo/sdk/` - shows TypeScript SDK structure:
- `context.d.ts` - FunctionContext interface
- `client.d.ts` - AlyxClient types
- `types/collections.d.ts` - Collection-specific types

## Research Findings
- [Finding]: Existing codegen generates client SDK (`types.ts`, `client.ts`, `index.ts`)
- [Finding]: Dev mode has `DevConfig.AutoGenerate` and `DevConfig.GenerateOutput` already
- [Finding]: Functions directory watched via `DevWatcher` for hot reload

## Decisions Made

### File Location (User Decision)
**Per-function `alyx.d.ts`**: Place type file inside each function directory that contains an `index.js` or `index.ts` file.
- Example: `./functions/my_function/alyx.d.ts`
- Rationale: Types are local to each function, easy to reference with relative imports
- Note: Will be regenerated for each function, so all functions have consistent types

### Schema-Aware DB Types (User Decision)
**Yes - typed collections**: Generate schema-aware database types.
- `db.users.find()` returns `User[]`
- `db.posts.create(data: PostCreateInput)` with proper typing
- Requires regeneration when schema changes

### Generation Triggers (User Decision)
**Startup + schema changes + new function detection**:
1. Generate on dev server start
2. Regenerate when `schema.yaml` changes
3. Generate for new functions when `index.js` or `index.ts` is detected

### JS Type Hints (User Decision)
**JSDoc @type imports**: Design types for JSDoc import syntax.
- Example: `/** @type {import('./alyx').FunctionContext} */`
- Works in both `.js` and `.ts` files

### tsconfig.json Generation (User Decision)
**Yes - generate tsconfig.json**: Auto-generate a `tsconfig.json` in each function directory.
- Provides full TypeScript/IDE support out of the box
- Should be configured for strict mode, proper module resolution

### Git Strategy (User Decision)
**Commit generated files**: Generated `alyx.d.ts` and `tsconfig.json` should be committed.
- Ensures CI/CD builds work without running dev mode
- Source of truth for function types
- No auto-modification of `.gitignore`

### TypeScript Function Handling (User Decision)
**Same treatment**: Generate `alyx.d.ts` for both JS and TS function directories.
- Consistent behavior regardless of entry file type
- TS functions can import types from the local `alyx.d.ts`

## Scope Boundaries
- INCLUDE:
  - Go code to generate per-function `alyx.d.ts` files
  - Go code to generate per-function `tsconfig.json` files
  - Schema-aware database collection types
  - Integration with dev mode (startup + schema changes + new function detection)
  - FunctionContext, Logger, AuthContext types
  - `defineFunction` helper type
  
- EXCLUDE:
  - Changes to the actual Node.js SDK (types only)
  - Production mode type generation (dev mode only)
  - Bundling or compilation of TypeScript functions

## Test Strategy (User Decision)
**Tests after implementation**: 
- Implement the type generator first
- Add tests to verify correct `.d.ts` output for various schemas
- Test infrastructure exists: `go test -v -race ./...`
