package schema

import (
	"os"
	"path/filepath"
	"testing"
)

// Helper function to create a test schema file
func createTestSchema(t *testing.T) (string, *Schema) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.yaml")

	schema := &Schema{
		Version: 1,
		Collections: map[string]*Collection{
			"users": {
				Name: "users",
				Fields: map[string]*Field{
					"id": {Name: "id", Type: FieldTypeID, Primary: true},
				},
				fieldOrder: []string{"id"},
			},
		},
		Buckets:   map[string]*Bucket{},
		Functions: map[string]*Function{},
	}

	if err := WriteFile(path, schema); err != nil {
		t.Fatalf("writing test schema: %v", err)
	}

	return path, schema
}

// Helper function to create a test schema with buckets
func createTestSchemaWithBucket(t *testing.T) (string, *Schema) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.yaml")

	schema := &Schema{
		Version: 1,
		Collections: map[string]*Collection{
			"users": {
				Name: "users",
				Fields: map[string]*Field{
					"id": {Name: "id", Type: FieldTypeID, Primary: true},
				},
				fieldOrder: []string{"id"},
			},
		},
		Buckets: map[string]*Bucket{
			"avatars": {
				Name:    "avatars",
				Backend: "filesystem",
			},
		},
		Functions: map[string]*Function{},
	}

	if err := WriteFile(path, schema); err != nil {
		t.Fatalf("writing test schema: %v", err)
	}

	return path, schema
}

func TestNewManager(t *testing.T) {
	path := "/tmp/test_schema.yaml"
	m := NewManager(path)

	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	if m.path != path {
		t.Errorf("path: got %s, want %s", m.path, path)
	}

	if m.schema != nil {
		t.Error("schema should be nil before Load()")
	}
}

func TestManagerLoad(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr bool
	}{
		{
			name: "success",
			setup: func(t *testing.T) string {
				path, _ := createTestSchema(t)
				return path
			},
			wantErr: false,
		},
		{
			name: "non-existent file",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nonexistent.yaml")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			m := NewManager(path)

			err := m.Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && m.schema == nil {
				t.Error("schema should not be nil after successful Load()")
			}
		})
	}
}

func TestManagerSave(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		path, _ := createTestSchema(t)
		m := NewManager(path)

		if err := m.Load(); err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		// Modify schema
		m.schema.Collections["users"].Fields["email"] = &Field{
			Name: "email",
			Type: FieldTypeEmail,
		}
		m.schema.Collections["users"].fieldOrder = append(m.schema.Collections["users"].fieldOrder, "email")

		if err := m.Save(); err != nil {
			t.Fatalf("Save() failed: %v", err)
		}

		// Verify file was written
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("schema file was not saved")
		}

		// Reload and verify
		m2 := NewManager(path)
		if err := m2.Load(); err != nil {
			t.Fatalf("reloading schema failed: %v", err)
		}

		if _, ok := m2.schema.Collections["users"].Fields["email"]; !ok {
			t.Error("email field not found after reload")
		}
	})

	t.Run("nil schema panics", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "schema.yaml")
		m := NewManager(path)

		// Don't load, schema is nil - Save() will panic in Validate()
		defer func() {
			if r := recover(); r == nil {
				t.Error("Save() should panic with nil schema")
			}
		}()

		_ = m.Save()
	})

	t.Run("onChange callback", func(t *testing.T) {
		path, _ := createTestSchema(t)
		m := NewManager(path)

		if err := m.Load(); err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		called := false
		m.SetOnChange(func(s *Schema) {
			called = true
			if s == nil {
				t.Error("onChange callback received nil schema")
			}
		})

		if err := m.Save(); err != nil {
			t.Fatalf("Save() failed: %v", err)
		}

		if !called {
			t.Error("onChange callback was not called")
		}
	})
}

