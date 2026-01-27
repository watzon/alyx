package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/watzon/alyx/internal/config"
)

func TestS3Backend(t *testing.T) {
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT not set, skipping S3 integration tests")
	}

	cfg := config.S3Config{
		Endpoint:        endpoint,
		Region:          os.Getenv("S3_REGION"),
		AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		BucketPrefix:    "alyx-test-",
		ForcePathStyle:  true,
	}

	backend, err := NewS3Backend(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewS3Backend failed: %v", err)
	}

	ctx := context.Background()
	bucket := "test-bucket"
	key := "test-file.txt"
	content := []byte("Hello, S3!")

	t.Run("Put", func(t *testing.T) {
		err := backend.Put(ctx, bucket, key, bytes.NewReader(content), int64(len(content)))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		exists, err := backend.Exists(ctx, bucket, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Error("Expected file to exist")
		}
	})

	t.Run("Get", func(t *testing.T) {
		rc, err := backend.Get(ctx, bucket, key)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}

		if !bytes.Equal(data, content) {
			t.Errorf("Expected %q, got %q", content, data)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		err := backend.Delete(ctx, bucket, key)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		exists, err := backend.Exists(ctx, bucket, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Error("Expected file to not exist after deletion")
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		_, err := backend.Get(ctx, bucket, "nonexistent.txt")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})
}

func TestS3BackendMultipart(t *testing.T) {
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT not set, skipping S3 integration tests")
	}

	cfg := config.S3Config{
		Endpoint:        endpoint,
		Region:          os.Getenv("S3_REGION"),
		AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		BucketPrefix:    "alyx-test-",
		ForcePathStyle:  true,
	}

	backend, err := NewS3Backend(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewS3Backend failed: %v", err)
	}

	ctx := context.Background()
	bucket := "test-bucket"
	key := "large-file.bin"

	content := make([]byte, 10*1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}

	t.Run("PutLargeFile", func(t *testing.T) {
		err := backend.Put(ctx, bucket, key, bytes.NewReader(content), int64(len(content)))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	})

	t.Run("GetLargeFile", func(t *testing.T) {
		rc, err := backend.Get(ctx, bucket, key)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}

		if !bytes.Equal(data, content) {
			t.Error("Large file content mismatch")
		}
	})

	t.Run("DeleteLargeFile", func(t *testing.T) {
		err := backend.Delete(ctx, bucket, key)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
	})
}

func TestS3BackendBucketPrefix(t *testing.T) {
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT not set, skipping S3 integration tests")
	}

	cfg := config.S3Config{
		Endpoint:        endpoint,
		Region:          os.Getenv("S3_REGION"),
		AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		BucketPrefix:    "prefix-",
		ForcePathStyle:  true,
	}

	backend, err := NewS3Backend(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewS3Backend failed: %v", err)
	}

	s3Backend, ok := backend.(*S3Backend)
	if !ok {
		t.Fatal("Expected *S3Backend")
	}

	actualBucket := s3Backend.bucketName("test")
	expectedBucket := "prefix-test"
	if actualBucket != expectedBucket {
		t.Errorf("Expected bucket name %q, got %q", expectedBucket, actualBucket)
	}
}

func TestS3BackendContextCancellation(t *testing.T) {
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT not set, skipping S3 integration tests")
	}

	cfg := config.S3Config{
		Endpoint:        endpoint,
		Region:          os.Getenv("S3_REGION"),
		AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		BucketPrefix:    "alyx-test-",
		ForcePathStyle:  true,
	}

	backend, err := NewS3Backend(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewS3Backend failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bucket := "test-bucket"
	key := "test-file.txt"
	content := []byte("Hello, S3!")

	err = backend.Put(ctx, bucket, key, bytes.NewReader(content), int64(len(content)))
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}
