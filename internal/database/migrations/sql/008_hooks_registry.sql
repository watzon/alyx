CREATE TABLE IF NOT EXISTS _alyx_hooks (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL CHECK(type IN ('database', 'webhook', 'schedule')),
    source TEXT NOT NULL,
    action TEXT,
    function_name TEXT NOT NULL,
    mode TEXT NOT NULL CHECK(mode IN ('sync', 'async')),
    enabled INTEGER NOT NULL DEFAULT 1,
    config TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_hooks_type ON _alyx_hooks(type);
CREATE INDEX IF NOT EXISTS idx_hooks_enabled ON _alyx_hooks(enabled);
CREATE INDEX IF NOT EXISTS idx_hooks_source_action ON _alyx_hooks(type, source, action);
CREATE INDEX IF NOT EXISTS idx_hooks_function ON _alyx_hooks(function_name);
