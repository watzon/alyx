# Alyx Functions v2: Event-Driven Architecture

## Context

### Original Request
Redesign the function system to be event-driven with hooks, webhooks, scheduling, and SDK generation - inspired by Appwrite (local-first with SDKs) and Windmill (powerful trigger system).

### Interview Summary
**Key Discussions**:
- Primary use case: Local-first development with SDK focus (like Appwrite)
- Database hooks: Both sync (can reject/modify) and async (fire-and-forget)
- Webhook security: Built-in HMAC verification + custom handling option
- Scheduler: Custom scheduler (CRON + interval + one-time) with timezone support
- Event architecture: Unified event bus (all triggers → central queue → dispatcher)
- SDK: Template-based generation, TypeScript first
- Custom routes: Both fixed endpoint + manifest-defined routes
- Deployment: Live filesystem (auto-reload on change)
- Log persistence: SQLite with configurable retention
- Sync hook failure: Configurable per-hook (reject | continue)
- Testing: Tests after implementation

**Research Findings**:
- Appwrite: Open Runtimes with queue-based execution, SDK auto-generation from declarative labels
- Windmill: PostgreSQL as job queue, stateless workers, pluggable trigger system
- Current Alyx: Container pools, file-based discovery, internal token system, HTTP-only invocation

### Metis Review
**Identified Gaps** (addressed):
- Event payload schemas and versioning: Define explicit schemas per trigger type
- Queue semantics: At-least-once delivery, no ordering guarantees, no DLQ for v1
- Sync hook timeout: Configurable per-hook, default 5s
- Hook cycles: Detect and prevent self-triggering loops
- Idempotency: Document as user responsibility, provide request_id for deduplication

---

## Work Objectives

### Core Objective
Transform Alyx functions from HTTP-only invocation to a full event-driven system with database hooks, webhooks, scheduling, execution logging, and TypeScript SDK generation.

### Concrete Deliverables
1. Event bus with SQLite-backed queue (`internal/events/`)
2. Hook registry for subscribing functions to events (`internal/hooks/`)
3. Webhook handler with HMAC verification (`internal/webhooks/`)
4. Custom scheduler (CRON/interval/one-time) (`internal/scheduler/`)
5. Execution logging with persistence (`internal/executions/`)
6. TypeScript SDK generator (`cmd/alyx/generate/`)
7. Enhanced function manifest schema
8. New API endpoints for hooks, webhooks, schedules, executions
9. Database migrations for new tables

### Definition of Done
- [ ] `make test` passes with all new tests
- [ ] `make lint` passes with zero issues
- [ ] All new API endpoints documented in OpenAPI spec
- [ ] Database hooks trigger on INSERT/UPDATE/DELETE
- [ ] Webhooks receive and verify external requests
- [ ] Scheduler fires at correct times (manual verification)
- [ ] Execution logs persisted and queryable
- [ ] TypeScript SDK compiles and invokes a function

### Must Have
- Unified event bus with SQLite backing
- Database event hooks (sync + async modes)
- Incoming webhook endpoints with optional HMAC
- CRON + interval + one-time scheduling
- Execution logging with configurable retention
- TypeScript SDK generation
- Custom HTTP routes via manifest
- Backward compatibility with existing function invocation

### Must NOT Have (Guardrails)
- No distributed queue (SQLite-backed only for v1)
- No new container runtimes (keep existing Node/Python/Go)
- No UI changes (admin UI out of scope)
- No breaking changes to existing `/api/functions/{name}` endpoint
- No dead-letter queue (DLQ) implementation
- No exactly-once delivery guarantees
- No multi-language SDK generation (TypeScript only)
- No versioned deployments (keep live filesystem)

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: YES (Go testing + testify)
- **User wants tests**: Tests after implementation
- **Framework**: Go standard testing with table-driven tests

### Manual Execution Verification

Each TODO includes verification procedures using:
- `curl` for API endpoint testing
- `make test` for unit/integration tests
- Manual database inspection for data verification
- Log file inspection for execution traces

---

## Task Flow

```
Phase 1: Foundation
    1 (Types) → 2 (Migrations) → 3 (Event Bus)
                                      ↓
Phase 2: Triggers
    4 (Hook Registry) ←───────────────┘
         ↓
    5 (DB Hooks) → 6 (Auth Hooks)
         ↓
    7 (Webhooks) → 8 (Scheduler)
                        ↓
Phase 3: Observability
    9 (Execution Logs) ←────────────────┘
         ↓
Phase 4: SDK & Integration
    10 (Manifest v2) → 11 (API Handlers) → 12 (SDK Generator)
         ↓
    13 (Integration Tests)
```

