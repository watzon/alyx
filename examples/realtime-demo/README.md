# Alyx Realtime Demo

A simple chat application demonstrating Alyx's real-time WebSocket subscriptions and serverless functions.

## Quick Start

1. **Build Alyx** (from project root):
   ```bash
   make build
   ```

2. **Start the server** (from this directory):
   ```bash
   ../../build/alyx dev
   ```

3. **Open the demo**:
   - Open `index.html` in your browser
   - Or serve it: `python3 -m http.server 3000` and visit http://localhost:3000

4. **Test it out**:
   - Open multiple browser tabs
   - Send messages from one tab
   - Watch them appear instantly in all tabs

## How It Works

### Schema

The demo uses a simple `messages` collection:

```yaml
collections:
  messages:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      channel:
        type: string
        index: true
      author:
        type: string
      content:
        type: text
      created_at:
        type: timestamp
        default: now
```

### WebSocket Protocol

1. **Connect** to `ws://localhost:8090/api/realtime`

2. **Receive** connection confirmation:
   ```json
   {"type": "connected", "payload": {"client_id": "abc123..."}}
   ```

3. **Subscribe** to messages:
   ```json
   {
     "id": "sub_1",
     "type": "subscribe",
     "payload": {
       "collection": "messages",
       "sort": ["-created_at"],
       "limit": 50
     }
   }
   ```

4. **Receive** initial snapshot:
   ```json
   {
     "id": "sub_1",
     "type": "snapshot",
     "payload": {
       "subscription_id": "sub_xyz",
       "docs": [...],
       "total": 10
     }
   }
   ```

5. **Receive** live updates (deltas):
   ```json
   {
     "type": "delta",
     "payload": {
       "subscription_id": "sub_xyz",
       "changes": {
         "inserts": [{"id": "...", "author": "Alice", "content": "Hello!"}],
         "updates": [],
         "deletes": []
       }
     }
   }
   ```

### Serverless Functions

This demo uses serverless functions instead of direct REST API calls:

#### sendMessage Function

Creates a new message with optional auto-generated author name:

```bash
curl -X POST http://localhost:8090/api/functions/sendMessage \
  -H "Content-Type: application/json" \
  -d '{
    "input": {
      "channel": "general",
      "content": "Hello world!",
      "author": "Bob"
    }
  }'
```

If `author` is omitted, the function generates a random name like `Anon-x7k2`.

Response:
```json
{
  "success": true,
  "output": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "channel": "general",
    "author": "Bob",
    "content": "Hello world!"
  },
  "duration_ms": 15
}
```

#### getStats Function

Returns message statistics across channels:

```bash
curl -X POST http://localhost:8090/api/functions/getStats \
  -H "Content-Type: application/json" \
  -d '{"input": {}}'
```

Response:
```json
{
  "success": true,
  "output": {
    "totalMessages": 42,
    "channels": {
      "general": 30,
      "random": 12
    },
    "recentAuthors": ["Alice", "Bob", "Anon-x7k2"]
  },
  "duration_ms": 8
}
```

### REST API (Direct)

You can also create messages directly via the REST API:

```bash
curl -X POST http://localhost:8090/api/collections/messages \
  -H "Content-Type: application/json" \
  -d '{"channel": "general", "author": "Bob", "content": "Hello world!"}'
```

The WebSocket subscription automatically receives the new message as a delta.

## Filtering

You can filter subscriptions to only receive specific messages:

```json
{
  "type": "subscribe",
  "payload": {
    "collection": "messages",
    "filter": {
      "channel": {"$eq": "general"}
    }
  }
}
```

Supported filter operators:
- `$eq` - Equal
- `$ne` - Not equal
- `$gt`, `$gte` - Greater than (or equal)
- `$lt`, `$lte` - Less than (or equal)
- `$like` - SQL LIKE pattern
- `$in` - In array
- `$contains` - Contains (for JSON fields)

## Configuration

See `alyx.yaml` for configuration options:

```yaml
realtime:
  enabled: true
  poll_interval: 50ms          # How often to check for changes
  max_connections: 1000        # Max concurrent WebSocket clients
  max_subscriptions_per_client: 100
  change_buffer_size: 1000     # Buffer size for change events

functions:
  enabled: true
  path: ./functions            # Directory containing function files
  runtime: docker              # Container runtime (docker or podman)
  timeout: 30s                 # Default execution timeout
  memory_limit: 128            # Default memory limit in MB
  pools:
    node:
      min_warm: 1              # Minimum warm containers
      max_instances: 5         # Maximum concurrent containers
      idle_timeout: 5m         # Idle container timeout
      image: ghcr.io/watzon/alyx-runtime-node:latest
```

## Functions Directory

The `functions/` directory contains serverless functions:

```
functions/
  sendMessage.js    # Message creation with validation
  sendMessage.yaml  # Optional manifest (timeout, memory overrides)
  getStats.js       # Message statistics aggregation
  getStats.yaml
```

Each function exports a default object with an async `handler`:

```javascript
export default {
  input: {
    channel: { type: "string", required: true },
    content: { type: "string", required: true },
  },
  async handler(input, context) {
    const result = await context.db.messages.create({
      channel: input.channel,
      content: input.content,
    });
    return { id: result.id };
  },
};
```

The `context` object provides:
- `context.db` - Database client with collection proxies
- `context.log` - Structured logger (debug, info, warn, error)
- `context.auth` - Authenticated user info (if present)
