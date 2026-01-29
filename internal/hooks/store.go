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

type Store struct {
	db *database.DB
}

func NewStore(db *database.DB) *Store {
	return &Store{db: db}
}

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

	var configJSON []byte
	var err error
	if hook.Config != nil {
		configJSON, err = json.Marshal(hook.Config)
		if err != nil {
			return fmt.Errorf("marshaling config: %w", err)
		}
	}

	query := `
		INSERT INTO _alyx_hooks (id, type, source, action, function_name, mode, enabled, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query,
		hook.ID,
		hook.Type,
		hook.Source,
		hook.Action,
		hook.FunctionName,
		hook.Mode,
		hook.Enabled,
		string(configJSON),
		hook.CreatedAt.UTC().Format(time.RFC3339),
		hook.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting hook: %w", err)
	}

	return nil
}

func (s *Store) Update(ctx context.Context, hook *Hook) error {
	hook.UpdatedAt = time.Now().UTC()

	var configJSON []byte
	var err error
	if hook.Config != nil {
		configJSON, err = json.Marshal(hook.Config)
		if err != nil {
			return fmt.Errorf("marshaling config: %w", err)
		}
	}

	query := `
		UPDATE _alyx_hooks
		SET type = ?, source = ?, action = ?, function_name = ?, mode = ?, enabled = ?, config = ?, updated_at = ?
		WHERE id = ?
	`

	_, err = s.db.ExecContext(ctx, query,
		hook.Type,
		hook.Source,
		hook.Action,
		hook.FunctionName,
		hook.Mode,
		hook.Enabled,
		string(configJSON),
		hook.UpdatedAt.UTC().Format(time.RFC3339),
		hook.ID,
	)
	if err != nil {
		return fmt.Errorf("updating hook: %w", err)
	}

	return nil
}

func (s *Store) Delete(ctx context.Context, hookID string) error {
	query := `DELETE FROM _alyx_hooks WHERE id = ?`

	_, err := s.db.ExecContext(ctx, query, hookID)
	if err != nil {
		return fmt.Errorf("deleting hook: %w", err)
	}

	return nil
}

func (s *Store) Get(ctx context.Context, hookID string) (*Hook, error) {
	query := `
		SELECT id, type, source, action, function_name, mode, enabled, config, created_at, updated_at
		FROM _alyx_hooks
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

func (s *Store) List(ctx context.Context) ([]*Hook, error) {
	query := `
		SELECT id, type, source, action, function_name, mode, enabled, config, created_at, updated_at
		FROM _alyx_hooks
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying hooks: %w", err)
	}
	defer rows.Close()

	return s.scanHooks(rows)
}

func (s *Store) ListByType(ctx context.Context, hookType HookType) ([]*Hook, error) {
	query := `
		SELECT id, type, source, action, function_name, mode, enabled, config, created_at, updated_at
		FROM _alyx_hooks
		WHERE type = ?
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, hookType)
	if err != nil {
		return nil, fmt.Errorf("querying hooks by type: %w", err)
	}
	defer rows.Close()

	return s.scanHooks(rows)
}

func (s *Store) ListBySourceAction(ctx context.Context, hookType HookType, source string, action string) ([]*Hook, error) {
	query := `
		SELECT id, type, source, action, function_name, mode, enabled, config, created_at, updated_at
		FROM _alyx_hooks
		WHERE type = ? AND source = ? AND (action = ? OR action IS NULL OR action = '') AND enabled = 1
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, hookType, source, action)
	if err != nil {
		return nil, fmt.Errorf("querying hooks by source/action: %w", err)
	}
	defer rows.Close()

	return s.scanHooks(rows)
}

func (s *Store) scanHook(row *sql.Row) (*Hook, error) {
	var hook Hook
	var configJSON sql.NullString
	var action sql.NullString
	var createdAt, updatedAt string
	var enabled int

	err := row.Scan(
		&hook.ID,
		&hook.Type,
		&hook.Source,
		&action,
		&hook.FunctionName,
		&hook.Mode,
		&enabled,
		&configJSON,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if action.Valid {
		hook.Action = action.String
	}

	if configJSON.Valid && configJSON.String != "" {
		if unmarshalErr := json.Unmarshal([]byte(configJSON.String), &hook.Config); unmarshalErr != nil {
			return nil, fmt.Errorf("unmarshaling config: %w", unmarshalErr)
		}
	}

	hook.Enabled = enabled == 1

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

func (s *Store) scanHooks(rows *sql.Rows) ([]*Hook, error) {
	var hooks []*Hook

	for rows.Next() {
		var hook Hook
		var configJSON sql.NullString
		var action sql.NullString
		var createdAt, updatedAt string
		var enabled int

		err := rows.Scan(
			&hook.ID,
			&hook.Type,
			&hook.Source,
			&action,
			&hook.FunctionName,
			&hook.Mode,
			&enabled,
			&configJSON,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning hook row: %w", err)
		}

		if action.Valid {
			hook.Action = action.String
		}

		if configJSON.Valid && configJSON.String != "" {
			if unmarshalErr := json.Unmarshal([]byte(configJSON.String), &hook.Config); unmarshalErr != nil {
				return nil, fmt.Errorf("unmarshaling config: %w", unmarshalErr)
			}
		}

		hook.Enabled = enabled == 1

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
