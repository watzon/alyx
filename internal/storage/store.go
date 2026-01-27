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

// File represents a file stored in the system.
type File struct {
	ID              string            `json:"id"`
	Bucket          string            `json:"bucket"`
	Name            string            `json:"name"`
	Path            string            `json:"path"`
	MimeType        string            `json:"mime_type"`
	Size            int64             `json:"size"`
	Checksum        string            `json:"checksum,omitempty"`
	Compressed      bool              `json:"compressed"`
	CompressionType string            `json:"compression_type,omitempty"`
	OriginalSize    int64             `json:"original_size,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	Version         int               `json:"version"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// Store handles database operations for file metadata.
type Store struct {
	db *database.DB
}

// NewStore creates a new file store.
func NewStore(db *database.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new file metadata record.
func (s *Store) Create(ctx context.Context, file *File) error {
	if file.CreatedAt.IsZero() {
		file.CreatedAt = time.Now().UTC()
	}
	if file.UpdatedAt.IsZero() {
		file.UpdatedAt = file.CreatedAt
	}
	if file.Version == 0 {
		file.Version = 1
	}

	// Serialize metadata
	var metadataJSON []byte
	var err error
	if file.Metadata != nil {
		metadataJSON, err = json.Marshal(file.Metadata)
		if err != nil {
			return fmt.Errorf("marshaling metadata: %w", err)
		}
	}

	query := `
		INSERT INTO _alyx_files (
			id, bucket, name, path, mime_type, size, checksum,
			compressed, compression_type, original_size, metadata,
			version, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query,
		file.ID,
		file.Bucket,
		file.Name,
		file.Path,
		file.MimeType,
		file.Size,
		nullString(file.Checksum),
		file.Compressed,
		nullString(file.CompressionType),
		nullInt64(file.OriginalSize),
		nullString(string(metadataJSON)),
		file.Version,
		file.CreatedAt.UTC().Format(time.RFC3339),
		file.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting file metadata: %w", err)
	}

	return nil
}

// Get retrieves a file metadata record by ID.
func (s *Store) Get(ctx context.Context, bucket, fileID string) (*File, error) {
	query := `
		SELECT id, bucket, name, path, mime_type, size, checksum,
		       compressed, compression_type, original_size, metadata,
		       version, created_at, updated_at
		FROM _alyx_files
		WHERE id = ? AND bucket = ?
	`

	row := s.db.QueryRowContext(ctx, query, fileID, bucket)

	file, err := s.scanFile(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting file metadata: %w", err)
	}

	return file, nil
}

// List retrieves file metadata records for a bucket with pagination.
func (s *Store) List(ctx context.Context, bucket string, offset, limit int) ([]*File, error) {
	query := `
		SELECT id, bucket, name, path, mime_type, size, checksum,
		       compressed, compression_type, original_size, metadata,
		       version, created_at, updated_at
		FROM _alyx_files
		WHERE bucket = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.QueryContext(ctx, query, bucket, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying file metadata: %w", err)
	}
	defer rows.Close()

	return s.scanFiles(rows)
}

// Delete removes a file metadata record.
func (s *Store) Delete(ctx context.Context, bucket, fileID string) error {
	query := `DELETE FROM _alyx_files WHERE id = ? AND bucket = ?`

	result, err := s.db.ExecContext(ctx, query, fileID, bucket)
	if err != nil {
		return fmt.Errorf("deleting file metadata: %w", err)
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

// scanFile scans a single row into a File struct.
func (s *Store) scanFile(row *sql.Row) (*File, error) {
	var file File
	var checksum, compressionType, metadataJSON sql.NullString
	var originalSize sql.NullInt64
	var createdAt, updatedAt string
	var compressed int

	err := row.Scan(
		&file.ID,
		&file.Bucket,
		&file.Name,
		&file.Path,
		&file.MimeType,
		&file.Size,
		&checksum,
		&compressed,
		&compressionType,
		&originalSize,
		&metadataJSON,
		&file.Version,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse optional fields
	if checksum.Valid {
		file.Checksum = checksum.String
	}
	if compressionType.Valid {
		file.CompressionType = compressionType.String
	}
	if originalSize.Valid {
		file.OriginalSize = originalSize.Int64
	}
	file.Compressed = compressed == 1

	// Deserialize metadata
	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &file.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}
	}

	// Parse timestamps
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	file.CreatedAt = t

	t, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", err)
	}
	file.UpdatedAt = t

	return &file, nil
}

// scanFiles scans rows into File structs.
func (s *Store) scanFiles(rows *sql.Rows) ([]*File, error) {
	var files []*File

	for rows.Next() {
		var file File
		var checksum, compressionType, metadataJSON sql.NullString
		var originalSize sql.NullInt64
		var createdAt, updatedAt string
		var compressed int

		err := rows.Scan(
			&file.ID,
			&file.Bucket,
			&file.Name,
			&file.Path,
			&file.MimeType,
			&file.Size,
			&checksum,
			&compressed,
			&compressionType,
			&originalSize,
			&metadataJSON,
			&file.Version,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning file row: %w", err)
		}

		// Parse optional fields
		if checksum.Valid {
			file.Checksum = checksum.String
		}
		if compressionType.Valid {
			file.CompressionType = compressionType.String
		}
		if originalSize.Valid {
			file.OriginalSize = originalSize.Int64
		}
		file.Compressed = compressed == 1

		// Deserialize metadata
		if metadataJSON.Valid && metadataJSON.String != "" {
			if err := json.Unmarshal([]byte(metadataJSON.String), &file.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshaling metadata: %w", err)
			}
		}

		// Parse timestamps
		t, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parsing created_at: %w", err)
		}
		file.CreatedAt = t

		t, err = time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parsing updated_at: %w", err)
		}
		file.UpdatedAt = t

		files = append(files, &file)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating file rows: %w", err)
	}

	return files, nil
}

// Helper functions for nullable fields
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt64(i int64) sql.NullInt64 {
	if i == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: i, Valid: true}
}
