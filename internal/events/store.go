package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/watzon/alyx/internal/database"
)

// Store handles database operations for events.
type Store struct {
	db *database.DB
}

// NewStore creates a new event store.
func NewStore(db *database.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new event into the database.
func (s *Store) Create(ctx context.Context, event *Event) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if event.Status == "" {
		event.Status = "pending"
	}

	// Serialize payload and metadata
	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	var processAt *string
	if event.ProcessAt != nil {
		t := event.ProcessAt.UTC().Format(time.RFC3339)
		processAt = &t
	}

	query := `
		INSERT INTO events (id, type, source, action, payload, metadata, created_at, process_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query,
		event.ID,
		event.Type,
		event.Source,
		event.Action,
		string(payloadJSON),
		string(metadataJSON),
		event.CreatedAt.UTC().Format(time.RFC3339),
		processAt,
		event.Status,
	)
	if err != nil {
		return fmt.Errorf("inserting event: %w", err)
	}

	return nil
}

// GetPending retrieves pending events ready to be processed (immediate events only).
func (s *Store) GetPending(ctx context.Context, limit int) ([]*Event, error) {
	query := `
		SELECT id, type, source, action, payload, metadata, created_at, process_at, processed_at, status
		FROM events
		WHERE status = 'pending' AND process_at IS NULL
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("querying pending events: %w", err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// GetScheduled retrieves events with process_at in the past.
func (s *Store) GetScheduled(ctx context.Context, limit int) ([]*Event, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	query := `
		SELECT id, type, source, action, payload, metadata, created_at, process_at, processed_at, status
		FROM events
		WHERE status = 'pending' AND process_at IS NOT NULL AND process_at <= ?
		ORDER BY process_at ASC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("querying scheduled events: %w", err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// UpdateStatus updates the status of an event.
func (s *Store) UpdateStatus(ctx context.Context, id string, status string) error {
	var processedAt *string
	if status == "completed" || status == "failed" {
		t := time.Now().UTC().Format(time.RFC3339)
		processedAt = &t
	}

	query := `
		UPDATE events
		SET status = ?, processed_at = ?
		WHERE id = ?
	`

	_, err := s.db.ExecContext(ctx, query, status, processedAt, id)
	if err != nil {
		return fmt.Errorf("updating event status: %w", err)
	}

	return nil
}

// DeleteOlderThan deletes events older than the given duration.
func (s *Store) DeleteOlderThan(ctx context.Context, duration time.Duration) error {
	cutoff := time.Now().UTC().Add(-duration).Format(time.RFC3339)

	query := `
		DELETE FROM events
		WHERE created_at < ? AND status IN ('completed', 'failed')
	`

	result, err := s.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return fmt.Errorf("deleting old events: %w", err)
	}

	_, _ = result.RowsAffected()
	return nil
}

// scanEvents scans rows into Event structs.
func (s *Store) scanEvents(rows *sql.Rows) ([]*Event, error) {
	var events []*Event

	for rows.Next() {
		var event Event
		var payloadJSON, metadataJSON string
		var createdAt, processAt, processedAt sql.NullString

		err := rows.Scan(
			&event.ID,
			&event.Type,
			&event.Source,
			&event.Action,
			&payloadJSON,
			&metadataJSON,
			&createdAt,
			&processAt,
			&processedAt,
			&event.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning event row: %w", err)
		}

		// Deserialize payload
		if err := json.Unmarshal([]byte(payloadJSON), &event.Payload); err != nil {
			return nil, fmt.Errorf("unmarshaling payload: %w", err)
		}

		// Deserialize metadata
		if err := json.Unmarshal([]byte(metadataJSON), &event.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshaling metadata: %w", err)
		}

		// Parse timestamps
		if createdAt.Valid {
			t, err := time.Parse(time.RFC3339, createdAt.String)
			if err != nil {
				return nil, fmt.Errorf("parsing created_at: %w", err)
			}
			event.CreatedAt = t
		}

		if processAt.Valid {
			t, err := time.Parse(time.RFC3339, processAt.String)
			if err != nil {
				return nil, fmt.Errorf("parsing process_at: %w", err)
			}
			event.ProcessAt = &t
		}

		if processedAt.Valid {
			t, err := time.Parse(time.RFC3339, processedAt.String)
			if err != nil {
				return nil, fmt.Errorf("parsing processed_at: %w", err)
			}
			event.ProcessedAt = &t
		}

		events = append(events, &event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating event rows: %w", err)
	}

	return events, nil
}
