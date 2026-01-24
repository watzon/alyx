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

**Current Status**: Phase 1 Complete ✅ (~19% to MVP)

| Phase | Status | Features |
|-------|--------|----------|
| **Phase 1: Foundation** | ✅ Complete | Schema system, database layer, REST API |
| **Phase 2: Realtime** | 🔜 Next | WebSocket subscriptions, live updates |
| **Phase 3: Authentication** | ⏳ Planned | JWT auth, OAuth, CEL-based access control |
| **Phase 4: Functions** | ⏳ Planned | Container-based serverless functions |
| **Phase 5: CLI & DX** | ⏳ Planned | Dev mode, code generation, deployment |
| **Phase 6: Polish** | ⏳ Planned | Admin UI, documentation, examples |

See [ROADMAP.md](ROADMAP.md) for detailed task breakdown.

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
