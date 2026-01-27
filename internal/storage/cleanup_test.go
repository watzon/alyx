package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

func setupCleanupTest(t *testing.T) (*CleanupService, *TUSService, *database.DB, string) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	storagePath := filepath.Join(tempDir, "storage")

	cfg := &config.DatabaseConfig{
		Path:         dbPath,
		WALMode:      true,
		ForeignKeys:  true,
		CacheSize:    -2000,
		BusyTimeout:  5 * time.Second,
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	}

	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("failed to close database: %v", closeErr)
		}
	})

	s := &schema.Schema{
		Buckets: map[string]*schema.Bucket{
			"uploads": {
				Name:        "uploads",
				Backend:     "filesystem",
				MaxFileSize: 100 * 1024 * 1024,
			},
		},
	}

	backend := NewFilesystemBackend(storagePath)
	backends := map[string]Backend{
		"filesystem": backend,
	}

	appCfg := &config.Config{}

	tusService := NewTUSService(db, backends, s, appCfg, tempDir)
	tusStore := NewTUSStore(db)
	cleanupService := NewCleanupService(tusStore, tempDir, 100*time.Millisecond)

	return cleanupService, tusService, db, tempDir
}

func TestCleanupServiceRunOnce(t *testing.T) {
	cleanupService, tusService, db, tempDir := setupCleanupTest(t)
	ctx := context.Background()

	// Create an expired upload with partial file
	upload, err := tusService.CreateUpload(ctx, "uploads", 1024, nil)
	if err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	// Upload partial chunk to create temp file
	chunk := bytes.NewReader([]byte("hello"))
	_, err = tusService.UploadChunk(ctx, "uploads", upload.ID, 0, chunk, 5)
	if err != nil {
		t.Fatalf("UploadChunk failed: %v", err)
	}

	// Verify temp file exists
	tempPath := filepath.Join(tempDir, "tus", upload.ID)
	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		t.Fatal("temp file should exist after partial upload")
	}

	// Expire the upload
	query := `UPDATE _alyx_uploads SET expires_at = ? WHERE id = ?`
	expiredTime := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	_, err = db.ExecContext(ctx, query, expiredTime, upload.ID)
	if err != nil {
		t.Fatalf("failed to expire upload: %v", err)
	}

	// Run cleanup once
	deleted, err := cleanupService.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("expected 1 deleted upload, got %d", deleted)
	}

	// Verify temp file is deleted
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("temp file should be deleted after cleanup")
	}

	// Verify upload record is deleted
	_, err = tusService.GetUploadOffset(ctx, "uploads", upload.ID)
	if err == nil {
		t.Error("expected error for deleted upload, got nil")
	}
}

func TestCleanupServiceActiveUploadsNotDeleted(t *testing.T) {
	cleanupService, tusService, _, tempDir := setupCleanupTest(t)
	ctx := context.Background()

	// Create an active upload (not expired)
	upload, err := tusService.CreateUpload(ctx, "uploads", 1024, nil)
	if err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	// Upload partial chunk to create temp file
	chunk := bytes.NewReader([]byte("hello"))
	_, err = tusService.UploadChunk(ctx, "uploads", upload.ID, 0, chunk, 5)
	if err != nil {
		t.Fatalf("UploadChunk failed: %v", err)
	}

	// Verify temp file exists
	tempPath := filepath.Join(tempDir, "tus", upload.ID)
	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		t.Fatal("temp file should exist after partial upload")
	}

	// Run cleanup
	deleted, err := cleanupService.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	if deleted != 0 {
		t.Errorf("expected 0 deleted uploads, got %d", deleted)
	}

	// Verify temp file still exists
	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		t.Error("temp file should still exist for active upload")
	}

	// Verify upload record still exists
	offset, err := tusService.GetUploadOffset(ctx, "uploads", upload.ID)
	if err != nil {
		t.Errorf("expected upload to still exist, got error: %v", err)
	}
	if offset != 5 {
		t.Errorf("expected offset 5, got %d", offset)
	}
}