## Parallelization

| Group | Tasks | Reason |
|-------|-------|--------|
| A | 5, 6 | DB hooks and auth hooks are independent |
| B | 7, 8 | Webhooks and scheduler are independent |

| Task | Depends On | Reason |
|------|------------|--------|
| 3 | 1, 2 | Event bus needs types and DB tables |
| 4 | 3 | Hook registry uses event bus |
| 5 | 4 | DB hooks register with hook registry |
| 7 | 4 | Webhooks register with hook registry |
| 8 | 4 | Scheduler uses hook registry for triggers |
| 9 | 3 | Execution logs integrate with event bus |
| 11 | 5, 7, 8, 9 | API handlers need all components |
| 12 | 11 | SDK generation needs API spec |

---

## TODOs

### Phase 1: Foundation

- [x] 1. Define core event system types

  **What to do**:
  - Create `internal/events/types.go` with Event, EventType, EventMetadata structs
  - Create `internal/hooks/types.go` with Hook, HookMode, HookConfig structs
  - Create `internal/webhooks/types.go` with WebhookEndpoint, WebhookVerification structs
  - Create `internal/scheduler/types.go` with Schedule, ScheduleType, ScheduleConfig structs
  - Create `internal/executions/types.go` with ExecutionLog, ExecutionStatus structs
  - Define EventType constants: http, webhook, database, auth, schedule, custom
  - Define HookMode constants: sync, async
  - Define ScheduleType constants: cron, interval, one_time
  - Define ExecutionStatus constants: pending, running, success, failed, timed_out, cancelled

  **Must NOT do**:
  - No business logic in types files
  - No database operations

  **Parallelizable**: NO (foundation for all other tasks)

  **References**:
  - `internal/functions/types.go` - Existing type patterns (Runtime, Container, FunctionRequest)
  - `internal/database/errors.go` - Error type patterns (ConstraintError)

  **Acceptance Criteria**:
  - [ ] `go build ./internal/events/...` compiles
  - [ ] `go build ./internal/hooks/...` compiles
  - [ ] `go build ./internal/webhooks/...` compiles
  - [ ] `go build ./internal/scheduler/...` compiles
  - [ ] `go build ./internal/executions/...` compiles
  - [ ] All types have godoc comments ending with period

  **Commit**: YES
  - Message: `feat(events): add core type definitions for event-driven functions`
  - Files: `internal/events/types.go`, `internal/hooks/types.go`, `internal/webhooks/types.go`, `internal/scheduler/types.go`, `internal/executions/types.go`

---

