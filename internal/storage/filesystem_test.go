package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestFilesystemBackend_PutGet(t *testing.T) {
	tmpDir := t.TempDir()
	backend := NewFilesystemBackend(tmpDir)
	ctx := context.Background()

	data := []byte("test file content")
	err := backend.Put(ctx, "test-bucket", "test-key", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Verify file exists on disk
	expectedPath := filepath.Join(tmpDir, "test-bucket", "test-key")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("File not created at expected path: %s", expectedPath)
	}

	// Get the file back
	rc, err := backend.Get(ctx, "test-bucket", "test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer rc.Close()

	retrieved, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if !bytes.Equal(data, retrieved) {
		t.Errorf("Retrieved data doesn't match. Got %q, want %q", retrieved, data)
	}
}

func TestFilesystemBackend_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	backend := NewFilesystemBackend(tmpDir)
	ctx := context.Background()

	// Put a file
	data := []byte("delete me")
	err := backend.Put(ctx, "test-bucket", "test-key", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Verify it exists
	exists, err := backend.Exists(ctx, "test-bucket", "test-key")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("File should exist before delete")
	}

	// Delete it
	err = backend.Delete(ctx, "test-bucket", "test-key")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	exists, err = backend.Exists(ctx, "test-bucket", "test-key")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("File should not exist after delete")
	}

	// Verify file removed from disk
	expectedPath := filepath.Join(tmpDir, "test-bucket", "test-key")
	if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
		t.Fatal("File still exists on disk after delete")
	}
}

func TestFilesystemBackend_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	backend := NewFilesystemBackend(tmpDir)
	ctx := context.Background()

	// Non-existent file
	exists, err := backend.Exists(ctx, "test-bucket", "nonexistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("Nonexistent file should not exist")
	}

	// Create a file
	data := []byte("exists test")
	err = backend.Put(ctx, "test-bucket", "test-key", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Now it should exist
	exists, err = backend.Exists(ctx, "test-bucket", "test-key")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("File should exist after Put")
	}
}

func TestFilesystemBackend_GetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	backend := NewFilesystemBackend(tmpDir)
	ctx := context.Background()

	_, err := backend.Get(ctx, "test-bucket", "nonexistent")
	if err != ErrNotFound {
		t.Errorf("Get should return ErrNotFound for nonexistent file, got: %v", err)
	}
}

func TestFilesystemBackend_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	backend := NewFilesystemBackend(tmpDir)
	ctx := context.Background()
	data := []byte("malicious")

	tests := []struct {
		name   string
		bucket string
		key    string
	}{
		{"parent directory unix", "bucket", "../etc/passwd"},
		{"parent directory windows", "bucket", "..\\windows\\system32"},
		{"absolute path unix", "bucket", "/etc/passwd"},
		{"absolute path windows", "bucket", "C:\\windows\\system32"},
		{"null byte", "bucket", "test\x00.txt"},
		{"bucket traversal", "../etc", "passwd"},
		{"double dot", "bucket", "foo/../../../etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := backend.Put(ctx, tt.bucket, tt.key, bytes.NewReader(data), int64(len(data)))
			if err == nil {
				t.Errorf("Put should reject path traversal attempt: bucket=%q key=%q", tt.bucket, tt.key)
			}
			if !strings.Contains(err.Error(), "invalid") {
				t.Errorf("Error should mention 'invalid', got: %v", err)
			}
		})
	}
}