func TestManagerGetSchema(t *testing.T) {
	t.Run("returns copy", func(t *testing.T) {
		path, _ := createTestSchema(t)
		m := NewManager(path)

		if err := m.Load(); err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		schema1 := m.GetSchema()
		schema2 := m.GetSchema()

		// Modify schema1
		schema1.Collections["users"].Fields["modified"] = &Field{
			Name: "modified",
			Type: FieldTypeString,
		}

		// schema2 should not be affected
		if _, ok := schema2.Collections["users"].Fields["modified"]; ok {
			t.Error("GetSchema() did not return a copy - modification affected other copy")
		}

		// Original should not be affected
		schema3 := m.GetSchema()
		if _, ok := schema3.Collections["users"].Fields["modified"]; ok {
			t.Error("GetSchema() did not return a copy - modification affected original")
		}
	})

	t.Run("nil schema", func(t *testing.T) {
		m := NewManager("/tmp/test.yaml")
		schema := m.GetSchema()

		if schema != nil {
			t.Error("GetSchema() should return nil when schema is not loaded")
		}
	})
}

func TestManagerUpdateFromYAML(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := NewManager("/tmp/test.yaml")

		yamlContent := []byte(`
version: 1
collections:
  posts:
    fields:
      id:
        type: id
        primary: true
      title:
        type: string
`)

		if err := m.UpdateFromYAML(yamlContent); err != nil {
			t.Fatalf("UpdateFromYAML() failed: %v", err)
		}

		schema := m.GetSchema()
		if schema == nil {
			t.Fatal("schema should not be nil after UpdateFromYAML()")
		}

		if _, ok := schema.Collections["posts"]; !ok {
			t.Error("posts collection not found")
		}
	})

	t.Run("invalid YAML", func(t *testing.T) {
		m := NewManager("/tmp/test.yaml")

		yamlContent := []byte(`invalid: yaml: content:`)

		err := m.UpdateFromYAML(yamlContent)
		if err == nil {
			t.Error("UpdateFromYAML() should fail with invalid YAML")
		}
	})
}

func TestManagerSetOnChange(t *testing.T) {
	m := NewManager("/tmp/test.yaml")

	called := false
	m.SetOnChange(func(s *Schema) {
		called = true
	})

	// Callback should not be called just by setting it
	if called {
		t.Error("onChange callback should not be called when setting it")
	}
}

func TestManagerAddBucket(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) *Manager
		bucket  *Bucket
		wantErr bool
	}{
		{
			name: "success",
			setup: func(t *testing.T) *Manager {
				path, _ := createTestSchema(t)
				m := NewManager(path)
				if err := m.Load(); err != nil {
					t.Fatalf("Load() failed: %v", err)
				}
				return m
			},
			bucket: &Bucket{
				Backend: "filesystem",
			},
			wantErr: false,
		},
		{
			name: "duplicate bucket",
			setup: func(t *testing.T) *Manager {
				path, _ := createTestSchemaWithBucket(t)
				m := NewManager(path)
				if err := m.Load(); err != nil {
					t.Fatalf("Load() failed: %v", err)
				}
				return m
			},
			bucket: &Bucket{
				Backend: "filesystem",
			},
			wantErr: true,
		},
		{
			name: "nil schema",
			setup: func(t *testing.T) *Manager {
				return NewManager("/tmp/test.yaml")
			},
			bucket: &Bucket{
				Backend: "filesystem",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup(t)
			bucketName := "avatars"

			err := m.AddBucket(bucketName, tt.bucket)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddBucket() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				schema := m.GetSchema()
				if _, ok := schema.Buckets[bucketName]; !ok {
					t.Error("bucket was not added")
				}
				if schema.Buckets[bucketName].Name != bucketName {
					t.Errorf("bucket name: got %s, want %s", schema.Buckets[bucketName].Name, bucketName)
				}
			}
		})
	}
}

