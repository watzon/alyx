# Schema Reference

Alyx uses YAML-based schema definitions to define your data model. This reference covers all available options for collections, fields, indexes, and access rules.

## Schema File Structure

```yaml
version: 1

collections:
  collection_name:
    fields:
      field_name:
        type: string
        # ... field options

    indexes:
      - name: idx_name
        fields: [field1, field2]

    rules:
      create: "expression"
      read: "expression"
      update: "expression"
      delete: "expression"
```

## Field Types

| Type        | SQLite Type | Go Type     | TypeScript Type | Description                             |
| ----------- | ----------- | ----------- | --------------- | --------------------------------------- |
| `uuid`      | TEXT        | `string`    | `string`        | UUID stored as string, validated format |
| `string`    | TEXT        | `string`    | `string`        | Text with optional length constraints   |
| `text`      | TEXT        | `string`    | `string`        | Unlimited text (no length validation)   |
| `int`       | INTEGER     | `int64`     | `number`        | 64-bit integer                          |
| `float`     | REAL        | `float64`   | `number`        | 64-bit floating point                   |
| `bool`      | INTEGER     | `bool`      | `boolean`       | Boolean (0/1 in SQLite)                 |
| `timestamp` | TEXT        | `time.Time` | `Date`          | ISO8601 timestamp string                |
| `json`      | TEXT        | `any`       | `unknown`       | JSON-encoded data                       |
| `blob`      | BLOB        | `[]byte`    | `Uint8Array`    | Binary data                             |

## Field Options

### Basic Options

```yaml
fields:
  email:
    type: string
    primary: false # Primary key (default: false)
    unique: true # Unique constraint (default: false)
    nullable: false # Allow NULL values (default: false)
    index: true # Create index on this field (default: false)
    internal: false # Exclude from API responses (default: false)
```

### Default Values

```yaml
fields:
  # Literal value
  status:
    type: string
    default: "pending"

  # Auto-generated UUID
  id:
    type: uuid
    default: auto

  # Current timestamp
  created_at:
    type: timestamp
    default: now

  # Auto-update timestamp on every update
  updated_at:
    type: timestamp
    default: now
    onUpdate: now
```

### Foreign Key References

```yaml
fields:
  author_id:
    type: uuid
    references: users.id # Table.field reference
    onDelete: cascade # cascade | set null | restrict (default)
```

**onDelete behaviors:**

- `restrict` - Prevent deletion if referenced (default)
- `cascade` - Delete referencing documents too
- `set null` - Set foreign key to NULL (field must be nullable)

## Validation Rules

### String Validation

```yaml
fields:
  username:
    type: string
    validate:
      minLength: 3
      maxLength: 50
      pattern: "^[a-zA-Z0-9_]+$" # Regex pattern

  email:
    type: string
    validate:
      format: email # Built-in format: email, url, uuid

  website:
    type: string
    validate:
      format: url

  role:
    type: string
    validate:
      enum: [user, moderator, admin] # Allowed values
```

### Numeric Validation

```yaml
fields:
  age:
    type: int
    validate:
      min: 0
      max: 150

  price:
    type: float
    validate:
      min: 0.01
      max: 999999.99
```

### Available Formats

| Format  | Description         | Example                                |
| ------- | ------------------- | -------------------------------------- |
| `email` | Valid email address | `user@example.com`                     |
| `url`   | Valid URL           | `https://example.com`                  |
| `uuid`  | Valid UUID v4       | `550e8400-e29b-41d4-a716-446655440000` |

## Indexes

### Single-Field Index

```yaml
fields:
  email:
    type: string
    index: true # Shorthand for single-field index
```

### Composite Indexes

```yaml
indexes:
  - name: idx_posts_author_date
    fields: [author_id, created_at]
    order: desc # asc | desc (default: asc)

  - name: idx_posts_status_published
    fields: [status, published_at]
```

### Unique Indexes

```yaml
indexes:
  - name: idx_users_email_unique
    fields: [email]
    unique: true
```

## Access Control Rules (CEL)

