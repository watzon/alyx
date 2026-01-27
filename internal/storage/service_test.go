package storage

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

func testService(t *testing.T) (*Service, Backend) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

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

	storagePath := filepath.Join(tmpDir, "storage")
	backend := NewFilesystemBackend(storagePath)

	backends := map[string]Backend{
		"local": backend,
	}

	s := &schema.Schema{
		Buckets: map[string]*schema.Bucket{
			"uploads": {
				Name:        "uploads",
				Backend:     "local",
				MaxFileSize: 10 * 1024 * 1024,
				AllowedTypes: []string{
					"text/plain",
					"image/*",
				},
			},
			"documents": {
				Name:    "documents",
				Backend: "local",
			},
		},
	}

	appCfg := &config.Config{}

	service := NewService(db, backends, s, appCfg, nil)

	return service, backend
}

func TestServiceUpload(t *testing.T) {
	service, _ := testService(t)
	ctx := context.Background()

	content := []byte("Hello, World!")
	r := bytes.NewReader(content)

	file, err := service.Upload(ctx, "uploads", "test.txt", r, int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if file.ID == "" {
		t.Error("File ID not set")
	}
	if file.Bucket != "uploads" {
		t.Errorf("Bucket = %s, want uploads", file.Bucket)
	}
	if file.Name != "test.txt" {
		t.Errorf("Name = %s, want test.txt", file.Name)
	}
	if file.MimeType != "text/plain; charset=utf-8" {
		t.Errorf("MimeType = %s, want text/plain; charset=utf-8", file.MimeType)
	}
	if file.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", file.Size, len(content))
	}
	if file.Checksum == "" {
		t.Error("Checksum not set")
	}
}

func TestServiceUploadSizeLimit(t *testing.T) {
	service, _ := testService(t)
	ctx := context.Background()

	content := make([]byte, 11*1024*1024)
	r := bytes.NewReader(content)

	_, err := service.Upload(ctx, "uploads", "large.bin", r, int64(len(content)))
	if err == nil {
		t.Error("Upload should fail for file exceeding size limit")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("Error message = %v, want size limit error", err)
	}
}

func TestServiceUploadMimeTypeValidation(t *testing.T) {
	service, _ := testService(t)
	ctx := context.Background()

	content := []byte("PK\x03\x04")
	r := bytes.NewReader(content)

	_, err := service.Upload(ctx, "uploads", "archive.zip", r, int64(len(content)))
	if err == nil {
		t.Fatal("Upload should fail for disallowed MIME type")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("Error message = %v, want MIME type error", err)
	}
}

func TestServiceUploadMimeTypeWildcard(t *testing.T) {
	service, _ := testService(t)
	ctx := context.Background()

	content := []byte("\x89PNG\r\n\x1a\n")
	r := bytes.NewReader(content)

	file, err := service.Upload(ctx, "uploads", "image.png", r, int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if !strings.HasPrefix(file.MimeType, "image/") {
		t.Errorf("MimeType = %s, want image/*", file.MimeType)
	}
}

func TestServiceUploadNoRestrictions(t *testing.T) {
	service, _ := testService(t)
	ctx := context.Background()

	content := []byte("any content")
	r := bytes.NewReader(content)

	file, err := service.Upload(ctx, "documents", "any.bin", r, int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if file.Bucket != "documents" {
		t.Errorf("Bucket = %s, want documents", file.Bucket)
	}
}

func TestServiceDownload(t *testing.T) {
	service, _ := testService(t)
	ctx := context.Background()

	content := []byte("Hello, World!")
	r := bytes.NewReader(content)

	file, err := service.Upload(ctx, "uploads", "test.txt", r, int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	rc, metadata, err := service.Download(ctx, "uploads", file.ID)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	defer rc.Close()

	if metadata.ID != file.ID {
		t.Errorf("Metadata ID = %s, want %s", metadata.ID, file.ID)
	}

	downloaded, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("Reading downloaded content failed: %v", err)
	}

	if !bytes.Equal(downloaded, content) {
		t.Errorf("Downloaded content = %q, want %q", downloaded, content)
	}
}

func TestServiceGetMetadata(t *testing.T) {
	service, _ := testService(t)
	ctx := context.Background()

	content := []byte("Hello, World!")
	r := bytes.NewReader(content)

	file, err := service.Upload(ctx, "uploads", "test.txt", r, int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	metadata, err := service.GetMetadata(ctx, "uploads", file.ID)
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	if metadata.ID != file.ID {
		t.Errorf("ID = %s, want %s", metadata.ID, file.ID)
	}
	if metadata.Name != file.Name {
		t.Errorf("Name = %s, want %s", metadata.Name, file.Name)
	}
	if metadata.Size != file.Size {
		t.Errorf("Size = %d, want %d", metadata.Size, file.Size)
	}
}

func TestServiceDelete(t *testing.T) {
	service, backend := testService(t)
	ctx := context.Background()

	content := []byte("Hello, World!")
	r := bytes.NewReader(content)

	file, err := service.Upload(ctx, "uploads", "test.txt", r, int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	err = service.Delete(ctx, "uploads", file.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = service.GetMetadata(ctx, "uploads", file.ID)
	if err != ErrNotFound {
		t.Errorf("GetMetadata after Delete error = %v, want ErrNotFound", err)
	}

	exists, err := backend.Exists(ctx, "uploads", file.ID)
	if err != nil {
		t.Fatalf("Exists check failed: %v", err)
	}
	if exists {
		t.Error("File still exists in backend after Delete")
	}
}

func TestServiceList(t *testing.T) {
	service, _ := testService(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		content := []byte("content")
		r := bytes.NewReader(content)
		filename := string(rune('a'+i)) + ".txt"
		_, err := service.Upload(ctx, "uploads", filename, r, int64(len(content)))
		if err != nil {
			t.Fatalf("Upload %d failed: %v", i, err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	files, err := service.List(ctx, "uploads", 0, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(files) != 5 {
		t.Errorf("List returned %d files, want 5", len(files))
	}
}

func TestServiceListPagination(t *testing.T) {
	service, _ := testService(t)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		content := []byte("content")
		r := bytes.NewReader(content)
		filename := string(rune('a'+i)) + ".txt"
		_, err := service.Upload(ctx, "uploads", filename, r, int64(len(content)))
		if err != nil {
			t.Fatalf("Upload %d failed: %v", i, err)
		}
		time.Sleep(2 * time.Millisecond)
	}

	files, err := service.List(ctx, "uploads", 2, 3)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("List returned %d files, want 3", len(files))
	}
}

func TestServiceBucketNotFound(t *testing.T) {
	service, _ := testService(t)
	ctx := context.Background()

	content := []byte("content")
	r := bytes.NewReader(content)

	_, err := service.Upload(ctx, "nonexistent", "file.txt", r, int64(len(content)))
	if err == nil {
		t.Error("Upload should fail for nonexistent bucket")
	}
	if !strings.Contains(err.Error(), "bucket not found") {
		t.Errorf("Error message = %v, want bucket not found error", err)
	}
}