func TestManagerUpdateBucket(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) *Manager
		bucket  *Bucket
		wantErr bool
	}{
		{
			name: "success",
			setup: func(t *testing.T) *Manager {
				path, _ := createTestSchemaWithBucket(t)
				m := NewManager(path)
				if err := m.Load(); err != nil {
					t.Fatalf("Load() failed: %v", err)
				}
				return m
			},
			bucket: &Bucket{
				Backend:     "filesystem",
				MaxFileSize: 1024,
			},
			wantErr: false,
		},
		{
			name: "non-existent bucket",
			setup: func(t *testing.T) *Manager {
				path, _ := createTestSchema(t)
				m := NewManager(path)
				if err := m.Load(); err != nil {
					t.Fatalf("Load() failed: %v", err)
				}
				return m
			},
			bucket: &Bucket{
				Backend: "filesystem",
			},
			wantErr: true,
		},
		{
			name: "nil schema",
			setup: func(t *testing.T) *Manager {
				return NewManager("/tmp/test.yaml")
			},
			bucket: &Bucket{
				Backend: "filesystem",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup(t)
			bucketName := "avatars"

			err := m.UpdateBucket(bucketName, tt.bucket)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateBucket() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				schema := m.GetSchema()
				if schema.Buckets[bucketName].MaxFileSize != 1024 {
					t.Error("bucket was not updated")
				}
			}
		})
	}
}

func TestManagerDeleteBucket(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		path, _ := createTestSchemaWithBucket(t)
		m := NewManager(path)
		if err := m.Load(); err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		err := m.DeleteBucket("avatars")
		if err != nil {
			t.Errorf("DeleteBucket() error = %v", err)
		}

		schema := m.GetSchema()
		if _, ok := schema.Buckets["avatars"]; ok {
			t.Error("bucket was not deleted")
		}
	})

	t.Run("non-existent bucket", func(t *testing.T) {
		path, _ := createTestSchema(t)
		m := NewManager(path)
		if err := m.Load(); err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		err := m.DeleteBucket("nonexistent")
		if err == nil {
			t.Error("DeleteBucket() should fail for non-existent bucket")
		}
	})

	t.Run("bucket referenced by file field", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "schema.yaml")

		schema := &Schema{
			Version: 1,
			Collections: map[string]*Collection{
				"users": {
					Name: "users",
					Fields: map[string]*Field{
						"id": {Name: "id", Type: FieldTypeID, Primary: true},
						"avatar": {
							Name: "avatar",
							Type: FieldTypeFile,
							File: &FileConfig{
								Bucket: "avatars",
							},
						},
					},
					fieldOrder: []string{"id", "avatar"},
				},
			},
			Buckets: map[string]*Bucket{
				"avatars": {
					Name:    "avatars",
					Backend: "filesystem",
				},
			},
			Functions: map[string]*Function{},
		}

		if err := WriteFile(path, schema); err != nil {
			t.Fatalf("writing test schema: %v", err)
		}

		m := NewManager(path)
		if err := m.Load(); err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		err := m.DeleteBucket("avatars")
		if err == nil {
			t.Error("DeleteBucket() should fail when bucket is referenced")
		}
	})

	t.Run("nil schema", func(t *testing.T) {
		m := NewManager("/tmp/test.yaml")

		err := m.DeleteBucket("avatars")
		if err == nil {
			t.Error("DeleteBucket() should fail with nil schema")
		}
	})
}

