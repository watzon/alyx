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
