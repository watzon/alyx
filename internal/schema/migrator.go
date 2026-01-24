package schema

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Migration struct {
	Version     int            `yaml:"version"`
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Operations  []*MigrationOp `yaml:"operations"`
	AppliedAt   time.Time      `yaml:"-"`
	Checksum    string         `yaml:"-"`
}

type MigrationOp struct {
	Type       string `yaml:"type"`
	Collection string `yaml:"collection"`
	From       string `yaml:"from"`
	To         string `yaml:"to"`
	Up         string `yaml:"up"`
	Down       string `yaml:"down"`
}

type AppliedMigration struct {
	ID        int64
	Version   string
	Name      string
	AppliedAt time.Time
	Checksum  string
}

type Migrator struct {
	db             *sql.DB
	schemaPath     string
	migrationsPath string
}

func NewMigrator(db *sql.DB, schemaPath, migrationsPath string) *Migrator {
	return &Migrator{
		db:             db,
		schemaPath:     schemaPath,
		migrationsPath: migrationsPath,
	}
}

func (m *Migrator) Init() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS _alyx_migrations (
			id INTEGER PRIMARY KEY,
			version TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			applied_at TEXT NOT NULL DEFAULT (datetime('now')),
			checksum TEXT NOT NULL
		)
	`)
	return err
}

func (m *Migrator) AppliedMigrations() ([]*AppliedMigration, error) {
	rows, err := m.db.Query(`
		SELECT id, version, name, applied_at, checksum 
		FROM _alyx_migrations 
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying migrations: %w", err)
	}
	defer rows.Close()

	var migrations []*AppliedMigration
	for rows.Next() {
		var m AppliedMigration
		var appliedAt string
		if err := rows.Scan(&m.ID, &m.Version, &m.Name, &appliedAt, &m.Checksum); err != nil {
			return nil, fmt.Errorf("scanning migration: %w", err)
		}
		if parsed, err := time.Parse(time.RFC3339, appliedAt); err == nil {
			m.AppliedAt = parsed
		}
		migrations = append(migrations, &m)
	}
	return migrations, rows.Err()
}

func (m *Migrator) PendingMigrations() ([]*Migration, error) {
	applied, err := m.AppliedMigrations()
	if err != nil {
		return nil, err
	}

	appliedVersions := make(map[string]bool)
	for _, a := range applied {
		appliedVersions[a.Version] = true
	}

	files, err := m.loadMigrationFiles()
	if err != nil {
		return nil, err
	}

	var pending []*Migration
	for _, mig := range files {
		versionStr := strconv.Itoa(mig.Version)
		if !appliedVersions[versionStr] {
			pending = append(pending, mig)
		}
	}

	sort.Slice(pending, func(i, j int) bool {
		return pending[i].Version < pending[j].Version
	})

	return pending, nil
}

func (m *Migrator) loadMigrationFiles() ([]*Migration, error) {
	if m.migrationsPath == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(m.migrationsPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory: %w", err)
	}

	migrations := make([]*Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}

		path := filepath.Join(m.migrationsPath, entry.Name())
		mig, err := m.loadMigrationFile(path)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", entry.Name(), err)
		}
		migrations = append(migrations, mig)
	}

	return migrations, nil
}

func (m *Migrator) loadMigrationFile(path string) (*Migration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var mig Migration
	if err := yaml.Unmarshal(data, &mig); err != nil {
		return nil, fmt.Errorf("parsing migration: %w", err)
	}

	mig.Checksum = checksumBytes(data)
	return &mig, nil
}

func (m *Migrator) Apply(mig *Migration) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for i, op := range mig.Operations {
		sql, sqlErr := m.operationToSQL(op)
		if sqlErr != nil {
			return fmt.Errorf("operation %d: %w", i, sqlErr)
		}
		if sql == "" {
			continue
		}

		if _, execErr := tx.Exec(sql); execErr != nil {
			return fmt.Errorf("executing operation %d: %w", i, execErr)
		}
		if _, err := tx.Exec(sql); err != nil {
			return fmt.Errorf("executing operation %d: %w", i, err)
		}
	}

	_, insertErr := tx.Exec(`
		INSERT INTO _alyx_migrations (version, name, checksum)
		VALUES (?, ?, ?)
	`, strconv.Itoa(mig.Version), mig.Name, mig.Checksum)
	if insertErr != nil {
		return fmt.Errorf("recording migration: %w", insertErr)
	}

	return tx.Commit()
}