func TestManagerAddCollection(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T) *Manager
		collection *Collection
		wantErr    bool
	}{
		{
			name: "success",
			setup: func(t *testing.T) *Manager {
				path, _ := createTestSchema(t)
				m := NewManager(path)
				if err := m.Load(); err != nil {
					t.Fatalf("Load() failed: %v", err)
				}
				return m
			},
			collection: &Collection{
				Fields: map[string]*Field{
					"id": {Name: "id", Type: FieldTypeID, Primary: true},
				},
				fieldOrder: []string{"id"},
			},
			wantErr: false,
		},
		{
			name: "duplicate collection",
			setup: func(t *testing.T) *Manager {
				path, _ := createTestSchema(t)
				m := NewManager(path)
				if err := m.Load(); err != nil {
					t.Fatalf("Load() failed: %v", err)
				}
				return m
			},
			collection: &Collection{
				Fields: map[string]*Field{
					"id": {Name: "id", Type: FieldTypeID, Primary: true},
				},
				fieldOrder: []string{"id"},
			},
			wantErr: true,
		},
		{
			name: "nil schema",
			setup: func(t *testing.T) *Manager {
				return NewManager("/tmp/test.yaml")
			},
			collection: &Collection{
				Fields: map[string]*Field{
					"id": {Name: "id", Type: FieldTypeID, Primary: true},
				},
				fieldOrder: []string{"id"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup(t)
			collectionName := "posts"
			if tt.name == "duplicate collection" {
				collectionName = "users"
			}

			err := m.AddCollection(collectionName, tt.collection)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddCollection() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				schema := m.GetSchema()
				if _, ok := schema.Collections[collectionName]; !ok {
					t.Error("collection was not added")
				}
				if schema.Collections[collectionName].Name != collectionName {
					t.Errorf("collection name: got %s, want %s", schema.Collections[collectionName].Name, collectionName)
				}
			}
		})
	}
}

func TestManagerUpdateCollection(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T) *Manager
		collection *Collection
		wantErr    bool
	}{
		{
			name: "success",
			setup: func(t *testing.T) *Manager {
				path, _ := createTestSchema(t)
				m := NewManager(path)
				if err := m.Load(); err != nil {
					t.Fatalf("Load() failed: %v", err)
				}
				return m
			},
			collection: &Collection{
				Fields: map[string]*Field{
					"id":    {Name: "id", Type: FieldTypeID, Primary: true},
					"email": {Name: "email", Type: FieldTypeEmail},
				},
				fieldOrder: []string{"id", "email"},
			},
			wantErr: false,
		},
		{
			name: "non-existent collection",
			setup: func(t *testing.T) *Manager {
				path, _ := createTestSchema(t)
				m := NewManager(path)
				if err := m.Load(); err != nil {
					t.Fatalf("Load() failed: %v", err)
				}
				return m
			},
			collection: &Collection{
				Fields: map[string]*Field{
					"id": {Name: "id", Type: FieldTypeID, Primary: true},
				},
				fieldOrder: []string{"id"},
			},
			wantErr: true,
		},
		{
			name: "nil schema",
			setup: func(t *testing.T) *Manager {
				return NewManager("/tmp/test.yaml")
			},
			collection: &Collection{
				Fields: map[string]*Field{
					"id": {Name: "id", Type: FieldTypeID, Primary: true},
				},
				fieldOrder: []string{"id"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.setup(t)
			collectionName := "users"
			if tt.name == "non-existent collection" {
				collectionName = "nonexistent"
			}

			err := m.UpdateCollection(collectionName, tt.collection)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateCollection() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				schema := m.GetSchema()
				if _, ok := schema.Collections[collectionName].Fields["email"]; !ok {
					t.Error("collection was not updated")
				}
			}
		})
	}
}