- [x] 2. Create database migrations for event system tables

  **What to do**:
  - Add migration to `internal/database/migrations/` for events table
  - Add migration for hooks table
  - Add migration for webhook_endpoints table
  - Add migration for schedules table
  - Add migration for executions table
  - Create indexes for common query patterns
  - Add foreign key constraints where appropriate

  **Schema**:
  ```sql
  -- Events queue
  CREATE TABLE events (
      id TEXT PRIMARY KEY,
      type TEXT NOT NULL,
      source TEXT,
      action TEXT,
      payload TEXT NOT NULL,
      metadata TEXT,
      created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
      process_at DATETIME,
      processed_at DATETIME,
      status TEXT NOT NULL DEFAULT 'pending'
  );
  CREATE INDEX idx_events_status ON events(status, process_at);
  CREATE INDEX idx_events_type ON events(type, source, action);

  -- Hooks
  CREATE TABLE hooks (
      id TEXT PRIMARY KEY,
      name TEXT NOT NULL,
      function_id TEXT NOT NULL,
      event_type TEXT NOT NULL,
      event_source TEXT,
      event_action TEXT,
      mode TEXT NOT NULL DEFAULT 'async',
      priority INTEGER NOT NULL DEFAULT 0,
      config TEXT,
      enabled INTEGER NOT NULL DEFAULT 1,
      created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
      updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
  );
  CREATE INDEX idx_hooks_event ON hooks(event_type, event_source, event_action);
  CREATE INDEX idx_hooks_function ON hooks(function_id);

  -- Webhook endpoints
  CREATE TABLE webhook_endpoints (
      id TEXT PRIMARY KEY,
      path TEXT NOT NULL UNIQUE,
      function_id TEXT NOT NULL,
      methods TEXT NOT NULL,
      verification TEXT,
      enabled INTEGER NOT NULL DEFAULT 1,
      created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
  );

  -- Schedules
  CREATE TABLE schedules (
      id TEXT PRIMARY KEY,
      name TEXT NOT NULL,
      function_id TEXT NOT NULL,
      type TEXT NOT NULL,
      expression TEXT NOT NULL,
      timezone TEXT DEFAULT 'UTC',
      next_run DATETIME,
      last_run DATETIME,
      last_status TEXT,
      enabled INTEGER NOT NULL DEFAULT 1,
      config TEXT,
      created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
      updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
  );
  CREATE INDEX idx_schedules_next_run ON schedules(enabled, next_run);

  -- Execution logs
  CREATE TABLE executions (
      id TEXT PRIMARY KEY,
      function_id TEXT NOT NULL,
      request_id TEXT NOT NULL,
      trigger_type TEXT NOT NULL,
      trigger_id TEXT,
      status TEXT NOT NULL DEFAULT 'pending',
      started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
      completed_at DATETIME,
      duration_ms INTEGER,
      input TEXT,
      output TEXT,
      error TEXT,
      logs TEXT
  );
  CREATE INDEX idx_executions_function ON executions(function_id, started_at DESC);
  CREATE INDEX idx_executions_status ON executions(status);
  CREATE INDEX idx_executions_trigger ON executions(trigger_type, trigger_id);
  ```

  **Must NOT do**:
  - No data migrations (schema only)
  - No dropping existing tables

  **Parallelizable**: NO (depends on task 1 for type definitions to match)

  **References**:
  - `internal/database/migrations/migrations.go` - Existing migration pattern
  - `internal/database/database.go:53` - How migrations are run

  **Acceptance Criteria**:
  - [ ] `make test` passes (migrations run successfully)
  - [ ] `sqlite3 data/alyx.db ".schema events"` shows events table
  - [ ] `sqlite3 data/alyx.db ".schema hooks"` shows hooks table
  - [ ] `sqlite3 data/alyx.db ".schema webhook_endpoints"` shows webhook_endpoints table
  - [ ] `sqlite3 data/alyx.db ".schema schedules"` shows schedules table
  - [ ] `sqlite3 data/alyx.db ".schema executions"` shows executions table

  **Commit**: YES
  - Message: `feat(database): add migrations for event system tables`
  - Files: `internal/database/migrations/migrations.go`

---

- [x] 3. Implement event bus with SQLite backing

  **What to do**:
  - Create `internal/events/bus.go` with EventBus struct
  - Implement `Publish(ctx, event)` to insert event into queue
  - Implement `Subscribe(eventType, source, action, handler)` for in-memory subscriptions
  - Implement `ProcessPending(ctx)` to poll and dispatch pending events
  - Implement `ProcessScheduled(ctx)` to poll events with `process_at` in past
  - Create `internal/events/store.go` for database operations
  - Add event retention cleanup (configurable, default 7 days)
  - Use zerolog for structured logging

  **Event Processing Flow**:
  1. Trigger publishes event → INSERT into events table
  2. Worker polls for pending events (status='pending', process_at <= now)
  3. For each event:
     - Update status to 'processing'
     - Find matching hooks
     - Execute handlers
     - Update status to 'completed' or 'failed'

  **Must NOT do**:
  - No distributed queue features
  - No exactly-once delivery (at-least-once is fine)
  - No ordering guarantees between events

  **Parallelizable**: NO (depends on tasks 1, 2)

  **References**:
  - `internal/functions/pool.go` - Background worker pattern (Pool.cleanupLoop)
  - `internal/functions/token.go` - TTL cleanup pattern (cleanupLoop)
  - `internal/database/database.go` - DB operation patterns

  **Acceptance Criteria**:
  - [ ] `go build ./internal/events/...` compiles
  - [ ] Unit test: Publish event → event appears in DB
  - [ ] Unit test: Subscribe and ProcessPending → handler called
  - [ ] Unit test: Cleanup removes old events

  **Commit**: YES
  - Message: `feat(events): implement SQLite-backed event bus`
  - Files: `internal/events/bus.go`, `internal/events/store.go`

---

### Phase 2: Triggers