func TestFilesystemBackend_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	backend := NewFilesystemBackend(tmpDir)
	ctx := context.Background()

	const numGoroutines = 10
	const numOpsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOpsPerGoroutine; j++ {
				bucket := "concurrent-bucket"
				key := filepath.Join("goroutine", string(rune('0'+id)), "file", string(rune('0'+j)))
				data := []byte("concurrent test data")

				// Put
				err := backend.Put(ctx, bucket, key, bytes.NewReader(data), int64(len(data)))
				if err != nil {
					t.Errorf("Concurrent Put failed: %v", err)
					return
				}

				// Get
				rc, err := backend.Get(ctx, bucket, key)
				if err != nil {
					t.Errorf("Concurrent Get failed: %v", err)
					return
				}
				rc.Close()

				// Delete
				err = backend.Delete(ctx, bucket, key)
				if err != nil {
					t.Errorf("Concurrent Delete failed: %v", err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestFilesystemBackend_NestedPaths(t *testing.T) {
	tmpDir := t.TempDir()
	backend := NewFilesystemBackend(tmpDir)
	ctx := context.Background()

	// Test nested directory creation
	data := []byte("nested file")
	err := backend.Put(ctx, "bucket", "path/to/nested/file.txt", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Put with nested path failed: %v", err)
	}

	// Verify file exists
	rc, err := backend.Get(ctx, "bucket", "path/to/nested/file.txt")
	if err != nil {
		t.Fatalf("Get nested file failed: %v", err)
	}
	defer rc.Close()

	retrieved, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if !bytes.Equal(data, retrieved) {
		t.Errorf("Retrieved data doesn't match. Got %q, want %q", retrieved, data)
	}
}

func TestFilesystemBackend_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	backend := NewFilesystemBackend(tmpDir)
	ctx := context.Background()

	// Put empty file
	err := backend.Put(ctx, "bucket", "empty.txt", bytes.NewReader([]byte{}), 0)
	if err != nil {
		t.Fatalf("Put empty file failed: %v", err)
	}

	// Get it back
	rc, err := backend.Get(ctx, "bucket", "empty.txt")
	if err != nil {
		t.Fatalf("Get empty file failed: %v", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(data) != 0 {
		t.Errorf("Empty file should have zero bytes, got %d", len(data))
	}
}

func TestFilesystemBackend_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	backend := NewFilesystemBackend(tmpDir)
	ctx := context.Background()

	size := int64(10 * 1024 * 1024)
	data := bytes.Repeat([]byte("x"), int(size))

	err := backend.Put(ctx, "test-bucket", "large-file", bytes.NewReader(data), size)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	rc, err := backend.Get(ctx, "test-bucket", "large-file")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer rc.Close()

	retrieved, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(retrieved) != len(data) {
		t.Errorf("Retrieved size doesn't match. Got %d, want %d", len(retrieved), len(data))
	}
}

func TestFilesystemBackend_WithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	backend := NewFilesystemBackendWithPrefix(tmpDir, "app-")
	ctx := context.Background()

	data := []byte("test content with prefix")
	err := backend.Put(ctx, "uploads", "test-file.txt", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, "app-uploads", "test-file.txt")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("File not created at expected prefixed path: %s", expectedPath)
	}

	rc, err := backend.Get(ctx, "uploads", "test-file.txt")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer rc.Close()

	retrieved, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if !bytes.Equal(data, retrieved) {
		t.Errorf("Retrieved data doesn't match. Got %q, want %q", retrieved, data)
	}

	exists, err := backend.Exists(ctx, "uploads", "test-file.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("File should exist after Put")
	}

	err = backend.Delete(ctx, "uploads", "test-file.txt")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := os.Stat(expectedPath); !os.IsNotExist(err) {
		t.Error("File should be deleted from prefixed path")
	}
}

func TestFilesystemBackend_MultipleBucketsWithPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	backend := NewFilesystemBackendWithPrefix(tmpDir, "prod-")
	ctx := context.Background()

	buckets := []string{"avatars", "documents", "backups"}
	for _, bucket := range buckets {
		data := []byte("content for " + bucket)
		err := backend.Put(ctx, bucket, "test.txt", bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("Put to bucket %s failed: %v", bucket, err)
		}

		expectedPath := filepath.Join(tmpDir, "prod-"+bucket, "test.txt")
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Errorf("File not created for bucket %s at: %s", bucket, expectedPath)
		}
	}
}
