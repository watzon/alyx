# Hello WASM - Example Function

This example demonstrates how to create a WASM function for Alyx using JavaScript and npm packages.

## Features

- ✅ Uses npm packages (date-fns) in WASM
- ✅ Hot reload during development
- ✅ Type definitions for IDE support
- ✅ Automatic build on file changes

## Prerequisites

Before you begin, ensure you have the following installed:

1. **Node.js** (v18 or later)
   ```bash
   node --version
   ```

2. **extism-js CLI** (for compiling JavaScript to WASM)
   
   **Linux/macOS:**
   ```bash
   curl -O https://raw.githubusercontent.com/extism/js-pdk/main/install.sh
   bash install.sh
   ```
   
   **Windows:**
   ```powershell
   # Requires 7zip (https://www.7-zip.org/)
   # Run as Administrator
   powershell Invoke-WebRequest -Uri https://raw.githubusercontent.com/extism/js-pdk/main/install-windows.ps1 -OutFile install-windows.ps1
   powershell -executionpolicy bypass -File .\install-windows.ps1
   ```
   
   **Verify installation:**
   ```bash
   extism-js --version
   ```

3. **Alyx server** (running locally)
   ```bash
   # From the Alyx project root
   make build
   ./build/alyx dev
   ```

## Setup

### 1. Install Dependencies

Navigate to this directory and install npm packages:

```bash
cd examples/functions/hello-wasm
npm install
```

This will install:
- `date-fns` - Date formatting library (production dependency)
- `@extism/js-pdk` - TypeScript type definitions for Extism (development dependency)

### 2. Build the Function

Compile the JavaScript code to WASM:

```bash
npm run build
```

This runs `extism-js src/index.js -i src/index.d.ts -o plugin.wasm` and produces:
- `plugin.wasm` - The compiled WASM module

### 3. Start Alyx Server

If not already running, start the Alyx development server:

```bash
# From the Alyx project root
./build/alyx dev
```

The server will:
- Discover the `hello-wasm` function
- Load the `plugin.wasm` module
- Watch for changes to source files
- Automatically rebuild and reload on changes

## Usage

### Invoke the Function

Once the server is running, you can invoke the function via HTTP:

```bash
curl http://localhost:8090/api/functions/hello-wasm
```

**Response:**
```json
{
  "success": true,
  "message": "Hello, World! 🎉",
  "timestamp": {
    "formatted": "2026-01-25 14:30:00",
    "distance": "24 days ago",
    "relative": "01/01/2026",
    "iso": "2026-01-25T14:30:00.000Z"
  },
  "metadata": {
    "function": "hello-wasm",
    "request_id": "550e8400-e29b-41d4-a716-446655440000",
    "runtime": "wasm",
    "npm_package": "date-fns@3.0.0"
  },
  "example_usage": {
    "description": "You can pass data to this function via the input field",
    "example": {
      "name": "Alice"
    }
  }
}
```

### Pass Input Data

You can pass custom input to the function:

```bash
curl -X POST http://localhost:8090/api/functions/hello-wasm \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice"}'
```

**Response:**
```json
{
  "success": true,
  "message": "Hello, Alice! 🎉",
  ...
}
```

## Development Workflow

### Hot Reload

The Alyx server automatically watches for changes and rebuilds:

1. **Edit source files** (`src/index.js`)
2. **SourceWatcher detects change** (100ms debounce)
3. **Build runs automatically** (`npm run build`)
4. **WASMWatcher detects new .wasm** (200ms debounce)
5. **Function reloads** (hot reload)
6. **Total latency: ~300ms**

Try it:

```bash
# In one terminal: watch the Alyx logs
./build/alyx dev

# In another terminal: edit the function
echo '// trigger rebuild' >> src/index.js

# Watch the logs - you'll see:
# [INFO] Build succeeded: hello-wasm
# [INFO] Reloaded function: hello-wasm
```

### Manual Watch Mode

You can also use extism-js's built-in watch mode:

```bash
npm run watch
```

This watches for changes and rebuilds automatically (independent of Alyx).

## Project Structure

```
hello-wasm/
├── manifest.yaml       # Function configuration
├── package.json        # npm dependencies and scripts
├── src/
│   ├── index.js       # Function implementation
│   └── index.d.ts     # Type definitions
├── plugin.wasm        # Compiled WASM module (generated)
└── README.md          # This file
```

## Configuration

### manifest.yaml

The manifest defines how Alyx should handle this function:

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

**Key fields:**
- `runtime: wasm` - Use WASM runtime
- `build.command` - Build command to run
- `build.args` - Arguments to pass to command
- `build.watch` - Glob patterns to watch for changes
- `build.output` - Output WASM file path
- `timeout` - Maximum execution time
- `memory` - Memory limit for WASM module

### package.json

Standard npm package with build scripts:

```json
{
  "scripts": {
    "build": "extism-js src/index.js -i src/index.d.ts -o plugin.wasm",
    "watch": "extism-js src/index.js -i src/index.d.ts -o plugin.wasm --watch"
  }
}
```

## Accessing Alyx APIs

Your function can call Alyx APIs using the internal token:

```javascript
export function handle(req) {
  const { alyx_url, internal_token } = req.context;
  
  // Call Alyx API
  const response = await fetch(`${alyx_url}/api/collections/users`, {
    headers: {
      'Authorization': `Bearer ${internal_token}`
    }
  });
  
  const users = await response.json();
  return { users };
}
```

**Note:** The internal token is automatically generated with a 5-minute TTL.

## Troubleshooting

### Build Fails

**Error:** `extism-js: command not found`

**Solution:** The `extism-js` CLI is a separate tool (not an npm package). Install it using the installation script:

**Linux/macOS:**
```bash
curl -O https://raw.githubusercontent.com/extism/js-pdk/main/install.sh
bash install.sh
```

**Windows:**
```powershell
# Requires 7zip (https://www.7-zip.org/)
powershell Invoke-WebRequest -Uri https://raw.githubusercontent.com/extism/js-pdk/main/install-windows.ps1 -OutFile install-windows.ps1
powershell -executionpolicy bypass -File .\install-windows.ps1
```

Verify installation:
```bash
extism-js --version
```

### Function Not Found

**Error:** `404 Not Found` when calling the function

**Solution:** Ensure:
1. The `plugin.wasm` file exists (run `npm run build`)
2. The Alyx server is running
3. The function directory is in the `functions/` directory (or configured path)

### Hot Reload Not Working

**Issue:** Changes to source files don't trigger rebuild

**Solution:** Check:
1. The `build.watch` patterns in `manifest.yaml` match your files
2. The Alyx server logs show watcher started
3. File system events are working (try `touch src/index.js`)

### Memory Errors

**Error:** WASM module runs out of memory

**Solution:** Increase the `memory` limit in `manifest.yaml`:
```yaml
memory: 512mb  # or higher
```

## Next Steps

- **Add more npm packages** - Install any npm package and use it in your function
- **Add database hooks** - Trigger this function on database events
- **Add webhooks** - Receive webhook requests from external services
- **Add scheduled execution** - Run this function on a schedule
- **Call other functions** - Invoke other Alyx functions from this one

## Learn More

- [Extism JavaScript PDK](https://github.com/extism/js-pdk)
- [Alyx Documentation](../../README.md)
- [WASM Functions Guide](../../docs/functions.md)