- [ ] 4. Implement hook registry

  **What to do**:
  - Create `internal/hooks/registry.go` with Registry struct
  - Implement `Register(hook)` to store hook in database
  - Implement `Unregister(hookID)` to remove hook
  - Implement `FindByEvent(eventType, source, action)` to find matching hooks
  - Implement `FindByFunction(functionID)` to list function's hooks
  - Add in-memory cache with cache invalidation on changes
  - Implement hook matching logic (exact match + wildcards)
  - Create `internal/hooks/store.go` for database operations

  **Hook Matching Rules**:
  - `event_source: "*"` matches any source
  - `event_action: "*"` matches any action
  - Multiple hooks can match same event (sorted by priority)

  **Must NOT do**:
  - No hook execution (that's event bus's job)
  - No cycle detection (handled separately)

  **Parallelizable**: NO (depends on task 3)

  **References**:
  - `internal/functions/discovery.go` - Registry pattern (Registry struct, Get, List)
  - `internal/functions/token.go` - In-memory cache with cleanup

  **Acceptance Criteria**:
  - [ ] `go build ./internal/hooks/...` compiles
  - [ ] Unit test: Register hook → hook in DB and cache
  - [ ] Unit test: FindByEvent returns matching hooks sorted by priority
  - [ ] Unit test: Wildcard matching works correctly

  **Commit**: YES
  - Message: `feat(hooks): implement hook registry with SQLite persistence`
  - Files: `internal/hooks/registry.go`, `internal/hooks/store.go`

---

- [ ] 5. Implement database event hooks

  **What to do**:
  - Create `internal/hooks/database.go` with DatabaseHookTrigger
  - Integrate with collection CRUD operations (`internal/database/collection.go`)
  - Emit events on INSERT, UPDATE, DELETE operations
  - For sync hooks: Execute inline, support reject/continue on failure
  - For async hooks: Publish to event bus
  - Pass document data, previous document (for update), and operation context
  - Implement cycle detection (prevent function from triggering itself)

  **Event Payload Schema**:
  ```json
  {
    "document": { ... },
    "previous_document": { ... },  // null for INSERT
    "action": "insert|update|delete",
    "collection": "collection_name",
    "changed_fields": ["field1", "field2"]  // for UPDATE
  }
  ```

  **Sync Hook Behavior**:
  - Hook config has `on_failure: "reject" | "continue"`
  - Default timeout: 5s (configurable in hook config)
  - On reject: Return error to client, rollback transaction
  - On continue: Log error, proceed with operation

  **Must NOT do**:
  - No modification of document by sync hooks (read-only validation)
  - No triggering hooks during hook execution (cycle prevention)

  **Parallelizable**: YES (with task 6)

  **References**:
  - `internal/database/collection.go` - CRUD operations to hook into
  - `internal/server/handlers/handlers.go:CreateDocument` - How CRUD is called

  **Acceptance Criteria**:
  - [ ] `go build ./internal/hooks/...` compiles
  - [ ] Integration test: INSERT triggers async hook
  - [ ] Integration test: UPDATE triggers hook with previous_document
  - [ ] Integration test: Sync hook reject prevents INSERT
  - [ ] Integration test: Cycle detection prevents infinite loop

  **Commit**: YES
  - Message: `feat(hooks): implement database event triggers with sync/async modes`
  - Files: `internal/hooks/database.go`, `internal/database/collection.go`

---

- [ ] 6. Implement auth event hooks

  **What to do**:
  - Create `internal/hooks/auth.go` with AuthHookTrigger
  - Integrate with auth operations (`internal/auth/service.go`)
  - Emit events for: signup, login, logout, password_reset, email_verify
  - Support sync hooks for signup (can reject registration)
  - Pass user data and auth context in payload

  **Event Payload Schema**:
  ```json
  {
    "user": {
      "id": "...",
      "email": "...",
      "created_at": "..."
    },
    "action": "signup|login|logout|password_reset|email_verify",
    "metadata": {
      "ip": "...",
      "user_agent": "..."
    }
  }
  ```

  **Must NOT do**:
  - No modification of user data by hooks
  - No OAuth callback hooks (future scope)

  **Parallelizable**: YES (with task 5)

  **References**:
  - `internal/auth/service.go` - Auth service to hook into
  - `internal/server/handlers/auth.go` - How auth handlers work

  **Acceptance Criteria**:
  - [ ] `go build ./internal/hooks/...` compiles
  - [ ] Integration test: User signup triggers async hook
  - [ ] Integration test: Sync hook reject prevents signup
  - [ ] Integration test: Login event includes user data

  **Commit**: YES
  - Message: `feat(hooks): implement auth event triggers`
  - Files: `internal/hooks/auth.go`, `internal/auth/service.go`

