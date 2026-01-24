package realtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/database"
)

// ChangeDetector polls the _alyx_changes table for new changes.
type ChangeDetector struct {
	db           *database.DB
	pollInterval time.Duration
	changeCh     chan<- *Change
	lastID       int64
	done         chan struct{}
	mu           sync.Mutex
}

// NewChangeDetector creates a new change detector.
func NewChangeDetector(db *database.DB, pollIntervalMs int64, changeCh chan<- *Change) *ChangeDetector {
	return &ChangeDetector{
		db:           db,
		pollInterval: time.Duration(pollIntervalMs) * time.Millisecond,
		changeCh:     changeCh,
		done:         make(chan struct{}),
	}
}

// Start begins polling for changes.
func (d *ChangeDetector) Start(ctx context.Context) {
	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.poll(ctx)
		case <-d.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stop halts the change detector.
func (d *ChangeDetector) Stop() {
	close(d.done)
}

func (d *ChangeDetector) poll(ctx context.Context) {
	d.mu.Lock()
	lastID := d.lastID
	d.mu.Unlock()

	query := `
		SELECT id, collection, operation, doc_id, changed_fields, timestamp
		FROM _alyx_changes
		WHERE id > ?
		ORDER BY id ASC
		LIMIT 1000
	`

	rows, err := d.db.QueryContext(ctx, query, lastID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to poll changes")
		return
	}
	defer rows.Close()

	var changes []*Change
	maxID := lastID

	for rows.Next() {
		var (
			id            int64
			collection    string
			operation     string
			docID         string
			changedFields sql.NullString
			timestamp     string
		)

		if err := rows.Scan(&id, &collection, &operation, &docID, &changedFields, &timestamp); err != nil {
			log.Error().Err(err).Msg("Failed to scan change row")
			continue
		}

		change := &Change{
			ID:         id,
			Collection: collection,
			Operation:  Operation(operation),
			DocID:      docID,
		}

		if changedFields.Valid {
			var fields []string
			if err := json.Unmarshal([]byte(changedFields.String), &fields); err == nil {
				var nonNullFields []string
				for _, f := range fields {
					if f != "" {
						nonNullFields = append(nonNullFields, f)
					}
				}
				change.ChangedFields = nonNullFields
			}
		}

		if ts, err := time.Parse(time.RFC3339, timestamp); err == nil {
			change.Timestamp = ts
		} else if ts, err := time.Parse("2006-01-02 15:04:05", timestamp); err == nil {
			change.Timestamp = ts
		}

		changes = append(changes, change)
		if id > maxID {
			maxID = id
		}
	}

	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("Error iterating change rows")
		return
	}

	if len(changes) > 0 {
		d.mu.Lock()
		d.lastID = maxID
		d.mu.Unlock()

		for _, change := range changes {
			select {
			case d.changeCh <- change:
			case <-d.done:
				return
			case <-ctx.Done():
				return
			default:
				log.Warn().Int64("change_id", change.ID).Msg("Change channel full, dropping change")
			}
		}

		d.markProcessed(ctx, maxID)
	}
}

func (d *ChangeDetector) markProcessed(ctx context.Context, maxID int64) {
	query := `UPDATE _alyx_changes SET processed = 1 WHERE id <= ? AND processed = 0`
	_, err := d.db.ExecContext(ctx, query, maxID)
	if err != nil {
		log.Error().Err(err).Int64("max_id", maxID).Msg("Failed to mark changes as processed")
	}
}

// CleanupOldChanges removes old processed changes.
func (d *ChangeDetector) CleanupOldChanges(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan).Format(time.RFC3339)
	query := `DELETE FROM _alyx_changes WHERE processed = 1 AND timestamp < ?`
	_, err := d.db.ExecContext(ctx, query, cutoff)
	return err
}
