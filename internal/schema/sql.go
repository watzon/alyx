package schema

import (
	"fmt"
	"strings"
)

type SQLGenerator struct {
	schema *Schema
}

func NewSQLGenerator(s *Schema) *SQLGenerator {
	return &SQLGenerator{schema: s}
}

func (g *SQLGenerator) GenerateAll() []string {
	var statements []string

	statements = append(statements, g.GenerateInternalTables()...)

	for _, col := range g.schema.Collections {
		statements = append(statements, g.GenerateCreateTable(col))
		statements = append(statements, g.GenerateIndexes(col)...)
		statements = append(statements, g.GenerateTriggers(col)...)
	}

	return statements
}

func (g *SQLGenerator) GenerateInternalTables() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS _alyx_migrations (
	id INTEGER PRIMARY KEY,
	version TEXT NOT NULL,
	name TEXT NOT NULL,
	applied_at TEXT NOT NULL DEFAULT (datetime('now')),
	checksum TEXT NOT NULL
)`,
		`CREATE TABLE IF NOT EXISTS _alyx_changes (
	id INTEGER PRIMARY KEY,
	collection TEXT NOT NULL,
	operation TEXT NOT NULL,
	doc_id TEXT NOT NULL,
	changed_fields TEXT,
	timestamp TEXT NOT NULL DEFAULT (datetime('now')),
	processed INTEGER NOT NULL DEFAULT 0
)`,
		`CREATE INDEX IF NOT EXISTS idx_changes_unprocessed ON _alyx_changes(processed, timestamp)`,
		`CREATE TABLE IF NOT EXISTS _alyx_users (
	id TEXT PRIMARY KEY,
	email TEXT UNIQUE NOT NULL,
	password_hash TEXT,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	updated_at TEXT NOT NULL DEFAULT (datetime('now')),
	verified INTEGER NOT NULL DEFAULT 0,
	metadata TEXT
)`,
		`CREATE TABLE IF NOT EXISTS _alyx_sessions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES _alyx_users(id) ON DELETE CASCADE,
	refresh_token_hash TEXT NOT NULL,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	user_agent TEXT,
	ip_address TEXT
)`,
		`CREATE TABLE IF NOT EXISTS _alyx_oauth_accounts (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES _alyx_users(id) ON DELETE CASCADE,
	provider TEXT NOT NULL,
	provider_user_id TEXT NOT NULL,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	UNIQUE(provider, provider_user_id)
)`,
	}
}

func (g *SQLGenerator) GenerateCreateTable(col *Collection) string {
	var sb strings.Builder

	sb.WriteString("CREATE TABLE IF NOT EXISTS ")
	sb.WriteString(col.Name)
	sb.WriteString(" (\n")

	var columnDefs []string
	var constraints []string

	for _, field := range col.OrderedFields() {
		columnDefs = append(columnDefs, g.generateColumnDef(field))

		if field.References != "" {
			constraints = append(constraints, g.generateForeignKey(field))
		}
	}

	allDefs := append(columnDefs, constraints...)
	sb.WriteString("\t")
	sb.WriteString(strings.Join(allDefs, ",\n\t"))
	sb.WriteString("\n)")

	return sb.String()
}

func (g *SQLGenerator) generateColumnDef(f *Field) string {
	var parts []string

	parts = append(parts, f.Name)
	parts = append(parts, f.Type.SQLiteType())

	if f.Primary {
		parts = append(parts, "PRIMARY KEY")
	}

	if !f.Nullable && !f.Primary {
		parts = append(parts, "NOT NULL")
	}

	if f.Unique && !f.Primary {
		parts = append(parts, "UNIQUE")
	}

	if def := f.SQLDefault(); def != "" {
		parts = append(parts, "DEFAULT", def)
	}

	return strings.Join(parts, " ")
}

func (g *SQLGenerator) generateForeignKey(f *Field) string {
	table, field, _ := f.ParseReference()
	onDelete := f.OnDelete
	if onDelete == "" {
		onDelete = OnDeleteRestrict
	}
	return fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s(%s) ON DELETE %s",
		f.Name, table, field, onDelete.SQL())
}

func (g *SQLGenerator) GenerateIndexes(col *Collection) []string {
	var indexes []string

	for _, field := range col.Fields {
		if field.Index && !field.Primary && !field.Unique {
			idxName := fmt.Sprintf("idx_%s_%s", col.Name, field.Name)
			indexes = append(indexes, fmt.Sprintf(
				"CREATE INDEX IF NOT EXISTS %s ON %s (%s)",
				idxName, col.Name, field.Name,
			))
		}
	}

	for _, idx := range col.Indexes {
		indexes = append(indexes, idx.SQL(col.Name))
	}

	return indexes
}

func (g *SQLGenerator) GenerateTriggers(col *Collection) []string {
	var triggers []string

	pk := col.PrimaryKeyField()
	if pk == nil {
		return triggers
	}

	triggers = append(triggers, fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS %s_after_insert
AFTER INSERT ON %s
BEGIN
	INSERT INTO _alyx_changes (collection, operation, doc_id)
	VALUES ('%s', 'INSERT', NEW.%s);
END`, col.Name, col.Name, col.Name, pk.Name))

	var updateFields []string
	for _, field := range col.OrderedFields() {
		if field.Primary {
			continue
		}
		updateFields = append(updateFields,
			fmt.Sprintf("CASE WHEN OLD.%s IS NOT NEW.%s THEN '%s' END", field.Name, field.Name, field.Name))
	}

	changedFieldsExpr := "NULL"
	if len(updateFields) > 0 {
		changedFieldsExpr = fmt.Sprintf("json_array(%s)", strings.Join(updateFields, ", "))
	}

	triggers = append(triggers, fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS %s_after_update
AFTER UPDATE ON %s
BEGIN
	INSERT INTO _alyx_changes (collection, operation, doc_id, changed_fields)
	VALUES ('%s', 'UPDATE', NEW.%s, %s);
END`, col.Name, col.Name, col.Name, pk.Name, changedFieldsExpr))

	triggers = append(triggers, fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS %s_after_delete
AFTER DELETE ON %s
BEGIN
	INSERT INTO _alyx_changes (collection, operation, doc_id)
	VALUES ('%s', 'DELETE', OLD.%s);
END`, col.Name, col.Name, col.Name, pk.Name))

	var autoUpdateFields []string
	for _, field := range col.Fields {
		if field.IsAutoUpdateTimestamp() {
			autoUpdateFields = append(autoUpdateFields,
				fmt.Sprintf("%s = datetime('now')", field.Name))
		}
	}

	if len(autoUpdateFields) > 0 {
		triggers = append(triggers, fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS %s_auto_update_timestamp
BEFORE UPDATE ON %s
BEGIN
	UPDATE %s SET %s WHERE %s = NEW.%s;
END`, col.Name, col.Name, col.Name, strings.Join(autoUpdateFields, ", "), pk.Name, pk.Name))
	}

	return triggers
}

func (g *SQLGenerator) GenerateDropTable(name string) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s", name)
}

func (g *SQLGenerator) GenerateDropIndex(name string) string {
	return fmt.Sprintf("DROP INDEX IF EXISTS %s", name)
}

func (g *SQLGenerator) GenerateDropTrigger(name string) string {
	return fmt.Sprintf("DROP TRIGGER IF EXISTS %s", name)
}
