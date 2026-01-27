package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
)

var (
	ErrNotFound      = errors.New("file not found")
	ErrInvalidConfig = errors.New("invalid backend configuration")
)

type Backend interface {
	Put(ctx context.Context, bucket, key string, r io.Reader, size int64) error
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, bucket, key string) error
	Exists(ctx context.Context, bucket, key string) (bool, error)
}

type BackendConfig struct {
	Type        string
	Path        string
	Endpoint    string
	Bucket      string
	Region      string
	AccessKeyID string
	SecretKey   string
}

func NewBackend(cfg BackendConfig) (Backend, error) {
	switch cfg.Type {
	case "filesystem":
		return nil, fmt.Errorf("%w: filesystem backend not implemented", ErrInvalidConfig)
	case "s3":
		return nil, fmt.Errorf("%w: s3 backend not implemented", ErrInvalidConfig)
	default:
		return nil, fmt.Errorf("%w: unknown backend type %q", ErrInvalidConfig, cfg.Type)
	}
}