Alyx uses [CEL (Common Expression Language)](https://github.com/google/cel-spec) for access control rules.

### Rule Operations

```yaml
rules:
  create: "expression" # Controls document creation
  read: "expression" # Controls document reading (list and get)
  update: "expression" # Controls document updates
  delete: "expression" # Controls document deletion
```

### Available Variables

| Variable         | Type      | Description                                          |
| ---------------- | --------- | ---------------------------------------------------- |
| `auth`           | object    | Current authenticated user (null if unauthenticated) |
| `auth.id`        | string    | User's unique ID                                     |
| `auth.email`     | string    | User's email address                                 |
| `auth.role`      | string    | User's role                                          |
| `auth.verified`  | bool      | Whether email is verified                            |
| `auth.metadata`  | map       | Custom user metadata                                 |
| `doc`            | object    | The document being accessed                          |
| `doc.<field>`    | varies    | Any field from the document                          |
| `request`        | object    | Request context                                      |
| `request.method` | string    | HTTP method                                          |
| `request.ip`     | string    | Client IP address                                    |
| `request.time`   | timestamp | Request timestamp                                    |

### Rule Examples

#### Public Read, Authenticated Write

```yaml
rules:
  create: "auth.id != null"
  read: "true"
  update: "auth.id != null"
  delete: "auth.id != null"
```

#### Owner-Only Access

```yaml
rules:
  create: "auth.id != null"
  read: "auth.id == doc.user_id"
  update: "auth.id == doc.user_id"
  delete: "auth.id == doc.user_id"
```

#### Role-Based Access

```yaml
rules:
  create: "auth.id != null"
  read: "true"
  update: "auth.id == doc.author_id || auth.role == 'admin'"
  delete: "auth.role in ['moderator', 'admin']"
```

#### Published Content with Author Override

```yaml
rules:
  read: "doc.published == true || auth.id == doc.author_id || auth.role == 'admin'"
```

#### Complex Multi-Condition Rules

```yaml
rules:
  create: |
    auth.id != null &&
    auth.verified == true &&
    size(request.body.title) <= 200

  update: |
    (auth.id == doc.author_id && doc.status != 'locked') ||
    auth.role == 'admin'
```

### CEL Functions

| Function                | Description               | Example                                    |
| ----------------------- | ------------------------- | ------------------------------------------ |
| `size(x)`               | Length of string/list/map | `size(doc.title) <= 100`                   |
| `has(x.y)`              | Check if field exists     | `has(doc.metadata)`                        |
| `matches(s, re)`        | Regex match               | `matches(doc.email, '@example\\.com$')`    |
| `startsWith(s, prefix)` | String prefix check       | `startsWith(doc.slug, 'blog-')`            |
| `endsWith(s, suffix)`   | String suffix check       | `endsWith(doc.email, '.edu')`              |
| `contains(s, sub)`      | String contains           | `contains(doc.tags, 'featured')`           |
| `timestamp(s)`          | Parse timestamp           | `timestamp(doc.expires_at) > request.time` |

## Complete Schema Example

```yaml
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
        validate:
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
      create: "true"
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
        validate:
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
        nullable: true
        validate:
          maxLength: 500
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
        validate:
          maxLength: 5000
      created_at:
        type: timestamp
        default: now

    rules:
      create: "auth.id != null"
      read: "true"
      update: "auth.id == doc.author_id"
      delete: "auth.id == doc.author_id || auth.role in ['moderator', 'admin']"
```

## Migrations

### Automatic Migrations

Alyx automatically applies safe schema changes:

- Adding new collections
- Adding new fields (with default or nullable)
- Adding new indexes
- Loosening constraints (e.g., adding nullable)

### Manual Migrations Required

These changes require explicit migration files:

- Removing collections
- Removing fields
- Renaming fields
- Changing field types
- Tightening constraints

### Migration File Format

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
      -- Rollback SQL (optional)
```

### Migration Commands

```bash
# View pending migrations
alyx migrate --status

# Apply all pending migrations
alyx migrate --apply

# Rollback last migration
alyx migrate --rollback

# Rollback multiple migrations
alyx migrate --rollback 3

# Create new migration file
alyx migrate --create add_user_phone
```

## Internal Tables

Alyx creates system tables prefixed with `_alyx_`:

| Table                  | Purpose                                 |
| ---------------------- | --------------------------------------- |
| `_alyx_migrations`     | Migration history tracking              |
| `_alyx_changes`        | Change feed for real-time subscriptions |
| `_alyx_users`          | User authentication accounts            |
| `_alyx_sessions`       | Active user sessions                    |
| `_alyx_oauth_accounts` | OAuth provider linkages                 |

These tables are managed by Alyx and should not be modified directly.

## Best Practices

1. **Always define primary keys** - Use `type: uuid` with `default: auto` for consistency

2. **Use references for relationships** - Foreign keys ensure data integrity

3. **Index frequently queried fields** - Especially foreign keys and filter fields

4. **Start with restrictive rules** - Then loosen as needed

5. **Use timestamp fields** - `created_at` and `updated_at` help with debugging and auditing

6. **Validate user input** - Use `validate` options to catch bad data early

7. **Use meaningful index names** - Format: `idx_{table}_{fields}`

8. **Keep rules simple** - Complex rules are harder to maintain and debug
