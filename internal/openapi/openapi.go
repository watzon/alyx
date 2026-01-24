package openapi

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/watzon/alyx/internal/schema"
)

type Spec struct {
	OpenAPI    string               `json:"openapi"`
	Info       Info                 `json:"info"`
	Servers    []Server             `json:"servers,omitempty"`
	Paths      map[string]*PathItem `json:"paths"`
	Components *Components          `json:"components,omitempty"`
	Tags       []Tag                `json:"tags,omitempty"`
}

type Info struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version"`
	Contact     *Contact `json:"contact,omitempty"`
	License     *License `json:"license,omitempty"`
}

type Contact struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

type License struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Put    *Operation `json:"put,omitempty"`
	Patch  *Operation `json:"patch,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
}

type Operation struct {
	Tags        []string            `json:"tags,omitempty"`
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	OperationID string              `json:"operationId,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
}

type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Content     map[string]MediaType `json:"content"`
}

type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}

type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

type Components struct {
	Schemas map[string]*Schema `json:"schemas,omitempty"`
}

type Schema struct {
	Type                 string             `json:"type,omitempty"`
	Format               string             `json:"format,omitempty"`
	Description          string             `json:"description,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Ref                  string             `json:"$ref,omitempty"`
	Nullable             bool               `json:"nullable,omitempty"`
	Enum                 []string           `json:"enum,omitempty"`
	MinLength            *int               `json:"minLength,omitempty"`
	MaxLength            *int               `json:"maxLength,omitempty"`
	Minimum              *float64           `json:"minimum,omitempty"`
	Maximum              *float64           `json:"maximum,omitempty"`
	Pattern              string             `json:"pattern,omitempty"`
	AdditionalProperties *Schema            `json:"additionalProperties,omitempty"`
}

type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type GeneratorConfig struct {
	Title       string
	Description string
	Version     string
	ServerURL   string
}

func Generate(s *schema.Schema, cfg GeneratorConfig) *Spec {
	spec := &Spec{
		OpenAPI: "3.1.0",
		Info: Info{
			Title:       cfg.Title,
			Description: cfg.Description,
			Version:     cfg.Version,
		},
		Paths: make(map[string]*PathItem),
		Components: &Components{
			Schemas: make(map[string]*Schema),
		},
	}

	if cfg.ServerURL != "" {
		spec.Servers = []Server{{URL: cfg.ServerURL}}
	}

	collectionNames := make([]string, 0, len(s.Collections))
	for name := range s.Collections {
		collectionNames = append(collectionNames, name)
	}
	sort.Strings(collectionNames)

	for _, name := range collectionNames {
		col := s.Collections[name]
		spec.Tags = append(spec.Tags, Tag{
			Name:        name,
			Description: fmt.Sprintf("Operations for %s collection", name),
		})

		spec.Components.Schemas[name] = generateSchema(col)
		spec.Components.Schemas[name+"Input"] = generateInputSchema(col)

		listPath := fmt.Sprintf("/api/collections/%s", name)
		itemPath := fmt.Sprintf("/api/collections/%s/{id}", name)

		spec.Paths[listPath] = &PathItem{
			Get:  generateListOperation(name, col),
			Post: generateCreateOperation(name),
		}

		spec.Paths[itemPath] = &PathItem{
			Get:    generateGetOperation(name),
			Patch:  generateUpdateOperation(name),
			Delete: generateDeleteOperation(name),
		}
	}

	spec.Components.Schemas["Error"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"error":   {Type: "string", Description: "Error message"},
			"code":    {Type: "string", Description: "Error code"},
			"details": {Type: "object", Description: "Additional error details"},
		},
		Required: []string{"error"},
	}

	spec.Components.Schemas["ListResponse"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"docs":   {Type: "array", Items: &Schema{Type: "object"}},
			"total":  {Type: "integer", Description: "Total number of documents"},
			"limit":  {Type: "integer", Description: "Limit used in query"},
			"offset": {Type: "integer", Description: "Offset used in query"},
		},
		Required: []string{"docs", "total"},
	}

	return spec
}

func generateSchema(col *schema.Collection) *Schema {
	s := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
	}

	for _, field := range col.OrderedFields() {
		if field.Internal {
			continue
		}

		prop := fieldToSchema(field)
		s.Properties[field.Name] = prop

		if !field.Nullable && !field.HasDefault() && !field.Primary {
			s.Required = append(s.Required, field.Name)
		}
	}

	return s
}

func generateInputSchema(col *schema.Collection) *Schema {
	s := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
	}

	for _, field := range col.OrderedFields() {
		if field.Internal || field.Primary || field.IsTimestampNow() || field.IsAutoUpdateTimestamp() {
			continue
		}

		prop := fieldToSchema(field)
		s.Properties[field.Name] = prop

		if !field.Nullable && !field.HasDefault() {
			s.Required = append(s.Required, field.Name)
		}
	}

	return s
}

func fieldToSchema(f *schema.Field) *Schema {
	s := &Schema{
		Nullable: f.Nullable,
	}

	switch f.Type {
	case schema.FieldTypeUUID:
		s.Type = "string"
		s.Format = "uuid"
	case schema.FieldTypeString:
		s.Type = "string"
		s.MinLength = f.MinLength
		s.MaxLength = f.MaxLength
	case schema.FieldTypeText:
		s.Type = "string"
		s.MaxLength = f.MaxLength
	case schema.FieldTypeInt:
		s.Type = "integer"
		s.Format = "int64"
	case schema.FieldTypeFloat:
		s.Type = "number"
		s.Format = "double"
	case schema.FieldTypeBool:
		s.Type = "boolean"
	case schema.FieldTypeTimestamp:
		s.Type = "string"
		s.Format = "date-time"
	case schema.FieldTypeJSON:
		s.Type = "object"
		s.AdditionalProperties = &Schema{}
	case schema.FieldTypeBlob:
		s.Type = "string"
		s.Format = "byte"
	default:
		s.Type = "string"
	}

	if f.Validate != nil {
		if f.Validate.Pattern != "" {
			s.Pattern = f.Validate.Pattern
		}
		if len(f.Validate.Enum) > 0 {
			s.Enum = f.Validate.Enum
		}
		if f.Validate.Min != nil {
			s.Minimum = f.Validate.Min
		}
		if f.Validate.Max != nil {
			s.Maximum = f.Validate.Max
		}
		if f.Validate.MinLength != nil && s.MinLength == nil {
			s.MinLength = f.Validate.MinLength
		}
		if f.Validate.MaxLength != nil && s.MaxLength == nil {
			s.MaxLength = f.Validate.MaxLength
		}
	}

	return s
}

func generateListOperation(name string, col *schema.Collection) *Operation {
	return &Operation{
		Tags:        []string{name},
		Summary:     fmt.Sprintf("List %s", name),
		Description: fmt.Sprintf("Retrieve a paginated list of %s documents", name),
		OperationID: fmt.Sprintf("list%s", capitalize(name)),
		Parameters: []Parameter{
			{Name: "limit", In: "query", Description: "Maximum number of documents to return (default: 100, max: 1000)", Schema: &Schema{Type: "integer"}},
			{Name: "offset", In: "query", Description: "Number of documents to skip", Schema: &Schema{Type: "integer"}},
			{Name: "sort", In: "query", Description: "Sort order (e.g., '-created_at' for descending)", Schema: &Schema{Type: "string"}},
			{Name: "filter", In: "query", Description: "Filter expression (e.g., 'field:eq:value')", Schema: &Schema{Type: "array", Items: &Schema{Type: "string"}}},
			{Name: "expand", In: "query", Description: "Relations to expand", Schema: &Schema{Type: "string"}},
		},
		Responses: map[string]Response{
			"200": {
				Description: "Successful response",
				Content: map[string]MediaType{
					"application/json": {Schema: &Schema{
						Type: "object",
						Properties: map[string]*Schema{
							"docs":   {Type: "array", Items: &Schema{Ref: "#/components/schemas/" + name}},
							"total":  {Type: "integer"},
							"limit":  {Type: "integer"},
							"offset": {Type: "integer"},
						},
					}},
				},
			},
			"400": {Description: "Invalid query parameters", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			"500": {Description: "Internal server error", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
		},
	}
}

func generateGetOperation(name string) *Operation {
	return &Operation{
		Tags:        []string{name},
		Summary:     fmt.Sprintf("Get %s by ID", name),
		Description: fmt.Sprintf("Retrieve a single %s document by its ID", name),
		OperationID: fmt.Sprintf("get%s", capitalize(name)),
		Parameters: []Parameter{
			{Name: "id", In: "path", Required: true, Description: "Document ID", Schema: &Schema{Type: "string"}},
		},
		Responses: map[string]Response{
			"200": {Description: "Successful response", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/" + name}}}},
			"404": {Description: "Document not found", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			"500": {Description: "Internal server error", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
		},
	}
}

func generateCreateOperation(name string) *Operation {
	return &Operation{
		Tags:        []string{name},
		Summary:     fmt.Sprintf("Create %s", name),
		Description: fmt.Sprintf("Create a new %s document", name),
		OperationID: fmt.Sprintf("create%s", capitalize(name)),
		RequestBody: &RequestBody{
			Required:    true,
			Description: fmt.Sprintf("The %s document to create", name),
			Content: map[string]MediaType{
				"application/json": {Schema: &Schema{Ref: "#/components/schemas/" + name + "Input"}},
			},
		},
		Responses: map[string]Response{
			"201": {Description: "Document created", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/" + name}}}},
			"400": {Description: "Invalid request body", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			"500": {Description: "Internal server error", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
		},
	}
}

func generateUpdateOperation(name string) *Operation {
	return &Operation{
		Tags:        []string{name},
		Summary:     fmt.Sprintf("Update %s", name),
		Description: fmt.Sprintf("Update an existing %s document", name),
		OperationID: fmt.Sprintf("update%s", capitalize(name)),
		Parameters: []Parameter{
			{Name: "id", In: "path", Required: true, Description: "Document ID", Schema: &Schema{Type: "string"}},
		},
		RequestBody: &RequestBody{
			Required:    true,
			Description: "Fields to update",
			Content: map[string]MediaType{
				"application/json": {Schema: &Schema{Ref: "#/components/schemas/" + name + "Input"}},
			},
		},
		Responses: map[string]Response{
			"200": {Description: "Document updated", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/" + name}}}},
			"400": {Description: "Invalid request body", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			"404": {Description: "Document not found", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			"500": {Description: "Internal server error", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
		},
	}
}

func generateDeleteOperation(name string) *Operation {
	return &Operation{
		Tags:        []string{name},
		Summary:     fmt.Sprintf("Delete %s", name),
		Description: fmt.Sprintf("Delete a %s document", name),
		OperationID: fmt.Sprintf("delete%s", capitalize(name)),
		Parameters: []Parameter{
			{Name: "id", In: "path", Required: true, Description: "Document ID", Schema: &Schema{Type: "string"}},
		},
		Responses: map[string]Response{
			"204": {Description: "Document deleted"},
			"404": {Description: "Document not found", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			"500": {Description: "Internal server error", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
		},
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}

func (s *Spec) JSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}
