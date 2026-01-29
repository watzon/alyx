package schema

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
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
	if err != nil {
		return err
	}

	_, err = m.db.Exec(`
		CREATE TABLE IF NOT EXISTS _alyx_schema_cache (
			collection TEXT PRIMARY KEY,
			rules_json TEXT,
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
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

func (m *Migrator) ApplySafeChanges(changes []*Change, schema *Schema) error {
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

	if err := tx.Commit(); err != nil {
		return err
	}

	if schema != nil {
		for name, collection := range schema.Collections {
			if err := m.SaveRulesToCache(name, collection.Rules); err != nil {
				return fmt.Errorf("saving rules for %s: %w", name, err)
			}
		}
	}

	return nil
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

func (m *Migrator) ApplyUnsafeChanges(changes []*Change, schema *Schema) error {
	if _, err := m.db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("disabling foreign keys: %w", err)
	}
	defer func() { _, _ = m.db.Exec("PRAGMA foreign_keys = ON") }()

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, change := range changes {
		if change.Safe {
			continue
		}

		stmts, err := m.unsafeChangeToSQLWithTx(tx, change)
		if err != nil {
			return fmt.Errorf("generating SQL for %s: %w", change, err)
		}

		for _, stmt := range stmts {
			if _, err := tx.Exec(stmt); err != nil {
				return fmt.Errorf("executing %q: %w", truncate(stmt, 100), err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	if _, err := m.db.Exec("PRAGMA foreign_key_check"); err != nil {
		return fmt.Errorf("foreign key check failed after migration: %w", err)
	}

	if schema != nil {
		for name, collection := range schema.Collections {
			if err := m.SaveRulesToCache(name, collection.Rules); err != nil {
				return fmt.Errorf("saving rules for %s: %w", name, err)
			}
		}
	}

	return nil
}

func (m *Migrator) unsafeChangeToSQL(change *Change) ([]string, error) {
	switch change.Type {
	case ChangeDropCollection:
		return []string{
			fmt.Sprintf("DROP TABLE IF EXISTS %s", change.Collection),
		}, nil

	case ChangeDropField:
		return m.dropColumnSQL(change.Collection, change.OldField.Name)

	case ChangeModifyField:
		return m.modifyFieldSQL(change)

	default:
		return nil, fmt.Errorf("unsupported unsafe change type: %s", change.Type)
	}
}

func (m *Migrator) unsafeChangeToSQLWithTx(tx *sql.Tx, change *Change) ([]string, error) {
	switch change.Type {
	case ChangeDropCollection:
		return []string{
			fmt.Sprintf("DROP TABLE IF EXISTS %s", change.Collection),
		}, nil

	case ChangeDropField:
		return m.dropColumnSQLWithTx(tx, change.Collection, change.OldField.Name)

	case ChangeModifyField:
		return m.modifyFieldSQLWithTx(tx, change)

	default:
		return nil, fmt.Errorf("unsupported unsafe change type: %s", change.Type)
	}
}

func (m *Migrator) dropColumnSQL(table, column string) ([]string, error) {
	stmts := m.dropTriggersSQL(table)

	indexStmts, err := m.dropIndexesForColumnSQL(table, column)
	if err != nil {
		return nil, fmt.Errorf("getting indexes for column: %w", err)
	}
	stmts = append(stmts, indexStmts...)

	stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column))
	return stmts, nil
}

func (m *Migrator) dropColumnSQLWithTx(tx *sql.Tx, table, column string) ([]string, error) {
	stmts := m.dropTriggersSQL(table)

	indexStmts, err := m.dropIndexesForColumnSQLWithTx(tx, table, column)
	if err != nil {
		return nil, fmt.Errorf("getting indexes for column: %w", err)
	}
	stmts = append(stmts, indexStmts...)

	stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column))
	return stmts, nil
}

func (m *Migrator) dropTriggersSQL(table string) []string {
	return []string{
		fmt.Sprintf("DROP TRIGGER IF EXISTS %s_after_insert", table),
		fmt.Sprintf("DROP TRIGGER IF EXISTS %s_after_update", table),
		fmt.Sprintf("DROP TRIGGER IF EXISTS %s_after_delete", table),
		fmt.Sprintf("DROP TRIGGER IF EXISTS %s_auto_update_timestamp", table),
	}
}

func (m *Migrator) dropIndexesForColumnSQL(table, column string) ([]string, error) {
	rows, err := m.db.Query(`
		SELECT name 
		FROM sqlite_master 
		WHERE type = 'index' 
			AND tbl_name = ?
			AND name NOT LIKE 'sqlite_%'
	`, table)
	if err != nil {
		return nil, fmt.Errorf("querying indexes: %w", err)
	}
	defer rows.Close()

	var indexNames []string
	for rows.Next() {
		var indexName string
		if err := rows.Scan(&indexName); err != nil {
			return nil, fmt.Errorf("scanning index name: %w", err)
		}
		indexNames = append(indexNames, indexName)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	var stmts []string
	for _, indexName := range indexNames {
		indexRows, err := m.db.Query("PRAGMA index_info(?)", indexName)
		if err != nil {
			continue
		}

		var referencesColumn bool
		for indexRows.Next() {
			var seqno int
			var cid int
			var name string
			if err := indexRows.Scan(&seqno, &cid, &name); err != nil {
				indexRows.Close()
				continue
			}
			if name == column {
				referencesColumn = true
				break
			}
		}
		indexRows.Close()

		if referencesColumn {
			stmts = append(stmts, fmt.Sprintf("DROP INDEX IF EXISTS %s", indexName))
		}
	}

	return stmts, nil
}

func (m *Migrator) dropIndexesForColumnSQLWithTx(tx *sql.Tx, table, column string) ([]string, error) {
	rows, err := tx.Query(`
		SELECT name 
		FROM sqlite_master 
		WHERE type = 'index' 
			AND tbl_name = ?
			AND name NOT LIKE 'sqlite_%'
	`, table)
	if err != nil {
		return nil, fmt.Errorf("querying indexes: %w", err)
	}
	defer rows.Close()

	var indexNames []string
	for rows.Next() {
		var indexName string
		if err := rows.Scan(&indexName); err != nil {
			return nil, fmt.Errorf("scanning index name: %w", err)
		}
		indexNames = append(indexNames, indexName)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	var stmts []string
	for _, indexName := range indexNames {
		indexRows, err := tx.Query(fmt.Sprintf("PRAGMA index_info(%s)", indexName))
		if err != nil {
			continue
		}

		var referencesColumn bool
		for indexRows.Next() {
			var seqno int
			var cid int
			var name string
			if err := indexRows.Scan(&seqno, &cid, &name); err != nil {
				indexRows.Close()
				continue
			}
			if name == column {
				referencesColumn = true
			}
		}
		indexRows.Close()

		if referencesColumn {
			stmts = append(stmts, fmt.Sprintf("DROP INDEX IF EXISTS %s", indexName))
		}
	}

	return stmts, nil
}

func (m *Migrator) modifyFieldSQL(change *Change) ([]string, error) {
	oldField := change.OldField
	newField := change.NewField
	table := change.Collection
	column := oldField.Name

	if oldField.Primary {
		return nil, fmt.Errorf("cannot modify primary key column %s.%s - requires manual table recreation", table, column)
	}

	if oldField.Type != newField.Type {
		return m.changeColumnTypeSQL(table, column, oldField, newField)
	}

	if oldField.Nullable && !newField.Nullable {
		return m.makeNonNullableSQL(table, column, newField)
	}

	if !oldField.Unique && newField.Unique {
		return m.addUniqueConstraintSQL(table, column)
	}

	return nil, fmt.Errorf("unsupported field modification")
}

func (m *Migrator) modifyFieldSQLWithTx(_ *sql.Tx, change *Change) ([]string, error) {
	return m.modifyFieldSQL(change)
}

func (m *Migrator) changeColumnTypeSQL(table, column string, oldField, newField *Field) ([]string, error) {
	tempCol := "_" + column + "_old"
	newType := newField.Type.SQLiteType()

	conversion := m.getTypeConversionExpr(column, tempCol, oldField.Type, newField.Type)

	stmts := m.dropTriggersSQL(table)
	stmts = append(stmts,
		fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", table, column, tempCol),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, newType),
		fmt.Sprintf("UPDATE %s SET %s = %s", table, column, conversion),
		fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, tempCol),
	)

	return stmts, nil
}

func (m *Migrator) getTypeConversionExpr(newCol, oldCol string, oldType, newType FieldType) string {
	switch {
	case oldType == FieldTypeInt && newType == FieldTypeString:
		return fmt.Sprintf("CAST(%s AS TEXT)", oldCol)
	case oldType == FieldTypeFloat && newType == FieldTypeString:
		return fmt.Sprintf("CAST(%s AS TEXT)", oldCol)
	case oldType == FieldTypeString && newType == FieldTypeInt:
		return fmt.Sprintf("CAST(%s AS INTEGER)", oldCol)
	case oldType == FieldTypeString && newType == FieldTypeFloat:
		return fmt.Sprintf("CAST(%s AS REAL)", oldCol)
	case oldType == FieldTypeBool && newType == FieldTypeString:
		return fmt.Sprintf("CASE WHEN %s THEN 'true' ELSE 'false' END", oldCol)
	case oldType == FieldTypeBool && newType == FieldTypeInt:
		return fmt.Sprintf("CASE WHEN %s THEN 1 ELSE 0 END", oldCol)
	case oldType == FieldTypeJSON && newType == FieldTypeString:
		return fmt.Sprintf("CAST(%s AS TEXT)", oldCol)
	default:
		return oldCol
	}
}

func (m *Migrator) makeNonNullableSQL(table, column string, newField *Field) ([]string, error) {
	defaultVal := newField.SQLDefault()
	if defaultVal == "" {
		defaultVal = m.getZeroValueForType(newField.Type)
	}

	tempCol := "_" + column + "_old"
	colType := newField.Type.SQLiteType()

	stmts := m.dropTriggersSQL(table)
	stmts = append(stmts,
		fmt.Sprintf("UPDATE %s SET %s = %s WHERE %s IS NULL", table, column, defaultVal, column),
		fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", table, column, tempCol),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s NOT NULL DEFAULT %s", table, column, colType, defaultVal),
		fmt.Sprintf("UPDATE %s SET %s = %s", table, column, tempCol),
		fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, tempCol),
	)

	return stmts, nil
}

func (m *Migrator) getZeroValueForType(t FieldType) string {
	switch t {
	case FieldTypeInt:
		return "0"
	case FieldTypeFloat:
		return "0.0"
	case FieldTypeBool:
		return "0"
	case FieldTypeString, FieldTypeText, FieldTypeRichText, FieldTypeEmail, FieldTypeURL:
		return "''"
	case FieldTypeJSON:
		return "'{}'"
	case FieldTypeTimestamp, FieldTypeDate:
		return "datetime('now')"
	default:
		return "''"
	}
}

func (m *Migrator) addUniqueConstraintSQL(table, column string) ([]string, error) {
	indexName := fmt.Sprintf("idx_%s_%s_unique", table, column)
	return []string{
		fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s)", indexName, table, column),
	}, nil
}

func (m *Migrator) ValidateUnsafeChanges(changes []*Change) []ValidationError {
	var errors []ValidationError

	for _, change := range changes {
		if change.Safe {
			continue
		}

		switch change.Type {
		case ChangeModifyField:
			if !change.OldField.Unique && change.NewField.Unique {
				duplicates, err := m.checkDuplicates(change.Collection, change.OldField.Name)
				if err != nil {
					errors = append(errors, ValidationError{
						Path:    fmt.Sprintf("%s.%s", change.Collection, change.OldField.Name),
						Message: fmt.Sprintf("failed to check for duplicates: %v", err),
					})
				} else if duplicates > 0 {
					errors = append(errors, ValidationError{
						Path:    fmt.Sprintf("%s.%s", change.Collection, change.OldField.Name),
						Message: fmt.Sprintf("cannot add unique constraint: %d duplicate values exist", duplicates),
					})
				}
			}

			if change.OldField.Nullable && !change.NewField.Nullable {
				nullCount, err := m.countNulls(change.Collection, change.OldField.Name)
				if err != nil {
					errors = append(errors, ValidationError{
						Path:    fmt.Sprintf("%s.%s", change.Collection, change.OldField.Name),
						Message: fmt.Sprintf("failed to check for null values: %v", err),
					})
				} else if nullCount > 0 && !change.NewField.HasDefault() {
					errors = append(errors, ValidationError{
						Path:    fmt.Sprintf("%s.%s", change.Collection, change.OldField.Name),
						Message: fmt.Sprintf("%d rows have NULL values; provide a default value or update existing data", nullCount),
					})
				}
			}
		}
	}

	return errors
}

func (m *Migrator) checkDuplicates(table, column string) (int, error) {
	var count int
	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT %s FROM %s 
			WHERE %s IS NOT NULL 
			GROUP BY %s 
			HAVING COUNT(*) > 1
		)
	`, column, table, column, column)

	err := m.db.QueryRow(query).Scan(&count)
	return count, err
}

