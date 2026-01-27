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

	"github.com/watzon/alyx/internal/auth"
	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/rules"
	"github.com/watzon/alyx/internal/schema"
)

type Service struct {
	db       *database.DB
	store    *Store
	backends map[string]Backend
	schema   *schema.Schema
	cfg      *config.Config
	rules    *rules.Engine
}

func NewService(db *database.DB, backends map[string]Backend, s *schema.Schema, cfg *config.Config, rulesEngine *rules.Engine) *Service {
	return &Service{
		db:       db,
		store:    NewStore(db),
		backends: backends,
		schema:   s,
		cfg:      cfg,
		rules:    rulesEngine,
	}
}

func (s *Service) Upload(ctx context.Context, bucket, filename string, r io.Reader, size int64) (*File, error) {
	bucketCfg, ok := s.schema.Buckets[bucket]
	if !ok {
		return nil, fmt.Errorf("bucket not found: %s", bucket)
	}

	if s.rules != nil {
		user := auth.UserFromContext(ctx)
		claims := auth.ClaimsFromContext(ctx)

		evalCtx := &rules.EvalContext{
			Auth: rules.BuildAuthContext(user, claims),
		}

		if err := s.rules.CheckAccess(bucket, rules.OpCreate, evalCtx); err != nil {
			return nil, fmt.Errorf("access denied: %w", err)
		}
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
	path := fileID + "/" + filename

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

	if s.rules != nil {
		if err := s.checkFileAccess(ctx, bucket, file, rules.OpDownload); err != nil {
			return nil, nil, err
		}
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
	file, err := s.store.Get(ctx, bucket, fileID)
	if err != nil {
		return nil, err
	}

	if s.rules != nil {
		if err := s.checkFileAccess(ctx, bucket, file, rules.OpRead); err != nil {
			return nil, err
		}
	}

	return file, nil
}

func (s *Service) Delete(ctx context.Context, bucket, fileID string) error {
	file, err := s.store.Get(ctx, bucket, fileID)
	if err != nil {
		return err
	}

	if s.rules != nil {
		if err := s.checkFileAccess(ctx, bucket, file, rules.OpDelete); err != nil {
			return err
		}
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

func (s *Service) List(ctx context.Context, bucket, search, mimeType string, offset, limit int) ([]*File, int, error) {
	if _, ok := s.schema.Buckets[bucket]; !ok {
		return nil, 0, fmt.Errorf("bucket not found: %s", bucket)
	}

	if s.rules != nil {
		user := auth.UserFromContext(ctx)
		claims := auth.ClaimsFromContext(ctx)

		evalCtx := &rules.EvalContext{
			Auth: rules.BuildAuthContext(user, claims),
		}

		if err := s.rules.CheckAccess(bucket, rules.OpRead, evalCtx); err != nil {
			return nil, 0, fmt.Errorf("access denied: %w", err)
		}
	}

	return s.store.List(ctx, bucket, search, mimeType, offset, limit)
}

func (s *Service) checkFileAccess(ctx context.Context, bucket string, file *File, op rules.Operation) error {
	user := auth.UserFromContext(ctx)
	claims := auth.ClaimsFromContext(ctx)

	fileCtx := map[string]any{
		"id":        file.ID,
		"name":      file.Name,
		"mime_type": file.MimeType,
		"size":      file.Size,
		"bucket":    file.Bucket,
	}

	evalCtx := &rules.EvalContext{
		Auth: rules.BuildAuthContext(user, claims),
		File: fileCtx,
	}

	if file.Metadata != nil && file.Metadata["file_security"] == "true" {
		if fileRule, ok := file.Metadata[string(op)]; ok && fileRule != "" {
			allowed, err := s.evaluateFileRule(fileRule, evalCtx)
			if err != nil {
				return fmt.Errorf("evaluating file-level rule: %w", err)
			}
			if !allowed {
				return rules.ErrAccessDenied
			}
			return nil
		}
	}

	if err := s.rules.CheckAccess(bucket, op, evalCtx); err != nil {
		return fmt.Errorf("access denied: %w", err)
	}

	return nil
}

func (s *Service) evaluateFileRule(ruleExpr string, ctx *rules.EvalContext) (bool, error) {
	tempEngine, err := rules.NewEngine()
	if err != nil {
		return false, fmt.Errorf("creating temp engine: %w", err)
	}

	tempSchema := &schema.Schema{
		Collections: map[string]*schema.Collection{},
		Buckets: map[string]*schema.Bucket{
			"_temp": {
				Name:    "_temp",
				Backend: "local",
				Rules: &schema.Rules{
					Download: ruleExpr,
				},
			},
		},
	}

	if err := tempEngine.LoadSchema(tempSchema); err != nil {
		return false, fmt.Errorf("loading file rule: %w", err)
	}

	return tempEngine.Evaluate("_temp", rules.OpDownload, ctx)
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
