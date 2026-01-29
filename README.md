# Alyx

![Alyx Logo](./assets/branding/alyx-logo.png)

[![Go Version](https://img.shields.io/badge/go-1.24.5-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-TBD-blue.svg)](LICENSE)

> A portable, polyglot Backend-as-a-Service (BaaS) written in Go

Alyx is a modern BaaS that combines the simplicity of PocketBase with the power of polyglot serverless functions. Define your data schema in YAML, deploy as a single binary, and extend with functions in Go, Node.js, or Python.

## Table of Contents

- [Background](#background)
- [Install](#install)
- [Usage](#usage)
- [API](#api)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)

## Background

Backend-as-a-Service platforms have become increasingly complex, requiring extensive setup and cloud dependencies. Alyx aims to provide:

- **Portability**: Single Go binary with embedded SQLite (or optional Turso for distributed deployments)
- **Simplicity**: YAML-defined schemas with automatic migrations and REST API generation
- **Extensibility**: Polyglot serverless functions (Go, Node.js, Python) with hot reload
- **Developer Experience**: OpenAPI documentation, type-safe client generation, and built-in admin UI

Alyx is inspired by PocketBase's deployment simplicity while adding first-class support for custom serverless functions across multiple languages.

## Install

### Prerequisites

- Go 1.24.5 or later
- (Optional) [Air](https://github.com/air-verse/air) for hot reload during development

### From Source

```bash
git clone https://github.com/watzon/alyx.git
cd alyx
make build
```

The binary will be available at `./build/alyx`.

### Using Go Install

```bash
go install github.com/watzon/alyx/cmd/alyx@latest
```

### Using Docker

Pull the pre-built image:

```bash
docker pull ghcr.io/watzon/alyx:latest
```

Or use Docker Compose (recommended):

```bash
# Clone your project or create docker-compose.yml
docker-compose up -d
```

See [Docker Deployment](#docker-deployment) below for details.

## Usage

### Quick Start

1. **Initialize a new project**:

```bash
alyx init myproject
cd myproject
```

2. **Define your schema** (`schema.yaml`):

```yaml
collections:
  - name: users
    fields:
      - name: email
        type: string
        required: true
        unique: true
      - name: name
        type: string
        required: true
```

3. **Start the development server**:

```bash
alyx dev
```

The API will be available at `http://localhost:8080` with automatic OpenAPI documentation at `/docs`.

### Configuration

Alyx uses a `alyx.yaml` configuration file. Key settings:

```yaml
server:
  host: localhost
  port: 8080

database:
  path: ./data/alyx.db
  # Optional: Use Turso for distributed deployments
  # turso:
  #   url: libsql://your-database.turso.io
  #   token: your-auth-token
```

Environment variables override config values with the `ALYX_` prefix:

```bash
export ALYX_SERVER_PORT=3000
export ALYX_DATABASE_PATH=/var/lib/alyx/db.sqlite
```

### Development

```bash
# Hot reload development server (requires Air)
make dev

# Run tests with coverage
make test

# Lint code
make lint

# Format code
make fmt
```

### Admin UI Development

The admin UI is a SvelteKit application located in the `ui/` directory. It's embedded into the Go binary at build time.

**Production Build:**
```bash
make ui-build  # Builds UI and copies to internal/adminui/dist/
make build     # Go binary now includes the admin UI
```

**Development with Hot Reload:**

Run two terminals for full hot-reload development:

```bash
# Terminal 1: Start Vite dev server
make ui-dev

# Terminal 2: Start Go server with UI proxy
make dev-ui
```

This setup:
- Runs Vite dev server at `http://localhost:5173`
- Runs Go server at `http://localhost:8090`
- Proxies `/_admin/*` requests from Go to Vite
- Provides hot reload for both Go and frontend changes

The `ALYX_ADMIN_UI_DEV` environment variable controls the proxy:
```bash
# Manually set if needed
ALYX_ADMIN_UI_DEV=http://localhost:5173 ./build/alyx dev
```

### Docker Deployment

Alyx provides official Docker images for easy deployment:

**Using Docker Compose** (recommended):

```yaml
# docker-compose.yml
version: '3.8'

services:
  alyx:
    image: ghcr.io/watzon/alyx:latest
    ports:
      - "8090:8090"
    volumes:
      - ./schema.yaml:/app/schema.yaml:ro
      - ./alyx.yaml:/app/alyx.yaml:ro
      - ./functions:/app/functions:ro
      - alyx-data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock  # For function containers
    environment:
      - JWT_SECRET=${JWT_SECRET}
    restart: unless-stopped

volumes:
  alyx-data:
```

```bash
# Start the server
docker-compose up -d

# View logs
docker-compose logs -f alyx

# Stop the server
docker-compose down
```

**Using Docker CLI**:

```bash
docker run -d \
  --name alyx \
  -p 8090:8090 \
  -v $(pwd)/schema.yaml:/app/schema.yaml:ro \
  -v $(pwd)/alyx.yaml:/app/alyx.yaml:ro \
  -v $(pwd)/functions:/app/functions:ro \
  -v alyx-data:/app/data \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e JWT_SECRET=your-secret-here \
  ghcr.io/watzon/alyx:latest
```

**Important Notes**:
- Mount `/var/run/docker.sock` to enable function containers
- Use a volume for `/app/data` to persist the database
- Set `JWT_SECRET` environment variable in production
- Mount your project files as read-only (`:ro`)

### Creating Serverless Functions

Alyx supports functions in multiple languages:

**Go** (`functions/hello.go`):
```go
package main

import (
    "net/http"
    "github.com/watzon/alyx/pkg/runtime"
)

func Handle(w http.ResponseWriter, r *http.Request) {
    runtime.JSON(w, 200, map[string]string{"message": "Hello from Go!"})
}
```

**Node.js** (`functions/hello.js`):
```javascript
export default async function handler(req, res) {
    return res.json({ message: "Hello from Node!" });
}
```

**Python** (`functions/hello.py`):
```python
def handler(req, res):
    return res.json({"message": "Hello from Python!"})
```

Functions are automatically discovered and hot-reloaded during development.

### Event-Driven Architecture

Alyx provides a comprehensive event system for building reactive applications with database hooks, webhooks, and scheduled functions.

#### Database Hooks

Trigger functions automatically when database operations occur:

```yaml
# functions/on-user-created/manifest.yaml
name: on-user-created
runtime: node
hooks:
  - type: database
    source: users
    action: insert
    mode: async
```

**Supported actions**: `insert`, `update`, `delete`  
**Modes**:
- `async`: Non-blocking, queued execution (default)
- `sync`: Blocking execution with configurable timeout

**Example function**:
```javascript
// functions/on-user-created/index.js
export default async function handler(req, res) {
  const { document, collection, action } = req.input;
  
  // Send welcome email
  await sendEmail(document.email, 'Welcome!');
  
  return res.json({ success: true });
}
```

#### Webhooks

Receive and verify webhook requests from external services:

```yaml
# functions/stripe-webhook/manifest.yaml
name: stripe-webhook
runtime: node
hooks:
  - type: webhook
    verification:
      type: hmac-sha256
      header: X-Stripe-Signature
      secret: ${STRIPE_WEBHOOK_SECRET}
```

**Verification types**: `hmac-sha256`, `hmac-sha1`

**Example function**:
```javascript
// functions/stripe-webhook/index.js
export default async function handler(req, res) {
  const { body, verified, verification_error } = req.input;
  
  if (!verified) {
    return res.json({ error: verification_error }, 401);
  }
  
  const event = JSON.parse(body);
  
  if (event.type === 'charge.succeeded') {
    // Handle successful charge
  }
  
  return res.json({ received: true });
}
```

#### Scheduled Functions

Run functions on a schedule using cron expressions, intervals, or one-time execution:

```yaml
# functions/daily-cleanup/manifest.yaml
name: daily-cleanup
runtime: node
schedules:
  - name: cleanup-old-logs
    type: cron
    expression: "0 2 * * *"  # Daily at 2 AM
    timezone: America/New_York
    config:
      input:
        retention_days: 30
```

**Schedule types**:
- `cron`: Standard cron expressions (e.g., `"0 * * * *"` for hourly)
- `interval`: Duration strings (e.g., `"5m"`, `"1h"`, `"30s"`)
- `one_time`: RFC3339 timestamps (e.g., `"2026-01-25T15:00:00Z"`)

**Example function**:
```javascript
// functions/daily-cleanup/index.js
export default async function handler(req, res) {
  const { retention_days } = req.input;
  const cutoff = new Date();
  cutoff.setDate(cutoff.getDate() - retention_days);
  
  // Clean up old logs
  const deleted = await ctx.alyx.collections.logs.deleteMany({
    created_at: { $lt: cutoff }
  });
  
  return res.json({ deleted: deleted.count });
}
```

#### Execution Logging

All function executions are automatically logged with:
- Input/output data
- Execution duration
- Success/failure status
- Error messages and stack traces
- Trigger information (HTTP, webhook, database, schedule)

Query execution logs via API:
```bash
GET /api/executions?function_id=on-user-created&status=success
GET /api/executions?trigger_type=database&limit=50
```

#### TypeScript SDK

Generate a type-safe SDK with full event system support:

```bash
alyx generate sdk --output ./sdk
```

**Usage in functions**:
```typescript
import { getContext } from './sdk';

export default async function handler(req, res) {
  const { alyx, auth, env } = getContext();
  
  // Access database
  const users = await alyx.collections.users.list();
  
  // Invoke other functions
  const result = await alyx.functions.invoke('send-email', {
    to: auth.email,
    subject: 'Hello!'
  });
  
  // Publish custom events
  await alyx.events.publish({
    type: 'custom',
    source: 'my-app',
    action: 'user-action',
    payload: { user_id: auth.id }
  });
  
  return res.json({ success: true });
}
```

## API

### REST Endpoints

Alyx automatically generates REST endpoints based on your schema:

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/collections/:collection` | List documents with filtering, sorting, pagination |
| `GET` | `/api/collections/:collection/:id` | Get single document by ID |
| `POST` | `/api/collections/:collection` | Create new document |
| `PATCH` | `/api/collections/:collection/:id` | Update document |
| `DELETE` | `/api/collections/:collection/:id` | Delete document |

### Query Parameters

**Filtering**:
```bash
GET /api/collections/users?email=user@example.com
GET /api/collections/posts?author_id=123&status=published
```

**Sorting**:
```bash
GET /api/collections/users?sort=created_at           # ascending
GET /api/collections/users?sort=-created_at          # descending
```

**Pagination**:
```bash
GET /api/collections/users?page=1&perPage=20
```

### OpenAPI Documentation

Interactive API documentation is available at `/docs` when running the dev server.

### Client Libraries

Generate type-safe client libraries:

```bash
alyx generate client --lang typescript --output ./client
```

Supported languages:
- TypeScript/JavaScript
- Go
- Python

## Roadmap

Alyx is under active development following a phased approach:

**Current Status**: Phase 3 Complete ✅ (~50% to MVP)

| Phase | Status | Features |
|-------|--------|----------|
| **Phase 1: Foundation** | ✅ Complete | Schema system, database layer, REST API |
| **Phase 2: Realtime** | ✅ Complete | WebSocket subscriptions, live updates |
| **Phase 3: Authentication** | ✅ Complete | JWT auth, OAuth, CEL-based access control |
| **Phase 4: Functions** | ⏳ Planned | Container-based serverless functions |
| **Phase 5: CLI & DX** | ⏳ Planned | Dev mode, code generation, deployment |
| **Phase 6: Polish** | ⏳ Planned | Admin UI, documentation, examples |

See [ROADMAP.md](ROADMAP.md) for detailed task breakdown.

## Known Limitations

See [V1 Limitations](docs/v1-limitations.md) for current architectural constraints.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Setup

1. Fork and clone the repository
2. Install dependencies: `go mod download`
3. Run tests: `make test`
4. Make your changes following the [code style guidelines](AGENTS.md#code-style-guidelines)
5. Run linters: `make lint`
6. Submit a pull request

### Project Structure

```
cmd/alyx/           # Application entrypoint
internal/
  adminui/          # Embedded admin UI (go:embed)
    dist/           # Built SvelteKit app (copied from ui/build)
    adminui.go      # HTTP handler with dev proxy support
  cli/              # Cobra CLI commands
  config/           # Viper-based configuration
  database/         # SQLite database layer, query builders, CRUD
  schema/           # YAML schema parser, SQL generator, migrations
  server/           # HTTP server, router, middleware
    handlers/       # HTTP handlers and response helpers
  openapi/          # OpenAPI spec generation
  functions/        # Serverless function runtime
  auth/             # Authentication and authorization
  realtime/         # WebSocket real-time subscriptions
ui/                 # Admin UI source (SvelteKit + Svelte 5)
  src/
    routes/         # SvelteKit routes
    lib/            # Components, stores, API client
  build/            # Production build output
pkg/
  client/           # Client library code
runtimes/           # Language-specific function runtimes
  go/
  node/
  python/
templates/          # Code generation templates
```

See [AGENTS.md](AGENTS.md) for detailed development guidelines.

## License

[License TBD]

---

**Project Status**: Early Development

Alyx is under active development. APIs and features are subject to change.
