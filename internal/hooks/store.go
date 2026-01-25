package hooks

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

// Store handles database operations for hooks.
type Store struct {
	db *database.DB
}

// NewStore creates a new hook store.
func NewStore(db *database.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new hook.
func (s *Store) Create(ctx context.Context, hook *Hook) error {
	if hook.ID == "" {
		hook.ID = uuid.New().String()
	}
	if hook.CreatedAt.IsZero() {
		hook.CreatedAt = time.Now().UTC()
	}
	if hook.UpdatedAt.IsZero() {
		hook.UpdatedAt = time.Now().UTC()
	}
	if hook.Mode == "" {
		hook.Mode = HookModeAsync
	}

	// Serialize config
	configJSON, err := json.Marshal(hook.Config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	query := `
		INSERT INTO hooks (id, name, function_id, event_type, event_source, event_action, mode, priority, config, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query,
		hook.ID,
		hook.Name,
		hook.FunctionID,
		hook.EventType,
		hook.EventSource,
		hook.EventAction,
		string(hook.Mode),
		hook.Priority,
		string(configJSON),
		hook.Enabled,
		hook.CreatedAt.UTC().Format(time.RFC3339),
		hook.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting hook: %w", err)
	}

	return nil
}

// Update updates an existing hook.
func (s *Store) Update(ctx context.Context, hook *Hook) error {
	hook.UpdatedAt = time.Now().UTC()

	// Serialize config
	configJSON, err := json.Marshal(hook.Config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	query := `
		UPDATE hooks
		SET name = ?, function_id = ?, event_type = ?, event_source = ?, event_action = ?, mode = ?, priority = ?, config = ?, enabled = ?, updated_at = ?
		WHERE id = ?
	`

	_, err = s.db.ExecContext(ctx, query,
		hook.Name,
		hook.FunctionID,
		hook.EventType,
		hook.EventSource,
		hook.EventAction,
		string(hook.Mode),
		hook.Priority,
		string(configJSON),
		hook.Enabled,
		hook.UpdatedAt.UTC().Format(time.RFC3339),
		hook.ID,
	)
	if err != nil {
		return fmt.Errorf("updating hook: %w", err)
	}

	return nil
}

// Delete removes a hook.
func (s *Store) Delete(ctx context.Context, hookID string) error {
	query := `DELETE FROM hooks WHERE id = ?`

	_, err := s.db.ExecContext(ctx, query, hookID)
	if err != nil {
		return fmt.Errorf("deleting hook: %w", err)
	}

	return nil
}

// Get retrieves a hook by ID.
func (s *Store) Get(ctx context.Context, hookID string) (*Hook, error) {
	query := `
		SELECT id, name, function_id, event_type, event_source, event_action, mode, priority, config, enabled, created_at, updated_at
		FROM hooks
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, hookID)

	hook, err := s.scanHook(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("hook not found: %s", hookID)
		}
		return nil, fmt.Errorf("getting hook: %w", err)
	}

	return hook, nil
}

// List retrieves all hooks.
func (s *Store) List(ctx context.Context) ([]*Hook, error) {
	query := `
		SELECT id, name, function_id, event_type, event_source, event_action, mode, priority, config, enabled, created_at, updated_at
		FROM hooks
		ORDER BY priority DESC, created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying hooks: %w", err)
	}
	defer rows.Close()

	return s.scanHooks(rows)
}

// FindByEvent finds hooks matching event pattern.
func (s *Store) FindByEvent(ctx context.Context, eventType, source, action string) ([]*Hook, error) {
	query := `
		SELECT id, name, function_id, event_type, event_source, event_action, mode, priority, config, enabled, created_at, updated_at
		FROM hooks
		WHERE enabled = 1
		  AND event_type = ?
		  AND (event_source = ? OR event_source = '*')
		  AND (event_action = ? OR event_action = '*')
		ORDER BY priority DESC, created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, eventType, source, action)
	if err != nil {
		return nil, fmt.Errorf("querying hooks by event: %w", err)
	}
	defer rows.Close()

	return s.scanHooks(rows)
}

// FindByFunction finds hooks for a function.
func (s *Store) FindByFunction(ctx context.Context, functionID string) ([]*Hook, error) {
	query := `
		SELECT id, name, function_id, event_type, event_source, event_action, mode, priority, config, enabled, created_at, updated_at
		FROM hooks
		WHERE function_id = ?
		ORDER BY priority DESC, created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, functionID)
	if err != nil {
		return nil, fmt.Errorf("querying hooks by function: %w", err)
	}
	defer rows.Close()

	return s.scanHooks(rows)
}

// scanHook scans a single row into a Hook struct.
func (s *Store) scanHook(row *sql.Row) (*Hook, error) {
	var hook Hook
	var configJSON string
	var mode string
	var createdAt, updatedAt string
	var enabled int

	err := row.Scan(
		&hook.ID,
		&hook.Name,
		&hook.FunctionID,
		&hook.EventType,
		&hook.EventSource,
		&hook.EventAction,
		&mode,
		&hook.Priority,
		&configJSON,
		&enabled,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Deserialize config
	if unmarshalErr := json.Unmarshal([]byte(configJSON), &hook.Config); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", unmarshalErr)
	}

	// Parse mode
	hook.Mode = HookMode(mode)

	// Parse enabled
	hook.Enabled = enabled == 1

	// Parse timestamps
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	hook.CreatedAt = t

	t, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parsing updated_at: %w", err)
	}
	hook.UpdatedAt = t

	return &hook, nil
}

// scanHooks scans rows into Hook structs.
func (s *Store) scanHooks(rows *sql.Rows) ([]*Hook, error) {
	var hooks []*Hook

	for rows.Next() {
		var hook Hook
		var configJSON string
		var mode string
		var createdAt, updatedAt string
		var enabled int

		err := rows.Scan(
			&hook.ID,
			&hook.Name,
			&hook.FunctionID,
			&hook.EventType,
			&hook.EventSource,
			&hook.EventAction,
			&mode,
			&hook.Priority,
			&configJSON,
			&enabled,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning hook row: %w", err)
		}

		// Deserialize config
		if unmarshalErr := json.Unmarshal([]byte(configJSON), &hook.Config); unmarshalErr != nil {
			return nil, fmt.Errorf("unmarshaling config: %w", unmarshalErr)
		}

		// Parse mode
		hook.Mode = HookMode(mode)

		// Parse enabled
		hook.Enabled = enabled == 1

		// Parse timestamps
		t, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parsing created_at: %w", err)
		}
		hook.CreatedAt = t

		t, err = time.Parse(time.RFC3339, updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parsing updated_at: %w", err)
		}
		hook.UpdatedAt = t

		hooks = append(hooks, &hook)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating hook rows: %w", err)
	}

	return hooks, nil
}
