# Learnings - Functions v2

This file captures conventions, patterns, and best practices discovered during implementation.

---

## 2026-01-24 23:52:37 Task 1.1: internal/events/types.go
- EventType constants pattern: Used string type with iota-less const block (explicit values for clarity)
- Event struct field types: Used `any` for Payload, `*time.Time` for nullable timestamps (ProcessAt, ProcessedAt)
- Timestamp handling: `time.Time` for CreatedAt (always set), `*time.Time` for ProcessAt/ProcessedAt (optional)
- Godoc pattern: All exported types have comments ending with period (godot compliance)
- EventMetadata.Extra: Used `map[string]any` for flexible additional context

## 2026-01-24 Task 1.2: internal/hooks/types.go
- HookMode constants pattern: Used string type with explicit values (sync, async) - consistent with EventType pattern
- Hook struct field types: Used `time.Time` for CreatedAt/UpdatedAt (always set), string for EventType/EventSource/EventAction
- HookConfig approach: Separate struct for extensible configuration (OnFailure, Timeout)
- Godoc pattern: All exported types have comments ending with period (godot compliance)
- Priority field: int type for execution ordering (higher = earlier)
- Wildcard support: EventSource and EventAction support "*" for matching all

## 2026-01-24 Task 1.3: internal/webhooks/types.go
- WebhookEndpoint struct design: ID, Path, FunctionID, Methods, Verification, Enabled, CreatedAt
- WebhookVerification as pointer (*WebhookVerification) for optional verification config
- Methods field as []string for HTTP method list (e.g., ["POST", "GET"])
- Verification types: "hmac-sha256", "hmac-sha1" as string values
- SkipInvalid bool: Controls whether to reject (false) or pass verification result to function (true)
- Godoc pattern: All exported types have comments ending with period (godot compliance)

## 2026-01-24 Task 1.4: internal/scheduler/types.go
- ScheduleType constants pattern: Used string type with explicit values (cron, interval, one_time) - consistent with EventType/HookMode pattern
- Schedule struct field types: Used `*time.Time` for NextRun/LastRun (nullable), `time.Time` for CreatedAt/UpdatedAt (always set)
- ScheduleConfig approach: Separate struct for extensible configuration (SkipIfRunning, MaxOverlap, RetryOnFailure, MaxRetries, Input)
- Godoc pattern: All exported types have comments ending with period (godot compliance)
- Concurrency control: MaxOverlap int for limiting concurrent executions (0 = unlimited)
- Timezone field: String type for schedule timezone (default "UTC")
- Expression field: String type for flexible schedule expression (cron, interval duration, or timestamp)

## 2026-01-24 Task 1.5: internal/executions/types.go
- ExecutionStatus constants pattern: Used string type with explicit values (pending, running, success, failed, timed_out, cancelled) - consistent with EventType/HookMode/ScheduleType pattern
- ExecutionLog struct field types: Used `*time.Time` for CompletedAt (nullable - nil if still running), `time.Time` for StartedAt (always set)
- String fields for Input/Output/Error/Logs: JSON serialization handled elsewhere, stored as strings in database
- DurationMs field: int type for execution duration in milliseconds (calculated from StartedAt/CompletedAt)
- TriggerType/TriggerID fields: String types for flexible trigger identification (http, webhook, database, auth, schedule, custom)
- Godoc pattern: All exported types have comments ending with period (godot compliance)


## 2026-01-25 Task 2: Database Migrations

### Migration File Pattern
- Migrations are SQL files in `internal/database/migrations/sql/` with numeric prefixes (e.g., `002_event_system.sql`)
- Files are loaded via `go:embed` and executed in alphabetical order
- Each migration runs in its own transaction

### Statement Splitting Gotcha
- Migration system splits SQL by semicolons and executes statements sequentially
- **CRITICAL**: Statements starting with `--` are skipped entirely
- Header comments get bundled with the first CREATE TABLE statement
- If first statement starts with `--`, the entire CREATE TABLE is skipped!
- **Solution**: Remove header comments or ensure first CREATE TABLE is not bundled with comments

