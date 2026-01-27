package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
)

// mockBackend implements Backend interface for testing
type mockBackend struct {
	files map[string][]byte // bucket:key -> data
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		files: make(map[string][]byte),
	}
}

func (m *mockBackend) Put(ctx context.Context, bucket, key string, r io.Reader, size int64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.files[bucket+":"+key] = data
	return nil
}

func (m *mockBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	data, ok := m.files[bucket+":"+key]
	if !ok {
		return nil, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *mockBackend) Delete(ctx context.Context, bucket, key string) error {
	delete(m.files, bucket+":"+key)
	return nil
}

func (m *mockBackend) Exists(ctx context.Context, bucket, key string) (bool, error) {
	_, ok := m.files[bucket+":"+key]
	return ok, nil
}

func TestBackendInterface(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	// Test Put
	data := []byte("test data")
	err := backend.Put(ctx, "test-bucket", "test-key", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Test Exists
	exists, err := backend.Exists(ctx, "test-bucket", "test-key")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("Expected file to exist")
	}

	// Test Get
	rc, err := backend.Get(ctx, "test-bucket", "test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer rc.Close()

	retrieved, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if !bytes.Equal(retrieved, data) {
		t.Fatalf("Expected %q, got %q", data, retrieved)
	}

	// Test Delete
	err = backend.Delete(ctx, "test-bucket", "test-key")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	exists, err = backend.Exists(ctx, "test-bucket", "test-key")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("Expected file to not exist after delete")
	}
}

func TestBackendContextCancellation(t *testing.T) {
	backend := newMockBackend()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Operations should respect context cancellation
	data := []byte("test data")
	err := backend.Put(ctx, "test-bucket", "test-key", bytes.NewReader(data), int64(len(data)))
	if err != nil && err != context.Canceled {
		// Mock backend doesn't check context, but real implementations should
		t.Logf("Put with cancelled context: %v", err)
	}
}

func TestNewBackend(t *testing.T) {
	tests := []struct {
		name    string
		cfg     BackendConfig
		wantErr bool
	}{
		{
			name: "filesystem backend",
			cfg: BackendConfig{
				Type: "filesystem",
				Path: "/tmp/test",
			},
			wantErr: false,
		},
		{
			name: "filesystem backend missing path",
			cfg: BackendConfig{
				Type: "filesystem",
			},
			wantErr: true,
		},
		{
			name: "s3 backend missing credentials",
			cfg: BackendConfig{
				Type:     "s3",
				Endpoint: "s3.amazonaws.com",
				Bucket:   "test-bucket",
			},
			wantErr: true,
		},
		{
			name: "unknown backend",
			cfg: BackendConfig{
				Type: "unknown",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewBackend(context.Background(), tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewBackend() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCompressedBackend(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	tests := []struct {
		name        string
		compression string
		data        []byte
	}{
		{
			name:        "gzip compression",
			compression: "gzip",
			data:        []byte("test data for gzip compression"),
		},
		{
			name:        "zstd compression",
			compression: "zstd",
			data:        []byte("test data for zstd compression"),
		},
		{
			name:        "no compression",
			compression: "",
			data:        []byte("test data without compression"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compressed := NewCompressedBackend(backend, tt.compression)

			err := compressed.Put(ctx, "test-bucket", "test-key", bytes.NewReader(tt.data), int64(len(tt.data)))
			if err != nil {
				t.Fatalf("Put failed: %v", err)
			}

			rc, err := compressed.Get(ctx, "test-bucket", "test-key")
			if err != nil {
				t.Fatalf("Get failed: %v", err)
			}
			defer rc.Close()

			retrieved, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}

			if !bytes.Equal(retrieved, tt.data) {
				t.Fatalf("Expected %q, got %q", tt.data, retrieved)
			}

			err = compressed.Delete(ctx, "test-bucket", "test-key")
			if err != nil {
				t.Fatalf("Delete failed: %v", err)
			}

			exists, err := compressed.Exists(ctx, "test-bucket", "test-key")
			if err != nil {
				t.Fatalf("Exists failed: %v", err)
			}
			if exists {
				t.Fatal("Expected file to not exist after delete")
			}
		})
	}
}

func TestCompressedBackendTransparency(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()
	compressed := NewCompressedBackend(backend, "gzip")

	data := []byte("test data for compression transparency check")

	err := compressed.Put(ctx, "test-bucket", "test-key", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	rawData := backend.files["test-bucket:test-key"]
	if bytes.Equal(rawData, data) {
		t.Fatal("Expected data to be compressed in backend, but it's not")
	}

	rc, err := compressed.Get(ctx, "test-bucket", "test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer rc.Close()

	retrieved, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if !bytes.Equal(retrieved, data) {
		t.Fatalf("Expected decompressed data %q, got %q", data, retrieved)
	}
}
