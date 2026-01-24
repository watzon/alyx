# Serverless Functions Guide

Alyx supports serverless functions in multiple languages (Node.js, Python, Go) that run in isolated containers. Functions can access the database, authenticate users, and extend your backend with custom logic.

## Prerequisites

- **Docker** or **Podman** must be installed and running
- Functions are enabled by default (see [Configuration](#configuration))

## Quick Start

### 1. Create a Function

Create a JavaScript file in the `functions/` directory:

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

### 2. Call the Function

```bash
curl -X POST http://localhost:8090/api/functions/hello \
  -H "Content-Type: application/json" \
  -d '{"name": "Alyx"}'
```

Response:

```json
{
  "message": "Hello, Alyx!",
  "timestamp": "2026-01-24T10:30:00.000Z"
}
```

## Function Structure

### Node.js

```javascript
// functions/myFunction.js
export default async function handler(ctx) {
  // ctx.input - Request body (parsed JSON)
  // ctx.auth - Authenticated user (or null)
  // ctx.db - Database client
  // ctx.log - Logger
  // ctx.env - Environment variables

  return { result: "success" };
}
```

### Python

```python
# functions/my_function.py
def handler(ctx):
    # ctx.input - Request body (dict)
    # ctx.auth - Authenticated user (or None)
    # ctx.db - Database client
    # ctx.log - Logger
    # ctx.env - Environment variables

    return {"result": "success"}
```

### Go

```go
// functions/myFunction.go
package main

import (
    "github.com/watzon/alyx/pkg/runtime"
)

func Handler(ctx *runtime.Context) (any, error) {
    // ctx.Input - Request body (map[string]any)
    // ctx.Auth - Authenticated user (or nil)
    // ctx.DB - Database client
    // ctx.Log - Logger
    // ctx.Env - Environment variables

    return map[string]string{"result": "success"}, nil
}
```

## Context Object

Every function receives a context object with these properties:

### `ctx.input`

The request body parsed as JSON:

```javascript
export default async function handler(ctx) {
  const { title, content, tags } = ctx.input;
  // ...
}
```

### `ctx.auth`

The authenticated user (if request includes a valid JWT token):

```javascript
export default async function handler(ctx) {
  if (!ctx.auth) {
    throw new Error("Authentication required");
  }

  console.log(ctx.auth.id); // User ID
  console.log(ctx.auth.email); // User email
  console.log(ctx.auth.role); // User role
  console.log(ctx.auth.metadata); // Custom metadata
}
```

### `ctx.db`

Database client for CRUD operations:

```javascript
export default async function handler(ctx) {
  // List documents
  const posts = await ctx.db.posts.list({
    filter: { author_id: ctx.auth.id },
    sort: "-created_at",
    limit: 10,
  });

  // Get single document
  const post = await ctx.db.posts.get("post-id-here");

  // Create document
  const newPost = await ctx.db.posts.create({
    title: "Hello World",
    content: "My first post",
    author_id: ctx.auth.id,
  });

  // Update document
  const updated = await ctx.db.posts.update("post-id", {
    title: "Updated Title",
  });

  // Delete document
  await ctx.db.posts.delete("post-id");

  return posts;
}
```

### `ctx.log`

Structured logger:

```javascript
export default async function handler(ctx) {
  ctx.log.info("Processing request", { userId: ctx.auth?.id });
  ctx.log.debug("Debug info", { input: ctx.input });
  ctx.log.warn("Warning message");
  ctx.log.error("Error occurred", { error: "details" });
}
```

### `ctx.env`

Environment variables:

```javascript
export default async function handler(ctx) {
  const apiKey = ctx.env.OPENAI_API_KEY;
  const webhookUrl = ctx.env.SLACK_WEBHOOK_URL;
}
```

## Database Operations

### Querying Collections

```javascript
// List with filters
const activePosts = await ctx.db.posts.list({
  filter: {
    published: true,
    author_id: ctx.auth.id,
  },
  sort: "-created_at", // Descending
  limit: 20,
  offset: 0,
});

// Returns: { docs: [...], total: 42 }
```

### Filter Operators

```javascript
// Equality
{ status: 'active' }
{ status: { $eq: 'active' } }

// Not equal
{ status: { $ne: 'deleted' } }

// Comparison
{ price: { $gt: 100 } }
{ price: { $gte: 100 } }
{ price: { $lt: 1000 } }
{ price: { $lte: 1000 } }

// In list
{ category: { $in: ['tech', 'science'] } }

// Like (SQL LIKE)
{ title: { $like: '%tutorial%' } }

// Combine multiple conditions (AND)
{
  published: true,
  author_id: ctx.auth.id,
  created_at: { $gte: '2026-01-01' }
}
```

### CRUD Operations

```javascript
// Create
const doc = await ctx.db.posts.create({
  title: "New Post",
  content: "Content here",
  author_id: ctx.auth.id,
});
// Returns: { id: 'uuid', title: 'New Post', ... }

// Read
const doc = await ctx.db.posts.get("document-id");
// Returns: { id: 'uuid', ... } or null

// Update
const updated = await ctx.db.posts.update("document-id", {
  title: "Updated Title",
});
// Returns: { id: 'uuid', title: 'Updated Title', ... }

// Delete
await ctx.db.posts.delete("document-id");
// Returns: void
```

### Raw SQL Queries

For complex queries not supported by the query builder:

```javascript
// Read query
const results = await ctx.db.query(
  "SELECT * FROM posts WHERE author_id = ? AND published = ?",
  [ctx.auth.id, true],
);

// Write query
await ctx.db.exec("UPDATE posts SET view_count = view_count + 1 WHERE id = ?", [
  postId,
]);
```

## Error Handling

### Throwing Errors

```javascript
export default async function handler(ctx) {
  if (!ctx.auth) {
    throw new Error("Authentication required");
  }

  const post = await ctx.db.posts.get(ctx.input.postId);
  if (!post) {
    throw new Error("Post not found");
  }

  if (post.author_id !== ctx.auth.id) {
    throw new Error("Permission denied");
  }

  // ...
}
```

### Custom Error Codes

```javascript
export default async function handler(ctx) {
  if (!ctx.input.email) {
    throw {
      code: "VALIDATION_ERROR",
      message: "Email is required",
      field: "email",
    };
  }
}
```

Error response:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Email is required",
    "field": "email"
  }
}
```

## Function Manifests

For advanced configuration, create a YAML manifest alongside your function:

```yaml
# functions/processImage.yaml
name: processImage
runtime: python
timeout: 60s # Override default timeout
memory: 512mb # Override default memory limit
env:
  OPENAI_API_KEY: ${OPENAI_API_KEY}
