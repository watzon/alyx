package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

func testDBSetup(t *testing.T) (*database.DB, *schema.Schema, string) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	schemaPath := filepath.Join(tmpDir, "schema.yaml")

	schemaContent := `version: 1
collections:
  items:
    fields:
      id:
        type: id
        primary: true
        default: auto
      name:
        type: string
        required: true
      price:
        type: float
        nullable: true
`
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := schema.ParseFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.DatabaseConfig{Path: dbPath}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}

	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db, s, schemaPath
}

func TestParseSeedData_JSON(t *testing.T) {
	jsonData := `{
		"users": [
			{"id": "1", "name": "Alice"},
			{"id": "2", "name": "Bob"}
		]
	}`

	seedData, err := parseSeedData("test.json", []byte(jsonData))
	if err != nil {
		t.Fatalf("parseSeedData() failed: %v", err)
	}

	if len(seedData) != 1 {
		t.Errorf("parseSeedData() returned %d collections, want 1", len(seedData))
	}

	users, ok := seedData["users"]
	if !ok {
		t.Fatal("parseSeedData() missing users collection")
	}

	if len(users) != 2 {
		t.Errorf("users collection has %d documents, want 2", len(users))
	}
}

func TestParseSeedData_YAML(t *testing.T) {
	yamlData := `
users:
  - id: "1"
    name: Alice
  - id: "2"
    name: Bob
`

	seedData, err := parseSeedData("test.yaml", []byte(yamlData))
	if err != nil {
		t.Fatalf("parseSeedData() failed: %v", err)
	}

	if len(seedData) != 1 {
		t.Errorf("parseSeedData() returned %d collections, want 1", len(seedData))
	}

	users, ok := seedData["users"]
	if !ok {
		t.Fatal("parseSeedData() missing users collection")
	}

	if len(users) != 2 {
		t.Errorf("users collection has %d documents, want 2", len(users))
	}
}

