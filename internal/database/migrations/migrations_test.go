package migrations

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestRun(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Run migrations
	if err := Run(ctx, db); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify version table exists
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM _alyx_internal_versions").Scan(&count)
	if err != nil {
		t.Fatalf("version table query failed: %v", err)
	}

	// Should have applied all migrations
	if count == 0 {
		t.Error("expected at least one migration to be applied")
	}
}

func TestRun_Idempotent(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Run migrations twice
	if err := Run(ctx, db); err != nil {
		t.Fatalf("first Run() failed: %v", err)
	}

	if err := Run(ctx, db); err != nil {
		t.Fatalf("second Run() failed: %v", err)
	}

	// Verify migrations weren't duplicated
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM _alyx_internal_versions").Scan(&count)
	if err != nil {
		t.Fatalf("version table query failed: %v", err)
	}

	// Count should match number of migration files
	applied, err := GetApplied(ctx, db)
	if err != nil {
		t.Fatalf("GetApplied() failed: %v", err)
	}

	if len(applied) != count {
		t.Errorf("expected %d applied migrations, got %d", count, len(applied))
	}
}

func TestStorageTablesMigration(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Run all migrations
	if err := Run(ctx, db); err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify _alyx_files table exists
	var filesTableExists int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='table' AND name='_alyx_files'
	`).Scan(&filesTableExists)
	if err != nil {
		t.Fatalf("checking _alyx_files table: %v", err)
	}
	if filesTableExists != 1 {
		t.Error("_alyx_files table does not exist")
	}

	// Verify _alyx_uploads table exists
	var uploadsTableExists int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='table' AND name='_alyx_uploads'
	`).Scan(&uploadsTableExists)
	if err != nil {
		t.Fatalf("checking _alyx_uploads table: %v", err)
	}
	if uploadsTableExists != 1 {
		t.Error("_alyx_uploads table does not exist")
	}

	// Verify _alyx_files schema
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(_alyx_files)")
	if err != nil {
		t.Fatalf("getting _alyx_files schema: %v", err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("scanning column info: %v", err)
		}
		columns[name] = true
	}

	requiredColumns := []string{
		"id", "bucket", "name", "path", "mime_type", "size",
		"checksum", "compressed", "compression_type", "original_size",
		"metadata", "version", "created_at", "updated_at",
	}
	for _, col := range requiredColumns {
		if !columns[col] {
			t.Errorf("_alyx_files missing required column: %s", col)
		}
	}

	// Verify _alyx_uploads schema
	rows, err = db.QueryContext(ctx, "PRAGMA table_info(_alyx_uploads)")
	if err != nil {
		t.Fatalf("getting _alyx_uploads schema: %v", err)
	}
	defer rows.Close()

	columns = make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("scanning column info: %v", err)
		}
		columns[name] = true
	}

	requiredUploadsColumns := []string{
		"id", "bucket", "filename", "size", "offset",
		"metadata", "expires_at", "created_at",
	}
	for _, col := range requiredUploadsColumns {
		if !columns[col] {
			t.Errorf("_alyx_uploads missing required column: %s", col)
		}
	}

	// Verify indexes exist
	var filesIndexExists int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='index' AND name='idx_files_bucket'
	`).Scan(&filesIndexExists)
	if err != nil {
		t.Fatalf("checking idx_files_bucket: %v", err)
	}
	if filesIndexExists != 1 {
		t.Error("idx_files_bucket index does not exist")
	}

	var uploadsIndexExists int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sqlite_master 
		WHERE type='index' AND name='idx_uploads_expires_at'
	`).Scan(&uploadsIndexExists)
	if err != nil {
		t.Fatalf("checking idx_uploads_expires_at: %v", err)
	}
	if uploadsIndexExists != 1 {
		t.Error("idx_uploads_expires_at index does not exist")
	}
}
