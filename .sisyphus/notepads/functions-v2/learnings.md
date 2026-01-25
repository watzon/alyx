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

