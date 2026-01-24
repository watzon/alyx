# Alyx - Comprehensive Planning Document

> **Alyx**: A portable, polyglot Backend-as-a-Service combining the single-binary simplicity of PocketBase with the real-time sync and developer experience of Convex.

**Version**: 1.0  
**Last Updated**: January 2026  
**Status**: Planning Phase

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Vision & Goals](#vision--goals)
3. [Architecture Overview](#architecture-overview)
4. [Technical Decisions](#technical-decisions)
5. [Core Components](#core-components)
6. [Schema System](#schema-system)
7. [Function Runtime](#function-runtime)
8. [Realtime Engine](#realtime-engine)
9. [Authentication & Authorization](#authentication--authorization)
10. [CLI & Developer Experience](#cli--developer-experience)
11. [Client SDK Generation](#client-sdk-generation)
12. [Repository Structure](#repository-structure)
13. [Phased Implementation Plan](#phased-implementation-plan)
14. [Risk Assessment & Mitigation](#risk-assessment--mitigation)
15. [Future Improvements (Post-MVP)](#future-improvements-post-mvp)
16. [Open Questions](#open-questions)
17. [Appendices](#appendices)

---

## Executive Summary

Alyx is a Backend-as-a-Service (BaaS) platform that provides:

- **Single Go binary** deployment (like PocketBase)
- **SQLite database** with optional Turso for distributed deployments
- **YAML-defined schema** with automatic migrations and type-safe client generation
- **Real-time subscriptions** via WebSocket with efficient change detection
- **Container-based serverless functions** supporting multiple languages
- **CEL-based access control** for fine-grained security rules
- **Hot-sync development** where schema and function changes are reflected instantly

**Target Users**: Indie developers, startups, and small teams who want a self-hostable, batteries-included backend without managing multiple services.

**Key Differentiators**:

- Unlike PocketBase: Real-time sync, schema-as-code, typed client generation
- Unlike Convex: Self-hostable, language-agnostic, no vendor lock-in
- Unlike Supabase: Single binary, no Postgres dependency, simpler ops

---

## Vision & Goals

### Primary Goals (MVP)

1. **Zero-config deployment**: Single binary + SQLite file = running backend
2. **Schema-as-code**: Define your data model in YAML, get migrations and typed clients
3. **Real-time by default**: Every query can become a live subscription
4. **Polyglot functions**: Write serverless functions in Node.js, Python, or Go
5. **Developer-first**: `alyx dev` provides instant feedback loop

### Non-Goals (MVP)

- Multi-tenancy / SaaS platform features
- Horizontal scaling / clustering
- Custom domain routing
- Built-in email/SMS services
- GraphQL API (REST-ish only for MVP)

### Success Metrics

- Time from `git clone` to running "Hello World": < 5 minutes
- Cold start for function execution: < 500ms
- Real-time update latency: < 100ms (p95)
- Schema change to client regeneration: < 2 seconds

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              Developer Workstation                              │
│                                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐│
│  │schema.yaml  │  │ functions/  │  │ alyx.yaml   │  │      alyx CLI           ││
│  │             │  │  *.js/*.py  │  │   (config)  │  │  • dev (watch mode)     ││
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  │  • generate (codegen)   ││
│         │                │                │         │  • migrate              ││
│         └────────────────┴────────────────┘         │  • deploy               ││
│                          │                          └───────────┬─────────────┘│
│                          │ File watching + sync                 │              │
│                          ▼                                      │              │
│  ┌──────────────────────────────────────────────────────────────┘              │
│  │                                                                             │
│  │  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │  │                        Generated Clients                            │   │
│  │  │  • generated/client.ts (TypeScript)                                 │   │
│  │  │  • generated/client.go (Go)                                         │   │
│  │  │  • generated/client.py (Python)                                     │   │
│  │  └─────────────────────────────────────────────────────────────────────┘   │
│  │                                                                             │
└──┼─────────────────────────────────────────────────────────────────────────────┘
   │
   │ HTTP/WebSocket
   ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              Alyx Server (Go Binary)                            │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                           HTTP/WebSocket Layer                          │   │
│  │  • REST API for collections                                             │   │
│  │  • WebSocket for real-time subscriptions                                │   │
│  │  • Function invocation endpoint                                         │   │
│  │  • Admin API                                                            │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                      │                                          │
│         ┌────────────────────────────┼────────────────────────────┐            │
│         │                            │                            │            │
│         ▼                            ▼                            ▼            │
│  ┌─────────────┐            ┌─────────────┐              ┌─────────────┐       │
│  │   Schema    │            │  Realtime   │              │  Function   │       │
│  │   Engine    │            │   Engine    │              │  Executor   │       │
│  │             │            │             │              │             │       │
│  │ • Parser    │            │ • Broker    │              │ • Pool Mgr  │       │
│  │ • Differ    │            │ • Sub Index │              │ • Container │       │
│  │ • Migrator  │            │ • Fan-out   │              │   Manager   │       │
│  └──────┬──────┘            └──────┬──────┘              └──────┬──────┘       │
│         │                          │                            │              │
│         │                          │                            │              │
│         ▼                          ▼                            ▼              │
│  ┌─────────────────────────────────────────────┐    ┌─────────────────────┐   │
│  │              SQLite Database                │    │  Container Runtime  │   │
│  │                                             │    │  (Docker/Podman)    │   │
│  │  • Collections (user-defined tables)        │    │                     │   │
│  │  • _alyx_migrations (migration history)     │    │  ┌───────────────┐  │   │
│  │  • _alyx_changes (change feed)              │    │  │ Node.js Pool  │  │   │
│  │  • _alyx_auth (users, sessions)             │    │  ├───────────────┤  │   │
│  │                                             │    │  │ Python Pool   │  │   │
│  │  + Triggers for change detection            │    │  ├───────────────┤  │   │
│  └─────────────────────────────────────────────┘    │  │ Go Pool       │  │   │
│                                                      │  └───────────────┘  │   │
│  ┌──────────────┐  ┌──────────────┐                 └─────────────────────┘   │
│  │     Auth     │  │    Rules     │                                           │
│  │   Provider   │  │   Engine     │                                           │
│  │              │  │    (CEL)     │                                           │
│  │ • JWT        │  │              │                                           │
│  │ • OAuth      │  │ • Compile    │                                           │
│  │ • Sessions   │  │ • Evaluate   │                                           │
│  └──────────────┘  └──────────────┘                                           │
│                                                                                 │
│  ┌─────────────────────────────────────────────────────────────────────────┐   │
│  │                         Embedded Admin UI                               │   │
│  │                      (Svelte SPA via go:embed)                          │   │
│  └─────────────────────────────────────────────────────────────────────────┘   │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Technical Decisions

### Decision Log

| Decision                    | Choice                           | Rationale                                                               | Alternatives Considered                                                        |
| --------------------------- | -------------------------------- | ----------------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| **Server Language**         | Go                               | Single binary, excellent concurrency, mature ecosystem, no runtime deps | Rust (steeper learning curve), Crystal (smaller ecosystem)                     |
| **Database**                | SQLite (modernc.org/sqlite)      | Zero-config, embedded, sufficient for target use case, pure Go driver   | PostgreSQL (operational complexity), embedded Postgres (large binary)          |
| **Function Runtime**        | Container-based (Docker/Podman)  | Language-agnostic, battle-tested isolation, standard tooling            | WASM (complexity, debugging issues), embedded interpreters (limited languages) |
| **Realtime Transport**      | WebSocket                        | Bidirectional, subscription multiplexing, better mobile support         | SSE (simpler but unidirectional), gRPC-Web (browser support issues)            |
| **Access Control**          | CEL (Common Expression Language) | Google-backed, type-safe, good Go library, readable syntax              | Rego/OPA (steeper learning curve), custom DSL (maintenance burden)             |
| **Schema Format**           | YAML                             | Language-agnostic, human-readable, easy to parse for codegen            | TypeScript (requires parser), Protobuf (verbose), JSON (less readable)         |
| **Container Communication** | HTTP                             | Simple, debuggable, uses same API as clients                            | Unix sockets (complexity), gRPC (overkill for MVP)                             |

### Key Architectural Decisions (from Oracle Review)

1. **Subscription Indexing**: Pre-parse subscription filters into normalized AST, group by collection + indexed fields to avoid O(N) matching per change.

2. **Migration Policy**: Auto-apply only additive schema changes; destructive changes (column removal, type changes) require explicit migration files.

3. **Function Transactions**: Provide explicit transaction API via HTTP callbacks; document atomicity guarantees clearly.

4. **CEL Caching**: Compile CEL programs once per rule, cache for lifetime of server process.

5. **Host API Versioning**: Version all internal APIs from day one (e.g., `/internal/v1/db/query`).

---

## Core Components

### 1. HTTP/WebSocket Server

**Technology**: `net/http` (stdlib) + `nhooyr.io/websocket`

**Endpoints**:

```
# Collections API (REST-ish)
GET    /api/collections/:name              # List/query documents
POST   /api/collections/:name              # Create document
GET    /api/collections/:name/:id          # Get single document
PATCH  /api/collections/:name/:id          # Update document
DELETE /api/collections/:name/:id          # Delete document

# Functions API
POST   /api/functions/:name                # Invoke function

# Auth API
POST   /api/auth/register                  # Create account
POST   /api/auth/login                     # Get tokens
POST   /api/auth/refresh                   # Refresh access token
POST   /api/auth/logout                    # Invalidate session
GET    /api/auth/providers                 # List OAuth providers
GET    /api/auth/oauth/:provider           # OAuth redirect
GET    /api/auth/oauth/:provider/callback  # OAuth callback

# Realtime API
GET    /api/realtime                       # WebSocket upgrade

# Admin API (requires admin auth)
GET    /api/admin/schema                   # Current schema
POST   /api/admin/schema                   # Update schema
GET    /api/admin/migrations               # Migration history
POST   /api/admin/migrate                  # Apply migrations
GET    /api/admin/functions                # List functions
GET    /api/admin/stats                    # Server statistics

# Internal API (function callbacks only)
POST   /internal/v1/db/query               # Execute query
POST   /internal/v1/db/exec                # Execute mutation
POST   /internal/v1/db/tx                  # Transaction operations

# Static
GET    /admin/*                            # Admin UI (embedded SPA)
GET    /                                   # Health check / info
```

**Query Syntax** (via query parameters):

```
GET /api/collections/posts?filter=author_id:eq:123&filter=published:eq:true&sort=-created_at&limit=10&offset=0&expand=author
```

Filter operators: `eq`, `ne`, `gt`, `gte`, `lt`, `lte`, `like`, `in`, `contains`

### 2. Database Layer

**Technology**: `modernc.org/sqlite` (pure Go, no CGO)

**Configuration**:

```go
// Recommended SQLite pragmas
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = -64000;  // 64MB
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;
```

**Internal Tables**:

```sql
-- Migration tracking
CREATE TABLE _alyx_migrations (
    id INTEGER PRIMARY KEY,
    version TEXT NOT NULL,
    name TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT (datetime('now')),
    checksum TEXT NOT NULL
);

-- Change feed for realtime
CREATE TABLE _alyx_changes (
    id INTEGER PRIMARY KEY,
    collection TEXT NOT NULL,
    operation TEXT NOT NULL,  -- INSERT, UPDATE, DELETE
    doc_id TEXT NOT NULL,
    changed_fields TEXT,      -- JSON array of field names (for UPDATE)
    timestamp TEXT NOT NULL DEFAULT (datetime('now')),
    processed INTEGER NOT NULL DEFAULT 0
);

-- Create index for efficient polling
CREATE INDEX idx_changes_unprocessed ON _alyx_changes(processed, timestamp);

-- Auth tables
CREATE TABLE _alyx_users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    verified INTEGER NOT NULL DEFAULT 0,
    metadata TEXT  -- JSON
);

CREATE TABLE _alyx_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES _alyx_users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    user_agent TEXT,
    ip_address TEXT
);

CREATE TABLE _alyx_oauth_accounts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES _alyx_users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(provider, provider_user_id)
);
```

**Change Detection Triggers** (generated per collection):

```sql
-- Example for 'posts' collection
CREATE TRIGGER posts_after_insert
AFTER INSERT ON posts
BEGIN
    INSERT INTO _alyx_changes (collection, operation, doc_id)
    VALUES ('posts', 'INSERT', NEW.id);
END;

CREATE TRIGGER posts_after_update
AFTER UPDATE ON posts
BEGIN
    INSERT INTO _alyx_changes (collection, operation, doc_id, changed_fields)
    VALUES ('posts', 'UPDATE', NEW.id,
        json_array(
            CASE WHEN OLD.title != NEW.title THEN 'title' END,
            CASE WHEN OLD.content != NEW.content THEN 'content' END,
            -- ... other fields
        )
    );
END;

CREATE TRIGGER posts_after_delete
AFTER DELETE ON posts
BEGIN
    INSERT INTO _alyx_changes (collection, operation, doc_id)
    VALUES ('posts', 'DELETE', OLD.id);
END;
```

---

## Schema System

### Schema File Format

```yaml
# schema.yaml
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      email:
        type: string
        unique: true
        index: true
        validate:
          format: email
      name:
        type: string
        maxLength: 100
        nullable: true
      avatar_url:
        type: string
        nullable: true
      role:
        type: string
        default: "user"
        validate:
          enum: [user, moderator, admin]
      created_at:
        type: timestamp
        default: now
      updated_at:
        type: timestamp
        default: now
        onUpdate: now

    rules:
      create: "true" # Anyone can register
      read: "auth.id == doc.id || auth.role == 'admin'"
      update: "auth.id == doc.id"
      delete: "auth.role == 'admin'"

  posts:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      title:
        type: string
        minLength: 1
        maxLength: 200
      slug:
        type: string
        unique: true
        index: true
      content:
        type: text
      excerpt:
        type: string
        maxLength: 500
        nullable: true
      author_id:
        type: uuid
        references: users.id
        onDelete: cascade
        index: true
      published:
        type: bool
        default: false
      published_at:
        type: timestamp
        nullable: true
      tags:
        type: json
        nullable: true
        # Expected: string[]
      view_count:
        type: int
        default: 0
      created_at:
        type: timestamp
        default: now
      updated_at:
        type: timestamp
        default: now
        onUpdate: now

    indexes:
      - name: idx_posts_published_date
        fields: [published, published_at]
        order: desc
      - name: idx_posts_author_date
        fields: [author_id, created_at]
        order: desc

    rules:
      create: "auth.id != null"
      read: "doc.published == true || auth.id == doc.author_id || auth.role == 'admin'"
      update: "auth.id == doc.author_id || auth.role == 'admin'"
      delete: "auth.id == doc.author_id || auth.role == 'admin'"

  comments:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      post_id:
        type: uuid
        references: posts.id
        onDelete: cascade
        index: true
      author_id:
        type: uuid
        references: users.id
        onDelete: cascade
      content:
        type: text
        maxLength: 5000
      created_at:
        type: timestamp
        default: now

    rules:
      create: "auth.id != null"
      read: "true" # Public comments
      update: "auth.id == doc.author_id"
      delete: "auth.id == doc.author_id || auth.role in ['moderator', 'admin']"
```

### Type System

| Alyx Type   | SQLite Type | Go Type     | TypeScript Type | Notes                              |
| ----------- | ----------- | ----------- | --------------- | ---------------------------------- |
| `uuid`      | TEXT        | `string`    | `string`        | Stored as string, validated format |
| `string`    | TEXT        | `string`    | `string`        | With optional length constraints   |
| `text`      | TEXT        | `string`    | `string`        | No length limit                    |
| `int`       | INTEGER     | `int64`     | `number`        |                                    |
| `float`     | REAL        | `float64`   | `number`        |                                    |
| `bool`      | INTEGER     | `bool`      | `boolean`       | 0/1 in SQLite                      |
| `timestamp` | TEXT        | `time.Time` | `Date`          | ISO8601 string                     |
| `json`      | TEXT        | `any`       | `unknown`       | JSON-encoded                       |
| `blob`      | BLOB        | `[]byte`    | `Uint8Array`    | Binary data                        |

### Field Options

```yaml
field_name:
  type: string # Required: data type
  primary: true # Primary key (default: false)
  unique: true # Unique constraint (default: false)
  nullable: true # Allow NULL (default: false)
  index: true # Create index (default: false)
  default: value # Default value (literal, "auto", "now")
  references: table.field # Foreign key reference
  onDelete: cascade # cascade, set null, restrict (default: restrict)
  onUpdate: now # Auto-update timestamp
  internal: true # Exclude from API responses (default: false)

  # Validation
  validate:
    minLength: 1
    maxLength: 200
    min: 0
    max: 100
    format: email # email, url, uuid
    pattern: "^[a-z]+$" # Regex pattern
    enum: [a, b, c] # Allowed values
```

### Migration Strategy

**Automatic (safe) migrations**:

- Add new collection
- Add new field (with default or nullable)
- Add new index
- Modify field constraints (looser only)

**Manual (requires migration file) migrations**:

- Remove collection
- Remove field
- Rename field
- Change field type
- Change constraints (stricter)

**Migration file format**:

```yaml
# migrations/002_rename_user_name.yaml
version: 2
name: rename_user_name
description: Rename 'name' field to 'display_name' in users

operations:
  - type: rename_field
    collection: users
    from: name
    to: display_name

  - type: sql
    up: |
      UPDATE users SET display_name = 'Anonymous' WHERE display_name IS NULL;
    down: |
      -- No rollback needed
```

---

## Function Runtime

### Container Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Function Executor (Go)                       │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                  Container Pool Manager                  │   │
│  │                                                          │   │
│  │  Responsibilities:                                       │   │
│  │  • Start/stop containers based on demand                 │   │
│  │  • Maintain warm pool per language                       │   │
│  │  • Route function calls to available containers          │   │
│  │  • Handle container health checks                        │   │
│  │  • Enforce resource limits                               │   │
│  │                                                          │   │
│  │  Configuration (per language):                           │   │
│  │  • min_warm: 1          # Minimum warm instances         │   │
│  │  • max_instances: 10    # Maximum concurrent             │   │
│  │  • idle_timeout: 60s    # Time before scaling down       │   │
│  │  • exec_timeout: 30s    # Max execution time             │   │
│  │  • memory_limit: 256MB  # Container memory limit         │   │
│  │  • cpu_limit: 1.0       # CPU cores                      │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│                              ▼                                  │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                   Container Registry                     │   │
│  │                                                          │   │
│  │  alyx-runtime-node:latest    (Node.js 20 LTS)           │   │
│  │  alyx-runtime-python:latest  (Python 3.11)              │   │
│  │  alyx-runtime-go:latest      (Go 1.21)                  │   │
│  │  alyx-runtime-deno:latest    (Deno, future)             │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Function Definition

**File structure**:

```
functions/
├── createPost.js         # Node.js function
├── processImage.py       # Python function
├── generateReport.go     # Go function
└── _shared/              # Shared code (copied to containers)
    ├── utils.js
    └── helpers.py
```

**Function manifest** (optional, for advanced config):

```yaml
# functions/createPost.yaml
name: createPost
runtime: node # node, python, go
timeout: 30s # Override default timeout
memory: 512mb # Override default memory
env: # Environment variables
  OPENAI_API_KEY: ${OPENAI_API_KEY}
```

### Function SDK (Node.js Example)

```javascript
// functions/createPost.js
import { defineFunction } from "@alyx/functions";

export default defineFunction({
  // Optional: input validation schema
  input: {
    title: { type: "string", required: true, maxLength: 200 },
    content: { type: "string", required: true },
    tags: { type: "array", items: "string", optional: true },
  },

  // Optional: output schema (for codegen)
  output: {
    id: { type: "string" },
    slug: { type: "string" },
  },

  async handler(input, ctx) {
    // ctx.auth - current user (null if unauthenticated)
    // ctx.db - database client
    // ctx.log - structured logger
    // ctx.env - environment variables

    if (!ctx.auth) {
      throw new Error("Authentication required");
    }

    const slug = input.title
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-|-$/g, "");

    // Single insert
    const post = await ctx.db.posts.create({
      title: input.title,
      content: input.content,
      slug: `${slug}-${Date.now()}`,
      author_id: ctx.auth.id,
      tags: input.tags || [],
    });

    ctx.log.info("Post created", { postId: post.id, authorId: ctx.auth.id });

    return { id: post.id, slug: post.slug };
  },
});
```

**With transactions**:

```javascript
export default defineFunction({
  async handler(input, ctx) {
    // Explicit transaction
    return await ctx.db.transaction(async (tx) => {
      const post = await tx.posts.create({ ... });

      await tx.users.update(ctx.auth.id, {
        post_count: ctx.db.raw('post_count + 1')
      });

      return { id: post.id };
    });
  }
});
```

### Container Protocol

**Request** (Alyx → Container):

```json
{
  "request_id": "req_abc123",
  "function": "createPost",
  "input": {
    "title": "Hello World",
    "content": "My first post"
  },
  "context": {
    "auth": {
      "id": "user_123",
      "email": "user@example.com",
      "role": "user"
    },
    "env": {
      "OPENAI_API_KEY": "sk-..."
    },
    "alyx_url": "http://host.docker.internal:8080",
    "internal_token": "eyJ..."
  }
}
```

**Response** (Container → Alyx):

```json
{
  "request_id": "req_abc123",
  "success": true,
  "output": {
    "id": "post_456",
    "slug": "hello-world-1706123456"
  },
  "logs": [
    {
      "level": "info",
      "message": "Post created",
      "data": { "postId": "post_456" }
    }
  ],
  "duration_ms": 145
}
```

**Error response**:

```json
{
  "request_id": "req_abc123",
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Title is required",
    "details": {"field": "title"}
  },
  "logs": [...],
  "duration_ms": 12
}
```

---

## Realtime Engine

### Subscription Protocol (WebSocket)

**Connection**:

```
GET /api/realtime
Upgrade: websocket
Authorization: Bearer <token>  # Optional, for authenticated subscriptions
```

**Client → Server messages**:

```typescript
// Subscribe to a query
{
  "id": "msg_1",
  "type": "subscribe",
  "payload": {
    "collection": "posts",
    "filter": {
      "author_id": {"$eq": "user_123"},
      "published": {"$eq": true}
    },
    "sort": ["-created_at"],
    "limit": 50,
    "expand": ["author_id"]  // Expand relations
  }
}

// Unsubscribe
{
  "id": "msg_2",
  "type": "unsubscribe",
  "payload": {
    "subscription_id": "sub_abc"
  }
}

// Ping (keepalive)
{
  "type": "ping"
}
```

**Server → Client messages**:

```typescript
// Connection established
{
  "type": "connected",
  "payload": {
    "client_id": "client_xyz"
  }
}

// Subscription confirmed with initial data
{
  "id": "msg_1",
  "type": "snapshot",
  "payload": {
    "subscription_id": "sub_abc",
    "docs": [...],
    "total": 42
  }
}

// Incremental update
{
  "type": "delta",
  "payload": {
    "subscription_id": "sub_abc",
    "changes": {
      "inserts": [{...}],
      "updates": [{...}],
      "deletes": ["id1", "id2"]
    }
  }
}

// Error
{
  "id": "msg_1",
  "type": "error",
  "payload": {
    "code": "INVALID_FILTER",
    "message": "Unknown field 'foo' in filter"
  }
}

// Pong
{
  "type": "pong"
}
```

### Subscription Indexing

To avoid O(N) matching per change, subscriptions are indexed:

```go
type SubscriptionIndex struct {
    // Map: collection -> field -> value -> [subscription_ids]
    // For equality filters only (most common case)
    equalityIndex map[string]map[string]map[any][]string

    // For range/complex filters, fall back to evaluation
    complexSubs map[string][]*Subscription
}

// On change to 'posts' with author_id = "user_123":
// 1. Look up equalityIndex["posts"]["author_id"]["user_123"]
// 2. Get candidate subscription IDs
// 3. For each candidate, verify full filter match
// 4. Check access rules (CEL)
// 5. Send delta
```

### Change Processing Pipeline

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐     ┌──────────────┐
│   SQLite    │     │   Change     │     │  Subscription   │     │  WebSocket   │
│   Trigger   │────►│   Detector   │────►│    Matcher      │────►│   Sender     │
└─────────────┘     └──────────────┘     └─────────────────┘     └──────────────┘
                           │                      │
                    Poll _alyx_changes      For each match:
                    every 50ms              1. Check filter
                    (configurable)          2. Check CEL rules
                                           3. Format delta
```

---

## Authentication & Authorization

### Auth Configuration

```yaml
# In alyx.yaml
auth:
  # JWT settings
  jwt:
    secret: ${JWT_SECRET} # Required, min 32 chars
    access_ttl: 15m # Access token lifetime
    refresh_ttl: 7d # Refresh token lifetime
    issuer: "alyx" # JWT issuer claim

  # Password requirements
  password:
    min_length: 8
    require_uppercase: false
    require_number: false
    require_special: false

  # OAuth providers
  oauth:
    github:
      client_id: ${GITHUB_CLIENT_ID}
      client_secret: ${GITHUB_CLIENT_SECRET}
      scopes: [user:email]
    google:
      client_id: ${GOOGLE_CLIENT_ID}
      client_secret: ${GOOGLE_CLIENT_SECRET}
      scopes: [email, profile]

  # Rate limiting
  rate_limit:
    login: 5/minute
    register: 3/minute
    password_reset: 3/hour
```

### CEL Rule Engine

**Available variables in rules**:

```
auth                    # Current user context (null if unauthenticated)
auth.id                 # User ID
auth.email              # User email
auth.role               # User role
auth.verified           # Email verified (bool)
auth.metadata           # Custom user metadata (map)

doc                     # Document being accessed
doc.<field>             # Any field from the document

request                 # Request context
request.method          # HTTP method
request.ip              # Client IP
request.time            # Request timestamp
```

**Example rules**:

```yaml
rules:
  # Anyone can read published posts
  read: "doc.published == true"

  # Only author can edit
  update: "auth.id == doc.author_id"

  # Only admins can delete
  delete: "auth.role == 'admin'"

  # Complex rule
  create: |
    auth.id != null && 
    auth.verified == true &&
    size(request.body.title) <= 200
```

### CEL Program Caching

```go
type RulesEngine struct {
    env      *cel.Env
    programs map[string]cel.Program  // key: "collection:operation"
    mu       sync.RWMutex
}

func (r *RulesEngine) Evaluate(collection, operation string, auth, doc map[string]any) (bool, error) {
    key := collection + ":" + operation

    r.mu.RLock()
    program, ok := r.programs[key]
    r.mu.RUnlock()

    if !ok {
        return false, ErrRuleNotFound
    }

    result, _, err := program.Eval(map[string]any{
        "auth": auth,
        "doc":  doc,
    })
    if err != nil {
        return false, err
    }

    return result.Value().(bool), nil
}
```

---

## CLI & Developer Experience

### Commands

```bash
# Initialize new project
alyx init [name]
  --template basic|blog|saas    # Project template

# Start development server with hot reload
alyx dev
  --port 8080                   # Server port (default: 8080)
  --host localhost              # Bind address
  --no-admin                    # Disable admin UI
  --verbose                     # Verbose logging

# Generate client SDKs
alyx generate
  --lang typescript,go,python   # Languages to generate
  --output ./generated          # Output directory

# Database migrations
alyx migrate
  --status                      # Show pending migrations
  --apply                       # Apply pending migrations
  --rollback [n]                # Rollback n migrations
  --create <name>               # Create new migration file

# Deploy to remote Alyx instance
alyx deploy
  --url https://api.myapp.com   # Remote Alyx URL
  --token <deploy_token>        # Deploy authentication
  --dry-run                     # Show what would change

# Database utilities
alyx db
  --seed <file>                 # Seed database from JSON/YAML
  --dump <file>                 # Export database
  --reset                       # Reset database (dev only!)

# Auth utilities
alyx auth
  --create-admin <email>        # Create admin user
  --reset-password <email>      # Send password reset
```

### Development Workflow

```
┌─────────────────────────────────────────────────────────────────┐
│                        alyx dev                                 │
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐      │
│  │ File Watcher │    │    Differ    │    │   Executor   │      │
│  │              │───►│              │───►│              │      │
│  │ • schema.yaml│    │ • Schema diff│    │ • Migrate DB │      │
│  │ • functions/ │    │ • Func diff  │    │ • Reload fns │      │
│  │ • alyx.yaml  │    │              │    │ • Regen SDKs │      │
│  └──────────────┘    └──────────────┘    └──────────────┘      │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │                    Dev Server                             │  │
│  │                                                           │  │
│  │  HTTP:      http://localhost:8080                         │  │
│  │  Admin UI:  http://localhost:8080/admin                   │  │
│  │  WebSocket: ws://localhost:8080/api/realtime              │  │
│  │                                                           │  │
│  │  Hot reload: ✓ Schema ✓ Functions ✓ Config                │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  Output:                                                        │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  [08:30:15] Starting Alyx dev server...                   │  │
│  │  [08:30:15] ✓ Schema loaded (3 collections)               │  │
│  │  [08:30:15] ✓ Functions loaded (5 functions)              │  │
│  │  [08:30:15] ✓ Server running at http://localhost:8080     │  │
│  │  [08:30:15] ✓ Admin UI at http://localhost:8080/admin     │  │
│  │                                                           │  │
│  │  [08:31:22] schema.yaml changed                           │  │
│  │  [08:31:22]   + Added field 'posts.view_count'            │  │
│  │  [08:31:22]   ✓ Migration applied                         │  │
│  │  [08:31:22]   ✓ Clients regenerated                       │  │
│  │                                                           │  │
│  │  [08:32:45] functions/createPost.js changed               │  │
│  │  [08:32:45]   ✓ Function reloaded                         │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### Sync Protocol (for `alyx deploy`)

```
┌────────────┐                              ┌──────────────┐
│ alyx CLI   │                              │ Alyx Server  │
│ (deploy)   │                              │  (remote)    │
└─────┬──────┘                              └──────┬───────┘
      │                                            │
      │  POST /api/admin/deploy/prepare            │
      │  { schema_hash, function_hashes }          │
      │────────────────────────────────────────────►
      │                                            │
      │        { changes_required: [...] }         │
      │◄────────────────────────────────────────────
      │                                            │
      │  [User confirms changes]                   │
      │                                            │
      │  POST /api/admin/deploy/execute            │
      │  { schema, functions[], migrations[] }     │
      │────────────────────────────────────────────►
      │                                            │
      │        { success: true, version: "v2" }    │
      │◄────────────────────────────────────────────
      │                                            │
```

---

## Client SDK Generation

### TypeScript Output

```typescript
// generated/alyx.ts
import { AlyxClient, Collection, Subscription } from "@alyx/client";

// Generated types
export interface User {
  id: string;
  email: string;
  name: string | null;
  avatar_url: string | null;
  role: "user" | "moderator" | "admin";
  created_at: Date;
  updated_at: Date;
}

export interface Post {
  id: string;
  title: string;
  slug: string;
  content: string;
  excerpt: string | null;
  author_id: string;
  published: boolean;
  published_at: Date | null;
  tags: string[] | null;
  view_count: number;
  created_at: Date;
  updated_at: Date;

  // Expanded relations (when requested)
  author?: User;
}

export interface Comment {
  id: string;
  post_id: string;
  author_id: string;
  content: string;
  created_at: Date;

  // Expanded relations
  post?: Post;
  author?: User;
}

// Generated function types
export interface CreatePostInput {
  title: string;
  content: string;
  tags?: string[];
}

export interface CreatePostOutput {
  id: string;
  slug: string;
}

// Client instance
export interface AlyxSchema {
  users: User;
  posts: Post;
  comments: Comment;
}

export interface AlyxFunctions {
  createPost: (input: CreatePostInput) => Promise<CreatePostOutput>;
}

export const alyx = new AlyxClient<AlyxSchema, AlyxFunctions>({
  url: process.env.ALYX_URL || "http://localhost:8080",
});

// Usage examples:

// Query with type safety
const posts = await alyx.posts
  .filter({ published: true })
  .sort("-created_at")
  .limit(10)
  .expand("author")
  .get();
// posts: Post[] (with author: User populated)

// Real-time subscription
const unsubscribe = alyx.posts
  .filter({ author_id: userId })
  .subscribe((snapshot) => {
    console.log("Posts updated:", snapshot.docs);
    // snapshot.docs: Post[]
  });

// Call function
const result = await alyx.fn.createPost({
  title: "Hello World",
  content: "My first post",
});
// result: { id: string, slug: string }

// Authentication
await alyx.auth.login({ email, password });
await alyx.auth.register({ email, password, name });
await alyx.auth.logout();
const user = alyx.auth.user; // Current user or null
```

### Go Output

```go
// generated/alyx.go
package alyx

import (
    "context"
    "time"

    client "github.com/your-org/alyx-client-go"
)

// Generated types
type User struct {
    ID        string     `json:"id"`
    Email     string     `json:"email"`
    Name      *string    `json:"name"`
    AvatarURL *string    `json:"avatar_url"`
    Role      string     `json:"role"`
    CreatedAt time.Time  `json:"created_at"`
    UpdatedAt time.Time  `json:"updated_at"`
}

type Post struct {
    ID          string     `json:"id"`
    Title       string     `json:"title"`
    Slug        string     `json:"slug"`
    Content     string     `json:"content"`
    Excerpt     *string    `json:"excerpt"`
    AuthorID    string     `json:"author_id"`
    Published   bool       `json:"published"`
    PublishedAt *time.Time `json:"published_at"`
    Tags        []string   `json:"tags"`
    ViewCount   int64      `json:"view_count"`
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`

    // Expanded
    Author *User `json:"author,omitempty"`
}

// Function input/output types
type CreatePostInput struct {
    Title   string   `json:"title"`
    Content string   `json:"content"`
    Tags    []string `json:"tags,omitempty"`
}

type CreatePostOutput struct {
    ID   string `json:"id"`
    Slug string `json:"slug"`
}

// Client
type Client struct {
    *client.BaseClient

    Users    *client.Collection[User]
    Posts    *client.Collection[Post]
    Comments *client.Collection[Comment]
}

func New(url string, opts ...client.Option) *Client {
    base := client.New(url, opts...)
    return &Client{
        BaseClient: base,
        Users:      client.NewCollection[User](base, "users"),
        Posts:      client.NewCollection[Post](base, "posts"),
        Comments:   client.NewCollection[Comment](base, "comments"),
    }
}

// Function calls
func (c *Client) CreatePost(ctx context.Context, input CreatePostInput) (*CreatePostOutput, error) {
    return client.CallFunction[CreatePostInput, CreatePostOutput](ctx, c.BaseClient, "createPost", input)
}

// Usage:
// c := alyx.New("http://localhost:8080")
// posts, _ := c.Posts.Filter("published", true).Sort("-created_at").Limit(10).Get(ctx)
// result, _ := c.CreatePost(ctx, alyx.CreatePostInput{Title: "Hello", Content: "World"})
```

---

## Repository Structure

```
alyx/
├── .github/
│   └── workflows/
│       ├── ci.yaml              # Test & lint
│       ├── release.yaml         # Build & publish binaries
│       └── docker.yaml          # Build runtime images
│
├── cmd/
│   └── alyx/
│       └── main.go              # CLI + server entrypoint
│
├── internal/
│   ├── server/
│   │   ├── server.go            # HTTP server setup
│   │   ├── router.go            # Route definitions
│   │   ├── middleware.go        # Auth, logging, CORS
│   │   └── handlers/
│   │       ├── collections.go   # CRUD handlers
│   │       ├── functions.go     # Function invocation
│   │       ├── auth.go          # Auth endpoints
│   │       ├── admin.go         # Admin endpoints
│   │       └── realtime.go      # WebSocket handler
│   │
│   ├── database/
│   │   ├── database.go          # Connection management
│   │   ├── query.go             # Query builder
│   │   ├── migrations.go        # Migration runner
│   │   └── triggers.go          # Change detection setup
│   │
│   ├── schema/
│   │   ├── types.go             # Schema type definitions
│   │   ├── parser.go            # YAML parser
│   │   ├── validator.go         # Schema validation
│   │   ├── differ.go            # Schema diff
│   │   └── sql.go               # SQL generation
│   │
│   ├── realtime/
│   │   ├── broker.go            # Subscription broker
│   │   ├── client.go            # WebSocket client
│   │   ├── subscription.go      # Subscription state
│   │   ├── index.go             # Subscription indexing
│   │   └── delta.go             # Change calculation
│   │
│   ├── functions/
│   │   ├── executor.go          # Function orchestration
│   │   ├── pool.go              # Container pool manager
│   │   ├── container.go         # Container lifecycle
│   │   └── protocol.go          # Container communication
│   │
│   ├── auth/
│   │   ├── auth.go              # Auth service
│   │   ├── jwt.go               # JWT utilities
│   │   ├── password.go          # Password hashing
│   │   ├── oauth.go             # OAuth providers
│   │   └── middleware.go        # Auth middleware
│   │
│   ├── rules/
│   │   ├── engine.go            # CEL engine
│   │   ├── compiler.go          # Rule compilation
│   │   └── context.go           # Evaluation context
│   │
│   ├── cli/
│   │   ├── root.go              # Root command
│   │   ├── init.go              # alyx init
│   │   ├── dev.go               # alyx dev
│   │   ├── generate.go          # alyx generate
│   │   ├── migrate.go           # alyx migrate
│   │   ├── deploy.go            # alyx deploy
│   │   └── watcher.go           # File watching
│   │
│   ├── codegen/
│   │   ├── generator.go         # Codegen orchestration
│   │   ├── typescript.go        # TypeScript generator
│   │   ├── golang.go            # Go generator
│   │   └── python.go            # Python generator
│   │
│   └── config/
│       ├── config.go            # Config types
│       └── loader.go            # Config loading
│
├── pkg/
│   └── client/                  # Go client library (public)
│       ├── client.go
│       ├── collection.go
│       ├── realtime.go
│       └── auth.go
│
├── runtimes/
│   ├── node/
│   │   ├── Dockerfile
│   │   ├── package.json
│   │   ├── executor.js          # Function executor
│   │   └── sdk/                 # @alyx/functions source
│   │       └── index.js
│   │
│   ├── python/
│   │   ├── Dockerfile
│   │   ├── requirements.txt
│   │   ├── executor.py
│   │   └── alyx_functions/      # alyx-functions package
│   │       └── __init__.py
│   │
│   └── go/
│       ├── Dockerfile
│       ├── go.mod
│       └── executor.go
│
├── ui/                          # Admin dashboard
│   ├── package.json
│   ├── svelte.config.js
│   ├── src/
│   │   ├── routes/
│   │   ├── lib/
│   │   └── app.html
│   └── build/                   # Built assets (embedded)
│
├── templates/                   # Codegen templates
│   ├── typescript/
│   │   ├── client.ts.tmpl
│   │   └── types.ts.tmpl
│   ├── golang/
│   │   └── client.go.tmpl
│   └── python/
│       └── client.py.tmpl
│
├── docs/
│   ├── getting-started.md
│   ├── schema-reference.md
│   ├── functions-guide.md
│   ├── client-sdks.md
│   └── deployment.md
│
├── examples/
│   ├── blog/
│   ├── todo-app/
│   └── saas-starter/
│
├── testdata/
│   ├── schemas/
│   └── functions/
│
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── LICENSE
└── PLANNING.md                  # This document
```

---

## Phased Implementation Plan

### Phase 1: Foundation (Weeks 1-2)

**Goal**: Basic server that can CRUD data via HTTP API

#### 1.1 Project Bootstrap (2 days)

- [x] Initialize Go module with dependencies
- [x] Set up project structure
- [x] Configure linting (golangci-lint)
- [x] Set up basic CI (GitHub Actions)
- [x] Create Makefile with common targets

**Dependencies**:

```go
require (
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.0
    modernc.org/sqlite v1.28.0
    github.com/google/cel-go v0.18.0
    nhooyr.io/websocket v1.8.10
    github.com/golang-jwt/jwt/v5 v5.2.0
    github.com/rs/zerolog v1.31.0
    golang.org/x/crypto v0.17.0
    gopkg.in/yaml.v3 v3.0.1
)
```

#### 1.2 Configuration System (1 day)

- [x] Define config structs
- [x] YAML config loading (alyx.yaml)
- [x] Environment variable overrides
- [x] Config validation

#### 1.3 Schema System (4 days)

- [x] Define schema types (Collection, Field, Index, Rule)
- [x] YAML parser with validation
- [x] Schema → SQL DDL generator
- [x] Schema differ (compare two schemas)
- [x] Migration SQL generator
- [x] Migration runner with history tracking

#### 1.4 Database Layer (3 days)

- [x] SQLite connection management
- [x] Query builder (parameterized queries)
- [x] CRUD operations
- [x] Transaction support
- [x] Change detection triggers

#### 1.5 HTTP API (3 days)

- [x] Server setup with middleware
- [x] Collection CRUD endpoints
- [x] Query parameter parsing (filter, sort, limit)
- [x] Error handling & responses
- [x] Request logging

**Deliverable**: Server that loads schema.yaml, creates tables, exposes REST API

---

### Phase 2: Realtime Engine (Week 3)

**Goal**: WebSocket subscriptions with live updates

#### 2.1 WebSocket Infrastructure (2 days)

- [x] WebSocket upgrade handler
- [x] Client connection management
- [x] Ping/pong keepalive
- [x] Clean disconnection handling

#### 2.2 Subscription System (3 days)

- [x] Subscription protocol implementation
- [x] Filter parsing and validation
- [x] Initial snapshot sending
- [x] Subscription indexing for efficient matching

#### 2.3 Change Broadcasting (2 days)

- [x] Change detector (poll \_alyx_changes)
- [x] Change → subscription matching
- [x] Delta calculation
- [x] Fan-out to subscribers

**Deliverable**: Clients can subscribe to queries and receive live updates

---

### Phase 3: Authentication (Week 4)

**Goal**: Complete auth system with CEL rules and OAuth

#### 3.1 Auth Infrastructure (2 days) ✅

- [x] User table and session management
- [x] Password hashing (bcrypt)
- [x] JWT generation and validation
- [x] Refresh token rotation

#### 3.2 Auth Endpoints (2 days) ✅

- [x] Register endpoint
- [x] Login endpoint
- [x] Refresh endpoint
- [x] Logout endpoint
- [x] Auth middleware

#### 3.3 CEL Rules Engine (3 days) ✅

- [x] CEL environment setup
- [x] Rule compilation and caching
- [x] Rule evaluation on CRUD operations
- [x] Integration with realtime (filter by permission)

#### 3.4 OAuth / Social Login (2 days) ✅

- [x] OAuth configuration in config structs
- [x] OAuth provider abstraction (internal/auth/oauth.go)
- [x] GitHub OAuth provider
- [x] Google OAuth provider
- [x] GET /api/auth/providers endpoint
- [x] GET /api/auth/oauth/:provider redirect endpoint
- [x] GET /api/auth/oauth/:provider/callback endpoint
- [x] _alyx_oauth_accounts table and user linking
- [x] Account linking (existing user with same email)
- [x] OpenAPI spec updates for OAuth endpoints

**Deliverable**: Protected endpoints with CEL-based access control and social login

---

### Phase 4: Function Runtime (Weeks 5-6)

**Goal**: Container-based serverless functions

#### 4.1 Container Management (3 days) ✅

- [x] Docker/Podman client integration
- [x] Container pool manager
- [x] Container lifecycle (create, start, stop, remove)
- [x] Health checking

#### 4.2 Runtime Images (2 days) ✅

- [x] Node.js runtime image + executor
- [x] Python runtime image + executor
- [x] Function SDK for each language
- [x] Build and publish images (GitHub Actions + GHCR)

#### 4.3 Function Execution (3 days) ✅

- [x] Function discovery (scan functions/)
- [x] Routing to appropriate runtime
- [x] Request/response protocol
- [x] Internal callback API for DB access
- [x] Timeout and error handling

#### 4.4 Function SDK Polish (2 days) ✅

- [x] Transaction support (placeholder - full support TBD)
- [x] Logging integration
- [x] Environment variables
- [ ] TypeScript types for SDK (deferred to Phase 5)

**Deliverable**: Invoke functions in Node.js/Python via HTTP ✅

---

### Phase 5: CLI & Developer Experience (Weeks 7-8)

**Goal**: Complete development workflow

#### 5.1 CLI Commands (3 days) ✅

- [x] `alyx init` with templates (basic, blog, saas)
- [x] `alyx migrate` commands (status, apply, rollback, create)
- [x] `alyx db` utilities (seed, dump, reset)

#### 5.2 Dev Mode (4 days) ✅

- [x] File watcher implementation (fsnotify-based)
- [x] Schema change detection + auto-migrate
- [x] Function change detection + hot-reload
- [x] Dev server with all features

#### 5.3 Code Generation (4 days) ✅

- [x] TypeScript client generator
- [x] Go client generator
- [x] Python client generator
- [x] Integration with dev mode (auto-regenerate)

#### 5.4 Deploy Command (2 days)

- [x] Bundle preparation
- [x] Remote diff checking
- [x] Deployment execution
- [x] Rollback support

**Deliverable**: Complete `alyx dev` workflow with codegen

---

### Phase 6: Polish & Documentation (Week 9)

**Goal**: Production-ready MVP

#### 6.1 Admin UI (3 days)

- [x] Basic Svelte app structure
- [x] Schema viewer
- [x] Collection browser (CRUD)
- [x] Function list and logs
- [x] Embed in binary

#### 6.2 Error Handling & Observability (2 days) ✅

- [x] Structured error responses (with request_id and timestamp)
- [x] Request ID tracing (middleware + context propagation)
- [x] Metrics endpoint (Prometheus at /metrics)
- [x] Health check endpoint (comprehensive /health, /health/live, /health/ready, /health/stats)

#### 6.3 Documentation (3 days) ✅

- [x] Getting started guide
- [x] Schema reference
- [x] Functions guide
- [x] Client SDK docs
- [x] Deployment guide

#### 6.4 Examples (2 days)

- [ ] Blog example
- [ ] Todo app example
- [ ] README polish

**Deliverable**: MVP ready for public use

---

### Timeline Summary

```
Week 1-2:   Foundation (schema, database, HTTP API)
Week 3:     Realtime engine (WebSocket, subscriptions)
Week 4:     Authentication (JWT, OAuth, CEL rules)
Week 5-6:   Function runtime (containers, SDKs)
Week 7-8:   CLI & DX (dev mode, codegen, deploy)
Week 9:     Polish (admin UI, docs, examples)

Total: ~9 weeks to MVP
Buffer: +2 weeks for unknowns = ~11 weeks
```

---

## Risk Assessment & Mitigation

### High Risk

| Risk                          | Impact               | Likelihood | Mitigation                                                   |
| ----------------------------- | -------------------- | ---------- | ------------------------------------------------------------ |
| Container cold start too slow | Users perceive lag   | Medium     | Pre-warm pools, snapshot optimization                        |
| SQLite performance at scale   | Service degradation  | Low        | Document limits, Turso upgrade path                          |
| CEL rule complexity           | Security bugs        | Medium     | Comprehensive test suite, limit rule complexity              |
| Schema migration edge cases   | Data loss/corruption | Medium     | Conservative auto-migrations, require manual for destructive |

### Medium Risk

| Risk                              | Impact              | Likelihood | Mitigation                                                |
| --------------------------------- | ------------------- | ---------- | --------------------------------------------------------- |
| Realtime fan-out performance      | High CPU under load | Medium     | Subscription indexing, batching                           |
| Container management complexity   | Ops burden          | Medium     | Good defaults, clear docs, fallback to process mode       |
| Codegen bugs                      | Broken client code  | Low-Medium | Generated code tests, multiple language parity tests      |
| Docker/Podman dependency friction | Adoption barrier    | Low        | Clear install docs, consider optional embedded JS runtime |

### Low Risk

| Risk                     | Impact           | Likelihood | Mitigation                               |
| ------------------------ | ---------------- | ---------- | ---------------------------------------- |
| Go ecosystem limitations | Dev friction     | Low        | Mature ecosystem, most needs covered     |
| WebSocket compatibility  | Browser issues   | Very Low   | Well-supported standard                  |
| YAML schema limitations  | Feature requests | Medium     | Extensible design, escape hatches to SQL |

---

## Future Improvements (Post-MVP)

### Near-term (v1.1 - v1.3)

#### Embedded User Frontend

Full-stack single-binary deployment allowing users to embed their own SPA alongside Alyx.

**Configuration**:

```yaml
frontend:
  enabled: true
  root: "/"                       # Mount path for user frontend
  dist: "./frontend/dist"         # Pre-built assets (embedded via go:embed)
  source: "./frontend"            # Optional: source to build
  build_command: "npm run build"  # Build command if source provided
  output: "dist"                  # Output directory relative to source
  dev_proxy: "http://localhost:5173"  # Vite dev server for HMR
```

**Behavior**:
- If `dist/` exists → embed and serve directly
- Else if `source/` exists → run `build_command`, then embed output
- Else → frontend disabled (warning logged)

**Route Priority** (critical ordering):
```
/_admin/*    → Admin UI (Svelte, embedded)
/api/*       → REST API
/internal/*  → Internal APIs (function callbacks)
/*           → User frontend (SPA fallback to index.html)
```

**Implementation Details**:
- Framework-agnostic: works with React, Svelte, Vue, SolidJS, etc.
- SPA fallback: unmatched routes serve `index.html` for client-side routing
- Cache headers: `no-cache` for HTML, `immutable` for hashed assets
- Dev mode: reverse proxy to Vite dev server (non-API routes only)
- Security: path traversal guard, CSP header support

**Dev Workflow**:
```bash
# Terminal 1: Vite dev server with HMR
cd frontend && npm run dev

# Terminal 2: Alyx dev server (proxies to Vite)
alyx dev
```

#### Multi-tenancy

- Database-per-tenant isolation
- Tenant-aware routing
- Per-tenant resource limits
- Tenant management API

#### File Storage

- Local filesystem backend
- S3-compatible storage backend
- File upload endpoints
- Image transformation (resize, crop)

#### Function Dependencies & Custom Runtime Images

Allow functions to declare dependencies that get baked into custom container images (similar to Appwrite, AWS Lambda).

**Function manifest with dependencies**:

```yaml
# functions/processImage.yaml
name: processImage
runtime: python
dependencies:
  - pillow==10.0.0
  - numpy==1.24.0
  - opencv-python==4.8.0
```

```javascript
// functions/sendEmail.js
export const config = {
  dependencies: {
    "nodemailer": "^6.9.0",
    "handlebars": "^4.7.8"
  }
};

export default defineFunction({
  async handler(input, ctx) {
    const nodemailer = await import('nodemailer');
    // Use dependencies...
  }
});
```

**Deployment workflow** (similar to Appwrite):

```bash
# Deploy a function (builds custom image with dependencies)
alyx deploy function processImage

# Alyx:
# 1. Detects function dependencies from manifest/code
# 2. Builds custom image: FROM alyx-runtime-python + RUN pip install ...
# 3. Tags image: alyx-function-processImage:v1
# 4. Caches image for reuse
# 5. Updates pool to use custom image
```

**Benefits**:
- Proper dependency isolation per function
- Portable across environments (no platform-specific binary issues)
- Can use native extensions (Pillow, numpy, etc.)
- Image layers cached for fast rebuilds
- Production-ready deployment model

**Implementation details**:
- Parse dependencies from function manifest or code exports
- Generate Dockerfile for each function: `FROM <base-runtime> + COPY + RUN install`
- Build images with BuildKit for layer caching
- Store images locally or push to registry
- Update pool configuration to use function-specific image
- Garbage collect unused images

**Dev mode optimization**:
- Option to use volume-mounted `node_modules/` or `packages/` for faster iteration
- Rebuild images only on dependency changes
- Pre-warm pool with custom images

#### Scheduled Functions (Cron)

```yaml
# functions/dailyDigest.yaml
schedule: "0 8 * * *" # Every day at 8am
```

#### Background Jobs / Queues

```javascript
// In a function
await ctx.queue.push("send-email", { to, subject, body });
```

#### OAuth Provider Expansion

- Apple Sign In
- Microsoft
- Discord
- Custom OIDC

### Medium-term (v1.4 - v2.0)

#### GraphQL API

- Auto-generated from schema
- Subscriptions support
- Query complexity limits

#### Edge Deployment (Turso + Fly.io)

- Read replicas at edge
- Function execution at edge
- Geo-routing

#### Webhooks

```yaml
webhooks:
  - name: notify-slack
    events: [posts.created, posts.updated]
    url: https://hooks.slack.com/...
    secret: ${WEBHOOK_SECRET}
```

#### Plugins / Extensions

- Custom authentication providers
- Custom field types
- Lifecycle hooks
- Third-party integrations

### Long-term (v2.0+)

#### Clustering / Horizontal Scaling

- Multiple Alyx instances
- Shared state coordination
- Load balancing

#### Version Control Integration

- Git-based schema history
- Branch deployments
- PR previews

#### Visual Schema Editor

- Drag-and-drop schema design
- Relationship visualization
- Migration preview

#### AI Features

- Natural language → schema
- Query optimization suggestions
- Anomaly detection

---

## Open Questions

### Resolved

1. ~~**WASM vs Containers for functions?**~~ → **Containers** (simpler, battle-tested, language-agnostic)

2. ~~**SSE vs WebSocket for realtime?**~~ → **WebSocket** (bidirectional, better mobile support)

3. ~~**Schema format?**~~ → **YAML** (language-agnostic, readable)

4. ~~**Rules language?**~~ → **CEL** (Google-backed, good Go library)

### Unresolved

1. **Naming: "collections" vs "tables"?**
   - PocketBase uses "collections"
   - More database-agnostic terminology
   - Decision: Use "collections" for consistency with PocketBase

2. **Default port?**
   - 8080 (common, but often taken)
   - 3000 (Node.js convention)
   - 9000 (less common)
   - Decision: 8090 (PocketBase uses 8090, provides consistency)

3. **License?**
   - MIT (maximum adoption)
   - Apache 2.0 (patent protection)
   - AGPL (copyleft, forces open source)
   - Decision: TBD, leaning MIT for adoption

4. **Binary name?**
   - `alyx` (simple)
   - `alyxd` for server, `alyx` for CLI?
   - Decision: Single binary `alyx`, subcommands for different modes

5. **Cloud offering?**
   - Managed Alyx cloud (like Convex, PocketBase Cloud)
   - Marketplace images only
   - Decision: Post-MVP consideration

---

## Appendices

### A. Glossary

| Term             | Definition                                            |
| ---------------- | ----------------------------------------------------- |
| **Collection**   | A table in the database, defined in schema.yaml       |
| **Document**     | A row/record in a collection                          |
| **Function**     | Serverless function executed in a container           |
| **Subscription** | A live query that receives real-time updates          |
| **Rule**         | A CEL expression that controls access to data         |
| **Schema**       | The YAML file defining collections, fields, and rules |
| **Migration**    | A change to the database schema                       |

### B. Comparison Matrix

| Feature            | Alyx       | PocketBase   | Convex | Supabase        |
| ------------------ | ---------- | ------------ | ------ | --------------- |
| Self-hosted        | ✅         | ✅           | ❌     | ✅              |
| Single binary      | ✅         | ✅           | ❌     | ❌              |
| Schema-as-code     | ✅         | ❌           | ✅     | ⚠️ (migrations) |
| Typed codegen      | ✅         | ❌           | ✅     | ⚠️ (manual)     |
| Realtime           | ✅         | ✅           | ✅     | ✅              |
| Functions          | ✅         | ⚠️ (JS only) | ✅     | ✅              |
| Polyglot functions | ✅         | ❌           | ❌     | ✅              |
| SQLite             | ✅         | ✅           | ❌     | ❌              |
| PostgreSQL         | ❌ (Turso) | ❌           | ❌     | ✅              |
| Auth built-in      | ✅         | ✅           | ✅     | ✅              |
| File storage       | 🔜         | ✅           | ✅     | ✅              |
| Admin UI           | ✅         | ✅           | ✅     | ✅              |

### C. Dependencies

**Go Dependencies**:

```
github.com/spf13/cobra          # CLI framework
github.com/spf13/viper          # Configuration
modernc.org/sqlite              # SQLite (pure Go)
github.com/google/cel-go        # CEL rules engine
nhooyr.io/websocket             # WebSocket
github.com/golang-jwt/jwt/v5    # JWT
github.com/rs/zerolog           # Structured logging
golang.org/x/crypto             # Password hashing
gopkg.in/yaml.v3                # YAML parsing
github.com/docker/docker        # Docker client
github.com/fsnotify/fsnotify    # File watching
github.com/google/uuid          # UUID generation
```

**Runtime Dependencies**:

- Docker or Podman (for function execution)
- Node.js 20 LTS (for Node.js functions)
- Python 3.11+ (for Python functions)
- Go 1.21+ (for Go functions)

### D. References

- [PocketBase](https://pocketbase.io/) - Inspiration for single-binary approach
- [Convex](https://convex.dev/) - Inspiration for realtime sync and DX
- [Extism](https://extism.org/) - WASM plugin system (considered, not used)
- [CEL Specification](https://github.com/google/cel-spec) - Rules language
- [Open Runtimes](https://github.com/open-runtimes/open-runtimes) - Appwrite's function runtimes

---

_Document created: January 2026_  
_Last updated: January 2026_
