package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

func setupTUSTest(t *testing.T) (*TUSService, *database.DB, string) {
	t.Helper()

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	storagePath := filepath.Join(tempDir, "storage")

	cfg := &config.DatabaseConfig{
		Path: dbPath,
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

	return tusService, db, tempDir
}

func TestTUSCreateUpload(t *testing.T) {
	tusService, _, _ := setupTUSTest(t)
	ctx := context.Background()

	metadata := map[string]string{
		"filename": "test.txt",
		"filetype": "text/plain",
	}

	upload, err := tusService.CreateUpload(ctx, "uploads", 1024, metadata)
	if err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	if upload.ID == "" {
		t.Error("upload ID is empty")
	}
	if upload.Bucket != "uploads" {
		t.Errorf("expected bucket 'uploads', got %q", upload.Bucket)
	}
	if upload.Size != 1024 {
		t.Errorf("expected size 1024, got %d", upload.Size)
	}
	if upload.Offset != 0 {
		t.Errorf("expected offset 0, got %d", upload.Offset)
	}
	if upload.Metadata["filename"] != "test.txt" {
		t.Errorf("expected filename 'test.txt', got %q", upload.Metadata["filename"])
	}
	if upload.ExpiresAt.IsZero() {
		t.Error("ExpiresAt is zero")
	}
}

func TestTUSGetUploadOffset(t *testing.T) {
	tusService, _, _ := setupTUSTest(t)
	ctx := context.Background()

	// Create upload
	upload, err := tusService.CreateUpload(ctx, "uploads", 1024, nil)
	if err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	// Get offset
	offset, err := tusService.GetUploadOffset(ctx, "uploads", upload.ID)
	if err != nil {
		t.Fatalf("GetUploadOffset failed: %v", err)
	}

	if offset != 0 {
		t.Errorf("expected offset 0, got %d", offset)
	}
}

func TestTUSUploadChunk(t *testing.T) {
	tusService, _, _ := setupTUSTest(t)
	ctx := context.Background()

	// Create upload for 10 bytes
	upload, err := tusService.CreateUpload(ctx, "uploads", 10, nil)
	if err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	// Upload first chunk (5 bytes)
	chunk1 := bytes.NewReader([]byte("hello"))
	newOffset, err := tusService.UploadChunk(ctx, "uploads", upload.ID, 0, chunk1, 5)
	if err != nil {
		t.Fatalf("UploadChunk failed: %v", err)
	}
	if newOffset != 5 {
		t.Errorf("expected offset 5, got %d", newOffset)
	}

	// Upload second chunk (5 bytes)
	chunk2 := bytes.NewReader([]byte("world"))
	newOffset, err = tusService.UploadChunk(ctx, "uploads", upload.ID, 5, chunk2, 5)
	if err != nil {
		t.Fatalf("UploadChunk failed: %v", err)
	}
	if newOffset != 10 {
		t.Errorf("expected offset 10, got %d", newOffset)
	}

	// Verify upload is complete (should be moved to _alyx_files)
	offset, err := tusService.GetUploadOffset(ctx, "uploads", upload.ID)
	if err == nil {
		t.Errorf("expected upload to be deleted after completion, but got offset %d", offset)
	}
}

func TestTUSUploadChunkInvalidOffset(t *testing.T) {
	tusService, _, _ := setupTUSTest(t)
	ctx := context.Background()

	// Create upload
	upload, err := tusService.CreateUpload(ctx, "uploads", 10, nil)
	if err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	// Try to upload with wrong offset
	chunk := bytes.NewReader([]byte("hello"))
	_, err = tusService.UploadChunk(ctx, "uploads", upload.ID, 5, chunk, 5)
	if err == nil {
		t.Error("expected error for invalid offset, got nil")
	}
	if !strings.Contains(err.Error(), "offset mismatch") {
		t.Errorf("expected 'offset mismatch' error, got: %v", err)
	}
}

func TestTUSCancelUpload(t *testing.T) {
	tusService, _, tempDir := setupTUSTest(t)
	ctx := context.Background()

	// Create upload
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

	// Verify temp file exists
	tempPath := filepath.Join(tempDir, "tus", upload.ID)
	if _, err := os.Stat(tempPath); os.IsNotExist(err) {
		t.Error("temp file should exist after partial upload")
	}

	// Cancel upload
	err = tusService.CancelUpload(ctx, "uploads", upload.ID)
	if err != nil {
		t.Fatalf("CancelUpload failed: %v", err)
	}

	// Verify temp file is deleted
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("temp file should be deleted after cancel")
	}

	// Verify upload record is deleted
	_, err = tusService.GetUploadOffset(ctx, "uploads", upload.ID)
	if err == nil {
		t.Error("expected error for deleted upload, got nil")
	}
}

