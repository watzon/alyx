package executions

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/watzon/alyx/internal/database"
)

// Store handles database operations for executions.
type Store struct {
	db *database.DB
}

// NewStore creates a new execution store.
func NewStore(db *database.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new execution log.
func (s *Store) Create(ctx context.Context, log *ExecutionLog) error {
	query := `
		INSERT INTO executions (
			id, function_id, request_id, trigger_type, trigger_id,
			status, started_at, completed_at, duration_ms,
			input, output, error, logs
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var completedAt sql.NullString
	if log.CompletedAt != nil {
		completedAt = sql.NullString{
			String: log.CompletedAt.UTC().Format(time.RFC3339),
			Valid:  true,
		}
	}

	_, err := s.db.ExecContext(ctx, query,
		log.ID,
		log.FunctionID,
		log.RequestID,
		log.TriggerType,
		log.TriggerID,
		log.Status,
		log.StartedAt.UTC().Format(time.RFC3339),
		completedAt,
		log.DurationMs,
		log.Input,
		log.Output,
		log.Error,
		log.Logs,
	)
	if err != nil {
		return fmt.Errorf("inserting execution log: %w", err)
	}

	return nil
}

// Update updates an execution log.
func (s *Store) Update(ctx context.Context, log *ExecutionLog) error {
	query := `
		UPDATE executions
		SET status = ?, completed_at = ?, duration_ms = ?,
		    output = ?, error = ?, logs = ?
		WHERE id = ?
	`

	var completedAt sql.NullString
	if log.CompletedAt != nil {
		completedAt = sql.NullString{
			String: log.CompletedAt.UTC().Format(time.RFC3339),
			Valid:  true,
		}
	}

	result, err := s.db.ExecContext(ctx, query,
		log.Status,
		completedAt,
		log.DurationMs,
		log.Output,
		log.Error,
		log.Logs,
		log.ID,
	)
	if err != nil {
		return fmt.Errorf("updating execution log: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("execution log not found: %s", log.ID)
	}

	return nil
}

// Get retrieves an execution log by ID.
func (s *Store) Get(ctx context.Context, id string) (*ExecutionLog, error) {
	query := `
		SELECT id, function_id, request_id, trigger_type, trigger_id,
		       status, started_at, completed_at, duration_ms,
		       input, output, error, logs
		FROM executions
		WHERE id = ?
	`

	var log ExecutionLog
	var startedAtStr string
	var completedAt sql.NullString

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&log.ID,
		&log.FunctionID,
		&log.RequestID,
		&log.TriggerType,
		&log.TriggerID,
		&log.Status,
		&startedAtStr,
		&completedAt,
		&log.DurationMs,
		&log.Input,
		&log.Output,
		&log.Error,
		&log.Logs,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("execution log not found: %s", id)
		}
		return nil, fmt.Errorf("querying execution log: %w", err)
	}

	startedAt, err := time.Parse(time.RFC3339, startedAtStr)
	if err != nil {
		return nil, fmt.Errorf("parsing started_at: %w", err)
	}
	log.StartedAt = startedAt

	// Parse completed_at if present
	if completedAt.Valid {
		t, err := time.Parse(time.RFC3339, completedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parsing completed_at: %w", err)
		}
		log.CompletedAt = &t
	}

	return &log, nil
}

// List retrieves execution logs with filters.
func (s *Store) List(ctx context.Context, filters map[string]any, limit, offset int) ([]*ExecutionLog, error) {
	query := `
		SELECT id, function_id, request_id, trigger_type, trigger_id,
		       status, started_at, completed_at, duration_ms,
		       input, output, error, logs
		FROM executions
		WHERE 1=1
	`
	args := []any{}

	// Apply filters
	if functionID, ok := filters["function_id"].(string); ok && functionID != "" {
		query += " AND function_id = ?"
		args = append(args, functionID)
	}
	if status, ok := filters["status"].(string); ok && status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if triggerType, ok := filters["trigger_type"].(string); ok && triggerType != "" {
		query += " AND trigger_type = ?"
		args = append(args, triggerType)
	}
	if triggerID, ok := filters["trigger_id"].(string); ok && triggerID != "" {
		query += " AND trigger_id = ?"
		args = append(args, triggerID)
	}

	// Order by started_at DESC
	query += " ORDER BY started_at DESC"

	// Apply limit and offset
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	if offset > 0 {
		query += " OFFSET ?"
		args = append(args, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying execution logs: %w", err)
	}
	defer rows.Close()

	var logs []*ExecutionLog
	for rows.Next() {
		var log ExecutionLog
		var startedAtStr string
		var completedAt sql.NullString

		if err := rows.Scan(
			&log.ID,
			&log.FunctionID,
			&log.RequestID,
			&log.TriggerType,
			&log.TriggerID,
			&log.Status,
			&startedAtStr,
			&completedAt,
			&log.DurationMs,
			&log.Input,
			&log.Output,
			&log.Error,
			&log.Logs,
		); err != nil {
			return nil, fmt.Errorf("scanning execution log: %w", err)
		}

		// Parse started_at
		startedAt, err := time.Parse(time.RFC3339, startedAtStr)
		if err != nil {
			return nil, fmt.Errorf("parsing started_at: %w", err)
		}
		log.StartedAt = startedAt

		// Parse completed_at if present
		if completedAt.Valid {
			t, err := time.Parse(time.RFC3339, completedAt.String)
			if err != nil {
				return nil, fmt.Errorf("parsing completed_at: %w", err)
			}
			log.CompletedAt = &t
		}

		logs = append(logs, &log)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating execution logs: %w", err)
	}

	return logs, nil
}

// DeleteOlderThan deletes logs older than the given duration.
func (s *Store) DeleteOlderThan(ctx context.Context, duration time.Duration) error {
	cutoff := time.Now().UTC().Add(-duration).Format(time.RFC3339)

	query := `
		DELETE FROM executions
		WHERE started_at < ?
		  AND status IN ('success', 'failed', 'timed_out', 'canceled')
	`

	result, err := s.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return fmt.Errorf("deleting old execution logs: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rows > 0 {
		// Log deletion count (optional)
		_ = rows
	}

	return nil
}

// serializeLogs converts log entries to JSON string.
func serializeLogs(logs []map[string]any) string {
	if len(logs) == 0 {
		return "[]"
	}
	data, err := json.Marshal(logs)
	if err != nil {
		return "[]"
	}
	return string(data)
}
