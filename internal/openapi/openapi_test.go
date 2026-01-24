package openapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/watzon/alyx/internal/schema"
)

func TestGenerate(t *testing.T) {
	schemaYAML := `
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
        validate:
          format: email
      name:
        type: string
        maxLength: 100
        nullable: true
      role:
        type: string
        default: "user"
        validate:
          enum: [user, admin]
      active:
        type: bool
        default: true
      created_at:
        type: timestamp
        default: now

  posts:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      title:
        type: string
        minLength: 1
        maxLength: 200
      author_id:
        type: uuid
        references: users.id
      created_at:
        type: timestamp
        default: now
`

	s, err := schema.Parse([]byte(schemaYAML))
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}

	spec := Generate(s, GeneratorConfig{
		Title:       "Test API",
		Description: "Test description",
		Version:     "1.0.0",
		ServerURL:   "http://localhost:8090",
	})

	if spec.OpenAPI != "3.1.0" {
		t.Errorf("expected OpenAPI 3.1.0, got %s", spec.OpenAPI)
	}

	if spec.Info.Title != "Test API" {
		t.Errorf("expected title 'Test API', got %s", spec.Info.Title)
	}

	if len(spec.Servers) != 1 || spec.Servers[0].URL != "http://localhost:8090" {
		t.Error("expected server URL")
	}

	if len(spec.Tags) != 5 {
		t.Errorf("expected 5 tags (2 collections + auth + functions + admin), got %d", len(spec.Tags))
	}

	usersPath := "/api/collections/users"
	if _, ok := spec.Paths[usersPath]; !ok {
		t.Errorf("expected path %s", usersPath)
	}

	usersItemPath := "/api/collections/users/{id}"
	if _, ok := spec.Paths[usersItemPath]; !ok {
		t.Errorf("expected path %s", usersItemPath)
	}

	usersSchema, ok := spec.Components.Schemas["users"]
	if !ok {
		t.Fatal("expected users schema")
	}

	if usersSchema.Properties["email"].Type != "string" {
		t.Error("expected email to be string")
	}

	if usersSchema.Properties["active"].Type != "boolean" {
		t.Error("expected active to be boolean")
	}

	if usersSchema.Properties["created_at"].Format != "date-time" {
		t.Error("expected created_at to have date-time format")
	}

	roleSchema := usersSchema.Properties["role"]
	if len(roleSchema.Enum) != 2 {
		t.Errorf("expected role to have 2 enum values, got %d", len(roleSchema.Enum))
	}

	postsSchema := spec.Components.Schemas["posts"]
	titleSchema := postsSchema.Properties["title"]
	if titleSchema.MinLength == nil || *titleSchema.MinLength != 1 {
		t.Error("expected title minLength 1")
	}
	if titleSchema.MaxLength == nil || *titleSchema.MaxLength != 200 {
		t.Error("expected title maxLength 200")
	}

	usersInput, ok := spec.Components.Schemas["usersInput"]
	if !ok {
		t.Fatal("expected usersInput schema")
	}

	if _, hasID := usersInput.Properties["id"]; hasID {
		t.Error("input schema should not include id")
	}

	if _, hasCreatedAt := usersInput.Properties["created_at"]; hasCreatedAt {
		t.Error("input schema should not include auto timestamp fields")
	}
}

func TestGenerateJSON(t *testing.T) {
	schemaYAML := `
version: 1
collections:
  items:
    fields:
      id:
        type: uuid
        primary: true
      name:
        type: string
`
	s, _ := schema.Parse([]byte(schemaYAML))
	spec := Generate(s, GeneratorConfig{Title: "Test"})

	data, err := spec.JSON()
	if err != nil {
		t.Fatalf("failed to generate JSON: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if parsed["openapi"] != "3.1.0" {
		t.Error("expected openapi version in JSON")
	}
}

func TestGenerateOperations(t *testing.T) {
	schemaYAML := `
version: 1
collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
`
	s, _ := schema.Parse([]byte(schemaYAML))
	spec := Generate(s, GeneratorConfig{Title: "Test"})

	listPath := spec.Paths["/api/collections/users"]
	if listPath.Get == nil {
		t.Error("expected GET operation for list")
	}
	if listPath.Post == nil {
		t.Error("expected POST operation for create")
	}
	if listPath.Get.OperationID != "listUsers" {
		t.Errorf("expected operationId 'listUsers', got %s", listPath.Get.OperationID)
	}

	itemPath := spec.Paths["/api/collections/users/{id}"]
	if itemPath.Get == nil {
		t.Error("expected GET operation for get")
	}
	if itemPath.Patch == nil {
		t.Error("expected PATCH operation for update")
	}
	if itemPath.Delete == nil {
		t.Error("expected DELETE operation for delete")
	}

	hasLimitParam := false
	for _, p := range listPath.Get.Parameters {
		if p.Name == "limit" {
			hasLimitParam = true
			break
		}
	}
	if !hasLimitParam {
		t.Error("expected limit parameter on list operation")
	}

	hasIDParam := false
	for _, p := range itemPath.Get.Parameters {
		if p.Name == "id" && p.In == "path" && p.Required {
			hasIDParam = true
			break
		}
	}
	if !hasIDParam {
		t.Error("expected id path parameter on get operation")
	}

	if itemPath.Delete.Responses["204"].Description == "" {
		t.Error("expected 204 response on delete")
	}
}

func TestCapitalize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"users", "Users"},
		{"posts", "Posts"},
		{"", ""},
		{"a", "A"},
	}

	for _, tt := range tests {
		if got := capitalize(tt.input); got != tt.expected {
			t.Errorf("capitalize(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFieldTypeMappings(t *testing.T) {
	tests := []struct {
		fieldType schema.FieldType
		oaType    string
		oaFormat  string
	}{
		{schema.FieldTypeUUID, "string", "uuid"},
		{schema.FieldTypeString, "string", ""},
		{schema.FieldTypeText, "string", ""},
		{schema.FieldTypeInt, "integer", "int64"},
		{schema.FieldTypeFloat, "number", "double"},
		{schema.FieldTypeBool, "boolean", ""},
		{schema.FieldTypeTimestamp, "string", "date-time"},
		{schema.FieldTypeJSON, "object", ""},
		{schema.FieldTypeBlob, "string", "byte"},
	}

	for _, tt := range tests {
		t.Run(string(tt.fieldType), func(t *testing.T) {
			f := &schema.Field{Type: tt.fieldType}
			s := fieldToSchema(f)

			if s.Type != tt.oaType {
				t.Errorf("expected type %q, got %q", tt.oaType, s.Type)
			}
			if s.Format != tt.oaFormat {
				t.Errorf("expected format %q, got %q", tt.oaFormat, s.Format)
			}
		})
	}
}

func TestErrorSchema(t *testing.T) {
	schemaYAML := `
version: 1
collections:
  items:
    fields:
      id:
        type: uuid
        primary: true
`
	s, _ := schema.Parse([]byte(schemaYAML))
	spec := Generate(s, GeneratorConfig{Title: "Test"})

	errSchema, ok := spec.Components.Schemas["Error"]
	if !ok {
		t.Fatal("expected Error schema")
	}

	if errSchema.Properties["error"] == nil {
		t.Error("expected error property")
	}

	if !strings.Contains(strings.Join(errSchema.Required, ","), "error") {
		t.Error("expected error to be required")
	}
}
