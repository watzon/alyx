# Alyx Realtime Demo

A simple chat application demonstrating Alyx's real-time WebSocket subscriptions.

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

### REST API

Messages are created via the REST API:

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
```
