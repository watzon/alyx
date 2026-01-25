CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    source TEXT,
    action TEXT,
    payload TEXT NOT NULL,
    metadata TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    process_at TEXT,
    processed_at TEXT,
    status TEXT NOT NULL DEFAULT 'pending'
);

CREATE INDEX IF NOT EXISTS idx_events_status ON events(status, process_at);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type, source, action);

CREATE TABLE IF NOT EXISTS hooks (
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
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_hooks_event ON hooks(event_type, event_source, event_action);
CREATE INDEX IF NOT EXISTS idx_hooks_function ON hooks(function_id);

CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    function_id TEXT NOT NULL,
    methods TEXT NOT NULL,
    verification TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS schedules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    function_id TEXT NOT NULL,
    type TEXT NOT NULL,
    expression TEXT NOT NULL,
    timezone TEXT DEFAULT 'UTC',
    next_run TEXT,
    last_run TEXT,
    last_status TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    config TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_schedules_next_run ON schedules(enabled, next_run);

CREATE TABLE IF NOT EXISTS executions (
    id TEXT PRIMARY KEY,
    function_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    trigger_type TEXT NOT NULL,
    trigger_id TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    started_at TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at TEXT,
    duration_ms INTEGER,
    input TEXT,
    output TEXT,
    error TEXT,
    logs TEXT
);

CREATE INDEX IF NOT EXISTS idx_executions_function ON executions(function_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_executions_status ON executions(status);
CREATE INDEX IF NOT EXISTS idx_executions_trigger ON executions(trigger_type, trigger_id);