```

### Manifest Options

| Option    | Default       | Description                 |
| --------- | ------------- | --------------------------- |
| `name`    | filename      | Function name (used in URL) |
| `runtime` | auto-detected | `node`, `python`, or `go`   |
| `timeout` | `30s`         | Maximum execution time      |
| `memory`  | `256mb`       | Container memory limit      |
| `env`     | `{}`          | Environment variables       |

## Input Validation

### Node.js with Schema

```javascript
export const config = {
  input: {
    title: { type: "string", required: true, maxLength: 200 },
    content: { type: "string", required: true },
    tags: { type: "array", items: "string", optional: true },
  },
  output: {
    id: { type: "string" },
    slug: { type: "string" },
  },
};

export default async function handler(ctx) {
  // Input is already validated
  const { title, content, tags } = ctx.input;
  // ...
}
```

### Python with Validation

```python
# functions/create_post.py
SCHEMA = {
    "input": {
        "title": {"type": "string", "required": True, "max_length": 200},
        "content": {"type": "string", "required": True},
        "tags": {"type": "array", "items": "string", "optional": True}
    }
}

def handler(ctx):
    # Input is validated according to SCHEMA
    title = ctx.input["title"]
    content = ctx.input["content"]
    # ...
```

## Real-World Examples

### Create Blog Post with Slug Generation

```javascript
// functions/createPost.js
export default async function handler(ctx) {
  if (!ctx.auth) {
    throw new Error("Authentication required");
  }

  const { title, content, tags } = ctx.input;

  // Generate slug
  const slug = title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");

  // Check for uniqueness
  const existing = await ctx.db.posts.list({
    filter: { slug: { $like: `${slug}%` } },
  });

  const finalSlug = existing.total > 0 ? `${slug}-${Date.now()}` : slug;

  // Create the post
  const post = await ctx.db.posts.create({
    title,
    content,
    slug: finalSlug,
    author_id: ctx.auth.id,
    tags: tags || [],
    published: false,
  });

  ctx.log.info("Post created", { postId: post.id, authorId: ctx.auth.id });

  return { id: post.id, slug: post.slug };
}
```

### Send Welcome Email (with external API)

```javascript
// functions/sendWelcome.js
export default async function handler(ctx) {
  const { userId } = ctx.input;

  // Get user
  const user = await ctx.db.users.get(userId);
  if (!user) {
    throw new Error("User not found");
  }

  // Send email via external API
  const response = await fetch("https://api.sendgrid.com/v3/mail/send", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${ctx.env.SENDGRID_API_KEY}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      personalizations: [{ to: [{ email: user.email }] }],
      from: { email: "hello@myapp.com" },
      subject: "Welcome to MyApp!",
      content: [
        {
          type: "text/html",
          value: `<h1>Welcome, ${user.name}!</h1>`,
        },
      ],
    }),
  });

  if (!response.ok) {
    throw new Error("Failed to send email");
  }

  return { sent: true };
}
```

### Python: Image Processing

```python
# functions/process_image.py
import base64
from PIL import Image
from io import BytesIO

