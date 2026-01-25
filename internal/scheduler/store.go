package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/watzon/alyx/internal/database"
)

// Store handles database operations for schedules.
type Store struct {
	db *database.DB
}

// NewStore creates a new schedule store.
func NewStore(db *database.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new schedule.
func (s *Store) Create(ctx context.Context, schedule *Schedule) error {
	if schedule.ID == "" {
		schedule.ID = uuid.New().String()
	}
	if schedule.CreatedAt.IsZero() {
		schedule.CreatedAt = time.Now().UTC()
	}
	if schedule.UpdatedAt.IsZero() {
		schedule.UpdatedAt = time.Now().UTC()
	}
	if schedule.Timezone == "" {
		schedule.Timezone = "UTC"
	}

	// Calculate initial next_run if not set
	if schedule.NextRun == nil {
		nextRun, err := CalculateNextRun(schedule, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("calculating initial next_run: %w", err)
		}
		schedule.NextRun = &nextRun
	}

	// Serialize config
	configJSON, err := json.Marshal(schedule.Config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	query := `
		INSERT INTO schedules (id, name, function_id, type, expression, timezone, next_run, last_run, last_status, enabled, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var nextRunStr, lastRunStr sql.NullString
	if schedule.NextRun != nil {
		nextRunStr = sql.NullString{String: schedule.NextRun.UTC().Format(time.RFC3339), Valid: true}
	}
	if schedule.LastRun != nil {
		lastRunStr = sql.NullString{String: schedule.LastRun.UTC().Format(time.RFC3339), Valid: true}
	}

	_, err = s.db.ExecContext(ctx, query,
		schedule.ID,
		schedule.Name,
		schedule.FunctionID,
		string(schedule.Type),
		schedule.Expression,
		schedule.Timezone,
		nextRunStr,
		lastRunStr,
		schedule.LastStatus,
		schedule.Enabled,
		string(configJSON),
		schedule.CreatedAt.UTC().Format(time.RFC3339),
		schedule.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting schedule: %w", err)
	}

	return nil
}

// Update updates an existing schedule.
func (s *Store) Update(ctx context.Context, schedule *Schedule) error {
	schedule.UpdatedAt = time.Now().UTC()

	// Serialize config
	configJSON, err := json.Marshal(schedule.Config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	query := `
		UPDATE schedules
		SET name = ?, function_id = ?, type = ?, expression = ?, timezone = ?, next_run = ?, last_run = ?, last_status = ?, enabled = ?, config = ?, updated_at = ?
		WHERE id = ?
	`

	var nextRunStr, lastRunStr sql.NullString
	if schedule.NextRun != nil {
		nextRunStr = sql.NullString{String: schedule.NextRun.UTC().Format(time.RFC3339), Valid: true}
	}
	if schedule.LastRun != nil {
		lastRunStr = sql.NullString{String: schedule.LastRun.UTC().Format(time.RFC3339), Valid: true}
	}

	_, err = s.db.ExecContext(ctx, query,
		schedule.Name,
		schedule.FunctionID,
		string(schedule.Type),
		schedule.Expression,
		schedule.Timezone,
		nextRunStr,
		lastRunStr,
		schedule.LastStatus,
		schedule.Enabled,
		string(configJSON),
		schedule.UpdatedAt.UTC().Format(time.RFC3339),
		schedule.ID,
	)
	if err != nil {
		return fmt.Errorf("updating schedule: %w", err)
	}

	return nil
}

// UpdateNextRun updates the next_run and last_run fields.
func (s *Store) UpdateNextRun(ctx context.Context, scheduleID string, nextRun, lastRun time.Time) error {
	query := `
		UPDATE schedules
		SET next_run = ?, last_run = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := s.db.ExecContext(ctx, query,
		nextRun.UTC().Format(time.RFC3339),
		lastRun.UTC().Format(time.RFC3339),
		time.Now().UTC().Format(time.RFC3339),
		scheduleID,
	)
	if err != nil {
		return fmt.Errorf("updating next_run: %w", err)
	}

	return nil
}

// UpdateStatus updates the last_status field.
func (s *Store) UpdateStatus(ctx context.Context, scheduleID, status string) error {
	query := `
		UPDATE schedules
		SET last_status = ?, updated_at = ?
		WHERE id = ?
	`

	_, err := s.db.ExecContext(ctx, query,
		status,
		time.Now().UTC().Format(time.RFC3339),
		scheduleID,
	)
	if err != nil {
		return fmt.Errorf("updating status: %w", err)
	}

	return nil
}

// Delete removes a schedule.
func (s *Store) Delete(ctx context.Context, scheduleID string) error {
	query := `DELETE FROM schedules WHERE id = ?`

	_, err := s.db.ExecContext(ctx, query, scheduleID)
	if err != nil {
		return fmt.Errorf("deleting schedule: %w", err)
	}

	return nil
}

// Get retrieves a schedule by ID.
func (s *Store) Get(ctx context.Context, scheduleID string) (*Schedule, error) {
	query := `
		SELECT id, name, function_id, type, expression, timezone, next_run, last_run, last_status, enabled, config, created_at, updated_at
		FROM schedules
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, scheduleID)

	schedule, err := s.scanSchedule(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("schedule not found: %s", scheduleID)
		}
		return nil, fmt.Errorf("getting schedule: %w", err)
	}

	return schedule, nil
}

// List retrieves all schedules.
func (s *Store) List(ctx context.Context) ([]*Schedule, error) {
	query := `
		SELECT id, name, function_id, type, expression, timezone, next_run, last_run, last_status, enabled, config, created_at, updated_at
		FROM schedules
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying schedules: %w", err)
	}
	defer rows.Close()

	return s.scanSchedules(rows)
}

// GetDue retrieves schedules that are due to run.
func (s *Store) GetDue(ctx context.Context, limit int) ([]*Schedule, error) {
	query := `
		SELECT id, name, function_id, type, expression, timezone, next_run, last_run, last_status, enabled, config, created_at, updated_at
		FROM schedules
		WHERE enabled = 1
		  AND next_run IS NOT NULL
		  AND next_run <= ?
		ORDER BY next_run ASC
		LIMIT ?
	`

	now := time.Now().UTC().Format(time.RFC3339)

	rows, err := s.db.QueryContext(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("querying due schedules: %w", err)
	}
	defer rows.Close()

	return s.scanSchedules(rows)
}

// FindByFunction finds schedules for a function.
func (s *Store) FindByFunction(ctx context.Context, functionID string) ([]*Schedule, error) {
	query := `
		SELECT id, name, function_id, type, expression, timezone, next_run, last_run, last_status, enabled, config, created_at, updated_at
		FROM schedules
		WHERE function_id = ?
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, functionID)
	if err != nil {
		return nil, fmt.Errorf("querying schedules by function: %w", err)
	}
	defer rows.Close()

	return s.scanSchedules(rows)
}

// scanSchedule scans a single row into a Schedule struct.
func (s *Store) scanSchedule(row *sql.Row) (*Schedule, error) {
	var schedule Schedule
	var configJSON string
	var scheduleType string
	var nextRun, lastRun sql.NullString
	var createdAt, updatedAt string
	var enabled int

	err := row.Scan(
		&schedule.ID,
		&schedule.Name,
		&schedule.FunctionID,
		&scheduleType,
		&schedule.Expression,
		&schedule.Timezone,
		&nextRun,
		&lastRun,
		&schedule.LastStatus,
		&enabled,
		&configJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Deserialize config
	if unmarshalErr := json.Unmarshal([]byte(configJSON), &schedule.Config); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", unmarshalErr)
	}

	// Parse type
	schedule.Type = ScheduleType(scheduleType)

	// Parse enabled
	schedule.Enabled = enabled == 1

	// Parse next_run
	if nextRun.Valid {
		nextRunTime, parseErr := time.Parse(time.RFC3339, nextRun.String)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing next_run: %w", parseErr)
		}
		schedule.NextRun = &nextRunTime
	}

	// Parse last_run
	if lastRun.Valid {
		lastRunTime, parseErr := time.Parse(time.RFC3339, lastRun.String)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing last_run: %w", parseErr)
		}
		schedule.LastRun = &lastRunTime
	}

	// Parse timestamps
	createdAtTime, parseErr := time.Parse(time.RFC3339, createdAt)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing created_at: %w", parseErr)
	}
	schedule.CreatedAt = createdAtTime

	updatedAtTime, parseErr := time.Parse(time.RFC3339, updatedAt)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", parseErr)
	}
	schedule.UpdatedAt = updatedAtTime

	return &schedule, nil
}

// scanSchedules scans rows into Schedule structs.
func (s *Store) scanSchedules(rows *sql.Rows) ([]*Schedule, error) {
	var schedules []*Schedule

	for rows.Next() {
		var schedule Schedule
		var configJSON string
		var scheduleType string
		var nextRun, lastRun sql.NullString
		var createdAt, updatedAt string
		var enabled int

		err := rows.Scan(
			&schedule.ID,
			&schedule.Name,
			&schedule.FunctionID,
			&scheduleType,
			&schedule.Expression,
			&schedule.Timezone,
			&nextRun,
			&lastRun,
			&schedule.LastStatus,
			&enabled,
			&configJSON,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning schedule row: %w", err)
		}

		// Deserialize config
		if unmarshalErr := json.Unmarshal([]byte(configJSON), &schedule.Config); unmarshalErr != nil {
			return nil, fmt.Errorf("unmarshaling config: %w", unmarshalErr)
		}

		// Parse type
		schedule.Type = ScheduleType(scheduleType)

		// Parse enabled
		schedule.Enabled = enabled == 1

		// Parse next_run
		if nextRun.Valid {
			nextRunTime, parseErr := time.Parse(time.RFC3339, nextRun.String)
			if parseErr != nil {
				return nil, fmt.Errorf("parsing next_run: %w", parseErr)
			}
			schedule.NextRun = &nextRunTime
		}

		// Parse last_run
		if lastRun.Valid {
			lastRunTime, parseErr := time.Parse(time.RFC3339, lastRun.String)
			if parseErr != nil {
				return nil, fmt.Errorf("parsing last_run: %w", parseErr)
			}
			schedule.LastRun = &lastRunTime
		}

		// Parse timestamps
		createdAtTime, parseErr := time.Parse(time.RFC3339, createdAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing created_at: %w", parseErr)
		}
		schedule.CreatedAt = createdAtTime

		updatedAtTime, parseErr := time.Parse(time.RFC3339, updatedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing updated_at: %w", parseErr)
		}
		schedule.UpdatedAt = updatedAtTime

		schedules = append(schedules, &schedule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating schedule rows: %w", err)
	}

	return schedules, nil
}
