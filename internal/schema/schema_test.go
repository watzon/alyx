package schema

import (
	"strings"
	"testing"
)

func TestParseSchema(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      email:
        type: string
        unique: true
        index: true
      name:
        type: string
        nullable: true
      created_at:
        type: timestamp
        default: now
    rules:
      create: "true"
      read: "auth.id == doc.id"
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	if schema.Version != 1 {
		t.Errorf("expected version 1, got %d", schema.Version)
	}

	users, ok := schema.Collections["users"]
	if !ok {
		t.Fatal("users collection not found")
	}

	if len(users.Fields) != 4 {
		t.Errorf("expected 4 fields, got %d", len(users.Fields))
	}

	idField := users.Fields["id"]
	if idField.Type != FieldTypeUUID {
		t.Errorf("expected id type uuid, got %s", idField.Type)
	}
	if !idField.Primary {
		t.Error("expected id to be primary")
	}

	emailField := users.Fields["email"]
	if !emailField.Unique {
		t.Error("expected email to be unique")
	}
	if !emailField.Index {
		t.Error("expected email to be indexed")
	}

	nameField := users.Fields["name"]
	if !nameField.Nullable {
		t.Error("expected name to be nullable")
	}

	if users.Rules.Create != "true" {
		t.Errorf("expected create rule 'true', got %q", users.Rules.Create)
	}
}

func TestFieldOrder(t *testing.T) {
	yaml := `
version: 1

collections:
  test:
    fields:
      id:
        type: uuid
        primary: true
      alpha:
        type: string
        nullable: true
      beta:
        type: string
        nullable: true
      gamma:
        type: string
        nullable: true
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	col := schema.Collections["test"]
	order := col.FieldOrder()

	expected := []string{"id", "alpha", "beta", "gamma"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d fields, got %d", len(expected), len(order))
	}

	for i, name := range expected {
		if order[i] != name {
			t.Errorf("expected field %d to be %q, got %q", i, name, order[i])
		}
	}
}

func TestValidation_MissingPrimaryKey(t *testing.T) {
	yaml := `
version: 1

collections:
  test:
    fields:
      name:
        type: string
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for missing primary key")
	}
}

func TestValidation_InvalidFieldType(t *testing.T) {
	yaml := `
version: 1

collections:
  test:
    fields:
      id:
        type: uuid
        primary: true
      bad:
        type: invalid_type
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for invalid field type")
	}
}

func TestValidation_InvalidReference(t *testing.T) {
	yaml := `
version: 1

collections:
  posts:
    fields:
      id:
        type: uuid
        primary: true
      author_id:
        type: uuid
        references: nonexistent.id
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for invalid reference")
	}
}

