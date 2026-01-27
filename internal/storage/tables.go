package storage

// FilesTableSQL returns the CREATE TABLE statement for _alyx_files.
// This table stores metadata for all uploaded files across all buckets.
func FilesTableSQL() string {
	return `
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
)`
}

// FilesTableIndexes returns CREATE INDEX statements for _alyx_files.
func FilesTableIndexes() []string {
	return []string{
		`CREATE INDEX IF NOT EXISTS idx_files_bucket ON _alyx_files(bucket)`,
	}
}

// UploadsTableSQL returns the CREATE TABLE statement for _alyx_uploads.
// This table stores TUS upload state for resumable uploads.
func UploadsTableSQL() string {
	return `
CREATE TABLE IF NOT EXISTS _alyx_uploads (
    id TEXT PRIMARY KEY,
    bucket TEXT NOT NULL,
    filename TEXT,
    size INTEGER NOT NULL,
    offset INTEGER DEFAULT 0,
    metadata TEXT,
    expires_at TEXT,
    created_at TEXT
)`
}

// UploadsTableIndexes returns CREATE INDEX statements for _alyx_uploads.
func UploadsTableIndexes() []string {
	return []string{
		`CREATE INDEX IF NOT EXISTS idx_uploads_expires_at ON _alyx_uploads(expires_at)`,
	}
}

// AllStorageTables returns all storage table CREATE statements.
func AllStorageTables() []string {
	return []string{
		FilesTableSQL(),
		UploadsTableSQL(),
	}
}

// AllStorageIndexes returns all storage index CREATE statements.
func AllStorageIndexes() []string {
	indexes := make([]string, 0)
	indexes = append(indexes, FilesTableIndexes()...)
	indexes = append(indexes, UploadsTableIndexes()...)
	return indexes
}
