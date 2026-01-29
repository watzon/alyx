package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

func testMigratorSetup(t *testing.T) (*schema.Migrator, *database.DB, string) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	schemaPath := filepath.Join(tmpDir, "schema.yaml")
	migrationsPath := filepath.Join(tmpDir, "migrations")

	if err := os.MkdirAll(migrationsPath, 0o755); err != nil {
		t.Fatal(err)
	}

	schemaContent := `version: 1
collections:
  users:
    fields:
      id:
        type: id
        primary: true
        default: auto
      email:
        type: string
        required: true
        unique: true
      name:
        type: string
        nullable: true
`
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.DatabaseConfig{Path: dbPath}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	migrator := schema.NewMigrator(db.DB, schemaPath, migrationsPath)
	if err := migrator.Init(); err != nil {
		t.Fatal(err)
	}

	return migrator, db, schemaPath
}

func TestGetMigrator(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	schemaContent := `version: 1
collections:
  items:
    fields:
      id:
        type: id
        primary: true
`
	if err := os.WriteFile("schema.yaml", []byte(schemaContent), 0o600); err != nil {
		t.Fatal(err)
	}

	configContent := `database:
  path: ./test.db
`
	if err := os.WriteFile("alyx.yaml", []byte(configContent), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll("migrations", 0o755); err != nil {
		t.Fatal(err)
	}

	migrator, db, err := getMigrator()
	if err != nil {
		t.Fatalf("getMigrator() failed: %v", err)
	}
	defer db.Close()

	if migrator == nil {
		t.Error("getMigrator() returned nil migrator")
	}
}

func TestSqliteTypeToFieldType(t *testing.T) {
	tests := []struct {
		sqlType  string
		expected schema.FieldType
	}{
		{"INTEGER", schema.FieldTypeInt},
		{"REAL", schema.FieldTypeFloat},
		{"BLOB", schema.FieldTypeBlob},
		{"TEXT", schema.FieldTypeString},
		{"VARCHAR", schema.FieldTypeString},
		{"CHAR", schema.FieldTypeString},
		{"", schema.FieldTypeString},
	}

	for _, tt := range tests {
		t.Run(tt.sqlType, func(t *testing.T) {
			result := sqliteTypeToFieldType(tt.sqlType)
			if result != tt.expected {
				t.Errorf("sqliteTypeToFieldType(%q) = %v, want %v", tt.sqlType, result, tt.expected)
			}
		})
	}
}

func TestFilterSafeChanges(t *testing.T) {
	changes := []*schema.Change{
		{Type: schema.ChangeAddCollection, Safe: true},
		{Type: schema.ChangeDropCollection, Safe: false},
		{Type: schema.ChangeAddField, Safe: true},
		{Type: schema.ChangeDropField, Safe: false},
		{Type: schema.ChangeModifyField, Safe: false},
	}

	safe := filterSafeChanges(changes)
	if len(safe) != 2 {
		t.Errorf("filterSafeChanges() returned %d changes, want 2", len(safe))
	}

	for _, c := range safe {
		if !c.Safe {
			t.Errorf("filterSafeChanges() returned unsafe change: %v", c)
		}
	}
}

func TestFilterUnsafeChanges(t *testing.T) {
	changes := []*schema.Change{
		{Type: schema.ChangeAddCollection, Safe: true},
		{Type: schema.ChangeDropCollection, Safe: false},
		{Type: schema.ChangeAddField, Safe: true},
		{Type: schema.ChangeDropField, Safe: false},
		{Type: schema.ChangeModifyField, Safe: false},
	}

	unsafe := filterUnsafeChanges(changes)
	if len(unsafe) != 3 {
		t.Errorf("filterUnsafeChanges() returned %d changes, want 3", len(unsafe))
	}

	for _, c := range unsafe {
		if c.Safe {
			t.Errorf("filterUnsafeChanges() returned safe change: %v", c)
		}
	}
}

func TestHasUnsafeChanges(t *testing.T) {
	tests := []struct {
		name     string
		changes  []*schema.Change
		expected bool
	}{
		{
			name:     "no changes",
			changes:  []*schema.Change{},
			expected: false,
		},
		{
			name: "only safe changes",
			changes: []*schema.Change{
				{Safe: true},
				{Safe: true},
			},
			expected: false,
		},
		{
			name: "has unsafe changes",
			changes: []*schema.Change{
				{Safe: true},
				{Safe: false},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasUnsafeChanges(tt.changes)
			if result != tt.expected {
				t.Errorf("hasUnsafeChanges() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetSchemaFromDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.DatabaseConfig{Path: dbPath}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			name TEXT
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	s, err := getSchemaFromDB(db)
	if err != nil {
		t.Fatalf("getSchemaFromDB() failed: %v", err)
	}

	if s == nil {
		t.Fatal("getSchemaFromDB() returned nil schema")
	}

	if len(s.Collections) != 1 {
		t.Errorf("getSchemaFromDB() returned %d collections, want 1", len(s.Collections))
	}

	users, ok := s.Collections["users"]
	if !ok {
		t.Fatal("getSchemaFromDB() missing users collection")
	}

	if len(users.Fields) != 3 {
		t.Errorf("users collection has %d fields, want 3", len(users.Fields))
	}

	expectedFields := []string{"id", "email", "name"}
	for _, fieldName := range expectedFields {
		if _, ok := users.Fields[fieldName]; !ok {
			t.Errorf("users collection missing field: %s", fieldName)
		}
	}
}

func TestGetCollectionFromDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.DatabaseConfig{Path: dbPath}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT,
			view_count INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	col, err := getCollectionFromDB(db, "posts")
	if err != nil {
		t.Fatalf("getCollectionFromDB() failed: %v", err)
	}

	if col.Name != "posts" {
		t.Errorf("collection name = %q, want %q", col.Name, "posts")
	}

	if len(col.Fields) != 4 {
		t.Errorf("collection has %d fields, want 4", len(col.Fields))
	}

	idField, ok := col.Fields["id"]
	if !ok {
		t.Fatal("collection missing id field")
	}
	if !idField.Primary {
		t.Error("id field should be primary")
	}

	titleField, ok := col.Fields["title"]
	if !ok {
		t.Fatal("collection missing title field")
	}
	if titleField.Nullable {
		t.Error("title field should not be nullable (NOT NULL constraint)")
	}

	contentField, ok := col.Fields["content"]
	if !ok {
		t.Fatal("collection missing content field")
	}
	if !contentField.Nullable {
		t.Error("content field should be nullable (no NOT NULL constraint)")
	}
}

func TestCheckSchemaChanges(t *testing.T) {
	migrator, db, schemaPath := testMigratorSetup(t)

	s, err := schema.ParseFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}

	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	updatedSchema := `version: 1
collections:
  users:
    fields:
      id:
        type: id
        primary: true
        default: auto
      email:
        type: string
        required: true
        unique: true
      name:
        type: string
        nullable: true
      age:
        type: int
        nullable: true
`
	updatedSchemaPath := filepath.Join(filepath.Dir(schemaPath), "schema_updated.yaml")
	if err := os.WriteFile(updatedSchemaPath, []byte(updatedSchema), 0o600); err != nil {
		t.Fatal(err)
	}

	changes, err := checkSchemaChanges(db, updatedSchemaPath)
	if err != nil {
		t.Fatalf("checkSchemaChanges() failed: %v", err)
	}

	if len(changes) == 0 {
		t.Error("checkSchemaChanges() returned no changes, expected at least one (age field)")
	}

	_ = migrator
}

func TestCheckSchemaChangesNoTables(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "empty.db")
	schemaPath := filepath.Join(tmpDir, "schema.yaml")

	schemaContent := `version: 1
collections:
  items:
    fields:
      id:
        type: id
        primary: true
`
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.DatabaseConfig{Path: dbPath}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	changes, err := checkSchemaChanges(db, schemaPath)
	if err != nil {
		t.Fatalf("checkSchemaChanges() with empty DB should not error: %v", err)
	}

	if changes != nil {
		t.Errorf("checkSchemaChanges() with empty DB returned changes, want nil")
	}
}