---

- [ ] 7. Implement webhook handler

  **What to do**:
  - Create `internal/webhooks/handler.go` with WebhookHandler
  - Create `internal/webhooks/store.go` for database operations
  - Create `internal/webhooks/verification.go` for HMAC verification
  - Implement endpoint registration (path, methods, verification config)
  - Implement request verification (HMAC-SHA256, HMAC-SHA1)
  - Implement route matching for custom webhook paths
  - Pass raw request body and headers to function

  **Verification Flow**:
  1. Extract signature from configured header
  2. Compute HMAC of request body with secret
  3. Constant-time compare
  4. If `skip_invalid: true`, pass verification result to function
  5. If `skip_invalid: false`, reject with 401

  **Event Payload Schema**:
  ```json
  {
    "method": "POST",
    "path": "/webhooks/stripe",
    "headers": { ... },
    "body": "raw body string",
    "query": { ... },
    "verified": true,
    "webhook_id": "..."
  }
  ```

  **Must NOT do**:
  - No response transformation (function returns response directly)
  - No request body parsing (pass raw)

  **Parallelizable**: YES (with task 8)

  **References**:
  - `internal/server/handlers/functions.go` - Invoke pattern
  - `internal/server/router.go` - Route registration pattern
  - Stripe webhook verification: https://stripe.com/docs/webhooks/signatures

  **Acceptance Criteria**:
  - [ ] `go build ./internal/webhooks/...` compiles
  - [ ] Unit test: HMAC-SHA256 verification passes with correct signature
  - [ ] Unit test: HMAC-SHA256 verification fails with incorrect signature
  - [ ] Integration test: Webhook endpoint receives POST and triggers function
  - [ ] `curl -X POST http://localhost:8090/webhooks/{id} -d '{"test":1}'` returns function response

  **Commit**: YES
  - Message: `feat(webhooks): implement incoming webhook handler with HMAC verification`
  - Files: `internal/webhooks/handler.go`, `internal/webhooks/store.go`, `internal/webhooks/verification.go`

---

- [ ] 8. Implement scheduler

  **What to do**:
  - Create `internal/scheduler/scheduler.go` with Scheduler struct
  - Create `internal/scheduler/store.go` for database operations
  - Create `internal/scheduler/cron.go` for cron expression parsing (use robfig/cron)
  - Implement schedule types: cron, interval, one_time
  - Implement timezone support using time.LoadLocation
  - Implement next run calculation
  - Implement scheduler loop (poll schedules, emit events when due)
  - Implement concurrency control (skip_if_running, max_overlap)

  **Scheduler Loop**:
  1. Poll schedules where `enabled=1 AND next_run <= now`
  2. For each schedule:
     - Check concurrency limits
     - Publish schedule event to event bus
     - Calculate and update next_run
     - Update last_run

  **Schedule Config**:
  ```yaml
  config:
    skip_if_running: true   # Skip if previous execution still running
    max_overlap: 1          # Max concurrent executions
    retry_on_failure: false # Retry if function fails
    max_retries: 3
    input: { ... }          # Static input for scheduled runs
  ```

  **Must NOT do**:
  - No sub-second scheduling
  - No complex calendar expressions (just standard cron)

  **Parallelizable**: YES (with task 7)

  **References**:
  - `internal/functions/pool.go` - Background loop pattern
  - `github.com/robfig/cron/v3` - Cron parsing library
  - Windmill scheduler: Uses similar poll-based approach

  **Acceptance Criteria**:
  - [ ] `go build ./internal/scheduler/...` compiles
  - [ ] Unit test: Cron expression parsing for "0 2 * * *"
  - [ ] Unit test: Interval parsing for "5m", "1h", "30s"
  - [ ] Unit test: Timezone calculation correct across DST
  - [ ] Integration test: Schedule fires at correct time
  - [ ] Integration test: skip_if_running prevents overlap

  **Commit**: YES
  - Message: `feat(scheduler): implement custom scheduler with CRON/interval/one-time support`
  - Files: `internal/scheduler/scheduler.go`, `internal/scheduler/store.go`, `internal/scheduler/cron.go`

