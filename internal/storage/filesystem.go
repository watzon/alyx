package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FilesystemBackend implements Backend interface for local filesystem storage.
// Files are organized as: {basePath}/{bucket}/{key}
type FilesystemBackend struct {
	basePath string
}

// NewFilesystemBackend creates a new filesystem backend with the given base path.
func NewFilesystemBackend(basePath string) *FilesystemBackend {
	return &FilesystemBackend{
		basePath: basePath,
	}
}

// validatePath checks for path traversal attempts and invalid characters.
// Returns error if path contains: .., null bytes, or is absolute.
func (f *FilesystemBackend) validatePath(bucket, key string) error {
	// Check for null bytes
	if strings.Contains(bucket, "\x00") || strings.Contains(key, "\x00") {
		return fmt.Errorf("invalid path: null byte not allowed")
	}

	// Check for absolute paths (Unix and Windows)
	if filepath.IsAbs(bucket) || filepath.IsAbs(key) {
		return fmt.Errorf("invalid path: absolute paths not allowed")
	}

	// Additional Windows absolute path check (C:, D:, etc.)
	if len(bucket) >= 2 && bucket[1] == ':' {
		return fmt.Errorf("invalid path: absolute paths not allowed")
	}
	if len(key) >= 2 && key[1] == ':' {
		return fmt.Errorf("invalid path: absolute paths not allowed")
	}

	// Clean paths and check for traversal
	cleanBucket := filepath.Clean(bucket)
	cleanKey := filepath.Clean(key)

	// After cleaning, paths should not start with .. or contain ..
	if strings.HasPrefix(cleanBucket, "..") || strings.Contains(cleanBucket, string(filepath.Separator)+"..") {
		return fmt.Errorf("invalid path: bucket contains path traversal")
	}

	if strings.HasPrefix(cleanKey, "..") || strings.Contains(cleanKey, string(filepath.Separator)+"..") {
		return fmt.Errorf("invalid path: key contains path traversal")
	}

	return nil
}

// buildPath constructs the full filesystem path for a bucket/key pair.
// Returns cleaned path after validation.
func (f *FilesystemBackend) buildPath(bucket, key string) (string, error) {
	if err := f.validatePath(bucket, key); err != nil {
		return "", err
	}

	// Use filepath.Join to handle OS-specific separators
	fullPath := filepath.Join(f.basePath, bucket, key)

	// Final safety check: ensure path is within basePath
	cleanPath := filepath.Clean(fullPath)
	cleanBase := filepath.Clean(f.basePath)

	if !strings.HasPrefix(cleanPath, cleanBase) {
		return "", fmt.Errorf("invalid path: path escapes base directory")
	}

	return cleanPath, nil
}

// Put stores data at the specified bucket/key location.
// Creates directories automatically if they don't exist.
func (f *FilesystemBackend) Put(ctx context.Context, bucket, key string, r io.Reader, size int64) error {
	fullPath, err := f.buildPath(bucket, key)
	if err != nil {
		return err
	}

	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer file.Close()

	// Copy data
	_, err = io.Copy(file, r)
	if err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	return nil
}

// Get retrieves data from the specified bucket/key location.
// Returns ErrNotFound if the file doesn't exist.
// Caller must close the returned ReadCloser.
func (f *FilesystemBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	fullPath, err := f.buildPath(bucket, key)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("opening file: %w", err)
	}

	return file, nil
}

// Delete removes the file at the specified bucket/key location.
// Returns nil if the file doesn't exist (idempotent).
func (f *FilesystemBackend) Delete(ctx context.Context, bucket, key string) error {
	fullPath, err := f.buildPath(bucket, key)
	if err != nil {
		return err
	}

	err = os.Remove(fullPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing file: %w", err)
	}

	return nil
}

// Exists checks if a file exists at the specified bucket/key location.
func (f *FilesystemBackend) Exists(ctx context.Context, bucket, key string) (bool, error) {
	fullPath, err := f.buildPath(bucket, key)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("checking file: %w", err)
	}

	return true, nil
}
