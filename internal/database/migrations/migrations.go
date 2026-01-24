// Package migrations provides embedded SQL migrations for Alyx internal tables.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

//go:embed sql/*.sql
var sqlFS embed.FS

// AppliedMigration represents a migration that has been applied to the database.
type AppliedMigration struct {
	ID        string
	AppliedAt time.Time
}

// Run executes all pending internal migrations against the database.
// Migrations are applied in alphabetical order by filename.
// Each migration runs in its own transaction.
func Run(ctx context.Context, db *sql.DB) error {
	if err := ensureVersionTable(ctx, db); err != nil {
		return fmt.Errorf("ensuring version table: %w", err)
	}

	applied, err := getAppliedMigrations(ctx, db)
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		return fmt.Errorf("loading migrations: %w", err)
	}

	for _, m := range migrations {
		if applied[m.id] {
			continue
		}

		if err := applyMigration(ctx, db, m); err != nil {
			return fmt.Errorf("applying migration %s: %w", m.id, err)
		}

		log.Info().Str("migration", m.id).Msg("Applied internal migration")
	}

	return nil
}

// GetApplied returns all applied migrations.
func GetApplied(ctx context.Context, db *sql.DB) ([]AppliedMigration, error) {
	if err := ensureVersionTable(ctx, db); err != nil {
		return nil, fmt.Errorf("ensuring version table: %w", err)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, applied_at FROM _alyx_internal_versions ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("querying migrations: %w", err)
	}
	defer rows.Close()

	var result []AppliedMigration
	for rows.Next() {
		var m AppliedMigration
		var appliedAt string
		if err := rows.Scan(&m.ID, &appliedAt); err != nil {
			return nil, fmt.Errorf("scanning migration: %w", err)
		}
		if t, parseErr := time.Parse(time.RFC3339, appliedAt); parseErr == nil {
			m.AppliedAt = t
		} else if t, parseErr := time.Parse("2006-01-02 15:04:05", appliedAt); parseErr == nil {
			m.AppliedAt = t
		}
		result = append(result, m)
	}

	return result, rows.Err()
}

type migration struct {
	id      string
	content string
}

func ensureVersionTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _alyx_internal_versions (
			id TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`)
	return err
}

func getAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT id FROM _alyx_internal_versions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		applied[id] = true
	}

	return applied, rows.Err()
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(sqlFS, "sql")
	if err != nil {
		return nil, fmt.Errorf("reading sql directory: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		content, err := fs.ReadFile(sqlFS, "sql/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}

		id := strings.TrimSuffix(entry.Name(), ".sql")
		migrations = append(migrations, migration{
			id:      id,
			content: string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].id < migrations[j].id
	})

	return migrations, nil
}

func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	statements := splitStatements(m.content)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("executing statement: %w\nSQL: %s", err, truncate(stmt, 100))
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO _alyx_internal_versions (id) VALUES (?)
	`, m.id); err != nil {
		return fmt.Errorf("recording migration: %w", err)
	}

	return tx.Commit()
}

// splitStatements splits SQL content into individual statements.
// Handles semicolons inside strings and comments.
func splitStatements(content string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := rune(0)

	for i, ch := range content {
		if (ch == '\'' || ch == '"') && (i == 0 || content[i-1] != '\\') {
			if !inString {
				inString = true
				stringChar = ch
			} else if ch == stringChar {
				inString = false
			}
		}

		if ch == ';' && !inString {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			continue
		}

		current.WriteRune(ch)
	}

	if stmt := strings.TrimSpace(current.String()); stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
