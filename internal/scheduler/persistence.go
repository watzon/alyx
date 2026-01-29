package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/watzon/alyx/internal/database"
)

type StateStore struct {
	db *database.DB
}

func NewStateStore(db *database.DB) *StateStore {
	return &StateStore{db: db}
}

type ScheduleState struct {
	ScheduleID      string
	LastExecutionAt *time.Time
	NextExecutionAt *time.Time
	ExecutionCount  int
	UpdatedAt       time.Time
}

func (s *StateStore) Save(ctx context.Context, state *ScheduleState) error {
	state.UpdatedAt = time.Now().UTC()

	var lastExecStr, nextExecStr sql.NullString
	if state.LastExecutionAt != nil {
		lastExecStr = sql.NullString{String: state.LastExecutionAt.UTC().Format(time.RFC3339), Valid: true}
	}
	if state.NextExecutionAt != nil {
		nextExecStr = sql.NullString{String: state.NextExecutionAt.UTC().Format(time.RFC3339), Valid: true}
	}

	query := `
		INSERT INTO _alyx_scheduler_state (schedule_id, last_execution_at, next_execution_at, execution_count, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(schedule_id) DO UPDATE SET
			last_execution_at = excluded.last_execution_at,
			next_execution_at = excluded.next_execution_at,
			execution_count = excluded.execution_count,
			updated_at = excluded.updated_at
	`

	_, err := s.db.ExecContext(ctx, query,
		state.ScheduleID,
		lastExecStr,
		nextExecStr,
		state.ExecutionCount,
		state.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("saving scheduler state: %w", err)
	}

	return nil
}

func (s *StateStore) Get(ctx context.Context, scheduleID string) (*ScheduleState, error) {
	query := `
		SELECT schedule_id, last_execution_at, next_execution_at, execution_count, updated_at
		FROM _alyx_scheduler_state
		WHERE schedule_id = ?
	`

	var state ScheduleState
	var lastExec, nextExec sql.NullString
	var updatedAt string

	err := s.db.QueryRowContext(ctx, query, scheduleID).Scan(
		&state.ScheduleID,
		&lastExec,
		&nextExec,
		&state.ExecutionCount,
		&updatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("getting scheduler state: %w", err)
	}

	if lastExec.Valid {
		t, parseErr := time.Parse(time.RFC3339, lastExec.String)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing last_execution_at: %w", parseErr)
		}
		state.LastExecutionAt = &t
	}

	if nextExec.Valid {
		t, parseErr := time.Parse(time.RFC3339, nextExec.String)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing next_execution_at: %w", parseErr)
		}
		state.NextExecutionAt = &t
	}

	t, parseErr := time.Parse(time.RFC3339, updatedAt)
	if parseErr != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", parseErr)
	}
	state.UpdatedAt = t

	return &state, nil
}

func (s *StateStore) Delete(ctx context.Context, scheduleID string) error {
	query := `DELETE FROM _alyx_scheduler_state WHERE schedule_id = ?`

	_, err := s.db.ExecContext(ctx, query, scheduleID)
	if err != nil {
		return fmt.Errorf("deleting scheduler state: %w", err)
	}

	return nil
}

func (s *StateStore) List(ctx context.Context) ([]*ScheduleState, error) {
	query := `
		SELECT schedule_id, last_execution_at, next_execution_at, execution_count, updated_at
		FROM _alyx_scheduler_state
		ORDER BY schedule_id
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying scheduler states: %w", err)
	}
	defer rows.Close()

	var states []*ScheduleState
	for rows.Next() {
		var state ScheduleState
		var lastExec, nextExec sql.NullString
		var updatedAt string

		err := rows.Scan(
			&state.ScheduleID,
			&lastExec,
			&nextExec,
			&state.ExecutionCount,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning scheduler state: %w", err)
		}

		if lastExec.Valid {
			t, parseErr := time.Parse(time.RFC3339, lastExec.String)
			if parseErr != nil {
				return nil, fmt.Errorf("parsing last_execution_at: %w", parseErr)
			}
			state.LastExecutionAt = &t
		}

		if nextExec.Valid {
			t, parseErr := time.Parse(time.RFC3339, nextExec.String)
			if parseErr != nil {
				return nil, fmt.Errorf("parsing next_execution_at: %w", parseErr)
			}
			state.NextExecutionAt = &t
		}

		t, parseErr := time.Parse(time.RFC3339, updatedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parsing updated_at: %w", parseErr)
		}
		state.UpdatedAt = t

		states = append(states, &state)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating scheduler states: %w", err)
	}

	return states, nil
}

func (s *StateStore) UpdateAfterExecution(ctx context.Context, scheduleID string, nextRun time.Time) error {
	query := `
		UPDATE _alyx_scheduler_state
		SET last_execution_at = ?,
		    next_execution_at = ?,
		    execution_count = execution_count + 1,
		    updated_at = ?
		WHERE schedule_id = ?
	`

	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, query,
		now.Format(time.RFC3339),
		nextRun.Format(time.RFC3339),
		now.Format(time.RFC3339),
		scheduleID,
	)

	if err != nil {
		return fmt.Errorf("updating scheduler state after execution: %w", err)
	}

	return nil
}
