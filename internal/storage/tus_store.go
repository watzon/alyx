package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/watzon/alyx/internal/database"
)

type Upload struct {
	ID        string            `json:"id"`
	Bucket    string            `json:"bucket"`
	Filename  string            `json:"filename,omitempty"`
	Size      int64             `json:"size"`
	Offset    int64             `json:"offset"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	ExpiresAt time.Time         `json:"expires_at"`
	CreatedAt time.Time         `json:"created_at"`
}

type TUSStore struct {
	db *database.DB
}

func NewTUSStore(db *database.DB) *TUSStore {
	return &TUSStore{db: db}
}

func (s *TUSStore) Create(ctx context.Context, upload *Upload) error {
	if upload.CreatedAt.IsZero() {
		upload.CreatedAt = time.Now().UTC()
	}
	if upload.ExpiresAt.IsZero() {
		upload.ExpiresAt = upload.CreatedAt.Add(24 * time.Hour)
	}

	var metadataJSON []byte
	var err error
	if upload.Metadata != nil {
		metadataJSON, err = json.Marshal(upload.Metadata)
		if err != nil {
			return fmt.Errorf("marshaling metadata: %w", err)
		}
	}

	query := `
		INSERT INTO _alyx_uploads (
			id, bucket, filename, size, offset, metadata, expires_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query,
		upload.ID,
		upload.Bucket,
		nullString(upload.Filename),
		upload.Size,
		upload.Offset,
		nullString(string(metadataJSON)),
		upload.ExpiresAt.UTC().Format(time.RFC3339),
		upload.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting upload: %w", err)
	}

	return nil
}

func (s *TUSStore) Get(ctx context.Context, bucket, uploadID string) (*Upload, error) {
	query := `
		SELECT id, bucket, filename, size, offset, metadata, expires_at, created_at
		FROM _alyx_uploads
		WHERE id = ? AND bucket = ?
	`

	row := s.db.QueryRowContext(ctx, query, uploadID, bucket)

	var upload Upload
	var filename, metadataJSON sql.NullString
	var expiresAt, createdAt string

	err := row.Scan(
		&upload.ID,
		&upload.Bucket,
		&filename,
		&upload.Size,
		&upload.Offset,
		&metadataJSON,
		&expiresAt,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting upload: %w", err)
	}

	if filename.Valid {
		upload.Filename = filename.String
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &upload.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
	}

	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("parsing expires_at: %w", err)
	}
	upload.ExpiresAt = t

	t, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	upload.CreatedAt = t

	return &upload, nil
}

func (s *TUSStore) UpdateOffset(ctx context.Context, bucket, uploadID string, offset int64) error {
	query := `UPDATE _alyx_uploads SET offset = ? WHERE id = ? AND bucket = ?`

	result, err := s.db.ExecContext(ctx, query, offset, uploadID, bucket)
	if err != nil {
		return fmt.Errorf("updating offset: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *TUSStore) Delete(ctx context.Context, bucket, uploadID string) error {
	query := `DELETE FROM _alyx_uploads WHERE id = ? AND bucket = ?`

	result, err := s.db.ExecContext(ctx, query, uploadID, bucket)
	if err != nil {
		return fmt.Errorf("deleting upload: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (s *TUSStore) ListExpired(ctx context.Context) ([]*Upload, error) {
	query := `
		SELECT id, bucket, filename, size, offset, metadata, expires_at, created_at
		FROM _alyx_uploads
		WHERE expires_at < ?
	`

	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := s.db.QueryContext(ctx, query, now)
	if err != nil {
		return nil, fmt.Errorf("querying expired uploads: %w", err)
	}
	defer rows.Close()

	var uploads []*Upload

	for rows.Next() {
		var upload Upload
		var filename, metadataJSON sql.NullString
		var expiresAt, createdAt string

		err := rows.Scan(
			&upload.ID,
			&upload.Bucket,
			&filename,
			&upload.Size,
			&upload.Offset,
			&metadataJSON,
			&expiresAt,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning upload row: %w", err)
		}

		if filename.Valid {
			upload.Filename = filename.String
		}

		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &upload.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshaling metadata: %w", err)
			}
		}

		t, err := time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			return nil, fmt.Errorf("parsing expires_at: %w", err)
		}
		upload.ExpiresAt = t

		t, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parsing created_at: %w", err)
		}
		upload.CreatedAt = t

		uploads = append(uploads, &upload)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating upload rows: %w", err)
	}

	return uploads, nil
}