func TestTUSResumeUpload(t *testing.T) {
	tusService, _, _ := setupTUSTest(t)
	ctx := context.Background()

	// Create upload for 15 bytes
	upload, err := tusService.CreateUpload(ctx, "uploads", 15, nil)
	if err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	// Upload first chunk (5 bytes)
	chunk1 := bytes.NewReader([]byte("hello"))
	offset1, err := tusService.UploadChunk(ctx, "uploads", upload.ID, 0, chunk1, 5)
	if err != nil {
		t.Fatalf("UploadChunk failed: %v", err)
	}
	if offset1 != 5 {
		t.Errorf("expected offset 5, got %d", offset1)
	}

	// Simulate disconnect - query offset
	offset, err := tusService.GetUploadOffset(ctx, "uploads", upload.ID)
	if err != nil {
		t.Fatalf("GetUploadOffset failed: %v", err)
	}
	if offset != 5 {
		t.Errorf("expected offset 5 after resume, got %d", offset)
	}

	// Resume with second chunk (5 bytes)
	chunk2 := bytes.NewReader([]byte("world"))
	offset2, err := tusService.UploadChunk(ctx, "uploads", upload.ID, 5, chunk2, 5)
	if err != nil {
		t.Fatalf("UploadChunk failed: %v", err)
	}
	if offset2 != 10 {
		t.Errorf("expected offset 10, got %d", offset2)
	}

	// Upload final chunk (5 bytes)
	chunk3 := bytes.NewReader([]byte("!!!!"))
	offset3, err := tusService.UploadChunk(ctx, "uploads", upload.ID, 10, chunk3, 4)
	if err != nil {
		t.Fatalf("UploadChunk failed: %v", err)
	}
	if offset3 != 14 {
		t.Errorf("expected offset 14, got %d", offset3)
	}

	// Upload last byte
	chunk4 := bytes.NewReader([]byte("!"))
	offset4, err := tusService.UploadChunk(ctx, "uploads", upload.ID, 14, chunk4, 1)
	if err != nil {
		t.Fatalf("UploadChunk failed: %v", err)
	}
	if offset4 != 15 {
		t.Errorf("expected offset 15, got %d", offset4)
	}

	// Verify upload is complete
	_, err = tusService.GetUploadOffset(ctx, "uploads", upload.ID)
	if err == nil {
		t.Error("expected upload to be deleted after completion")
	}
}

func TestTUSExpiredUploadCleanup(t *testing.T) {
	tusService, db, tempDir := setupTUSTest(t)
	ctx := context.Background()

	// Create upload
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

	query := `UPDATE _alyx_uploads SET expires_at = ? WHERE id = ?`
	expiredTime := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	_, err = db.ExecContext(ctx, query, expiredTime, upload.ID)
	if err != nil {
		t.Fatalf("failed to expire upload: %v", err)
	}

	// Run cleanup
	deleted, err := tusService.CleanupExpiredUploads(ctx)
	if err != nil {
		t.Fatalf("CleanupExpiredUploads failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted upload, got %d", deleted)
	}

	// Verify temp file is deleted
	tempPath := filepath.Join(tempDir, "tus", upload.ID)
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Error("temp file should be deleted after cleanup")
	}

	// Verify upload record is deleted
	_, err = tusService.GetUploadOffset(ctx, "uploads", upload.ID)
	if err == nil {
		t.Error("expected error for deleted upload, got nil")
	}
}

func TestTUSMetadataParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "single key-value",
			input: "filename " + base64.StdEncoding.EncodeToString([]byte("test.txt")),
			expected: map[string]string{
				"filename": "test.txt",
			},
		},
		{
			name: "multiple key-values",
			input: "filename " + base64.StdEncoding.EncodeToString([]byte("test.txt")) +
				",filetype " + base64.StdEncoding.EncodeToString([]byte("text/plain")),
			expected: map[string]string{
				"filename": "test.txt",
				"filetype": "text/plain",
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseTUSMetadata(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d keys, got %d", len(tt.expected), len(result))
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("expected %s=%q, got %q", k, v, result[k])
				}
			}
		})
	}
}

func TestTUSLargeFileUpload(t *testing.T) {
	tusService, _, _ := setupTUSTest(t)
	ctx := context.Background()

	// Create upload for 10MB
	fileSize := int64(10 * 1024 * 1024)
	upload, err := tusService.CreateUpload(ctx, "uploads", fileSize, nil)
	if err != nil {
		t.Fatalf("CreateUpload failed: %v", err)
	}

	// Upload in 3 chunks
	chunkSize := int64(5 * 1024 * 1024) // 5MB chunks
	offset := int64(0)

	// Chunk 1: 5MB
	chunk1 := bytes.NewReader(make([]byte, chunkSize))
	newOffset, err := tusService.UploadChunk(ctx, "uploads", upload.ID, offset, chunk1, chunkSize)
	if err != nil {
		t.Fatalf("UploadChunk 1 failed: %v", err)
	}
	if newOffset != chunkSize {
		t.Errorf("expected offset %d, got %d", chunkSize, newOffset)
	}
	offset = newOffset

	// Chunk 2: 5MB
	chunk2 := bytes.NewReader(make([]byte, chunkSize))
	newOffset, err = tusService.UploadChunk(ctx, "uploads", upload.ID, offset, chunk2, chunkSize)
	if err != nil {
		t.Fatalf("UploadChunk 2 failed: %v", err)
	}
	if newOffset != 2*chunkSize {
		t.Errorf("expected offset %d, got %d", 2*chunkSize, newOffset)
	}

	// Verify upload is complete
	_, err = tusService.GetUploadOffset(ctx, "uploads", upload.ID)
	if err == nil {
		t.Error("expected upload to be deleted after completion")
	}
}