---

### Phase 3: Observability

- [ ] 9. Implement execution logging

  **What to do**:
  - Create `internal/executions/logger.go` with ExecutionLogger
  - Create `internal/executions/store.go` for database operations
  - Integrate with function service to log all executions
  - Capture: trigger type, input, output, error, logs, duration
  - Implement log streaming via WebSocket (optional enhancement)
  - Implement retention cleanup (configurable, default 30 days)
  - Add execution status tracking (pending → running → success/failed)

  **Integration Points**:
  - `internal/functions/executor.go:Invoke` - Wrap execution with logging
  - `internal/events/bus.go` - Log event processing

  **Execution Flow**:
  1. Before execution: Create execution record (status=pending)
  2. Start execution: Update status=running
  3. After execution: Update status, output, error, duration
  4. Capture function logs from response

  **Must NOT do**:
  - No real-time log streaming (v2 feature)
  - No log aggregation/analysis

  **Parallelizable**: NO (depends on task 3)

  **References**:
  - `internal/functions/executor.go` - Service.Invoke method
  - `internal/functions/types.go:FunctionResponse` - Logs field

  **Acceptance Criteria**:
  - [ ] `go build ./internal/executions/...` compiles
  - [ ] Integration test: Function execution creates log entry
  - [ ] Integration test: Log entry contains input, output, duration
  - [ ] Integration test: Retention cleanup removes old logs
  - [ ] `curl http://localhost:8090/api/executions` returns execution history

  **Commit**: YES
  - Message: `feat(executions): implement persistent execution logging`
  - Files: `internal/executions/logger.go`, `internal/executions/store.go`

---

### Phase 4: SDK & Integration

- [ ] 10. Enhance function manifest schema

  **What to do**:
  - Update `internal/functions/discovery.go` to parse enhanced manifest
  - Add hooks section to manifest schema
  - Add schedules section to manifest schema
  - Add routes section to manifest schema
  - Implement manifest validation
  - Auto-register hooks/schedules/routes on function discovery

  **Enhanced Manifest Schema**:
  ```yaml
  name: on-user-created
  runtime: node
  timeout: 30s
  memory: 256mb

  routes:
    - path: /api/users/sync
      methods: [POST]
    - path: /api/users/{id}/avatar
      methods: [GET, PUT]

  hooks:
    - type: database
      source: users
      action: insert
      mode: async
      
    - type: auth
      action: signup
      mode: sync
      config:
        on_failure: reject
        timeout: 5s
      
    - type: webhook
      path: /webhooks/stripe
      methods: [POST]
      verification:
        type: hmac-sha256
        header: Stripe-Signature
        secret: ${STRIPE_WEBHOOK_SECRET}

  schedules:
    - name: daily-cleanup
      type: cron
      expression: "0 2 * * *"
      timezone: America/New_York
      config:
        skip_if_running: true
        
    - name: health-check
      type: interval
      expression: "5m"

  env:
    OPENAI_API_KEY: ${OPENAI_API_KEY}
  ```

  **Must NOT do**:
  - No breaking changes to existing manifest format
  - No auto-migration of existing manifests

  **Parallelizable**: NO (depends on tasks 4, 7, 8)

  **References**:
  - `internal/functions/discovery.go` - Current manifest parsing
  - `internal/functions/discovery.go:FunctionManifest` - Current manifest struct

  **Acceptance Criteria**:
  - [ ] `go build ./internal/functions/...` compiles
  - [ ] Unit test: Parse manifest with hooks section
  - [ ] Unit test: Parse manifest with schedules section
  - [ ] Unit test: Parse manifest with routes section
  - [ ] Integration test: Function discovery auto-registers hooks
  - [ ] Integration test: Existing simple manifests still work

  **Commit**: YES
  - Message: `feat(functions): enhance manifest schema with hooks, schedules, routes`
  - Files: `internal/functions/discovery.go`, `internal/functions/manifest.go`

---

