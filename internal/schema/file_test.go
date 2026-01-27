package schema

import (
	"testing"
)

func TestFileFieldType_SQLiteType(t *testing.T) {
	tests := []struct {
		name     string
		field    *Field
		expected string
	}{
		{
			name: "single file field",
			field: &Field{
				Type: FieldTypeFile,
			},
			expected: "TEXT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.field.Type.SQLiteType()
			if got != tt.expected {
				t.Errorf("SQLiteType() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestFileFieldType_GoType(t *testing.T) {
	tests := []struct {
		name     string
		field    *Field
		nullable bool
		expected string
	}{
		{
			name: "single file field non-nullable",
			field: &Field{
				Type: FieldTypeFile,
			},
			nullable: false,
			expected: "string",
		},
		{
			name: "single file field nullable",
			field: &Field{
				Type: FieldTypeFile,
			},
			nullable: true,
			expected: "*string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.field.Type.GoType(tt.nullable)
			if got != tt.expected {
				t.Errorf("GoType(%v) = %q, want %q", tt.nullable, got, tt.expected)
			}
		})
	}
}

func TestFileFieldType_TypeScriptType(t *testing.T) {
	tests := []struct {
		name     string
		field    *Field
		nullable bool
		expected string
	}{
		{
			name: "single file field non-nullable",
			field: &Field{
				Type: FieldTypeFile,
			},
			nullable: false,
			expected: "string",
		},
		{
			name: "single file field nullable",
			field: &Field{
				Type: FieldTypeFile,
			},
			nullable: true,
			expected: "string | null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.field.Type.TypeScriptType(tt.nullable)
			if got != tt.expected {
				t.Errorf("TypeScriptType(%v) = %q, want %q", tt.nullable, got, tt.expected)
			}
		})
	}
}

func TestFileConfig_Parsing(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantErr   bool
		errMsg    string
		checkFunc func(*testing.T, *Schema)
	}{
		{
			name: "file field with bucket reference",
			yaml: `
version: 1
collections:
  posts:
    fields:
      id:
        type: uuid
        primary: true
      avatar:
        type: file
        file:
          bucket: avatars
          max_size: 5242880
          allowed_types:
            - image/jpeg
            - image/png
          on_delete: cascade
buckets:
  avatars:
    backend: local
`,
			wantErr: false,
			checkFunc: func(t *testing.T, s *Schema) {
				t.Helper()
				col := s.Collections["posts"]
				if col == nil {
					t.Fatal("collection 'posts' not found")
				}
				field := col.Fields["avatar"]
				if field == nil {
					t.Fatal("field 'avatar' not found")
				}
				if field.Type != FieldTypeFile {
					t.Errorf("field type = %q, want %q", field.Type, FieldTypeFile)
				}
				if field.File == nil {
					t.Fatal("file config is nil")
				}
				if field.File.Bucket != "avatars" {
					t.Errorf("bucket = %q, want %q", field.File.Bucket, "avatars")
				}
				if field.File.MaxSize != 5242880 {
					t.Errorf("max_size = %d, want %d", field.File.MaxSize, 5242880)
				}
				if len(field.File.AllowedTypes) != 2 {
					t.Errorf("allowed_types length = %d, want 2", len(field.File.AllowedTypes))
				}
				if field.File.OnDelete != OnDeleteCascade {
					t.Errorf("on_delete = %q, want %q", field.File.OnDelete, OnDeleteCascade)
				}
			},
		},
		{
			name: "file field missing bucket",
			yaml: `
version: 1
collections:
  posts:
    fields:
      id:
        type: uuid
        primary: true
      avatar:
        type: file
`,
			wantErr: true,
			errMsg:  "file field type requires file config with bucket",
		},
		{
			name: "file field with non-existent bucket",
			yaml: `
version: 1
collections:
  posts:
    fields:
      id:
        type: uuid
        primary: true
      avatar:
        type: file
        file:
          bucket: nonexistent
`,
			wantErr: true,
			errMsg:  "referenced bucket \"nonexistent\" does not exist",
		},
		{
			name: "file field with minimal config",
			yaml: `
version: 1
collections:
  posts:
    fields:
      id:
        type: uuid
        primary: true
      avatar:
        type: file
        file:
          bucket: avatars
buckets:
  avatars:
    backend: local
`,
			wantErr: false,
			checkFunc: func(t *testing.T, s *Schema) {
				t.Helper()
				col := s.Collections["posts"]
				field := col.Fields["avatar"]
				if field.File == nil {
					t.Fatal("file config is nil")
				}
				if field.File.Bucket != "avatars" {
					t.Errorf("bucket = %q, want %q", field.File.Bucket, "avatars")
				}
				// Zero values should be allowed (unlimited)
				if field.File.MaxSize != 0 {
					t.Errorf("max_size = %d, want 0", field.File.MaxSize)
				}
				if len(field.File.AllowedTypes) != 0 {
					t.Errorf("allowed_types length = %d, want 0", len(field.File.AllowedTypes))
				}
			},
		},
		{
			name: "file field with on_delete keep",
			yaml: `
version: 1
collections:
  posts:
    fields:
      id:
        type: uuid
        primary: true
      avatar:
        type: file
        file:
          bucket: avatars
          on_delete: restrict
buckets:
  avatars:
    backend: local
`,
			wantErr: false,
			checkFunc: func(t *testing.T, s *Schema) {
				t.Helper()
				col := s.Collections["posts"]
				field := col.Fields["avatar"]
				if field.File.OnDelete != OnDeleteRestrict {
					t.Errorf("on_delete = %q, want %q", field.File.OnDelete, OnDeleteRestrict)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, err := Parse([]byte(tt.yaml))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" {
					if !contains(err.Error(), tt.errMsg) {
						t.Errorf("error = %q, want substring %q", err.Error(), tt.errMsg)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, schema)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
