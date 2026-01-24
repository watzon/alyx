# Client SDKs Guide

Alyx generates type-safe client libraries from your schema. This guide covers how to generate and use clients in TypeScript, Go, and Python.

## Generating Clients

### Generate All Languages

```bash
alyx generate --output ./generated
```

This generates clients for all supported languages:

- `./generated/typescript/` - TypeScript/JavaScript client
- `./generated/go/` - Go client
- `./generated/python/` - Python client

### Generate Specific Language

```bash
# TypeScript only
alyx generate --lang typescript --output ./client

# Go only
alyx generate --lang go --output ./client

# Python only
alyx generate --lang python --output ./client
```

### Auto-Generate in Dev Mode

During development, clients regenerate automatically when schema changes:

```bash
alyx dev --generate ./generated
```

## TypeScript/JavaScript Client

### Installation

The generated client requires the `@alyx/client` runtime package:

```bash
npm install @alyx/client
# or
pnpm add @alyx/client
# or
yarn add @alyx/client
```

### Setup

```typescript
// Import the generated client
import { alyx } from "./generated/typescript";

// Or configure manually
import { AlyxClient } from "@alyx/client";

const client = new AlyxClient({
  url: "http://localhost:8090",
  // Optional: provide auth token
  token: "your-jwt-token",
});
```

### Configuration Options

```typescript
const client = new AlyxClient({
  url: "http://localhost:8090",
  token: "jwt-token", // Optional: JWT for authenticated requests
  timeout: 30000, // Request timeout in ms (default: 30000)
  retries: 3, // Retry attempts on network errors
  headers: {
    // Additional headers
    "X-Custom-Header": "value",
  },
});
```

### Querying Collections

```typescript
// List all documents
const posts = await alyx.posts.get();
// Type: Post[]

// Filter documents
const publishedPosts = await alyx.posts.filter({ published: true }).get();

// Multiple filters (AND)
const myPublishedPosts = await alyx.posts
  .filter({
    published: true,
    author_id: userId,
  })
  .get();

// Comparison operators
const recentPosts = await alyx.posts
  .filter({ created_at: { $gte: "2026-01-01" } })
  .get();

// Sorting
const latestPosts = await alyx.posts
  .sort("-created_at") // Descending
  .get();

const oldestPosts = await alyx.posts
  .sort("created_at") // Ascending
  .get();

// Pagination
const page1 = await alyx.posts.limit(10).offset(0).get();

const page2 = await alyx.posts.limit(10).offset(10).get();

// Combined query
const result = await alyx.posts
  .filter({ published: true })
  .sort("-created_at")
  .limit(10)
  .offset(0)
  .get();
// result: { docs: Post[], total: number }
```

### Getting a Single Document

```typescript
// By ID
const post = await alyx.posts.getById("post-uuid");
// Type: Post | null

// First matching document
const featured = await alyx.posts.filter({ featured: true }).first();
// Type: Post | null
```

### Creating Documents

```typescript
const newPost = await alyx.posts.create({
  title: "Hello World",
  content: "My first post",
  author_id: userId,
  tags: ["intro", "tutorial"],
});
// Type: Post (with generated id, created_at, etc.)
```

### Updating Documents

```typescript
// Full update
const updated = await alyx.posts.update("post-id", {
  title: "Updated Title",
  content: "Updated content",
});
// Type: Post

// Partial update
const patched = await alyx.posts.patch("post-id", {
  published: true,
});
// Type: Post
```

### Deleting Documents

```typescript
await alyx.posts.delete("post-id");
// Returns: void
```

### Expanding Relations

```typescript
// Expand author relation
const posts = await alyx.posts.expand("author_id").get();
// Each post includes: { ..., author: User }

// Multiple expansions
const comments = await alyx.comments.expand(["post_id", "author_id"]).get();
// Each comment includes: { ..., post: Post, author: User }
```

### Real-Time Subscriptions

```typescript
// Subscribe to a query
const unsubscribe = alyx.posts
  .filter({ published: true })
  .sort("-created_at")
  .limit(20)
  .subscribe((snapshot) => {
    console.log("Documents:", snapshot.docs);
    console.log("Total:", snapshot.total);
  });

// Handle changes
const unsubscribe = alyx.posts.filter({ author_id: userId }).subscribe({
  onSnapshot: (snapshot) => {
    console.log("Current data:", snapshot.docs);
  },
  onDelta: (changes) => {
    console.log("Inserts:", changes.inserts);
    console.log("Updates:", changes.updates);
    console.log("Deletes:", changes.deletes);
  },
  onError: (error) => {
    console.error("Subscription error:", error);
  },
});

// Unsubscribe when done
unsubscribe();
```

