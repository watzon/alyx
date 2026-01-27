package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

type CleanupService struct {
	store    *TUSStore
	tempDir  string
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewCleanupService(store *TUSStore, tempDir string, interval time.Duration) *CleanupService {
	if interval == 0 {
		interval = 1 * time.Hour
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &CleanupService{
		store:    store,
		tempDir:  tempDir,
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (s *CleanupService) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.cleanupLoop(ctx)

	log.Info().
		Dur("interval", s.interval).
		Msg("TUS cleanup service started")
}

func (s *CleanupService) Stop() {
	s.cancel()
	s.wg.Wait()
	log.Info().Msg("TUS cleanup service stopped")
}

func (s *CleanupService) cleanupLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted, err := s.RunOnce(ctx)
			if err != nil {
				log.Error().Err(err).Msg("Failed to cleanup expired uploads")
			} else if deleted > 0 {
				log.Info().
					Int("deleted", deleted).
					Msg("Cleaned up expired uploads")
			}
		}
	}
}

func (s *CleanupService) RunOnce(ctx context.Context) (int, error) {
	uploads, err := s.store.ListExpired(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing expired uploads: %w", err)
	}

	deleted := 0
	var errs []error

	for _, upload := range uploads {
		tempPath := filepath.Join(s.tempDir, "tus", upload.ID)

		if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
			log.Warn().
				Err(err).
				Str("upload_id", upload.ID).
				Str("path", tempPath).
				Msg("Failed to delete temp file")
			errs = append(errs, fmt.Errorf("deleting temp file %s: %w", upload.ID, err))
			continue
		}

		if err := s.store.Delete(ctx, upload.Bucket, upload.ID); err != nil {
			log.Warn().
				Err(err).
				Str("upload_id", upload.ID).
				Str("bucket", upload.Bucket).
				Msg("Failed to delete upload record")
			errs = append(errs, fmt.Errorf("deleting upload record %s: %w", upload.ID, err))
			continue
		}

		log.Debug().
			Str("upload_id", upload.ID).
			Str("bucket", upload.Bucket).
			Int64("size", upload.Size).
			Int64("offset", upload.Offset).
			Msg("Deleted expired upload")

		deleted++
	}

	if len(errs) > 0 {
		return deleted, fmt.Errorf("cleanup completed with %d errors (deleted %d/%d uploads)", len(errs), deleted, len(uploads))
	}

	return deleted, nil
}
