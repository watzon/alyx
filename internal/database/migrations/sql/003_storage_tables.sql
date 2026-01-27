CREATE TABLE IF NOT EXISTS _alyx_files (
    id TEXT PRIMARY KEY,
    bucket TEXT NOT NULL,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    size INTEGER NOT NULL,
    checksum TEXT,
    compressed BOOLEAN DEFAULT FALSE,
    compression_type TEXT,
    original_size INTEGER,
    metadata TEXT,
    version INTEGER DEFAULT 1,
    created_at TEXT,
    updated_at TEXT,
    UNIQUE(bucket, path)
);

CREATE INDEX IF NOT EXISTS idx_files_bucket ON _alyx_files(bucket);

CREATE TABLE IF NOT EXISTS _alyx_uploads (
    id TEXT PRIMARY KEY,
    bucket TEXT NOT NULL,
    filename TEXT,
    size INTEGER NOT NULL,
    offset INTEGER DEFAULT 0,
    metadata TEXT,
    expires_at TEXT,
    created_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_uploads_expires_at ON _alyx_uploads(expires_at);