### Calling Functions

```typescript
// Call a function
const result = await alyx.fn.createPost({
  title: "New Post",
  content: "Content here",
});
// Type: { id: string, slug: string } (from function output schema)

// With authentication
alyx.setToken(accessToken);
const result = await alyx.fn.processOrder({ orderId: "123" });
```

### Authentication

```typescript
// Register a new user
const { user, access_token, refresh_token } = await alyx.auth.register({
  email: "user@example.com",
  password: "securepassword",
  name: "John Doe",
});

// Login
const { user, access_token, refresh_token } = await alyx.auth.login({
  email: "user@example.com",
  password: "securepassword",
});

// Set token for subsequent requests
alyx.setToken(access_token);

// Get current user
const currentUser = alyx.auth.user;

// Refresh token
const { access_token: newToken } = await alyx.auth.refresh(refresh_token);
alyx.setToken(newToken);

// Logout
await alyx.auth.logout();

// OAuth login
const authUrl = alyx.auth.getOAuthUrl("github");
// Redirect user to authUrl, then handle callback

// Listen for auth state changes
alyx.auth.onAuthChange((user) => {
  if (user) {
    console.log("Logged in:", user.email);
  } else {
    console.log("Logged out");
  }
});
```

### Error Handling

```typescript
import { AlyxError } from "@alyx/client";

try {
  await alyx.posts.create({ title: "" });
} catch (error) {
  if (error instanceof AlyxError) {
    console.log("Code:", error.code); // 'VALIDATION_ERROR'
    console.log("Message:", error.message); // 'Title is required'
    console.log("Status:", error.status); // 400
    console.log("Details:", error.details); // { field: 'title' }
  }
}
```

## Go Client

### Installation

```bash
go get github.com/watzon/alyx/generated/go
```

Or copy the generated files to your project.

### Setup

```go
package main

import (
    "context"
    alyx "github.com/yourapp/generated/go"
)

func main() {
    client := alyx.New("http://localhost:8090")

    // With authentication
    client.SetToken("your-jwt-token")
}
```

### Querying Collections

```go
ctx := context.Background()

// List all documents
posts, err := client.Posts.List(ctx, nil)
if err != nil {
    log.Fatal(err)
}

// With filters
posts, err := client.Posts.List(ctx, &alyx.ListOptions{
    Filter: map[string]any{
        "published": true,
        "author_id": userId,
    },
    Sort:   "-created_at",
    Limit:  10,
    Offset: 0,
})

// Get single document
post, err := client.Posts.Get(ctx, "post-id")
if err != nil {
    log.Fatal(err)
}
if post == nil {
    log.Fatal("post not found")
}
```

### CRUD Operations

```go
// Create
newPost, err := client.Posts.Create(ctx, &alyx.Post{
    Title:    "Hello World",
    Content:  "My first post",
    AuthorID: userId,
})

// Update
updated, err := client.Posts.Update(ctx, "post-id", &alyx.PostUpdate{
    Title:     alyx.String("Updated Title"),
    Published: alyx.Bool(true),
})

// Delete
err := client.Posts.Delete(ctx, "post-id")
```

### Authentication

```go
// Register
result, err := client.Auth.Register(ctx, &alyx.RegisterInput{
    Email:    "user@example.com",
    Password: "securepassword",
    Name:     "John Doe",
})

// Login
result, err := client.Auth.Login(ctx, &alyx.LoginInput{
    Email:    "user@example.com",
    Password: "securepassword",
})

// Set token
client.SetToken(result.AccessToken)

// Refresh
newTokens, err := client.Auth.Refresh(ctx, result.RefreshToken)
```

### Real-Time Subscriptions

