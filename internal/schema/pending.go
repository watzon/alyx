package schema

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type PendingChange struct {
	ID          string     `json:"id"`
	Type        ChangeType `json:"type"`
	Collection  string     `json:"collection"`
	Field       string     `json:"field,omitempty"`
	Description string     `json:"description"`
	Safe        bool       `json:"safe"`
	CreatedAt   time.Time  `json:"created_at"`
	OldField    *Field     `json:"old_field,omitempty"`
	NewField    *Field     `json:"new_field,omitempty"`
	Index       *Index     `json:"index,omitempty"`
}

type PendingChangesStore struct {
	db *sql.DB
}

func NewPendingChangesStore(db *sql.DB) *PendingChangesStore {
	return &PendingChangesStore{db: db}
}

func (s *PendingChangesStore) Init() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS _alyx_pending_changes (
			id TEXT PRIMARY KEY,
			change_type TEXT NOT NULL,
			collection TEXT NOT NULL,
			field TEXT,
			description TEXT NOT NULL,
			safe INTEGER NOT NULL DEFAULT 0,
			old_field_json TEXT,
			new_field_json TEXT,
			index_json TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	return err
}

func (s *PendingChangesStore) Store(changes []*Change) error {
	if err := s.Clear(); err != nil {
		return fmt.Errorf("clearing existing changes: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		INSERT INTO _alyx_pending_changes 
		(id, change_type, collection, field, description, safe, old_field_json, new_field_json, index_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	for i, change := range changes {
		id := fmt.Sprintf("change_%d_%d", time.Now().Unix(), i)

		var oldFieldJSON, newFieldJSON, indexJSON sql.NullString

		if change.OldField != nil {
			data, _ := json.Marshal(change.OldField)
			oldFieldJSON = sql.NullString{String: string(data), Valid: true}
		}
		if change.NewField != nil {
			data, _ := json.Marshal(change.NewField)
			newFieldJSON = sql.NullString{String: string(data), Valid: true}
		}
		if change.Index != nil {
			data, _ := json.Marshal(change.Index)
			indexJSON = sql.NullString{String: string(data), Valid: true}
		}

		safe := 0
		if change.Safe {
			safe = 1
		}

		_, err := stmt.Exec(
			id,
			string(change.Type),
			change.Collection,
			change.Field,
			change.Description,
			safe,
			oldFieldJSON,
			newFieldJSON,
			indexJSON,
			time.Now().UTC().Format(time.RFC3339),
		)
		if err != nil {
			return fmt.Errorf("inserting change: %w", err)
		}
	}

	return tx.Commit()
}

func (s *PendingChangesStore) List() ([]*PendingChange, error) {
	rows, err := s.db.Query(`
		SELECT id, change_type, collection, field, description, safe, old_field_json, new_field_json, index_json, created_at
		FROM _alyx_pending_changes
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying pending changes: %w", err)
	}
	defer rows.Close()

	var changes []*PendingChange
	for rows.Next() {
		var pc PendingChange
		var field sql.NullString
		var safe int
		var oldFieldJSON, newFieldJSON, indexJSON sql.NullString
		var createdAt string

		if err := rows.Scan(
			&pc.ID,
			&pc.Type,
			&pc.Collection,
			&field,
			&pc.Description,
			&safe,
			&oldFieldJSON,
			&newFieldJSON,
			&indexJSON,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}

		pc.Field = field.String
		pc.Safe = safe == 1

		if oldFieldJSON.Valid {
			var f Field
			if err := json.Unmarshal([]byte(oldFieldJSON.String), &f); err == nil {
				pc.OldField = &f
			}
		}
		if newFieldJSON.Valid {
			var f Field
			if err := json.Unmarshal([]byte(newFieldJSON.String), &f); err == nil {
				pc.NewField = &f
			}
		}
		if indexJSON.Valid {
			var idx Index
			if err := json.Unmarshal([]byte(indexJSON.String), &idx); err == nil {
				pc.Index = &idx
			}
		}

		if parsed, err := time.Parse(time.RFC3339, createdAt); err == nil {
			pc.CreatedAt = parsed
		}

		changes = append(changes, &pc)
	}

	return changes, rows.Err()
}

func (s *PendingChangesStore) ListUnsafe() ([]*PendingChange, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}

	var unsafe []*PendingChange
	for _, c := range all {
		if !c.Safe {
			unsafe = append(unsafe, c)
		}
	}
	return unsafe, nil
}

func (s *PendingChangesStore) HasPending() (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM _alyx_pending_changes WHERE safe = 0").Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *PendingChangesStore) Clear() error {
	_, err := s.db.Exec("DELETE FROM _alyx_pending_changes")
	return err
}

func (s *PendingChangesStore) ToChanges(pending []*PendingChange) []*Change {
	var changes []*Change
	for _, p := range pending {
		changes = append(changes, &Change{
			Type:        p.Type,
			Collection:  p.Collection,
			Field:       p.Field,
			OldField:    p.OldField,
			NewField:    p.NewField,
			Index:       p.Index,
			Safe:        p.Safe,
			Description: p.Description,
		})
	}
	return changes
}
