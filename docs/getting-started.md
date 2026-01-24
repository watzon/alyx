# Getting Started with Alyx

This guide will help you get Alyx up and running in minutes. By the end, you'll have a working backend with a REST API, real-time subscriptions, and authentication.

## Prerequisites

- **Go 1.24.5+** (for building from source)
- **Docker** or **Podman** (optional, required for serverless functions)

## Installation

### Option 1: Download Binary (Recommended)

Download the latest release for your platform:

```bash
# Linux (amd64)
curl -L https://github.com/watzon/alyx/releases/latest/download/alyx-linux-amd64 -o alyx
chmod +x alyx
sudo mv alyx /usr/local/bin/

# macOS (Apple Silicon)
curl -L https://github.com/watzon/alyx/releases/latest/download/alyx-darwin-arm64 -o alyx
chmod +x alyx
sudo mv alyx /usr/local/bin/

# macOS (Intel)
curl -L https://github.com/watzon/alyx/releases/latest/download/alyx-darwin-amd64 -o alyx
chmod +x alyx
sudo mv alyx /usr/local/bin/
```

### Option 2: Using Go Install

```bash
go install github.com/watzon/alyx/cmd/alyx@latest
```

### Option 3: Build from Source

```bash
git clone https://github.com/watzon/alyx.git
cd alyx
make build
# Binary is at ./build/alyx
```

### Option 4: Docker

```bash
docker pull ghcr.io/watzon/alyx:latest
```

## Quick Start

### 1. Initialize a New Project

```bash
mkdir myapp && cd myapp
alyx init
```

This creates a project structure:

```
myapp/
  alyx.yaml         # Server configuration
  schema.yaml       # Data schema definition
  functions/        # Serverless functions (optional)
```

### 2. Define Your Schema

Edit `schema.yaml` to define your data model:

```yaml
version: 1

collections:
  tasks:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      title:
        type: string
        validate:
          minLength: 1
          maxLength: 200
      completed:
        type: bool
        default: false
      created_at:
        type: timestamp
        default: now
      updated_at:
        type: timestamp
        default: now
        onUpdate: now

    rules:
      create: "true"
      read: "true"
      update: "true"
      delete: "true"
```

### 3. Start the Development Server

```bash
alyx dev
```

You'll see output like:

```
[INFO] Starting Alyx dev server...
[INFO] Schema loaded (1 collection)
[INFO] Database initialized: ./data/alyx.db
[INFO] Server running at http://localhost:8090
[INFO] Admin UI at http://localhost:8090/_admin
[INFO] API docs at http://localhost:8090/docs
[INFO] Watching for changes...
```

### 4. Try the API

The REST API is automatically generated from your schema:

```bash
# Create a task
curl -X POST http://localhost:8090/api/collections/tasks \
  -H "Content-Type: application/json" \
  -d '{"title": "Learn Alyx"}'

# List all tasks
curl http://localhost:8090/api/collections/tasks

# Get a specific task
curl http://localhost:8090/api/collections/tasks/{id}

# Update a task
curl -X PATCH http://localhost:8090/api/collections/tasks/{id} \
  -H "Content-Type: application/json" \
  -d '{"completed": true}'

# Delete a task
curl -X DELETE http://localhost:8090/api/collections/tasks/{id}
```

### 5. Explore the Admin UI

Open http://localhost:8090/\_admin in your browser to:

- Browse collections and documents
- View and edit schema
- Monitor function execution
- View server statistics

## Configuration

### Server Settings (alyx.yaml)

```yaml
server:
  host: localhost
  port: 8090
  cors:
    enabled: true
    origins:
      - "*"

database:
  path: ./data/alyx.db

auth:
  jwt:
    secret: "change-this-in-production"
    access_ttl: 15m
    refresh_ttl: 7d

functions:
  enabled: true
  timeout: 30s
  pool:
    min_warm: 1
    max_instances: 10
```

### Environment Variables

Override any config with environment variables using the `ALYX_` prefix:

```bash
export ALYX_SERVER_PORT=3000
export ALYX_DATABASE_PATH=/var/lib/alyx/data.db
export ALYX_AUTH_JWT_SECRET="your-secure-secret"
```

## Adding Authentication

### 1. Enable Auth in Your Schema

Add rules that check authentication:

```yaml
collections:
  tasks:
    fields:
      # ... fields ...
      user_id:
        type: uuid
        references: _alyx_users.id

    rules:
      create: "auth.id != null"
      read: "auth.id == doc.user_id"
      update: "auth.id == doc.user_id"
      delete: "auth.id == doc.user_id"
```

### 2. Register and Login

```bash
# Register a new user
curl -X POST http://localhost:8090/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "securepassword"}'

# Login
curl -X POST http://localhost:8090/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "securepassword"}'
# Returns: { "access_token": "...", "refresh_token": "...", "user": {...} }

# Use the token
curl http://localhost:8090/api/collections/tasks \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"
```

## Real-Time Subscriptions

Connect via WebSocket to receive live updates:

```javascript
const ws = new WebSocket("ws://localhost:8090/api/realtime");

ws.onopen = () => {
  // Subscribe to tasks
  ws.send(
    JSON.stringify({
      id: "1",
      type: "subscribe",
      payload: {
        collection: "tasks",
        filter: { completed: { $eq: false } },
      },
    }),
  );
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);

  if (msg.type === "snapshot") {
    console.log("Initial data:", msg.payload.docs);
  }

  if (msg.type === "delta") {
    console.log("Changes:", msg.payload.changes);
  }
};
```

## Serverless Functions

Create custom backend logic with serverless functions:

### Node.js Function

```javascript
// functions/hello.js
export default async function handler(ctx) {
  const name = ctx.input.name || "World";

  return {
    message: `Hello, ${name}!`,
    timestamp: new Date().toISOString(),
  };
}
```

### Invoke the Function

```bash
curl -X POST http://localhost:8090/api/functions/hello \
  -H "Content-Type: application/json" \
  -d '{"name": "Alyx"}'
# Returns: { "message": "Hello, Alyx!", "timestamp": "..." }
```

## Generate Type-Safe Clients

Generate client libraries for your frontend:

```bash
# TypeScript
alyx generate --lang typescript --output ./client

# Go
alyx generate --lang go --output ./client

# Python
alyx generate --lang python --output ./client
```

Example TypeScript usage:

```typescript
import { alyx } from "./client";

// Fully typed!
const tasks = await alyx.tasks
  .filter({ completed: false })
  .sort("-created_at")
  .limit(10)
  .get();

// Create with type checking
await alyx.tasks.create({
  title: "New task", // Required
  completed: false, // Optional, has default
});
```

## Next Steps

- **[Schema Reference](./schema-reference.md)** - Complete guide to schema definitions
- **[Functions Guide](./functions-guide.md)** - Writing serverless functions
- **[Client SDKs](./client-sdks.md)** - Using generated clients
- **[Deployment Guide](./deployment.md)** - Production deployment

## Getting Help

- **GitHub Issues**: https://github.com/watzon/alyx/issues
- **Discussions**: https://github.com/watzon/alyx/discussions
- **API Docs**: Available at `/docs` on your running server