func TestManagerDeleteCollection(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		path, _ := createTestSchema(t)
		m := NewManager(path)
		if err := m.Load(); err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		err := m.DeleteCollection("users")
		if err != nil {
			t.Errorf("DeleteCollection() error = %v", err)
		}

		schema := m.GetSchema()
		if _, ok := schema.Collections["users"]; ok {
			t.Error("collection was not deleted")
		}
	})

	t.Run("non-existent collection", func(t *testing.T) {
		path, _ := createTestSchema(t)
		m := NewManager(path)
		if err := m.Load(); err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		err := m.DeleteCollection("nonexistent")
		if err == nil {
			t.Error("DeleteCollection() should fail for non-existent collection")
		}
	})

	t.Run("collection referenced by relation field", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "schema.yaml")

		schema := &Schema{
			Version: 1,
			Collections: map[string]*Collection{
				"users": {
					Name: "users",
					Fields: map[string]*Field{
						"id": {Name: "id", Type: FieldTypeID, Primary: true},
					},
					fieldOrder: []string{"id"},
				},
				"posts": {
					Name: "posts",
					Fields: map[string]*Field{
						"id": {Name: "id", Type: FieldTypeID, Primary: true},
						"author": {
							Name: "author",
							Type: FieldTypeRelation,
							Relation: &RelationConfig{
								Collection: "users",
								Field:      "id",
							},
						},
					},
					fieldOrder: []string{"id", "author"},
				},
			},
			Buckets:   map[string]*Bucket{},
			Functions: map[string]*Function{},
		}

		if err := WriteFile(path, schema); err != nil {
			t.Fatalf("writing test schema: %v", err)
		}

		m := NewManager(path)
		if err := m.Load(); err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		err := m.DeleteCollection("users")
		if err == nil {
			t.Error("DeleteCollection() should fail when collection is referenced")
		}
	})

	t.Run("nil schema", func(t *testing.T) {
		m := NewManager("/tmp/test.yaml")

		err := m.DeleteCollection("users")
		if err == nil {
			t.Error("DeleteCollection() should fail with nil schema")
		}
	})
}

func TestManagerValidationRollback(t *testing.T) {
	t.Run("AddBucket validation failure rolls back", func(t *testing.T) {
		path, _ := createTestSchema(t)
		m := NewManager(path)
		if err := m.Load(); err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		// Try to add invalid bucket (missing backend)
		err := m.AddBucket("invalid", &Bucket{})
		if err == nil {
			t.Error("AddBucket() should fail validation")
		}

		// Verify bucket was not added
		schema := m.GetSchema()
		if _, ok := schema.Buckets["invalid"]; ok {
			t.Error("invalid bucket should not be in schema after validation failure")
		}
	})

	t.Run("UpdateBucket validation failure rolls back", func(t *testing.T) {
		path, _ := createTestSchemaWithBucket(t)
		m := NewManager(path)
		if err := m.Load(); err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		// Try to update with invalid bucket (missing backend)
		err := m.UpdateBucket("avatars", &Bucket{})
		if err == nil {
			t.Error("UpdateBucket() should fail validation")
		}

		// Verify bucket was not updated
		schema := m.GetSchema()
		if schema.Buckets["avatars"].Backend != "filesystem" {
			t.Error("bucket should not be updated after validation failure")
		}
	})

	t.Run("AddCollection validation failure rolls back", func(t *testing.T) {
		path, _ := createTestSchema(t)
		m := NewManager(path)
		if err := m.Load(); err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		// Try to add invalid collection (no primary key)
		err := m.AddCollection("invalid", &Collection{
			Fields: map[string]*Field{
				"name": {Name: "name", Type: FieldTypeString},
			},
			fieldOrder: []string{"name"},
		})
		if err == nil {
			t.Error("AddCollection() should fail validation")
		}

		// Verify collection was not added
		schema := m.GetSchema()
		if _, ok := schema.Collections["invalid"]; ok {
			t.Error("invalid collection should not be in schema after validation failure")
		}
	})

	t.Run("UpdateCollection validation failure rolls back", func(t *testing.T) {
		path, _ := createTestSchema(t)
		m := NewManager(path)
		if err := m.Load(); err != nil {
			t.Fatalf("Load() failed: %v", err)
		}

		// Try to update with invalid collection (no primary key)
		err := m.UpdateCollection("users", &Collection{
			Fields: map[string]*Field{
				"name": {Name: "name", Type: FieldTypeString},
			},
			fieldOrder: []string{"name"},
		})
		if err == nil {
			t.Error("UpdateCollection() should fail validation")
		}

		// Verify collection was not updated
		schema := m.GetSchema()
		if _, ok := schema.Collections["users"].Fields["id"]; !ok {
			t.Error("collection should not be updated after validation failure")
		}
	})
}
