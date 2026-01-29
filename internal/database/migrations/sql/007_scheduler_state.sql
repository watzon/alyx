CREATE TABLE IF NOT EXISTS _alyx_scheduler_state (
    schedule_id TEXT PRIMARY KEY REFERENCES schedules(id) ON DELETE CASCADE,
    last_execution_at TEXT,
    next_execution_at TEXT,
    execution_count INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_scheduler_state_next_exec ON _alyx_scheduler_state(next_execution_at);
