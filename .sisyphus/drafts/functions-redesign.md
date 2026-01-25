# Draft: Serverless Functions Redesign

## User's Stated Concerns
- **Complexity**: Multiple language runtimes (Node, Python, Go) adds complexity
- **Deployment**: Current implementation requires Docker
- **Opinionation**: Wants something more opinionated/constrained
- **Library Support**: MUST have access to language ecosystem (npm, etc.)

## Research Findings

### PocketBase Approach
- Uses **goja** (pure Go JS engine) for embedded JavaScript
- JS files placed in `pb_hooks` directory with `.pb.js` extension
- Supports `require()` with `goja_nodejs` modules (console, process, buffer)
- Can traverse `node_modules` directories
- Also allows Go SDK for embedding PocketBase as a library

### Goja/Sobek (JavaScript in Go)
- **Sobek** = maintained fork of goja by Grafana/k6 team
- ES6+ support (let/const, arrow functions, classes, promises, generators)
- **Experimental ESM support** (import/export)
- Pure Go - no CGO, single binary deployment
- `require()` can be configured to resolve node_modules
- Limitations: No WeakRef, no SharedArrayBuffer, no Atomics
- Performance: Fast for embedded, but not V8-level

### Gopher-Lua (Lua in Go)
- Mature, pure Go Lua 5.1 implementation
- NO LuaRocks support (no C FFI)
- Must implement libraries in Go and preload them
- Smaller ecosystem problem

### Extism (WASM Plugins)
- Universal plugin system using WebAssembly
- Write plugins in: Rust, Go, TypeScript/JavaScript, C#, Zig, etc.
- Plugins compiled to `.wasm` files
- **JS PDK**: Uses QuickJS, bundles npm dependencies at compile time
- Sandboxed execution, memory-safe
- Complexity: Requires WASM compilation toolchain

## Current Alyx Implementation
- Container-based (Docker required)
- Supports Node, Python, Go runtimes
- Pool manager with warm containers
- Rich manifest system (hooks, webhooks, schedules)

## Options Under Consideration

### Option A: Sobek/Goja (PocketBase-style)
**Pros:**
- Single binary, no Docker
- Pure Go, simple deployment
- Can support node_modules via custom resolver
- ES6+ with modules (experimental)

**Cons:**
- JavaScript only
- Not full Node.js API compatibility
- Performance limitations for CPU-heavy work

### Option B: Extism (WASM Plugins)
**Pros:**
- Polyglot (write in TS, Go, Rust, etc.)
- Sandboxed, secure
- npm dependencies bundled at compile time
- Single binary deployment

**Cons:**
- Requires compilation step for each function
- More complex development workflow
- Learning curve for users

### Option C: Hybrid (Sobek + WASM)
- Sobek for simple scripts/hooks
- Extism for complex functions needing full libraries

### Option D: Keep Container-Based (Optional)
- Make functions feature optional
- For users who need full runtimes, Docker is the escape hatch

## User Preferences (from interview)
- **Primary Goal**: Simplicity first, but ecosystem access is close second
- **Runtime Approach**: Interested in WASM (Extism)
- **Polyglot**: Starting with one language is fine; future polyglot is appealing
- **Key Concern**: Is WASM complexity worth it?

## Extism JS-PDK Deep Dive (CRITICAL FINDINGS)

### How It Works
1. Uses **QuickJS** compiled to WASM (not V8, not Node)
2. `extism-js` compiler: takes JS → bundles with QuickJS runtime → outputs `.wasm`
3. **Uses esbuild** to bundle npm dependencies at compile time
4. Result is a single `.wasm` file with all deps baked in

### Developer Workflow
```bash
# 1. Write plugin.js (CJS or ESM with bundler)
# 2. Write plugin.d.ts (declares exports)
# 3. Compile: extism-js plugin.js -i plugin.d.ts -o plugin.wasm
# 4. Run: extism call plugin.wasm myFunc --input="..."
```