func (m *Migrator) operationToSQL(op *MigrationOp) (string, error) {
	switch op.Type {
	case "sql":
		return op.Up, nil
	case "rename_field":
		return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
			op.Collection, op.From, op.To), nil
	case "add_field":
		return "", fmt.Errorf("add_field requires full field definition in 'up' SQL")
	case "drop_field":
		return "", fmt.Errorf("SQLite doesn't support DROP COLUMN directly; use 'sql' type with table recreation")
	default:
		return "", fmt.Errorf("unknown operation type: %s", op.Type)
	}
}

func (m *Migrator) ApplySchema(schema *Schema) error {
	gen := NewSQLGenerator(schema)
	statements := gen.GenerateAll()

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("executing %q: %w", truncate(stmt, 100), err)
		}
	}

	return tx.Commit()
}

func (m *Migrator) ApplySafeChanges(changes []*Change) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, change := range changes {
		if !change.Safe {
			continue
		}

		stmts, err := m.changeToSQL(change)
		if err != nil {
			return fmt.Errorf("generating SQL for %s: %w", change, err)
		}

		for _, stmt := range stmts {
			if _, err := tx.Exec(stmt); err != nil {
				return fmt.Errorf("executing %q: %w", truncate(stmt, 100), err)
			}
		}
	}

	return tx.Commit()
}

func (m *Migrator) changeToSQL(change *Change) ([]string, error) {
	switch change.Type {
	case ChangeAddCollection:
		return nil, fmt.Errorf("add collection requires full schema regeneration")

	case ChangeAddField:
		f := change.NewField
		colDef := fmt.Sprintf("%s %s", f.Name, f.Type.SQLiteType())
		if !f.Nullable {
			colDef += " NOT NULL"
		}
		if def := f.SQLDefault(); def != "" {
			colDef += " DEFAULT " + def
		}
		return []string{
			fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", change.Collection, colDef),
		}, nil

	case ChangeAddIndex:
		return []string{change.Index.SQL(change.Collection)}, nil

	case ChangeDropIndex:
		return []string{fmt.Sprintf("DROP INDEX IF EXISTS %s", change.Index.Name)}, nil

	case ChangeModifyRules:
		return nil, nil

	default:
		return nil, fmt.Errorf("change type %s requires manual migration", change.Type)
	}
}

func (m *Migrator) CreateMigrationFile(name string, version int) (string, error) {
	if m.migrationsPath == "" {
		return "", fmt.Errorf("migrations path not configured")
	}

	if err := os.MkdirAll(m.migrationsPath, 0o755); err != nil {
		return "", fmt.Errorf("creating migrations directory: %w", err)
	}

	filename := fmt.Sprintf("%03d_%s.yaml", version, sanitizeFilename(name))
	path := filepath.Join(m.migrationsPath, filename)

	content := fmt.Sprintf(`version: %d
name: %s
description: ""

operations:
  - type: sql
    up: |
      -- Add your migration SQL here
    down: |
      -- Add your rollback SQL here
`, version, name)

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("writing migration file: %w", err)
	}

	return path, nil
}

func (m *Migrator) NextVersion() (int, error) {
	applied, err := m.AppliedMigrations()
	if err != nil {
		return 0, err
	}

	files, err := m.loadMigrationFiles()
	if err != nil {
		return 0, err
	}

	maxVersion := 0
	for _, a := range applied {
		if v, err := strconv.Atoi(a.Version); err == nil && v > maxVersion {
			maxVersion = v
		}
	}
	for _, f := range files {
		if f.Version > maxVersion {
			maxVersion = f.Version
		}
	}

	return maxVersion + 1, nil
}

func checksumBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:8])
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func sanitizeFilename(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		}
	}
	return result.String()
}
