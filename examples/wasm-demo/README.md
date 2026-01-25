# WASM Functions Demo

A complete example project demonstrating Alyx's WASM-based serverless functions.

## What's Included

- ✅ Complete Alyx configuration (`alyx.yaml`)
- ✅ Simple database schema (`schema.yaml`)
- ✅ Working WASM function (`functions/hello-wasm/`)
- ✅ Pre-built WASM binary (no build tools required)

## Quick Start

### 1. Build Alyx (if not already built)

```bash
# From the project root
cd ../..
make build
cd examples/wasm-demo
```

### 2. Start the Server

```bash
../../build/alyx dev
```

The server will:
- Create the database at `./data/wasm-demo.db`
- Apply the schema (users collection)
- Discover and load the `hello-wasm` function
- Start watching for changes

### 3. Test the Function

```bash
curl http://localhost:8090/api/functions/hello-wasm
```

**Expected response:**
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
    "runtime": "wasm"
  }
}
```

### 4. Pass Input Data

```bash
curl -X POST http://localhost:8090/api/functions/hello-wasm \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice"}'
```

## Project Structure

```
wasm-demo/
├── alyx.yaml              # Server configuration
├── schema.yaml            # Database schema
├── data/                  # Database files (created on first run)
│   └── wasm-demo.db
└── functions/             # WASM functions directory
    └── hello-wasm/        # Example function
        ├── manifest.yaml  # Function configuration
        ├── plugin.wasm    # Pre-built WASM binary
        ├── src/           # Source code (for reference)
        │   ├── index.js
        │   └── index.d.ts
        └── README.md      # Function-specific docs
```

## Configuration Files

### alyx.yaml

Configures the Alyx server:
- **Server**: Runs on `localhost:8090`
- **Database**: SQLite at `./data/wasm-demo.db`
- **Functions**: Loaded from `./functions` directory
- **Dev Mode**: Auto-migration and hot reload enabled

### schema.yaml

Defines the database schema:
- **users** collection with `name`, `email`, `created_at` fields

## Hot Reload

The server automatically watches for changes:

1. **Edit source**: Modify `functions/hello-wasm/src/index.js`
2. **Auto-build**: Function rebuilds (if `extism-js` installed)
3. **Auto-reload**: WASM plugin reloads (~300ms)
4. **Test**: Call the function again to see changes

**Note**: The pre-built `plugin.wasm` works without `extism-js`. To rebuild from source, see `functions/hello-wasm/README.md`.

## Adding More Functions

### 1. Create Function Directory

```bash
mkdir -p functions/my-function/src
```

### 2. Create Manifest

```yaml
# functions/my-function/manifest.yaml
name: my-function
runtime: wasm
build:
  command: npm
  args: ["run", "build"]
  watch: ["src/**/*.js"]
  output: plugin.wasm
timeout: 30s
memory: 256mb
```

### 3. Add Source Code

```javascript
// functions/my-function/src/index.js
export function handle(req) {
  return {
    message: "Hello from my-function!",
    input: req.input
  };
}
```

### 4. Build and Test

```bash
cd functions/my-function
npm install
npm run build  # Requires extism-js
cd ../..

# Restart server or wait for hot reload
curl http://localhost:8090/api/functions/my-function
```

## Modifying the Schema

### 1. Edit schema.yaml

```yaml
collections:
  - name: users
    fields:
      - name: name
        type: string
        required: true
      - name: email
        type: string
        required: true
        unique: true
      - name: role
        type: string
        default: "user"  # New field
      - name: created_at
        type: datetime
        default: now
```

### 2. Restart Server

```bash
# Stop server (Ctrl+C)
../../build/alyx dev
```

The migration will apply automatically in dev mode.

## API Endpoints

### Functions

- `GET /api/functions` - List all functions
- `POST /api/functions/{name}` - Invoke function

### Collections

- `GET /api/collections/users` - List users
- `POST /api/collections/users` - Create user
- `GET /api/collections/users/{id}` - Get user
- `PATCH /api/collections/users/{id}` - Update user
- `DELETE /api/collections/users/{id}` - Delete user

### Documentation

- `GET /docs` - OpenAPI documentation (Scalar UI)

## Accessing Alyx APIs from Functions

Functions can call Alyx APIs using the internal token:

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

## Troubleshooting

### Server Won't Start

**Error**: `failed to open database`

**Solution**: Ensure the `data/` directory is writable:
```bash
mkdir -p data
chmod 755 data
```

### Function Not Found

**Error**: `404 Not Found`

**Solution**:
1. Check `functions/` directory exists
2. Verify `manifest.yaml` is valid
3. Ensure `plugin.wasm` exists
4. Restart server

### Hot Reload Not Working

**Issue**: Changes don't trigger rebuild

**Solution**:
1. Check `build.watch` patterns in `manifest.yaml`
2. Verify `extism-js` is installed (for rebuilding)
3. Check server logs for watcher errors

## Next Steps

- **Add database hooks**: Trigger functions on database events
- **Add webhooks**: Receive external webhook requests
- **Add scheduled functions**: Run functions on a schedule
- **Call other functions**: Invoke functions from within functions
- **Add authentication**: Protect endpoints with JWT

## Learn More

- [Alyx Documentation](../../README.md)
- [WASM Functions Guide](../../docs/functions.md)
- [Function Example](functions/hello-wasm/README.md)
- [Extism JavaScript PDK](https://github.com/extism/js-pdk)
