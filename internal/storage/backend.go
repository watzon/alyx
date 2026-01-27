package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/watzon/alyx/internal/config"
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

func NewBackend(ctx context.Context, cfg BackendConfig) (Backend, error) {
	switch cfg.Type {
	case "filesystem":
		if cfg.Path == "" {
			return nil, fmt.Errorf("%w: filesystem backend requires path", ErrInvalidConfig)
		}
		return NewFilesystemBackend(cfg.Path), nil
	case "s3":
		s3Cfg := config.S3Config{
			Endpoint:        cfg.Endpoint,
			Region:          cfg.Region,
			AccessKeyID:     cfg.AccessKeyID,
			SecretAccessKey: cfg.SecretKey,
			ForcePathStyle:  cfg.Endpoint != "",
		}
		return NewS3Backend(ctx, s3Cfg)
	default:
		return nil, fmt.Errorf("%w: unknown backend type %q", ErrInvalidConfig, cfg.Type)
	}
}