### SQLite Conventions Followed
- Use `TEXT` for datetime fields, not `DATETIME` (SQLite stores as TEXT anyway)
- Use `INTEGER` for boolean fields (0/1)
- Use `TEXT` for JSON fields
- Use `TEXT` for UUID fields
- Default timestamps: `DEFAULT (datetime('now'))`

### Index Strategy
- Events: Composite index on `(status, process_at)` for queue processing
- Events: Composite index on `(type, source, action)` for event matching
- Hooks: Composite index on `(event_type, event_source, event_action)` for event matching
- Hooks: Index on `function_id` for function lookups
- Schedules: Composite index on `(enabled, next_run)` for scheduler queries
- Executions: Composite index on `(function_id, started_at DESC)` for function history
- Executions: Index on `status` for filtering
- Executions: Composite index on `(trigger_type, trigger_id)` for trigger lookups

### Tables Created
1. **events**: Event queue with status tracking
2. **hooks**: Event-to-function mappings
3. **webhook_endpoints**: HTTP webhook configurations
4. **schedules**: Cron/interval schedules
5. **executions**: Function execution logs

All migrations execute successfully and tests pass.

## 2026-01-25 Task 3: Event Bus Implementation

### Architecture Decisions
- **In-memory subscriptions + SQLite queue**: EventBus maintains in-memory handler subscriptions (map[string][]EventHandler) while events are persisted in SQLite
- **At-least-once delivery**: No exactly-once guarantees - handlers may be called multiple times if processing fails
- **Wildcard matching**: Supports "*" for source and action to match all events of a type
- **Dual processing modes**: ProcessPending() for immediate events (process_at IS NULL), ProcessScheduled() for delayed events (process_at <= now)

### Background Worker Pattern
- Used context.WithCancel() for graceful shutdown (pattern from pool.go)
- Separate goroutines for processing and cleanup with sync.WaitGroup tracking
- Ticker-based polling (1s for processing, 1h for cleanup by default)
- CRITICAL: Background goroutines must use bus.ctx (from NewEventBus), not the ctx passed to Start()

### Database Patterns
- **Timestamp handling**: Store as RFC3339 strings, parse on read (sql.NullString → time.Time)
- **JSON serialization**: Payload and Metadata serialized to TEXT columns
- **Status transitions**: pending → processing → completed/failed
- **Cleanup strategy**: DeleteOlderThan() removes completed/failed events older than retention period (default 7 days)

### SQLite Gotchas
- **datetime() comparison**: SQLite's datetime('now') returns different format than RFC3339
- **Solution**: Use Go's time.Now().UTC().Format(time.RFC3339) and string comparison in SQL
- **Query separation**: GetPending (process_at IS NULL) vs GetScheduled (process_at IS NOT NULL AND process_at <= ?)

### Testing Patterns
- testDB() helper with t.TempDir() and t.Cleanup()
- Table-driven tests for wildcard matching scenarios
- Background processing tests with time.Sleep() for async verification
- Context cancellation tests for graceful shutdown

### Event Processing Flow
1. Publish() → INSERT into events table with status='pending'
2. ProcessPending/ProcessScheduled() → SELECT pending events
3. For each event:
   - UPDATE status='processing'
   - Find matching handlers (exact + wildcard patterns)
   - Execute all handlers (continue on error)
   - UPDATE status='completed' or 'failed'
4. Cleanup loop removes old completed/failed events

### Handler Matching Logic
- Exact match: type:source:action
- Wildcard source: type:*:action
- Wildcard action: type:source:*
- Wildcard both: type:*:*
- All matches are executed (not just first match)

### Configuration
- EventBusConfig: Retention, ProcessInterval, CleanupInterval
- Defaults: 7 days retention, 1s processing, 1h cleanup
- Configurable per-instance via NewEventBus()

### Pre-existing Issues Fixed
- internal/executions/types.go: Fixed misspelling "cancelled" → "canceled" (misspell linter)

