package storage

import (
	"strings"
	"testing"
)

func TestFilesTableSQL(t *testing.T) {
	sql := FilesTableSQL()

	// Verify table name
	if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS _alyx_files") {
		t.Error("Expected table name _alyx_files")
	}

	// Verify all required fields
	requiredFields := []string{
		"id TEXT PRIMARY KEY",
		"bucket TEXT NOT NULL",
		"name TEXT NOT NULL",
		"path TEXT NOT NULL",
		"mime_type TEXT NOT NULL",
		"size INTEGER NOT NULL",
		"checksum TEXT",
		"compressed BOOLEAN DEFAULT FALSE",
		"compression_type TEXT",
		"original_size INTEGER",
		"metadata TEXT",
		"version INTEGER DEFAULT 1",
		"created_at TEXT",
		"updated_at TEXT",
	}

	for _, field := range requiredFields {
		if !strings.Contains(sql, field) {
			t.Errorf("Expected field definition: %s", field)
		}
	}

	// Verify unique constraint on (bucket, path)
	if !strings.Contains(sql, "UNIQUE(bucket, path)") {
		t.Error("Expected unique constraint on (bucket, path)")
	}
}

func TestFilesTableIndexes(t *testing.T) {
	indexes := FilesTableIndexes()

	if len(indexes) == 0 {
		t.Fatal("Expected at least one index")
	}

	// Verify bucket index exists
	foundBucketIndex := false
	for _, idx := range indexes {
		if strings.Contains(idx, "idx_files_bucket") && strings.Contains(idx, "ON _alyx_files(bucket)") {
			foundBucketIndex = true
			break
		}
	}

	if !foundBucketIndex {
		t.Error("Expected index on bucket column")
	}
}

func TestUploadsTableSQL(t *testing.T) {
	sql := UploadsTableSQL()

	// Verify table name
	if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS _alyx_uploads") {
		t.Error("Expected table name _alyx_uploads")
	}

	// Verify all required fields
	requiredFields := []string{
		"id TEXT PRIMARY KEY",
		"bucket TEXT NOT NULL",
		"filename TEXT",
		"size INTEGER NOT NULL",
		"offset INTEGER DEFAULT 0",
		"metadata TEXT",
		"expires_at TEXT",
		"created_at TEXT",
	}

	for _, field := range requiredFields {
		if !strings.Contains(sql, field) {
			t.Errorf("Expected field definition: %s", field)
		}
	}
}

func TestUploadsTableIndexes(t *testing.T) {
	indexes := UploadsTableIndexes()

	if len(indexes) == 0 {
		t.Fatal("Expected at least one index")
	}

	// Verify expires_at index exists (for cleanup queries)
	foundExpiresIndex := false
	for _, idx := range indexes {
		if strings.Contains(idx, "idx_uploads_expires_at") && strings.Contains(idx, "ON _alyx_uploads(expires_at)") {
			foundExpiresIndex = true
			break
		}
	}

	if !foundExpiresIndex {
		t.Error("Expected index on expires_at column for cleanup queries")
	}
}

func TestAllStorageTables(t *testing.T) {
	tables := AllStorageTables()

	if len(tables) != 2 {
		t.Fatalf("Expected 2 tables, got %d", len(tables))
	}

	// Verify both tables are present
	foundFiles := false
	foundUploads := false

	for _, table := range tables {
		if strings.Contains(table, "_alyx_files") {
			foundFiles = true
		}
		if strings.Contains(table, "_alyx_uploads") {
			foundUploads = true
		}
	}

	if !foundFiles {
		t.Error("Expected _alyx_files table in AllStorageTables()")
	}
	if !foundUploads {
		t.Error("Expected _alyx_uploads table in AllStorageTables()")
	}
}

func TestAllStorageIndexes(t *testing.T) {
	indexes := AllStorageIndexes()

	if len(indexes) < 2 {
		t.Fatalf("Expected at least 2 indexes, got %d", len(indexes))
	}

	// Verify both index types are present
	foundBucketIndex := false
	foundExpiresIndex := false

	for _, idx := range indexes {
		if strings.Contains(idx, "idx_files_bucket") {
			foundBucketIndex = true
		}
		if strings.Contains(idx, "idx_uploads_expires_at") {
			foundExpiresIndex = true
		}
	}

	if !foundBucketIndex {
		t.Error("Expected bucket index in AllStorageIndexes()")
	}
	if !foundExpiresIndex {
		t.Error("Expected expires_at index in AllStorageIndexes()")
	}
}
