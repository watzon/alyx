package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
)

func testDB(t *testing.T) *database.DB {
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

	return db
}

func TestStoreCreate(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	file := &File{
		ID:       "test-file-1",
		Bucket:   "uploads",
		Name:     "test.txt",
		Path:     "test.txt",
		MimeType: "text/plain",
		Size:     1024,
		Checksum: "abc123",
		Metadata: map[string]string{"key": "value"},
	}

	err := store.Create(ctx, file)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if file.CreatedAt.IsZero() {
		t.Error("CreatedAt not set")
	}
	if file.UpdatedAt.IsZero() {
		t.Error("UpdatedAt not set")
	}
	if file.Version != 1 {
		t.Errorf("Version = %d, want 1", file.Version)
	}
}

func TestStoreGet(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	file := &File{
		ID:       "test-file-2",
		Bucket:   "uploads",
		Name:     "test.txt",
		Path:     "test.txt",
		MimeType: "text/plain",
		Size:     1024,
	}

	if err := store.Create(ctx, file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := store.Get(ctx, "uploads", "test-file-2")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.ID != file.ID {
		t.Errorf("ID = %s, want %s", retrieved.ID, file.ID)
	}
	if retrieved.Bucket != file.Bucket {
		t.Errorf("Bucket = %s, want %s", retrieved.Bucket, file.Bucket)
	}
	if retrieved.Name != file.Name {
		t.Errorf("Name = %s, want %s", retrieved.Name, file.Name)
	}
	if retrieved.Size != file.Size {
		t.Errorf("Size = %d, want %d", retrieved.Size, file.Size)
	}
}

func TestStoreGetNotFound(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	_, err := store.Get(ctx, "uploads", "nonexistent")
	if err != ErrNotFound {
		t.Errorf("Get error = %v, want ErrNotFound", err)
	}
}

func TestStoreList(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	baseTime := time.Now().UTC()
	for i := 0; i < 5; i++ {
		id := string(rune('a' + i))
		file := &File{
			ID:        id,
			Bucket:    "uploads",
			Name:      id + ".txt",
			Path:      id + ".txt",
			MimeType:  "text/plain",
			Size:      int64(i * 100),
			CreatedAt: baseTime.Add(time.Duration(i) * time.Second),
			UpdatedAt: baseTime.Add(time.Duration(i) * time.Second),
		}
		if err := store.Create(ctx, file); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	files, total, err := store.List(ctx, "uploads", "", "", 0, 10)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(files) != 5 {
		t.Errorf("List returned %d files, want 5", len(files))
	}

	if total != 5 {
		t.Errorf("List returned total %d, want 5", total)
	}

	if files[0].ID != "e" {
		t.Errorf("First file ID = %s, want e (DESC order)", files[0].ID)
	}
}

func TestStoreListPagination(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		id := string(rune('a' + i))
		file := &File{
			ID:       id,
			Bucket:   "uploads",
			Name:     id + ".txt",
			Path:     id + ".txt",
			MimeType: "text/plain",
			Size:     int64(i * 100),
		}
		if err := store.Create(ctx, file); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		time.Sleep(time.Millisecond)
	}

	files, total, err := store.List(ctx, "uploads", "", "", 2, 3)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("List returned %d files, want 3", len(files))
	}

	if total != 10 {
		t.Errorf("List returned total %d, want 10", total)
	}
}

func TestStoreDelete(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	file := &File{
		ID:       "test-file-3",
		Bucket:   "uploads",
		Name:     "test.txt",
		Path:     "test.txt",
		MimeType: "text/plain",
		Size:     1024,
	}

	if err := store.Create(ctx, file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	err := store.Delete(ctx, "uploads", "test-file-3")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(ctx, "uploads", "test-file-3")
	if err != ErrNotFound {
		t.Errorf("Get after Delete error = %v, want ErrNotFound", err)
	}
}

func TestStoreDeleteNotFound(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	err := store.Delete(ctx, "uploads", "nonexistent")
	if err != ErrNotFound {
		t.Errorf("Delete error = %v, want ErrNotFound", err)
	}
}

func TestStoreMetadata(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	file := &File{
		ID:       "test-file-4",
		Bucket:   "uploads",
		Name:     "test.txt",
		Path:     "test.txt",
		MimeType: "text/plain",
		Size:     1024,
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	if err := store.Create(ctx, file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := store.Get(ctx, "uploads", "test-file-4")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if len(retrieved.Metadata) != 2 {
		t.Errorf("Metadata length = %d, want 2", len(retrieved.Metadata))
	}
	if retrieved.Metadata["key1"] != "value1" {
		t.Errorf("Metadata[key1] = %s, want value1", retrieved.Metadata["key1"])
	}
	if retrieved.Metadata["key2"] != "value2" {
		t.Errorf("Metadata[key2] = %s, want value2", retrieved.Metadata["key2"])
	}
}

func TestStoreCompression(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)
	ctx := context.Background()

	file := &File{
		ID:              "test-file-5",
		Bucket:          "uploads",
		Name:            "test.txt",
		Path:            "test.txt",
		MimeType:        "text/plain",
		Size:            512,
		Compressed:      true,
		CompressionType: "gzip",
		OriginalSize:    1024,
	}

	if err := store.Create(ctx, file); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := store.Get(ctx, "uploads", "test-file-5")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !retrieved.Compressed {
		t.Error("Compressed = false, want true")
	}
	if retrieved.CompressionType != "gzip" {
		t.Errorf("CompressionType = %s, want gzip", retrieved.CompressionType)
	}
	if retrieved.OriginalSize != 1024 {
		t.Errorf("OriginalSize = %d, want 1024", retrieved.OriginalSize)
	}
}
