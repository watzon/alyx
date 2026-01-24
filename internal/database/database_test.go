package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/schema"
)

func testDB(t *testing.T) *DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.DatabaseConfig{
		Path:         dbPath,
		WALMode:      true,
		ForeignKeys:  true,
		CacheSize:    -2000,
		BusyTimeout:  5 * time.Second,
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	}

	db, err := Open(cfg)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestOpenAndClose(t *testing.T) {
	db := testDB(t)

	if err := db.Ping(context.Background()); err != nil {
		t.Errorf("ping failed: %v", err)
	}
}

func TestTransaction(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	err = db.Transaction(ctx, func(tx *Tx) error {
		_, err := tx.Exec("INSERT INTO test (id, name) VALUES (1, 'alice')")
		if err != nil {
			return err
		}
		_, err = tx.Exec("INSERT INTO test (id, name) VALUES (2, 'bob')")
		return err
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test").Scan(&count)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 rows, got %d", count)
	}
}

func TestTransactionRollback(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT UNIQUE)")
	if err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	err = db.Transaction(ctx, func(tx *Tx) error {
		_, err := tx.Exec("INSERT INTO test (id, name) VALUES (1, 'alice')")
		if err != nil {
			return err
		}
		_, err = tx.Exec("INSERT INTO test (id, name) VALUES (2, 'alice')")
		return err
	})
	if err == nil {
		t.Fatal("expected transaction to fail")
	}

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM test").Scan(&count)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", count)
	}
}

func TestQueryBuilder(t *testing.T) {
	tests := []struct {
		name     string
		build    func() *QueryBuilder
		expected string
	}{
		{
			name: "simple select",
			build: func() *QueryBuilder {
				return NewQuery("users")
			},
			expected: "SELECT * FROM users",
		},
		{
			name: "select with columns",
			build: func() *QueryBuilder {
				return NewQuery("users").Select("id", "name")
			},
			expected: "SELECT id, name FROM users",
		},
		{
			name: "with filter",
			build: func() *QueryBuilder {
				return NewQuery("users").Where("active", true)
			},
			expected: "SELECT * FROM users WHERE active = ?",
		},
		{
			name: "with sort",
			build: func() *QueryBuilder {
				return NewQuery("users").OrderByDesc("created_at")
			},
			expected: "SELECT * FROM users ORDER BY created_at DESC",
		},
		{
			name: "with limit and offset",
			build: func() *QueryBuilder {
				return NewQuery("users").Limit(10).Offset(20)
			},
			expected: "SELECT * FROM users LIMIT 10 OFFSET 20",
		},
		{
			name: "complex query",
			build: func() *QueryBuilder {
				return NewQuery("posts").
					Select("id", "title", "author_id").
					Where("published", true).
					Filter("view_count", OpGte, 100).
					OrderByDesc("created_at").
					Limit(10)
			},
			expected: "SELECT id, title, author_id FROM posts WHERE published = ? AND view_count >= ? ORDER BY created_at DESC LIMIT 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, _ := tt.build().Build()
			if sql != tt.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", tt.expected, sql)
			}
		})
	}
}

func TestInsertBuilder(t *testing.T) {
	sql, args := NewInsert("users").
		Set("id", "123").
		Set("name", "alice").
		Set("active", true).
		Build()

	expected := "INSERT INTO users (id, name, active) VALUES (?, ?, ?)"
	if sql != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, sql)
	}

	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestUpdateBuilder(t *testing.T) {
	sql, args := NewUpdate("users").
		Set("name", "bob").
		Set("active", false).
		Where("id", "123").
		Build()

	expected := "UPDATE users SET name = ?, active = ? WHERE id = ?"
	if sql != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, sql)
	}

	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestDeleteBuilder(t *testing.T) {
	sql, args := NewDelete("users").
		Where("id", "123").
		Build()

	expected := "DELETE FROM users WHERE id = ?"
	if sql != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, sql)
	}

	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

func TestCollection_CRUD(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	schemaYAML := `
version: 1
collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      name:
        type: string
      email:
        type: string
        unique: true
      active:
        type: bool
        default: true
      created_at:
        type: timestamp
        default: now
`
	s, err := schema.Parse([]byte(schemaYAML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Logf("SQL: %s", stmt)
			t.Fatalf("execute DDL: %v", err)
		}
	}

	col := NewCollection(db, s.Collections["users"])

	created, err := col.Create(ctx, Row{
		"name":  "Alice",
		"email": "alice@example.com",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	id, ok := created["id"].(string)
	if !ok || id == "" {
		t.Error("expected auto-generated id")
	}

	if created["name"] != "Alice" {
		t.Errorf("expected name 'Alice', got %v", created["name"])
	}

	found, err := col.FindOne(ctx, id)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found["email"] != "alice@example.com" {
		t.Errorf("expected email 'alice@example.com', got %v", found["email"])
	}

	updated, err := col.Update(ctx, id, Row{
		"name": "Alice Smith",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated["name"] != "Alice Smith" {
		t.Errorf("expected updated name 'Alice Smith', got %v", updated["name"])
	}

	results, err := col.Find(ctx, &QueryOptions{
		Filters: []*Filter{{Field: "active", Op: OpEq, Value: 1}},
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(results.Docs) != 1 {
		t.Errorf("expected 1 doc, got %d", len(results.Docs))
	}

	err = col.Delete(ctx, id)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = col.FindOne(ctx, id)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestScanRows(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, "CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT, active INTEGER)")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	_, err = db.ExecContext(ctx, "INSERT INTO test VALUES (1, 'alice', 1), (2, 'bob', 0)")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	rows, err := db.QueryContext(ctx, "SELECT * FROM test ORDER BY id")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	results, err := ScanRows(rows)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(results))
	}

	if results[0]["name"] != "alice" {
		t.Errorf("expected 'alice', got %v", results[0]["name"])
	}
}

func TestParseSortString(t *testing.T) {
	tests := []struct {
		input string
		field string
		order SortOrder
	}{
		{"-created_at", "created_at", SortDesc},
		{"+name", "name", SortAsc},
		{"email", "email", SortAsc},
	}

	for _, tt := range tests {
		field, order := ParseSortString(tt.input)
		if field != tt.field {
			t.Errorf("input %q: expected field %q, got %q", tt.input, tt.field, field)
		}
		if order != tt.order {
			t.Errorf("input %q: expected order %v, got %v", tt.input, tt.order, order)
		}
	}
}

func TestParseFilterString(t *testing.T) {
	tests := []struct {
		input   string
		field   string
		op      FilterOp
		value   string
		wantErr bool
	}{
		{"name:eq:alice", "name", OpEq, "alice", false},
		{"age:gte:18", "age", OpGte, "18", false},
		{"invalid", "", "", "", true},
	}

	for _, tt := range tests {
		f, err := ParseFilterString(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("input %q: expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("input %q: unexpected error: %v", tt.input, err)
			continue
		}
		if f.Field != tt.field {
			t.Errorf("input %q: expected field %q, got %q", tt.input, tt.field, f.Field)
		}
		if f.Op != tt.op {
			t.Errorf("input %q: expected op %v, got %v", tt.input, tt.op, f.Op)
		}
	}
}

func init() {
	os.Setenv("TZ", "UTC")
}
