package storage

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

const (
	DefaultChunkSize      = 5 * 1024 * 1024
	DefaultUploadExpiry   = 24 * 60 * 60
	TUSVersion            = "1.0.0"
	TUSResumableSupported = "1.0.0"
)

const (
	defaultFilePerm   = 0644
	headerReadSize    = 512
	uploadExpiryHours = 24
)

type TUSService struct {
	db       *database.DB
	store    *TUSStore
	backends map[string]Backend
	schema   *schema.Schema
	cfg      *config.Config
	tempDir  string
}

func NewTUSService(db *database.DB, backends map[string]Backend, s *schema.Schema, cfg *config.Config, tempDir string) *TUSService {
	return &TUSService{
		db:       db,
		store:    NewTUSStore(db),
		backends: backends,
		schema:   s,
		cfg:      cfg,
		tempDir:  tempDir,
	}
}

func (s *TUSService) CreateUpload(ctx context.Context, bucket string, size int64, metadata map[string]string) (*Upload, error) {
	bucketCfg, ok := s.schema.Buckets[bucket]
	if !ok {
		return nil, fmt.Errorf("bucket not found: %s", bucket)
	}

	if bucketCfg.MaxFileSize > 0 && size > bucketCfg.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum %d", size, bucketCfg.MaxFileSize)
	}

	uploadID := uuid.New().String()

	upload := &Upload{
		ID:       uploadID,
		Bucket:   bucket,
		Size:     size,
		Offset:   0,
		Metadata: metadata,
	}

	if filename, ok := metadata["filename"]; ok {
		upload.Filename = filename
	}

	if err := s.store.Create(ctx, upload); err != nil {
		return nil, fmt.Errorf("creating upload: %w", err)
	}

	return upload, nil
}

func (s *TUSService) GetUploadOffset(ctx context.Context, bucket, uploadID string) (int64, error) {
	upload, err := s.store.Get(ctx, bucket, uploadID)
	if err != nil {
		return 0, err
	}

	return upload.Offset, nil
}

func (s *TUSService) UploadChunk(ctx context.Context, bucket, uploadID string, offset int64, r io.Reader, chunkSize int64) (int64, error) {
	upload, err := s.store.Get(ctx, bucket, uploadID)
	if err != nil {
		return 0, err
	}

	if offset != upload.Offset {
		return 0, fmt.Errorf("offset mismatch: expected %d, got %d", upload.Offset, offset)
	}

	tempPath := filepath.Join(s.tempDir, "tus", uploadID)
	if err := os.MkdirAll(filepath.Dir(tempPath), 0755); err != nil {
		return 0, fmt.Errorf("creating temp directory: %w", err)
	}

	f, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, defaultFilePerm)
	if err != nil {
		return 0, fmt.Errorf("opening temp file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, r)
	if err != nil {
		return 0, fmt.Errorf("writing chunk: %w", err)
	}

	if written != chunkSize {
		return 0, fmt.Errorf("chunk size mismatch: expected %d, wrote %d", chunkSize, written)
	}

	newOffset := offset + written

	if err := s.store.UpdateOffset(ctx, bucket, uploadID, newOffset); err != nil {
		return 0, fmt.Errorf("updating offset: %w", err)
	}

	if newOffset == upload.Size {
		if err := s.finalizeUpload(ctx, upload, tempPath); err != nil {
			return 0, fmt.Errorf("finalizing upload: %w", err)
		}
	}

	return newOffset, nil
}

func (s *TUSService) CancelUpload(ctx context.Context, bucket, uploadID string) error {
	if err := s.store.Delete(ctx, bucket, uploadID); err != nil {
		return err
	}

	tempPath := filepath.Join(s.tempDir, "tus", uploadID)
	if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting temp file: %w", err)
	}

	return nil
}

func (s *TUSService) CleanupExpiredUploads(ctx context.Context) (int, error) {
	uploads, err := s.store.ListExpired(ctx)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, upload := range uploads {
		if err := s.CancelUpload(ctx, upload.Bucket, upload.ID); err != nil {
			continue
		}
		deleted++
	}

	return deleted, nil
}

func (s *TUSService) finalizeUpload(ctx context.Context, upload *Upload, tempPath string) error {
	bucketCfg, ok := s.schema.Buckets[upload.Bucket]
	if !ok {
		return fmt.Errorf("bucket not found: %s", upload.Bucket)
	}

	backend, ok := s.backends[bucketCfg.Backend]
	if !ok {
		return fmt.Errorf("backend not found: %s", bucketCfg.Backend)
	}

	f, err := os.Open(tempPath)
	if err != nil {
		return fmt.Errorf("opening temp file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, headerReadSize)
	n, readErr := io.ReadFull(f, buf)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return fmt.Errorf("reading file header: %w", readErr)
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
			return fmt.Errorf("mime type %s not allowed", mimeType)
		}
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seeking to start: %w", err)
	}

	fileID := uuid.New().String()
	filename := upload.Filename
	if filename == "" {
		filename = fileID
	}

	hasher := sha256.New()
	teeReader := io.TeeReader(f, hasher)

	if err := backend.Put(ctx, upload.Bucket, fileID, teeReader, upload.Size); err != nil {
		return fmt.Errorf("storing file: %w", err)
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))

	file := &File{
		ID:       fileID,
		Bucket:   upload.Bucket,
		Name:     filename,
		Path:     filename,
		MimeType: mimeType,
		Size:     upload.Size,
		Checksum: checksum,
		Metadata: upload.Metadata,
	}

	fileStore := NewStore(s.db)
	if err := fileStore.Create(ctx, file); err != nil {
		_ = backend.Delete(ctx, upload.Bucket, fileID)
		return fmt.Errorf("storing file metadata: %w", err)
	}

	if err := s.store.Delete(ctx, upload.Bucket, upload.ID); err != nil {
		return fmt.Errorf("deleting upload record: %w", err)
	}

	if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting temp file: %w", err)
	}

	return nil
}

func ParseTUSMetadata(header string) map[string]string {
	metadata := make(map[string]string)
	if header == "" {
		return metadata
	}

	pairs := strings.Split(header, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), " ", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		encodedValue := parts[1]

		decoded, err := base64.StdEncoding.DecodeString(encodedValue)
		if err != nil {
			continue
		}

		metadata[key] = string(decoded)
	}

	return metadata
}