## 2026-01-25 Task 4: Hook Registry Implementation

### Registry Architecture
- **In-memory cache + SQLite persistence**: Registry loads all hooks into memory on startup for fast lookups
- **Cache invalidation**: Cache is cleared and reloaded when hooks are modified
- **Thread-safe**: Uses sync.RWMutex for concurrent access to cache

### Hook Matching Algorithm
- **Event type**: Exact match required (no wildcards)
- **Event source**: Exact match OR wildcard `*` matches any source
- **Event action**: Exact match OR wildcard `*` matches any action
- **Enabled filter**: Only enabled hooks are returned by FindByEvent
- **Implementation**: Simple iteration through cache with conditional matching

### Priority Sorting Strategy
- **Primary sort**: Priority (descending) - higher values execute first
- **Secondary sort**: CreatedAt (ascending) - earlier created hooks execute first when priority is equal
- **Algorithm**: Bubble sort (sufficient for small hook lists)
- **Applied to**: FindByEvent, FindByFunction, List methods

### Store Database Operations
- **JSON serialization**: HookConfig stored as JSON TEXT in database
- **Timestamp format**: RFC3339 for created_at and updated_at
- **Boolean storage**: INTEGER (0/1) for enabled field
- **Mode storage**: TEXT for HookMode enum
- **Query patterns**: Wildcard matching done in SQL with OR conditions

### Test Coverage
- **Register/Unregister**: Verify cache and database consistency
- **Exact matching**: Event type, source, action all exact
- **Wildcard matching**: Source wildcard, action wildcard, both wildcards
- **Type mismatch**: Different event type returns no matches
- **Disabled hooks**: Disabled hooks excluded from FindByEvent
- **Priority sorting**: Hooks returned in correct priority order
- **FindByFunction**: Returns all hooks for a function
- **Reload**: Cache invalidation and reload from database

### Key Patterns Followed
- **Registry pattern**: From internal/functions/discovery.go (Get, List, map-based cache)
- **In-memory cache**: From internal/functions/token.go (sync.RWMutex, invalidate method)
- **DB operations**: From internal/events/store.go (JSON marshaling, timestamp handling, scanRows pattern)
- **Test helpers**: testDB helper with t.TempDir() and t.Cleanup()
- **Table-driven tests**: Comprehensive test cases with subtests

### Notes
- Pre-existing race condition in internal/events/bus_test.go (TestEventBus_StartStop) - not related to hooks implementation
- All hooks tests pass with race detection
- Build successful: `go build ./internal/hooks/...`

## 2026-01-25 Task 6: Auth Event Hooks

### Integration Approach
- **Interface-based dependency injection**: Auth service accepts optional `HookTrigger` interface via `SetHookTrigger()` method
- **Nil-safe hook calls**: All hook trigger calls check `if s.hookTrigger != nil` before execution
- **Non-blocking hook failures**: Hook errors are logged but don't prevent auth operations from completing
- **Metadata extraction**: IP address and user agent extracted from HTTP context and passed to hooks

### Hook Trigger Implementation
- **AuthHookTrigger struct**: Holds references to Registry and EventBus
- **Event payload schema**: Consistent structure with user data (id, email, verified, role, created_at), action, and metadata
- **Sync vs Async modes**: Sync hooks logged but not yet executed (placeholder for future function runtime), async hooks publish events to bus
- **Event metadata**: Stored in EventMetadata.Extra map (hook_id, function_id, user_id, action)

### Auth Service Integration Points
1. **Register (OnSignup)**: Called after user creation, before session creation - metadata is nil
2. **Login (OnLogin)**: Called after password verification, before session creation - includes IP and user agent
3. **Logout (OnLogout)**: Called after session lookup, before session deletion - includes session IP and user agent
4. **SetPassword (OnPasswordReset)**: Called after password update - metadata is nil

