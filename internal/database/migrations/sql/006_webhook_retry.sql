CREATE TABLE IF NOT EXISTS _alyx_webhook_queue (
    id TEXT PRIMARY KEY,
    webhook_id TEXT,
    endpoint_url TEXT NOT NULL,
    payload TEXT NOT NULL,
    headers TEXT,
    attempt INTEGER NOT NULL DEFAULT 0,
    next_retry_at TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'retrying', 'failed', 'succeeded')),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_webhook_queue_retry ON _alyx_webhook_queue(next_retry_at, status);
CREATE INDEX IF NOT EXISTS idx_webhook_queue_status ON _alyx_webhook_queue(status);

CREATE TABLE IF NOT EXISTS _alyx_webhook_dlq (
    id TEXT PRIMARY KEY,
    webhook_id TEXT,
    endpoint_url TEXT NOT NULL,
    payload TEXT NOT NULL,
    headers TEXT,
    attempts INTEGER NOT NULL,
    last_error TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_webhook_dlq_created ON _alyx_webhook_dlq(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhook_dlq_webhook ON _alyx_webhook_dlq(webhook_id);
