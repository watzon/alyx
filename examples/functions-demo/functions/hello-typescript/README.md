# Hello TypeScript Function

TypeScript function example with build configuration for production deployment.

## Overview

This function demonstrates how Alyx handles TypeScript functions with a build step:

- **Development Mode**: Runs `index.ts` directly using the Node runtime (no build needed)
- **Production Mode**: Runs the compiled `dist/index.js` output from the build step

## Development Mode

**Important**: Node.js cannot run TypeScript files directly. Even in development mode, you must build the function first:

```bash
# Install dependencies
npm install

# Build the function
npm run build

# Run in development mode
alyx dev
```

**Alternative**: For TypeScript without build steps, use the Deno runtime instead (see `hello-deno` example). Deno natively supports TypeScript and requires no compilation.

## Production Mode

In production mode, you must build the function first:

```bash
# Install dependencies
npm install

# Build the function (compiles TypeScript to JavaScript)
npm run build

# Run in production mode
ALYX_DEV_ENABLED=false alyx serve
```

The build step:
1. Compiles `index.ts` to `dist/index.js` using esbuild
2. Bundles all dependencies into a single file
3. Optimizes for production (minification, tree-shaking)

## Build Configuration

The `manifest.yaml` includes a build configuration:

```yaml
build:
  command: npm
  args: ["run", "build"]
  watch: ["*.ts"]
  output: dist/index.js
```

This tells Alyx:
- **Dev mode**: Run `index.ts` with Node interpreter (no build)
- **Production mode**: Run `dist/index.js` (compiled output)
- **Watch**: Rebuild when `*.ts` files change (optional, for build watchers)

## How It Works

### Development Mode Flow
1. User runs `npm install && npm run build` (required even for dev)
2. Alyx discovers function with `manifest.yaml`
3. Sees `runtime: node` and `build.output: dist/index.js`
4. In dev mode with build present, uses `dist/index.js` as entrypoint
5. Executes: `node dist/index.js` with JSON stdin/stdout protocol
6. Hot reload: watches `*.ts` files and rebuilds automatically (if watcher enabled)

### Production Mode Flow
1. User runs `npm install && npm run build`
2. esbuild compiles `index.ts` → `dist/index.js`
3. Alyx discovers function with `manifest.yaml`
4. In production mode, uses `dist/index.js` as entrypoint (build output)
5. Executes: `node dist/index.js` with JSON stdin/stdout protocol

## Testing

```bash
# Test the function
curl -X POST http://localhost:8090/api/functions/hello-typescript \
  -H "Content-Type: application/json" \
  -d '{"name": "World"}'
```

Expected response:
```json
{
  "request_id": "...",
  "success": true,
  "output": {
    "message": "Hello, World! (from TypeScript)",
    "timestamp": "2026-01-25T...",
    "runtime": "node",
    "language": "typescript",
    "version": "v20.x.x",
    "mode": "development"
  }
}
```

## Build Script Details

The `package.json` build script uses esbuild:

```json
{
  "scripts": {
    "build": "esbuild index.ts --bundle --platform=node --outfile=dist/index.js --format=esm"
  }
}
```

**esbuild options**:
- `--bundle`: Bundle all dependencies into a single file
- `--platform=node`: Target Node.js runtime
- `--outfile=dist/index.js`: Output path (matches `manifest.yaml`)
- `--format=esm`: Use ES modules format

## TypeScript Types

The function includes proper TypeScript types for the Alyx function protocol:

```typescript
interface FunctionRequest {
  request_id: string;
  function: string;
  input: Record<string, any>;
  context: {
    auth?: any;
    env?: Record<string, string>;
    alyx_url: string;
    internal_token: string;
  };
}

interface FunctionResponse {
  request_id: string;
  success: boolean;
  output?: any;
  error?: {
    code: string;
    message: string;
  };
}
```

These types provide IDE autocomplete and type checking during development.

## Dependencies

- **typescript**: TypeScript compiler (for type checking)
- **esbuild**: Fast JavaScript bundler and minifier
- **tsx**: TypeScript execution engine (for dev mode)
- **@types/node**: Node.js type definitions

All dependencies are dev dependencies since the compiled output has no runtime dependencies.

## File Structure

```
hello-typescript/
├── index.ts           # TypeScript source (dev mode entrypoint)
├── manifest.yaml      # Function manifest with build config
├── package.json       # Dependencies and build script
├── README.md          # This file
├── .gitignore         # Ignore node_modules and dist
└── dist/              # Build output (created by npm run build)
    └── index.js       # Compiled JavaScript (production entrypoint)
```

## Comparison with Examples

| Feature | hello-node | hello-typescript | hello-deno |
|---------|------------|------------------|------------|
| Language | JavaScript | TypeScript | TypeScript |
| Runtime | Node.js | Node.js | Deno |
| Build Step | No | Yes (required) | No |
| Dev Mode | Runs index.js | Runs dist/index.js | Runs index.ts |
| Prod Mode | Runs index.js | Runs dist/index.js | Runs index.ts |
| Type Safety | No | Yes | Yes |
| Dependencies | None | esbuild, typescript | None |

**When to use each**:
- **hello-node**: Simple JavaScript functions, no build complexity
- **hello-typescript**: Node.js with TypeScript, production builds with bundling
- **hello-deno**: TypeScript without build steps, fastest development iteration

## Next Steps

- Add more complex TypeScript features (generics, decorators)
- Add unit tests with Jest or Vitest
- Add linting with ESLint
- Add formatting with Prettier
- Add CI/CD pipeline for automated builds