def handler(ctx):
    if not ctx.auth:
        raise Exception("Authentication required")

    image_data = ctx.input.get("image")
    if not image_data:
        raise Exception("Image data required")

    # Decode base64 image
    image_bytes = base64.b64decode(image_data)
    image = Image.open(BytesIO(image_bytes))

    # Resize
    thumbnail = image.copy()
    thumbnail.thumbnail((200, 200))

    # Encode result
    buffer = BytesIO()
    thumbnail.save(buffer, format="PNG")
    result = base64.b64encode(buffer.getvalue()).decode()

    return {
        "thumbnail": result,
        "original_size": f"{image.width}x{image.height}",
        "thumbnail_size": f"{thumbnail.width}x{thumbnail.height}"
    }
```

## Configuration

### Server Configuration (alyx.yaml)

```yaml
functions:
  enabled: true
  timeout: 30s # Default timeout per function

  pool:
    min_warm: 1 # Minimum warm containers per runtime
    max_instances: 10 # Maximum concurrent containers
    idle_timeout: 60s # Time before scaling down
    memory_limit: 256mb # Default memory limit
    cpu_limit: 1.0 # CPU cores limit

  runtimes:
    node:
      image: ghcr.io/watzon/alyx-runtime-node:latest
      enabled: true
    python:
      image: ghcr.io/watzon/alyx-runtime-python:latest
      enabled: true
    go:
      image: ghcr.io/watzon/alyx-runtime-go:latest
      enabled: false # Disable if not needed
```

### Environment Variables

Set environment variables for functions:

```yaml
# alyx.yaml
functions:
  env:
    OPENAI_API_KEY: ${OPENAI_API_KEY}
    SENDGRID_API_KEY: ${SENDGRID_API_KEY}
    APP_URL: https://myapp.com
```

Or via shell:

```bash
export ALYX_FUNCTIONS_ENV_OPENAI_API_KEY="sk-..."
```

## Hot Reloading

During development (`alyx dev`), functions are hot-reloaded automatically:

```
[INFO] functions/hello.js changed
[INFO] Function 'hello' reloaded
```

No container restart needed - changes are picked up instantly.

## Debugging

### View Function Logs

In the admin UI at `/_admin/functions`, you can:

- View recent function invocations
- See input/output for each call
- View logs and errors

### Local Testing

Test functions locally before deployment:

```bash
# Test with curl
curl -X POST http://localhost:8090/api/functions/hello \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"name": "Test"}'

# View container logs
docker logs alyx-node-pool-1
```

## Best Practices

1. **Validate input early** - Check required fields at the start of your function

2. **Use structured logging** - Include relevant context in log messages

3. **Handle errors gracefully** - Return meaningful error messages

4. **Keep functions focused** - One function = one responsibility

5. **Use timeouts wisely** - Set appropriate timeouts for your use case

6. **Secure sensitive data** - Use environment variables for API keys

7. **Test locally** - Use `alyx dev` for rapid iteration

8. **Monitor in production** - Check function logs and metrics regularly