```go
// Subscribe to changes
sub, err := client.Posts.Subscribe(ctx, &alyx.SubscribeOptions{
    Filter: map[string]any{"published": true},
})
if err != nil {
    log.Fatal(err)
}
defer sub.Close()

// Handle updates
for {
    select {
    case snapshot := <-sub.Snapshots:
        log.Printf("Received %d documents", len(snapshot.Docs))
    case delta := <-sub.Deltas:
        log.Printf("Inserts: %d, Updates: %d, Deletes: %d",
            len(delta.Inserts), len(delta.Updates), len(delta.Deletes))
    case err := <-sub.Errors:
        log.Printf("Error: %v", err)
    case <-ctx.Done():
        return
    }
}
```

### Calling Functions

```go
// Call function
result, err := client.Functions.Call(ctx, "createPost", map[string]any{
    "title":   "New Post",
    "content": "Content here",
})

// Typed function call (if schema available)
output, err := client.Fn.CreatePost(ctx, &alyx.CreatePostInput{
    Title:   "New Post",
    Content: "Content here",
})
// output.ID, output.Slug are typed
```

## Python Client

### Installation

```bash
pip install alyx-client
```

### Setup

```python
from generated.python import AlyxClient

client = AlyxClient("http://localhost:8090")

# With authentication
client.set_token("your-jwt-token")
```

### Querying Collections

```python
# List all documents
posts = client.posts.list()

# With filters
posts = client.posts.list(
    filter={"published": True, "author_id": user_id},
    sort="-created_at",
    limit=10,
    offset=0
)

# Get single document
post = client.posts.get("post-id")
if post is None:
    raise Exception("Post not found")
```

### CRUD Operations

```python
# Create
new_post = client.posts.create({
    "title": "Hello World",
    "content": "My first post",
    "author_id": user_id
})

# Update
updated = client.posts.update("post-id", {
    "title": "Updated Title",
    "published": True
})

# Delete
client.posts.delete("post-id")
```

### Authentication

```python
# Register
result = client.auth.register(
    email="user@example.com",
    password="securepassword",
    name="John Doe"
)

# Login
result = client.auth.login(
    email="user@example.com",
    password="securepassword"
)

# Set token
client.set_token(result.access_token)

# Refresh
new_tokens = client.auth.refresh(result.refresh_token)
```

### Real-Time Subscriptions

```python
# Callback-based subscription
def on_snapshot(snapshot):
    print(f"Received {len(snapshot.docs)} documents")

def on_delta(delta):
    print(f"Inserts: {len(delta.inserts)}")
    print(f"Updates: {len(delta.updates)}")
    print(f"Deletes: {len(delta.deletes)}")

subscription = client.posts.subscribe(
    filter={"published": True},
    on_snapshot=on_snapshot,
    on_delta=on_delta
)

# Later: unsubscribe
subscription.close()
```

### Calling Functions

```python
# Call function
result = client.functions.call("createPost", {
    "title": "New Post",
    "content": "Content here"
})

# Typed call (if schema available)
output = client.fn.create_post(
    title="New Post",
    content="Content here"
)
print(output.id, output.slug)
```

### Async Support (Python)

```python
import asyncio
from generated.python import AsyncAlyxClient

async def main():
    client = AsyncAlyxClient("http://localhost:8090")

    # All methods are async
    posts = await client.posts.list(
        filter={"published": True}
    )

    new_post = await client.posts.create({
        "title": "Async Post"
    })

asyncio.run(main())
```

## Type Definitions

### Generated Types (TypeScript)

```typescript
// From schema.yaml -> TypeScript types
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

// Function types
export interface CreatePostInput {
  title: string;
  content: string;
  tags?: string[];
}

export interface CreatePostOutput {
  id: string;
  slug: string;
}
```

### Generated Types (Go)

```go
type User struct {
    ID        string    `json:"id"`
    Email     string    `json:"email"`
    Name      *string   `json:"name"`
    AvatarURL *string   `json:"avatar_url"`
    Role      string    `json:"role"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
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
```

## Best Practices

1. **Regenerate after schema changes** - Always regenerate clients after modifying `schema.yaml`

2. **Use type-safe methods** - Let the compiler catch errors before runtime

3. **Handle null/optional fields** - Check for null values on nullable fields

4. **Manage tokens securely** - Store JWT tokens securely (e.g., httpOnly cookies, secure storage)

5. **Use subscriptions sparingly** - Only subscribe to data that needs real-time updates

6. **Implement error handling** - Catch and handle `AlyxError` appropriately

7. **Set appropriate timeouts** - Configure timeouts based on your network conditions

8. **Use environment variables** - Don't hardcode server URLs
