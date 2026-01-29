package schema

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMarshal(t *testing.T) {
	s := &Schema{
		Version: 1,
		Collections: map[string]*Collection{
			"users": {
				Name: "users",
				Fields: map[string]*Field{
					"id": {
						Name:    "id",
						Type:    FieldTypeID,
						Primary: true,
						Default: "auto",
					},
					"email": {
						Name:   "email",
						Type:   FieldTypeEmail,
						Unique: true,
					},
					"name": {
						Name:     "name",
						Type:     FieldTypeString,
						Nullable: true,
					},
				},
				fieldOrder: []string{"id", "email", "name"},
			},
		},
		Buckets: map[string]*Bucket{
			"avatars": {
				Name:         "avatars",
				Backend:      "filesystem",
				MaxFileSize:  5242880,
				AllowedTypes: []string{"image/*"},
			},
		},
	}

	data, err := Marshal(s)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Marshal returned empty data")
	}

	t.Logf("Marshaled YAML:\n%s", string(data))

	s2, err := Parse(data)
	if err != nil {
		t.Fatalf("Re-parsing marshaled YAML failed: %v", err)
	}

	if s2.Version != s.Version {
		t.Errorf("Version mismatch: got %d, want %d", s2.Version, s.Version)
	}

	if len(s2.Collections) != len(s.Collections) {
		t.Errorf("Collections count mismatch: got %d, want %d", len(s2.Collections), len(s.Collections))
	}

	if len(s2.Buckets) != len(s.Buckets) {
		t.Errorf("Buckets count mismatch: got %d, want %d", len(s2.Buckets), len(s.Buckets))
	}

	if col, ok := s2.Collections["users"]; !ok {
		t.Error("users collection not found after round-trip")
	} else {
		if len(col.Fields) != 3 {
			t.Errorf("users fields count: got %d, want 3", len(col.Fields))
		}
		if col.Fields["id"].Primary != true {
			t.Error("id field should be primary")
		}
		if col.Fields["email"].Unique != true {
			t.Error("email field should be unique")
		}
	}

	if bucket, ok := s2.Buckets["avatars"]; !ok {
		t.Error("avatars bucket not found after round-trip")
	} else {
		if bucket.Backend != "filesystem" {
			t.Errorf("bucket backend: got %s, want filesystem", bucket.Backend)
		}
		if bucket.MaxFileSize != 5242880 {
			t.Errorf("bucket max_file_size: got %d, want 5242880", bucket.MaxFileSize)
		}
	}
}

func TestWriteFile(t *testing.T) {
	s := &Schema{
		Version: 1,
		Collections: map[string]*Collection{
			"posts": {
				Name: "posts",
				Fields: map[string]*Field{
					"id": {
						Name:    "id",
						Type:    FieldTypeID,
						Primary: true,
					},
					"title": {
						Name: "title",
						Type: FieldTypeString,
					},
				},
				fieldOrder: []string{"id", "title"},
			},
		},
		Buckets: map[string]*Bucket{},
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_schema.yaml")

	if err := WriteFile(path, s); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("File was not created")
	}

	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("Temporary file was not cleaned up")
	}

	s2, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(s2.Collections) != 1 {
		t.Errorf("Collections count: got %d, want 1", len(s2.Collections))
	}
}

func TestMarshalFieldOrder(t *testing.T) {
	s := &Schema{
		Version: 1,
		Collections: map[string]*Collection{
			"test": {
				Name: "test",
				Fields: map[string]*Field{
					"id":      {Name: "id", Type: FieldTypeID, Primary: true},
					"z_field": {Name: "z_field", Type: FieldTypeString},
					"a_field": {Name: "a_field", Type: FieldTypeString},
					"m_field": {Name: "m_field", Type: FieldTypeString},
				},
				fieldOrder: []string{"id", "z_field", "a_field", "m_field"},
			},
		},
		Buckets: map[string]*Bucket{},
	}

	data, err := Marshal(s)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	s2, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	order := s2.Collections["test"].FieldOrder()
	expected := []string{"id", "z_field", "a_field", "m_field"}

	if len(order) != len(expected) {
		t.Fatalf("Field order length: got %d, want %d", len(order), len(expected))
	}

	for i, name := range expected {
		if order[i] != name {
			t.Errorf("Field order[%d]: got %s, want %s", i, order[i], name)
		}
	}
}

func TestMarshalSortedNames(t *testing.T) {
	s := &Schema{
		Version: 1,
		Collections: map[string]*Collection{
			"zebra": {
				Name:       "zebra",
				Fields:     map[string]*Field{"id": {Name: "id", Type: FieldTypeID, Primary: true}},
				fieldOrder: []string{"id"},
			},
			"apple": {
				Name:       "apple",
				Fields:     map[string]*Field{"id": {Name: "id", Type: FieldTypeID, Primary: true}},
				fieldOrder: []string{"id"},
			},
			"mango": {
				Name:       "mango",
				Fields:     map[string]*Field{"id": {Name: "id", Type: FieldTypeID, Primary: true}},
				fieldOrder: []string{"id"},
			},
		},
		Buckets: map[string]*Bucket{
			"zoo": {Name: "zoo", Backend: "filesystem"},
			"art": {Name: "art", Backend: "filesystem"},
		},
	}

	data, err := Marshal(s)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	yamlStr := string(data)
	t.Logf("YAML output:\n%s", yamlStr)
}

func TestMarshalNilSchema(t *testing.T) {
	_, err := Marshal(nil)
	if err == nil {
		t.Error("Expected error for nil schema, got nil")
	}
}

func TestMarshalEmptyOptionalFields(t *testing.T) {
	s := &Schema{
		Version: 1,
		Collections: map[string]*Collection{
			"simple": {
				Name: "simple",
				Fields: map[string]*Field{
					"id": {
						Name:    "id",
						Type:    FieldTypeID,
						Primary: true,
					},
				},
				fieldOrder: []string{"id"},
			},
		},
		Buckets: map[string]*Bucket{},
	}

	data, err := Marshal(s)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	yamlStr := string(data)
	t.Logf("YAML with omitempty:\n%s", yamlStr)
}