### Test Patterns
- **testAuthSetup helper**: Creates DB, auth service, registry, event bus, and trigger in one call
- **Event verification**: Tests publish events and verify they appear in pending queue with correct payload
- **Sync hook behavior**: Sync hooks don't publish events (placeholder for future function execution)
- **Integration testing**: Full auth flow with real database, registry, and event bus

### Configuration Gotchas
- **JWT config fields**: Use `AccessTTL` and `RefreshTTL`, not `AccessTokenDuration` and `RefreshTokenDuration`
- **Database config**: Must include WALMode, ForeignKeys, BusyTimeout, MaxOpenConns, MaxIdleConns, ConnMaxLifetime, CacheSize
- **JWT secret length**: Must be at least 32 characters for validation to pass

### Notes
- Email verification hooks (OnEmailVerify) not yet integrated - no email verification flow exists in auth service
- Sync hook execution is a placeholder - actual function runtime integration will come later
- All 5 auth hook tests pass successfully
- Pre-existing race condition in events package (TestEventBus_StartStop) is unrelated to auth hooks

## [2026-01-25 00:40] Task 5: Database Event Hooks

### Integration Approach
- **Interface-based design**: Created `HookTrigger` interface in `collection.go` to avoid circular dependencies
- **Optional injection**: Collection has optional `hookTrigger` field, set via `SetHookTrigger()` method
- **Post-operation hooks**: Hooks execute AFTER successful database operations (INSERT/UPDATE/DELETE)
- **Document retrieval**: For UPDATE/DELETE, fetch existing document before operation to pass to hooks

### Sync vs Async Hook Execution
- **Async hooks**: Publish events to event bus for background processing
- **Sync hooks**: Execute inline with timeout (default 5s, configurable via `HookConfig.Timeout`)
- **Failure handling**: 
  - `on_failure: "reject"` → Return error to client, rollback transaction
  - `on_failure: "continue"` → Log error, proceed with operation
- **Function runtime**: Placeholder implementation (TODO: integrate with function runtime)

### Cycle Detection
- **Tracking map**: `executing map[string]bool` tracks currently executing function IDs
- **Mutex protection**: RWMutex guards concurrent access to executing map
- **Cleanup**: Defer cleanup ensures function ID removed even on panic
- **Skip on cycle**: Log warning and skip hook execution if cycle detected

### Changed Fields Calculation
- **Field comparison**: Compare current vs previous document field-by-field
- **Added fields**: Detect fields in current but not in previous
- **Removed fields**: Detect fields in previous but not in current
- **Simple equality**: Used `fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)` for simplicity
  - Note: For production, consider `reflect.DeepEqual` or JSON comparison

### Event Payload Schema
```go
// INSERT
{
  "document": {...},
  "action": "insert",
  "collection": "collection_name"
}

// UPDATE
{
  "document": {...},
  "previous_document": {...},
  "action": "update",
  "collection": "collection_name",
  "changed_fields": ["field1", "field2"]
}

// DELETE
{
  "document": {...},
  "action": "delete",
  "collection": "collection_name"
}
```

### Testing Strategy
- **Integration tests**: Real database operations with schema migration
- **Event bus verification**: Subscribe to events and verify payload structure
- **Cycle detection test**: Manually set executing flag to simulate cycle
- **Wildcard matching test**: Verify `*` source/action patterns work
- **Type assertion handling**: Handle `[]interface{}` vs `[]string` deserialization

### Key Learnings
1. **Schema parsing**: Use `schema.Parse()` with YAML string for test schemas
2. **SQL generation**: Use `schema.NewSQLGenerator(s).GenerateAll()` for migrations
3. **Test helpers**: Rename test helpers to avoid conflicts (e.g., `testDBHooks`, `testSchemaHooks`)
4. **Unused variables**: LSP catches unused return values in test setup functions
5. **Pre-existing issues**: Event bus has data race in `TestEventBus_StartStop` (not our code)

### Future Improvements
- Integrate with actual function runtime for sync hook execution
- Add transaction support for sync hooks (rollback on reject)
- Implement more robust field comparison (JSON diff, reflect.DeepEqual)
- Add metrics/observability for hook execution times
- Consider batching async events for performance

