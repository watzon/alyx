package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

type Service struct {
	db       *database.DB
	store    *Store
	backends map[string]Backend
	schema   *schema.Schema
	cfg      *config.Config
}

func NewService(db *database.DB, backends map[string]Backend, s *schema.Schema, cfg *config.Config) *Service {
	return &Service{
		db:       db,
		store:    NewStore(db),
		backends: backends,
		schema:   s,
		cfg:      cfg,
	}
}

func (s *Service) Upload(ctx context.Context, bucket, filename string, r io.Reader, size int64) (*File, error) {
	bucketCfg, ok := s.schema.Buckets[bucket]
	if !ok {
		return nil, fmt.Errorf("bucket not found: %s", bucket)
	}

	if bucketCfg.MaxFileSize > 0 && size > bucketCfg.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum %d", size, bucketCfg.MaxFileSize)
	}

	backend, ok := s.backends[bucketCfg.Backend]
	if !ok {
		return nil, fmt.Errorf("backend not found: %s", bucketCfg.Backend)
	}

	buf := make([]byte, 512)
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("reading file header: %w", err)
	}
	buf = buf[:n]

	mimeType := http.DetectContentType(buf)

	if len(bucketCfg.AllowedTypes) > 0 {
		allowed := false
		for _, allowedType := range bucketCfg.AllowedTypes {
			if matchesMimeType(mimeType, allowedType) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("mime type %s not allowed", mimeType)
		}
	}

	fileID := uuid.New().String()
	path := filename

	hasher := sha256.New()
	teeReader := io.TeeReader(io.MultiReader(strings.NewReader(string(buf)), r), hasher)

	if err := backend.Put(ctx, bucket, fileID, teeReader, size); err != nil {
		return nil, fmt.Errorf("storing file: %w", err)
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))

	file := &File{
		ID:       fileID,
		Bucket:   bucket,
		Name:     filename,
		Path:     path,
		MimeType: mimeType,
		Size:     size,
		Checksum: checksum,
	}

	if err := s.store.Create(ctx, file); err != nil {
		_ = backend.Delete(ctx, bucket, fileID)
		return nil, fmt.Errorf("storing file metadata: %w", err)
	}

	return file, nil
}

func (s *Service) Download(ctx context.Context, bucket, fileID string) (io.ReadCloser, *File, error) {
	file, err := s.store.Get(ctx, bucket, fileID)
	if err != nil {
		return nil, nil, err
	}

	bucketCfg, ok := s.schema.Buckets[bucket]
	if !ok {
		return nil, nil, fmt.Errorf("bucket not found: %s", bucket)
	}

	backend, ok := s.backends[bucketCfg.Backend]
	if !ok {
		return nil, nil, fmt.Errorf("backend not found: %s", bucketCfg.Backend)
	}

	rc, err := backend.Get(ctx, bucket, fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("retrieving file: %w", err)
	}

	return rc, file, nil
}

func (s *Service) GetMetadata(ctx context.Context, bucket, fileID string) (*File, error) {
	return s.store.Get(ctx, bucket, fileID)
}

func (s *Service) Delete(ctx context.Context, bucket, fileID string) error {
	_, err := s.store.Get(ctx, bucket, fileID)
	if err != nil {
		return err
	}

	bucketCfg, ok := s.schema.Buckets[bucket]
	if !ok {
		return fmt.Errorf("bucket not found: %s", bucket)
	}

	backend, ok := s.backends[bucketCfg.Backend]
	if !ok {
		return fmt.Errorf("backend not found: %s", bucketCfg.Backend)
	}

	if err := backend.Delete(ctx, bucket, fileID); err != nil {
		return fmt.Errorf("deleting file from backend: %w", err)
	}

	if err := s.store.Delete(ctx, bucket, fileID); err != nil {
		return fmt.Errorf("deleting file metadata: %w", err)
	}

	return nil
}

func (s *Service) List(ctx context.Context, bucket string, offset, limit int) ([]*File, error) {
	if _, ok := s.schema.Buckets[bucket]; !ok {
		return nil, fmt.Errorf("bucket not found: %s", bucket)
	}

	return s.store.List(ctx, bucket, offset, limit)
}

func matchesMimeType(mimeType, pattern string) bool {
	if pattern == "*/*" || pattern == "*" {
		return true
	}

	baseMimeType := mimeType
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		baseMimeType = strings.TrimSpace(mimeType[:idx])
	}

	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(baseMimeType, prefix+"/")
	}

	return baseMimeType == pattern
}