- [ ] 11. Implement API handlers for event system

  **What to do**:
  - Create `internal/server/handlers/hooks.go` for hook CRUD
  - Create `internal/server/handlers/webhooks.go` for webhook endpoint CRUD
  - Create `internal/server/handlers/schedules.go` for schedule CRUD
  - Create `internal/server/handlers/executions.go` for execution logs
  - Update `internal/server/router.go` to register new routes
  - Add OpenAPI documentation for all endpoints

  **New Routes**:
  ```
  # Hooks
  GET    /api/hooks                    - List all hooks
  POST   /api/hooks                    - Create hook
  GET    /api/hooks/{id}               - Get hook
  PATCH  /api/hooks/{id}               - Update hook
  DELETE /api/hooks/{id}               - Delete hook
  GET    /api/functions/{name}/hooks   - List hooks for function

  # Webhooks
  POST   /webhooks/{id}                - Receive webhook (invoke)
  GET    /api/webhooks                 - List webhook endpoints
  POST   /api/webhooks                 - Create webhook endpoint
  GET    /api/webhooks/{id}            - Get webhook endpoint
  DELETE /api/webhooks/{id}            - Delete webhook endpoint

  # Schedules
  GET    /api/schedules                - List schedules
  POST   /api/schedules                - Create schedule
  GET    /api/schedules/{id}           - Get schedule
  PATCH  /api/schedules/{id}           - Update schedule
  DELETE /api/schedules/{id}           - Delete schedule
  POST   /api/schedules/{id}/trigger   - Trigger immediately

  # Executions
  GET    /api/executions               - List executions (with filters)
  GET    /api/executions/{id}          - Get execution details
  GET    /api/functions/{name}/executions - Executions for function
  ```

  **Must NOT do**:
  - No breaking changes to existing function endpoints
  - No authentication changes (use existing patterns)

  **Parallelizable**: NO (depends on tasks 5, 7, 8, 9)

  **References**:
  - `internal/server/handlers/functions.go` - Handler patterns
  - `internal/server/handlers/response.go` - Response helpers
  - `internal/server/router.go` - Route registration

  **Acceptance Criteria**:
  - [ ] `go build ./internal/server/...` compiles
  - [ ] `curl http://localhost:8090/api/hooks` returns 200
  - [ ] `curl http://localhost:8090/api/webhooks` returns 200
  - [ ] `curl http://localhost:8090/api/schedules` returns 200
  - [ ] `curl http://localhost:8090/api/executions` returns 200
  - [ ] OpenAPI spec includes all new endpoints

  **Commit**: YES
  - Message: `feat(api): add handlers for hooks, webhooks, schedules, executions`
  - Files: `internal/server/handlers/hooks.go`, `internal/server/handlers/webhooks.go`, `internal/server/handlers/schedules.go`, `internal/server/handlers/executions.go`, `internal/server/router.go`

---