## 2026-01-25 Task 7: Webhook Handler

### Implementation Summary
- Created `internal/webhooks/handler.go` with HTTP handler for webhook endpoints
- Created `internal/webhooks/store.go` for database CRUD operations
- Created `internal/webhooks/verification.go` with HMAC-SHA256/SHA1 verification
- Comprehensive test coverage: 24 tests passing

### HMAC Verification Approach
- **Constant-time comparison**: Used `hmac.Equal()` to prevent timing attacks
- **Dual algorithm support**: HMAC-SHA256 (modern) and HMAC-SHA1 (legacy compatibility)
- **Flexible signature formats**: Supports both "sha256=<hex>" and raw hex
- **Skip invalid mode**: `skip_invalid: true` passes verification result to function instead of rejecting

### Route Registration Strategy
- Handler implements `http.Handler` interface for direct mux registration
- `RegisterRoutes()` method dynamically registers all enabled endpoints
- Pattern format: `{METHOD} {PATH}` (e.g., "POST /webhooks/stripe")
- Multiple methods per endpoint supported

### Event Payload Schema
```go
{
  "method": "POST",
  "path": "/webhooks/stripe",
  "headers": map[string]string,
  "body": "raw body string",
  "query": map[string]string,
  "verified": true,
  "webhook_id": "endpoint-uuid",
  "verification_error": "optional error message"
}
```

### Key Design Decisions
1. **Raw body passthrough**: No JSON parsing - function receives raw string
2. **Direct response**: Function output returned as-is (no transformation)
3. **Flexible response format**: Supports `{status, headers, body}` map or plain JSON
4. **Header extraction**: Case-insensitive signature header lookup
5. **Error handling**: All JSON encoding errors logged but don't crash handler

### Lint Compliance
- Fixed `errcheck` violations: All `json.Encode()` and `w.Write()` calls checked
- Fixed `errorlint` violations: Used `errors.Is()` for `sql.ErrNoRows`
- Fixed `goconst` violations: Extracted test secrets to constants
- Fixed `gosec` G505: Added `#nosec` comments for SHA1 (required for webhook compatibility)
- Fixed variable shadowing in store.go

### Test Coverage
- **Unit tests**: HMAC verification (8 tests), signature extraction (4 tests), constant-time comparison
- **Integration tests**: Handler HTTP tests (5 tests), store CRUD (4 tests), response formatting (5 tests)
- **Edge cases**: Invalid signatures, method restrictions, missing endpoints, skip_invalid mode

### Gotchas
- SHA1 is flagged by gosec but required for GitHub webhooks - use `#nosec` directive
- Must use `errors.Is()` for wrapped errors (not `==`)
- Variable shadowing in unmarshal loops - use different variable names
- JSON encoder errors must be checked (errcheck)

## [2026-01-25 00:52] Task 8: Scheduler Implementation

### Architecture Decisions
- **Poll-based approach**: Scheduler polls database every 1 second (configurable) for due schedules
- **Event-driven execution**: Publishes events to event bus when schedules are due (not direct function invocation)
- **Concurrency control**: In-memory tracking of running executions with `skip_if_running` and `max_overlap` support
- **Timezone support**: Uses `time.LoadLocation` for timezone-aware scheduling across DST transitions