### NPM Support (IMPORTANT)
- **Works via esbuild bundling** - npm packages bundled at compile time
- Must use esbuild with `format: "cjs"` and `target: "es2020"`
- Some Node APIs NOT available (no fs, no net, no crypto)
- Can polyfill some things with pure JS implementations
- HTTP available via `Http.request()` (synchronous)

### Limitations
- No ES Modules directly (must bundle to CJS)
- QuickJS only supports ES2020 (no newer features)
- No direct Node.js API (no fs, net, crypto)
- Requires separate compile step
- Requires Binaryen tools (`wasm-merge`, `wasm-opt`)

### What Alyx Would Need to Provide
1. **Go Host SDK integration** - embed Extism runtime
2. **Build tooling** - either:
   - Users run `extism-js` locally (simpler)
   - Alyx CLI wraps build process (better DX)
3. **Templates** - starter projects for each supported language
4. **Hot reload** - watch `.wasm` files for changes

## Comparison Summary

| Aspect | Sobek (Embedded JS) | Extism (WASM) |
|--------|---------------------|---------------|
| **Deployment** | Single binary | Single binary |
| **npm support** | Via custom resolver | Via esbuild bundling |
| **Node APIs** | Some via goja_nodejs | None (polyfills only) |
| **Build step** | None (interpret at runtime) | Required (compile to wasm) |
| **Performance** | Faster startup, slower exec | Slower startup, faster exec |
| **Polyglot** | JavaScript only | TS, Go, Rust, C#, Zig, etc. |
| **Isolation** | In-process | Sandboxed WASM |
| **Complexity** | Lower | Higher |

## Final Decisions (from interview)

### Build Process
**Manifest-defined build steps with two-watcher hot reload:**
1. User edits source file (e.g., `plugin.js`)
2. **Source watcher** detects change → runs manifest-defined build command
3. Build command produces `.wasm` file
4. **WASM watcher** detects `.wasm` change → reloads plugin in runtime

**Manifest would define build step per-language:**
```yaml
name: my-function
runtime: wasm
build:
  command: extism-js
  args: ["src/index.js", "-i", "src/index.d.ts", "-o", "plugin.wasm"]
  watch: ["src/**/*.js", "src/**/*.ts"]  # What triggers rebuild
```

### Hook System
**Everything is WASM** - uniform mental model. Database hooks, webhooks, scheduled functions - all compiled to `.wasm`.

### Node.js API Limitation
**Acceptable** - Functions use generated Alyx SDK for DB/auth/storage. Pure computation + HTTP covers most use cases. The SDK becomes the standard way for plugins to interact with Alyx.

## Architecture Summary

```
functions/
  my-function/
    manifest.yaml      # Defines build, triggers, config
    src/
      index.js         # Source code
      index.d.ts       # Type definitions
    plugin.wasm        # Compiled output (gitignored or committed)
```

**Hot Reload Flow:**
```
[Source files] → [Source watcher] → [Build command] → [.wasm file]
                                                            ↓
                                               [WASM watcher] → [Reload plugin]
```

## Scope Boundaries

### IN SCOPE
- WASM-based function runtime using Extism
- Manifest-defined build steps
- Two-watcher hot reload system
- JavaScript/TypeScript support via extism-js + esbuild
- Database hooks, webhooks, scheduled functions (all WASM)
- Generated SDK for plugins to call Alyx APIs

### OUT OF SCOPE (for now)
- Sobek/embedded JS (replaced by WASM)
- Container-based functions (removed entirely)
- Direct Node.js API access (use SDK instead)
- Template repository setup (future CLI work)

## Additional Decisions

### Existing Code
**Remove entirely** - Delete all container/Docker/pool code. Clean slate for WASM.

### Plugin ↔ Alyx Communication  
**HTTP only** - Plugins call Alyx REST API over localhost. Uses existing infrastructure, simple to reason about, already works with generated SDK.

### Templates
**CLI clones from external repo** - Future CLI command (`alyx functions new --lang=typescript`) will clone from a GitHub templates repository. Template repo setup is out of scope for this plan.