func (m *Migrator) countNulls(table, column string) (int, error) {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s IS NULL", table, column)
	err := m.db.QueryRow(query).Scan(&count)
	return count, err
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

func (m *Migrator) SaveRulesToCache(collection string, rules *Rules) error {
	if rules == nil {
		_, err := m.db.Exec(`DELETE FROM _alyx_schema_cache WHERE collection = ?`, collection)
		return err
	}

	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("marshaling rules: %w", err)
	}

	_, err = m.db.Exec(`
		INSERT INTO _alyx_schema_cache (collection, rules_json)
		VALUES (?, ?)
		ON CONFLICT(collection) DO UPDATE SET
			rules_json = excluded.rules_json,
			updated_at = datetime('now')
	`, collection, string(rulesJSON))
	return err
}

func (m *Migrator) LoadRulesFromCache(collection string) (*Rules, error) {
	var rulesJSON sql.NullString
	err := m.db.QueryRow(`
		SELECT rules_json FROM _alyx_schema_cache WHERE collection = ?
	`, collection).Scan(&rulesJSON)

	if err == sql.ErrNoRows || !rulesJSON.Valid {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying cache: %w", err)
	}

	var rules Rules
	if err := json.Unmarshal([]byte(rulesJSON.String), &rules); err != nil {
		return nil, fmt.Errorf("unmarshaling rules: %w", err)
	}

	return &rules, nil
}

func (m *Migrator) EnsureRulesCacheSeeded(schema *Schema) error {
	if schema == nil {
		return nil
	}

	var count int
	err := m.db.QueryRow(`SELECT COUNT(*) FROM _alyx_schema_cache`).Scan(&count)
	if err != nil {
		return fmt.Errorf("checking cache: %w", err)
	}

	if count > 0 {
		return nil
	}

	for name, collection := range schema.Collections {
		if err := m.SaveRulesToCache(name, collection.Rules); err != nil {
			return fmt.Errorf("seeding rules for %s: %w", name, err)
		}
	}
	return nil
}