func TestValidation_ReservedCollectionName(t *testing.T) {
	yaml := `
version: 1

collections:
  _alyx_test:
    fields:
      id:
        type: uuid
        primary: true
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected validation error for reserved collection name")
	}
}

func TestSQLGenerator_CreateTable(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
      email:
        type: string
        unique: true
      name:
        type: string
        nullable: true
      active:
        type: bool
        default: true
      created_at:
        type: timestamp
        default: now
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	gen := NewSQLGenerator(schema)
	sql := gen.GenerateCreateTable(schema.Collections["users"])

	if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS users") {
		t.Error("expected CREATE TABLE statement")
	}
	if !strings.Contains(sql, "id TEXT PRIMARY KEY") {
		t.Error("expected primary key definition")
	}
	if !strings.Contains(sql, "email TEXT NOT NULL UNIQUE") {
		t.Error("expected unique constraint on email")
	}
	if !strings.Contains(sql, "name TEXT") && strings.Contains(sql, "name TEXT NOT NULL") {
		t.Error("name should be nullable (no NOT NULL)")
	}
	if !strings.Contains(sql, "active INTEGER NOT NULL DEFAULT 1") {
		t.Error("expected default value for bool field")
	}
	if !strings.Contains(sql, "datetime('now')") {
		t.Error("expected datetime default for timestamp")
	}
}

func TestSQLGenerator_ForeignKey(t *testing.T) {
	yaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true

  posts:
    fields:
      id:
        type: uuid
        primary: true
      author_id:
        type: uuid
        references: users.id
        onDelete: cascade
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	gen := NewSQLGenerator(schema)
	sql := gen.GenerateCreateTable(schema.Collections["posts"])

	if !strings.Contains(sql, "FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE CASCADE") {
		t.Errorf("expected foreign key constraint, got:\n%s", sql)
	}
}

func TestSQLGenerator_Triggers(t *testing.T) {
	yaml := `
version: 1

collections:
  posts:
    fields:
      id:
        type: uuid
        primary: true
      title:
        type: string
      updated_at:
        type: timestamp
        onUpdate: now
`
	schema, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	gen := NewSQLGenerator(schema)
	triggers := gen.GenerateTriggers(schema.Collections["posts"])

	hasInsert := false
	hasUpdate := false
	hasDelete := false
	hasAutoUpdate := false

	for _, trig := range triggers {
		if strings.Contains(trig, "posts_after_insert") {
			hasInsert = true
		}
		if strings.Contains(trig, "posts_after_update") {
			hasUpdate = true
		}
		if strings.Contains(trig, "posts_after_delete") {
			hasDelete = true
		}
		if strings.Contains(trig, "posts_auto_update_timestamp") {
			hasAutoUpdate = true
		}
	}

	if !hasInsert {
		t.Error("expected insert trigger")
	}
	if !hasUpdate {
		t.Error("expected update trigger")
	}
	if !hasDelete {
		t.Error("expected delete trigger")
	}
	if !hasAutoUpdate {
		t.Error("expected auto-update timestamp trigger")
	}
}

func TestDiffer_AddCollection(t *testing.T) {
	oldYaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
`
	newYaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
  posts:
    fields:
      id:
        type: uuid
        primary: true
`
	old, _ := Parse([]byte(oldYaml))
	new, _ := Parse([]byte(newYaml))

	differ := NewDiffer()
	changes := differ.Diff(old, new)

	found := false
	for _, c := range changes {
		if c.Type == ChangeAddCollection && c.Collection == "posts" {
			found = true
			if !c.Safe {
				t.Error("adding collection should be safe")
			}
		}
	}
	if !found {
		t.Error("expected add collection change")
	}
}

func TestDiffer_AddField(t *testing.T) {
	oldYaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
`
	newYaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
      email:
        type: string
        nullable: true
`
	old, _ := Parse([]byte(oldYaml))
	new, _ := Parse([]byte(newYaml))

	differ := NewDiffer()
	changes := differ.Diff(old, new)

	found := false
	for _, c := range changes {
		if c.Type == ChangeAddField && c.Field == "email" {
			found = true
			if !c.Safe {
				t.Error("adding nullable field should be safe")
			}
		}
	}
	if !found {
		t.Error("expected add field change")
	}
}

func TestDiffer_DropCollection(t *testing.T) {
	oldYaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
  posts:
    fields:
      id:
        type: uuid
        primary: true
`
	newYaml := `
version: 1

collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
`
	old, _ := Parse([]byte(oldYaml))
	new, _ := Parse([]byte(newYaml))

	differ := NewDiffer()
	changes := differ.Diff(old, new)

	found := false
	for _, c := range changes {
		if c.Type == ChangeDropCollection && c.Collection == "posts" {
			found = true
			if c.Safe {
				t.Error("dropping collection should NOT be safe")
			}
			if !c.RequiresManual {
				t.Error("dropping collection should require manual migration")
			}
		}
	}
	if !found {
		t.Error("expected drop collection change")
	}
}

func TestFieldType_SQLiteType(t *testing.T) {
	tests := []struct {
		fieldType FieldType
		expected  string
	}{
		{FieldTypeUUID, "TEXT"},
		{FieldTypeString, "TEXT"},
		{FieldTypeText, "TEXT"},
		{FieldTypeInt, "INTEGER"},
		{FieldTypeFloat, "REAL"},
		{FieldTypeBool, "INTEGER"},
		{FieldTypeTimestamp, "TEXT"},
		{FieldTypeJSON, "TEXT"},
		{FieldTypeBlob, "BLOB"},
	}

	for _, tt := range tests {
		t.Run(string(tt.fieldType), func(t *testing.T) {
			if got := tt.fieldType.SQLiteType(); got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestFieldType_GoType(t *testing.T) {
	tests := []struct {
		fieldType FieldType
		nullable  bool
		expected  string
	}{
		{FieldTypeString, false, "string"},
		{FieldTypeString, true, "*string"},
		{FieldTypeInt, false, "int64"},
		{FieldTypeBool, true, "*bool"},
		{FieldTypeJSON, false, "any"},
		{FieldTypeJSON, true, "any"},
	}

	for _, tt := range tests {
		t.Run(string(tt.fieldType), func(t *testing.T) {
			if got := tt.fieldType.GoType(tt.nullable); got != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}