func TestCleanupServiceMultipleExpiredUploads(t *testing.T) {
	cleanupService, tusService, db, tempDir := setupCleanupTest(t)
	ctx := context.Background()

	// Create 3 expired uploads
	uploadIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		upload, err := tusService.CreateUpload(ctx, "uploads", 1024, nil)
		if err != nil {
			t.Fatalf("CreateUpload %d failed: %v", i, err)
		}
		uploadIDs[i] = upload.ID

		// Upload partial chunk
		chunk := bytes.NewReader([]byte("hello"))
		_, err = tusService.UploadChunk(ctx, "uploads", upload.ID, 0, chunk, 5)
		if err != nil {
			t.Fatalf("UploadChunk %d failed: %v", i, err)
		}

		// Expire the upload
		query := `UPDATE _alyx_uploads SET expires_at = ? WHERE id = ?`
		expiredTime := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
		_, err = db.ExecContext(ctx, query, expiredTime, upload.ID)
		if err != nil {
			t.Fatalf("failed to expire upload %d: %v", i, err)
		}
	}

	// Run cleanup
	deleted, err := cleanupService.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	if deleted != 3 {
		t.Errorf("expected 3 deleted uploads, got %d", deleted)
	}

	// Verify all temp files are deleted
	for i, uploadID := range uploadIDs {
		tempPath := filepath.Join(tempDir, "tus", uploadID)
		if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
			t.Errorf("temp file %d should be deleted after cleanup", i)
		}
	}
}

func TestCleanupServiceBackgroundExecution(t *testing.T) {
	cleanupService, tusService, db, tempDir := setupCleanupTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create an expired upload
	upload, err := tusService.CreateUpload(ctx, "uploads", 1024, nil)
	if err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	// Upload partial chunk
	chunk := bytes.NewReader([]byte("hello"))
	_, err = tusService.UploadChunk(ctx, "uploads", upload.ID, 0, chunk, 5)
	if err != nil {
		t.Fatalf("UploadChunk failed: %v", err)
	}

	// Expire the upload
	query := `UPDATE _alyx_uploads SET expires_at = ? WHERE id = ?`
	expiredTime := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	_, err = db.ExecContext(ctx, query, expiredTime, upload.ID)
	if err != nil {
		t.Fatalf("failed to expire upload: %v", err)
	}

	// Start cleanup service
	cleanupService.Start(ctx)

	// Wait for cleanup to run (interval is 100ms in test setup)
	time.Sleep(300 * time.Millisecond)

	// Stop cleanup service
	cleanupService.Stop()

	// Verify temp file is deleted
	tempPath := filepath.Join(tempDir, "tus", upload.ID)
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("temp file should be deleted after background cleanup")
	}

	// Verify upload record is deleted
	_, err = tusService.GetUploadOffset(ctx, "uploads", upload.ID)
	if err == nil {
		t.Error("expected error for deleted upload, got nil")
	}
}

func TestCleanupServiceContextCancellation(t *testing.T) {
	cleanupService, _, _, _ := setupCleanupTest(t)
	ctx, cancel := context.WithCancel(context.Background())

	// Start cleanup service
	cleanupService.Start(ctx)

	// Cancel context immediately
	cancel()

	// Stop should complete quickly
	done := make(chan struct{})
	go func() {
		cleanupService.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success - cleanup stopped
	case <-time.After(1 * time.Second):
		t.Error("cleanup service did not stop within 1 second after context cancellation")
	}
}

func TestCleanupServicePartialFileRemoval(t *testing.T) {
	cleanupService, tusService, db, tempDir := setupCleanupTest(t)
	ctx := context.Background()

	// Create upload and partial file
	upload, err := tusService.CreateUpload(ctx, "uploads", 1024, nil)
	if err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	// Upload partial chunk
	chunk := bytes.NewReader([]byte("test data"))
	_, err = tusService.UploadChunk(ctx, "uploads", upload.ID, 0, chunk, 9)
	if err != nil {
		t.Fatalf("UploadChunk failed: %v", err)
	}

	tempPath := filepath.Join(tempDir, "tus", upload.ID)

	// Verify file exists and has content
	stat, err := os.Stat(tempPath)
	if err != nil {
		t.Fatalf("temp file should exist: %v", err)
	}
	if stat.Size() != 9 {
		t.Errorf("expected file size 9, got %d", stat.Size())
	}

	// Expire the upload
	query := `UPDATE _alyx_uploads SET expires_at = ? WHERE id = ?`
	expiredTime := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	_, err = db.ExecContext(ctx, query, expiredTime, upload.ID)
	if err != nil {
		t.Fatalf("failed to expire upload: %v", err)
	}

	// Run cleanup
	deleted, err := cleanupService.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("expected 1 deleted upload, got %d", deleted)
	}

	// Verify file is completely removed
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("temp file should be completely removed")
	}
}