- [ ] 12. Implement TypeScript SDK generator

  **What to do**:
  - Create `cmd/alyx/generate/sdk.go` for SDK generation command
  - Create `internal/sdk/typescript/` for TypeScript generator
  - Create templates for TypeScript SDK structure
  - Generate types from OpenAPI spec
  - Generate client methods for collections, auth, functions
  - Generate hook helpers (event types, payload types)
  - Include Alyx context helpers for runtime use

  **SDK Structure**:
  ```
  dist/
    index.ts           - Main exports
    client.ts          - AlyxClient class
    types/
      collections.ts   - Collection types
      auth.ts          - Auth types
      functions.ts     - Function types
      events.ts        - Event payload types
    resources/
      collections.ts   - Collection operations
      auth.ts          - Auth operations
      functions.ts     - Function operations
      events.ts        - Event emission
    context.ts         - Function runtime context
  ```

  **Generated Code Example**:
  ```typescript
  // client.ts
  export class AlyxClient {
    constructor(config: AlyxConfig) { ... }
    
    collections = {
      users: new CollectionClient<User>(this, 'users'),
      posts: new CollectionClient<Post>(this, 'posts'),
    };
    
    auth = new AuthClient(this);
    functions = new FunctionsClient(this);
    events = new EventsClient(this);
  }
  
  // context.ts (for function runtime)
  export function getContext(): FunctionContext {
    return {
      alyx: new AlyxClient({
        url: process.env.ALYX_URL,
        token: process.env.ALYX_INTERNAL_TOKEN,
      }),
      auth: JSON.parse(process.env.ALYX_AUTH || 'null'),
      env: process.env,
    };
  }
  ```

  **Must NOT do**:
  - No bundling/minification (user's responsibility)
  - No npm publishing (local generation only)

  **Parallelizable**: NO (depends on task 11)

  **References**:
  - `internal/openapi/spec.go` - OpenAPI spec generation
  - Appwrite SDK pattern: https://github.com/appwrite/sdk-generator

  **Acceptance Criteria**:
  - [ ] `alyx generate sdk --lang typescript --output ./sdk` works
  - [ ] Generated SDK compiles with `tsc`
  - [ ] SDK can create document: `alyx.collections.users.create({...})`
  - [ ] SDK can invoke function: `alyx.functions.invoke('hello', {...})`
  - [ ] SDK works in function runtime with context helpers

  **Commit**: YES
  - Message: `feat(sdk): implement TypeScript SDK generator`
  - Files: `cmd/alyx/generate/sdk.go`, `internal/sdk/typescript/generator.go`, `internal/sdk/typescript/templates/`

---

- [ ] 13. Write integration tests and documentation

  **What to do**:
  - Create integration tests for complete event flow
  - Create integration tests for webhook signature verification
  - Create integration tests for schedule execution
  - Create integration tests for execution logging
  - Update README with event system documentation
  - Add examples to `examples/` directory

  **Test Scenarios**:
  1. Database hook flow: Create document → hook triggered → execution logged
  2. Webhook flow: POST to webhook → signature verified → function invoked
  3. Schedule flow: Wait for schedule → function invoked → next_run updated
  4. SDK flow: Generate SDK → compile → invoke function

  **Must NOT do**:
  - No performance benchmarks (future scope)
  - No load testing

  **Parallelizable**: NO (depends on all previous tasks)

  **References**:
  - `internal/database/database_test.go` - Integration test patterns
  - `internal/functions/discovery_test.go` - Function test patterns

  **Acceptance Criteria**:
  - [ ] `make test` passes with >80% coverage on new code
  - [ ] Integration test: Complete database hook flow works
  - [ ] Integration test: Complete webhook flow works
  - [ ] Integration test: Complete schedule flow works
  - [ ] README documents all new features
  - [ ] Example function with hooks exists in `examples/`

  **Commit**: YES
  - Message: `test(events): add integration tests and documentation`
  - Files: `internal/events/*_test.go`, `internal/hooks/*_test.go`, `internal/webhooks/*_test.go`, `internal/scheduler/*_test.go`, `internal/executions/*_test.go`, `README.md`, `examples/`

---

## Commit Strategy

| After Task | Message | Files | Verification |
|------------|---------|-------|--------------|
| 1 | `feat(events): add core type definitions` | `internal/*/types.go` | `go build ./...` |
| 2 | `feat(database): add migrations for event system` | `internal/database/migrations/` | `make test` |
| 3 | `feat(events): implement SQLite-backed event bus` | `internal/events/` | `make test` |
| 4 | `feat(hooks): implement hook registry` | `internal/hooks/` | `make test` |
| 5 | `feat(hooks): implement database event triggers` | `internal/hooks/`, `internal/database/` | `make test` |
| 6 | `feat(hooks): implement auth event triggers` | `internal/hooks/`, `internal/auth/` | `make test` |
| 7 | `feat(webhooks): implement webhook handler` | `internal/webhooks/` | `make test` |
| 8 | `feat(scheduler): implement custom scheduler` | `internal/scheduler/` | `make test` |
| 9 | `feat(executions): implement execution logging` | `internal/executions/` | `make test` |
| 10 | `feat(functions): enhance manifest schema` | `internal/functions/` | `make test` |
| 11 | `feat(api): add event system handlers` | `internal/server/` | `make test` |
| 12 | `feat(sdk): implement TypeScript SDK generator` | `cmd/alyx/generate/`, `internal/sdk/` | `make test` |
| 13 | `test(events): add integration tests` | `*_test.go`, `README.md` | `make test` |

---

## Success Criteria

### Verification Commands
```bash
# All tests pass
make test

# Lint passes
make lint

# Server starts without errors
./build/alyx dev

# API endpoints respond
curl http://localhost:8090/api/hooks
curl http://localhost:8090/api/webhooks
curl http://localhost:8090/api/schedules
curl http://localhost:8090/api/executions

# SDK generates
./build/alyx generate sdk --lang typescript --output ./sdk
cd sdk && npm install && npx tsc
```

### Final Checklist
- [ ] All 13 tasks completed
- [ ] All "Must Have" features present
- [ ] All "Must NOT Have" guardrails respected
- [ ] `make test` passes with >80% coverage
- [ ] `make lint` passes with zero issues
- [ ] README updated with event system documentation
- [ ] Example functions with hooks in `examples/`
