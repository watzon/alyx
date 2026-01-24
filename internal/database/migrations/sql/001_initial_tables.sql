-- Initial internal tables for Alyx

CREATE TABLE IF NOT EXISTS _alyx_migrations (
    id INTEGER PRIMARY KEY,
    version TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    applied_at TEXT NOT NULL DEFAULT (datetime('now')),
    checksum TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS _alyx_changes (
    id INTEGER PRIMARY KEY,
    collection TEXT NOT NULL,
    operation TEXT NOT NULL,
    doc_id TEXT NOT NULL,
    changed_fields TEXT,
    timestamp TEXT NOT NULL DEFAULT (datetime('now')),
    processed INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_changes_unprocessed ON _alyx_changes(processed, timestamp);

CREATE TABLE IF NOT EXISTS _alyx_users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    verified INTEGER NOT NULL DEFAULT 0,
    role TEXT NOT NULL DEFAULT 'user',
    metadata TEXT
);

CREATE TABLE IF NOT EXISTS _alyx_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES _alyx_users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    user_agent TEXT,
    ip_address TEXT
);

CREATE TABLE IF NOT EXISTS _alyx_oauth_accounts (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES _alyx_users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_user_id TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(provider, provider_user_id)
);

CREATE TABLE IF NOT EXISTS _alyx_deployments (
    id INTEGER PRIMARY KEY,
    version TEXT NOT NULL UNIQUE,
    schema_hash TEXT NOT NULL,
    functions_hash TEXT NOT NULL,
    schema_snapshot TEXT NOT NULL,
    functions_snapshot TEXT,
    deployed_at TEXT NOT NULL DEFAULT (datetime('now')),
    deployed_by TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    rollback_to TEXT,
    description TEXT
);

CREATE INDEX IF NOT EXISTS idx_deployments_status ON _alyx_deployments(status);
CREATE INDEX IF NOT EXISTS idx_deployments_deployed_at ON _alyx_deployments(deployed_at);

CREATE TABLE IF NOT EXISTS _alyx_admin_tokens (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    token_hash TEXT NOT NULL,
    permissions TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at TEXT,
    last_used_at TEXT,
    created_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_admin_tokens_name ON _alyx_admin_tokens(name);