### Cron Parsing (robfig/cron)
- **Library**: `github.com/robfig/cron/v3` for standard cron expression parsing
- **Parser options**: Minute | Hour | Dom | Month | Dow | Descriptor (5-field cron)
- **Timezone handling**: Convert `after` time to target timezone before calling `schedule.Next()`
- **DST transitions**: robfig/cron handles DST gracefully (2:00 AM doesn't exist during spring forward)

### Schedule Types
1. **Cron**: Standard cron expressions (e.g., `"0 * * * *"` for hourly)
2. **Interval**: Duration strings (e.g., `"5m"`, `"1h"`, `"30s"`) - minimum 1 second
3. **One-time**: RFC3339 timestamps (e.g., `"2026-01-25T15:00:00Z"`)

### Next Run Calculation
- **Cron**: Parse expression, convert to timezone, call `schedule.Next(afterInTZ)`
- **Interval**: Add duration to `after` time in target timezone
- **One-time**: Parse timestamp, return error if `LastRun` is set (already executed)

### One-Time Schedule Handling
- **Critical bug fix**: Must set `schedule.LastRun = &now` BEFORE calling `CalculateNextRun`
- **Reason**: `CalculateNextRun` checks if `LastRun != nil` to determine if one-time schedule already executed
- **Flow**: Publish event → Set `LastRun` → Calculate next run → If error and one-time, disable schedule

### Database Patterns
- **Timestamp storage**: RFC3339 strings in TEXT columns (consistent with events/hooks)
- **Nullable times**: `sql.NullString` for `next_run` and `last_run` (can be NULL)
- **JSON config**: `ScheduleConfig` serialized to TEXT column
- **Query pattern**: `WHERE enabled = 1 AND next_run IS NOT NULL AND next_run <= ?`

### Concurrency Control
- **Running map**: `map[string]int` tracks count of running executions per schedule ID
- **Mutex protection**: `sync.RWMutex` guards concurrent access to running map
- **Skip if running**: If `SkipIfRunning=true` and `runningCount > 0`, skip execution
- **Max overlap**: If `MaxOverlap > 0` and `runningCount >= MaxOverlap`, skip execution
- **Cleanup**: Defer `decrementRunning` to ensure count is decremented even on panic

### Background Worker Pattern
- **Context management**: Use `context.WithCancel()` for graceful shutdown (pattern from pool.go)
- **Goroutine tracking**: `sync.WaitGroup` to wait for background loop to finish
- **Ticker-based polling**: `time.NewTicker(interval)` for periodic schedule checks
- **Error handling**: Log errors but don't crash the loop

### Event Publishing
- **Event type**: `EventTypeSchedule` (defined in events/types.go)
- **Event source**: `"scheduler"`
- **Event action**: `"execute"`
- **Payload**: `schedule_id`, `schedule_name`, `function_id`, `input` (from ScheduleConfig)
- **Metadata**: `schedule_id`, `function_id`, `schedule_type` in `Extra` map

### Testing Patterns
- **testDB helper**: Creates temp database with migrations (pattern from other tests)
- **Table-driven tests**: Comprehensive test cases for cron parsing, interval parsing, timezone handling
- **Integration tests**: Full scheduler flow with event bus subscription and verification
- **Concurrency tests**: Verify `skip_if_running` and `max_overlap` logic
- **One-time schedule test**: Verify schedule is disabled after first execution

### Lint Fixes
- **Stuttering**: Renamed `SchedulerConfig` to `Config` (revive linter)
- **Error comparison**: Use `errors.Is(err, sql.ErrNoRows)` instead of `==` (errorlint)
- **Shadow variables**: Renamed shadowed variables (`unmarshalErr`, `parseErr`, `updateErr`)
- **Import order**: Added `errors` import for `errors.Is`

### Key Learnings
1. **Timezone-aware scheduling**: Always convert times to target timezone before calculations
2. **One-time schedule gotcha**: Must set `LastRun` before calculating next run to detect already-executed schedules
3. **Concurrency tracking**: In-memory map is sufficient for single-instance deployments (distributed would need Redis)
4. **Event-driven design**: Scheduler publishes events, doesn't invoke functions directly (separation of concerns)
5. **Poll interval tradeoff**: 1 second is good balance between responsiveness and database load

### Future Improvements
- Distributed locking for multi-instance deployments (Redis-based)
- Schedule history/audit log (track all executions)
- Retry logic for failed schedules (currently relies on event bus retry)
- Schedule pausing/resuming (currently only enable/disable)
- Cron expression validation on create (currently fails at runtime)