func TestIsYAML(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"test.yaml", true},
		{"test.yml", true},
		{"test.json", false},
		{"test.txt", false},
		{"test", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := isYAML(tt.filename)
			if result != tt.expected {
				t.Errorf("isYAML(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestHasExtension(t *testing.T) {
	tests := []struct {
		filename string
		ext      string
		expected bool
	}{
		{"test.yaml", ".yaml", true},
		{"test.yml", ".yml", true},
		{"test.json", ".yaml", false},
		{"test", ".txt", false},
		{"", ".txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.filename+"_"+tt.ext, func(t *testing.T) {
			result := hasExtension(tt.filename, tt.ext)
			if result != tt.expected {
				t.Errorf("hasExtension(%q, %q) = %v, want %v", tt.filename, tt.ext, result, tt.expected)
			}
		})
	}
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		name     string
		strs     []string
		sep      string
		expected string
	}{
		{"empty", []string{}, ", ", ""},
		{"single", []string{"a"}, ", ", "a"},
		{"multiple", []string{"a", "b", "c"}, ", ", "a, b, c"},
		{"different separator", []string{"x", "y"}, "-", "x-y"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinStrings(tt.strs, tt.sep)
			if result != tt.expected {
				t.Errorf("joinStrings() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSeedCollection(t *testing.T) {
	db, s, _ := testDBSetup(t)

	col := s.Collections["items"]
	documents := []map[string]any{
		{"id": "item1", "name": "Widget", "price": 9.99},
		{"id": "item2", "name": "Gadget", "price": 19.99},
	}

	inserted, err := seedCollection(db, col, documents)
	if err != nil {
		t.Fatalf("seedCollection() failed: %v", err)
	}

	if inserted != 2 {
		t.Errorf("seedCollection() inserted %d documents, want 2", inserted)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count items: %v", err)
	}

	if count != 2 {
		t.Errorf("database has %d items, want 2", count)
	}
}

func TestSeedCollectionEmpty(t *testing.T) {
	db, s, _ := testDBSetup(t)

	col := s.Collections["items"]
	documents := []map[string]any{}

	inserted, err := seedCollection(db, col, documents)
	if err != nil {
		t.Fatalf("seedCollection() with empty documents failed: %v", err)
	}

	if inserted != 0 {
		t.Errorf("seedCollection() inserted %d documents, want 0", inserted)
	}
}

func TestLoadConfigAndSchema(t *testing.T) {
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
  test:
    fields:
      id:
        type: id
        primary: true
`
	if err := os.WriteFile("schema.yaml", []byte(schemaContent), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, s, err := loadConfigAndSchema()
	if err != nil {
		t.Fatalf("loadConfigAndSchema() failed: %v", err)
	}

	if cfg == nil {
		t.Error("loadConfigAndSchema() returned nil config")
	}

	if s == nil {
		t.Error("loadConfigAndSchema() returned nil schema")
	}

	if len(s.Collections) != 1 {
		t.Errorf("schema has %d collections, want 1", len(s.Collections))
	}
}

func TestListAllTables(t *testing.T) {
	db, _, _ := testDBSetup(t)

	_, err := db.Exec("CREATE TABLE test_table (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}

	tables, err := listAllTables(db)
	if err != nil {
		t.Fatalf("listAllTables() failed: %v", err)
	}

	expectedTables := []string{"items", "test_table"}
	if len(tables) < len(expectedTables) {
		t.Errorf("listAllTables() returned %d tables, want at least %d", len(tables), len(expectedTables))
	}

	for _, expected := range expectedTables {
		found := false
		for _, table := range tables {
			if table == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("listAllTables() missing table: %s", expected)
		}
	}
}

func TestDropAllTables(t *testing.T) {
	db, _, _ := testDBSetup(t)

	_, err := db.Exec("CREATE TABLE temp_table (id INTEGER)")
	if err != nil {
		t.Fatal(err)
	}

	tables := []string{"items", "temp_table"}
	if err := dropAllTables(db, tables); err != nil {
		t.Fatalf("dropAllTables() failed: %v", err)
	}

	for _, table := range tables {
		var exists int
		err = db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master 
			WHERE type='table' AND name = ?
		`, table).Scan(&exists)
		if err != nil {
			t.Fatal(err)
		}

		if exists > 0 {
			t.Errorf("table %q still exists after dropAllTables()", table)
		}
	}
}

func TestRecreateSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.DatabaseConfig{Path: dbPath}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	schemaContent := `version: 1
collections:
  products:
    fields:
      id:
        type: id
        primary: true
      title:
        type: string
`
	schemaPath := filepath.Join(tmpDir, "schema.yaml")
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := schema.ParseFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := recreateSchema(db, s); err != nil {
		t.Fatalf("recreateSchema() failed: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='products'").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Errorf("recreateSchema() did not create products table")
	}
}

func TestConfirmReset(t *testing.T) {
	t.Skip("confirmReset() requires stdin interaction, skipping")
}

func TestDumpAndSeed_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	schemaPath := filepath.Join(tmpDir, "schema.yaml")
	dumpPath := filepath.Join(tmpDir, "dump.json")

	schemaContent := `version: 1
collections:
  products:
    fields:
      id:
        type: id
        primary: true
        default: auto
      name:
        type: string
      price:
        type: float
`
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0o600); err != nil {
		t.Fatal(err)
	}

	s, err := schema.ParseFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &config.DatabaseConfig{Path: dbPath}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	_, err = db.Exec("INSERT INTO products (id, name, price) VALUES (?, ?, ?)", "p1", "Widget", 9.99)
	if err != nil {
		t.Fatal(err)
	}

	rows, err := db.Query("SELECT * FROM products")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	documents, err := database.ScanRows(rows)
	if err != nil {
		t.Fatal(err)
	}

	dump := map[string][]database.Row{"products": documents}
	dumpData, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(dumpPath, dumpData, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec("DELETE FROM products"); err != nil {
		t.Fatal(err)
	}

	seedData, err := parseSeedData(dumpPath, dumpData)
	if err != nil {
		t.Fatal(err)
	}

	col := s.Collections["products"]
	inserted, err := seedCollection(db, col, seedData["products"])
	if err != nil {
		t.Fatal(err)
	}

	if inserted != 1 {
		t.Errorf("seedCollection() inserted %d documents, want 1", inserted)
	}

	var name string
	var price float64
	err = db.QueryRow("SELECT name, price FROM products WHERE id = ?", "p1").Scan(&name, &price)
	if err != nil {
		t.Fatalf("failed to query restored data: %v", err)
	}

	if name != "Widget" {
		t.Errorf("restored name = %q, want %q", name, "Widget")
	}
	if price != 9.99 {
		t.Errorf("restored price = %v, want %v", price, 9.99)
	}
}

func TestParseSeedData_InvalidJSON(t *testing.T) {
	invalidJSON := `{"invalid": json}`

	_, err := parseSeedData("test.json", []byte(invalidJSON))
	if err == nil {
		t.Error("parseSeedData() with invalid JSON should error")
	}
}

func TestParseSeedData_InvalidYAML(t *testing.T) {
	invalidYAML := `
invalid:
  - yaml
  - - - structure
`

	_, err := parseSeedData("test.yaml", []byte(invalidYAML))
	if err == nil {
		t.Error("parseSeedData() with invalid YAML should error")
	}
}
