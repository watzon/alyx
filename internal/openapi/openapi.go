package openapi

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/watzon/alyx/internal/schema"
)

const (
	typeString  = "string"
	typeInteger = "integer"
	typeNumber  = "number"
	typeBoolean = "boolean"
	typeObject  = "object"
)

const (
	defaultPasswordMinLength = 8
)

type Spec struct {
	OpenAPI    string                `json:"openapi"`
	Info       Info                  `json:"info"`
	Servers    []Server              `json:"servers,omitempty"`
	Paths      map[string]*PathItem  `json:"paths"`
	Components *Components           `json:"components,omitempty"`
	Tags       []Tag                 `json:"tags,omitempty"`
	Security   []SecurityRequirement `json:"security,omitempty"`
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
	Tags        []string              `json:"tags,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Description string                `json:"description,omitempty"`
	OperationID string                `json:"operationId,omitempty"`
	Parameters  []Parameter           `json:"parameters,omitempty"`
	RequestBody *RequestBody          `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []SecurityRequirement `json:"security,omitempty"`
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
	Schemas         map[string]*Schema         `json:"schemas,omitempty"`
	SecuritySchemes map[string]*SecurityScheme `json:"securitySchemes,omitempty"`
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

type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Description  string `json:"description,omitempty"`
}

type SecurityRequirement map[string][]string

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
			Schemas:         make(map[string]*Schema),
			SecuritySchemes: make(map[string]*SecurityScheme),
		},
		Security: []SecurityRequirement{
			{"bearerAuth": []string{}},
		},
	}

	if cfg.ServerURL != "" {
		spec.Servers = []Server{{URL: cfg.ServerURL}}
	}

	// Add JWT Bearer authentication security scheme
	spec.Components.SecuritySchemes["bearerAuth"] = &SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
		Description:  "JWT access token obtained from /api/auth/login or /api/auth/register",
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
			"error":      {Type: "string", Description: "Error message"},
			"code":       {Type: "string", Description: "Error code"},
			"details":    {Type: "object", Description: "Additional error details"},
			"request_id": {Type: "string", Description: "Request ID for tracing"},
			"timestamp":  {Type: "string", Format: "date-time", Description: "Error timestamp in RFC3339 format"},
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

	addHealthEndpoints(spec)
	addAuthEndpoints(spec)
	addFunctionEndpoints(spec)
	addAdminEndpoints(spec)

	return spec
}

func addHealthEndpoints(spec *Spec) {
	spec.Tags = append(spec.Tags, Tag{
		Name:        "health",
		Description: "Health and observability endpoints",
	})

	spec.Components.Schemas["HealthResponse"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"status":    {Type: "string", Enum: []string{"healthy", "degraded", "unhealthy"}},
			"version":   {Type: "string"},
			"uptime":    {Type: "string"},
			"timestamp": {Type: "string", Format: "date-time"},
			"components": {
				Type: "object",
				AdditionalProperties: &Schema{
					Type: "object",
					Properties: map[string]*Schema{
						"status":  {Type: "string", Enum: []string{"healthy", "degraded", "unhealthy"}},
						"latency": {Type: "string"},
						"message": {Type: "string"},
					},
				},
			},
		},
		Required: []string{"status", "version", "timestamp"},
	}

	spec.Paths["/health"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"health"},
			Summary:     "Comprehensive health check",
			Description: "Returns detailed health status including all components (database, realtime, functions)",
			OperationID: "health",
			Security:    []SecurityRequirement{},
			Responses: map[string]Response{
				"200": {Description: "Healthy", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/HealthResponse"}}}},
				"503": {Description: "Unhealthy", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/HealthResponse"}}}},
			},
		},
	}

	spec.Paths["/health/live"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"health"},
			Summary:     "Liveness probe",
			Description: "Simple liveness check for Kubernetes. Returns 200 if the server is running.",
			OperationID: "liveness",
			Security:    []SecurityRequirement{},
			Responses: map[string]Response{
				"200": {Description: "Server is alive", Content: map[string]MediaType{"application/json": {Schema: &Schema{Type: "object", Properties: map[string]*Schema{"status": {Type: "string"}}}}}},
			},
		},
	}

	spec.Paths["/health/ready"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"health"},
			Summary:     "Readiness probe",
			Description: "Readiness check for Kubernetes. Returns 200 if the server can handle requests (database connected).",
			OperationID: "readiness",
			Security:    []SecurityRequirement{},
			Responses: map[string]Response{
				"200": {Description: "Server is ready", Content: map[string]MediaType{"application/json": {Schema: &Schema{Type: "object", Properties: map[string]*Schema{"status": {Type: "string"}}}}}},
				"503": {Description: "Server is not ready", Content: map[string]MediaType{"application/json": {Schema: &Schema{Type: "object", Properties: map[string]*Schema{"status": {Type: "string"}, "reason": {Type: "string"}}}}}},
			},
		},
	}

	spec.Paths["/health/stats"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"health"},
			Summary:     "Runtime statistics",
			Description: "Returns detailed runtime statistics including memory usage, goroutines, database pool stats, and function pool stats.",
			OperationID: "stats",
			Security:    []SecurityRequirement{},
			Responses: map[string]Response{
				"200": {Description: "Runtime statistics", Content: map[string]MediaType{"application/json": {Schema: &Schema{Type: "object"}}}},
			},
		},
	}

	spec.Paths["/metrics"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"health"},
			Summary:     "Prometheus metrics",
			Description: "Returns metrics in Prometheus text format",
			OperationID: "metrics",
			Security:    []SecurityRequirement{},
			Responses: map[string]Response{
				"200": {Description: "Prometheus metrics", Content: map[string]MediaType{"text/plain": {Schema: &Schema{Type: "string"}}}},
			},
		},
	}
}

func addAuthEndpoints(spec *Spec) {
	spec.Tags = append(spec.Tags, Tag{
		Name:        "auth",
		Description: "Authentication endpoints",
	})

	spec.Components.Schemas["User"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"id":         {Type: "string", Format: "uuid"},
			"email":      {Type: "string", Format: "email"},
			"verified":   {Type: "boolean"},
			"role":       {Type: "string", Enum: []string{"user", "admin"}},
			"created_at": {Type: "string", Format: "date-time"},
			"updated_at": {Type: "string", Format: "date-time"},
			"metadata":   {Type: "object", AdditionalProperties: &Schema{}},
		},
		Required: []string{"id", "email", "verified", "role", "created_at", "updated_at"},
	}

	spec.Components.Schemas["TokenPair"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"access_token":  {Type: "string", Description: "JWT access token"},
			"refresh_token": {Type: "string", Description: "JWT refresh token"},
			"expires_at":    {Type: "string", Format: "date-time", Description: "Access token expiration time"},
			"token_type":    {Type: "string", Description: "Token type (Bearer)"},
		},
		Required: []string{"access_token", "refresh_token", "expires_at", "token_type"},
	}

	spec.Components.Schemas["AuthResponse"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"user":   {Ref: "#/components/schemas/User"},
			"tokens": {Ref: "#/components/schemas/TokenPair"},
		},
		Required: []string{"user", "tokens"},
	}

	spec.Components.Schemas["RegisterInput"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"email":    {Type: "string", Format: "email"},
			"password": {Type: "string", MinLength: intPtr(defaultPasswordMinLength)},
			"metadata": {Type: "object", AdditionalProperties: &Schema{}},
		},
		Required: []string{"email", "password"},
	}

	spec.Components.Schemas["LoginInput"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"email":    {Type: "string", Format: "email"},
			"password": {Type: "string"},
		},
		Required: []string{"email", "password"},
	}

	spec.Components.Schemas["RefreshInput"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"refresh_token": {Type: "string"},
		},
		Required: []string{"refresh_token"},
	}

	spec.Paths["/api/auth/register"] = &PathItem{
		Post: &Operation{
			Tags:        []string{"auth"},
			Summary:     "Register a new user",
			Description: "Create a new user account and return authentication tokens",
			OperationID: "register",
			Security:    []SecurityRequirement{},
			RequestBody: &RequestBody{
				Required:    true,
				Description: "User registration data",
				Content: map[string]MediaType{
					"application/json": {Schema: &Schema{Ref: "#/components/schemas/RegisterInput"}},
				},
			},
			Responses: map[string]Response{
				"201": {Description: "User registered successfully", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/AuthResponse"}}}},
				"400": {Description: "Invalid input or password too weak", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"403": {Description: "Registration is disabled", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"409": {Description: "User already exists", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
	}

	spec.Paths["/api/auth/login"] = &PathItem{
		Post: &Operation{
			Tags:        []string{"auth"},
			Summary:     "Login",
			Description: "Authenticate with email and password",
			OperationID: "login",
			Security:    []SecurityRequirement{},
			RequestBody: &RequestBody{
				Required:    true,
				Description: "Login credentials",
				Content: map[string]MediaType{
					"application/json": {Schema: &Schema{Ref: "#/components/schemas/LoginInput"}},
				},
			},
			Responses: map[string]Response{
				"200": {Description: "Login successful", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/AuthResponse"}}}},
				"401": {Description: "Invalid credentials", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"403": {Description: "Email not verified", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
	}

	spec.Paths["/api/auth/refresh"] = &PathItem{
		Post: &Operation{
			Tags:        []string{"auth"},
			Summary:     "Refresh tokens",
			Description: "Exchange a refresh token for new access and refresh tokens",
			OperationID: "refreshToken",
			RequestBody: &RequestBody{
				Required:    true,
				Description: "Refresh token",
				Content: map[string]MediaType{
					"application/json": {Schema: &Schema{Ref: "#/components/schemas/RefreshInput"}},
				},
			},
			Responses: map[string]Response{
				"200": {Description: "Tokens refreshed", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/AuthResponse"}}}},
				"401": {Description: "Invalid or expired refresh token", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
	}

	spec.Paths["/api/auth/logout"] = &PathItem{
		Post: &Operation{
			Tags:        []string{"auth"},
			Summary:     "Logout",
			Description: "Invalidate a refresh token",
			OperationID: "logout",
			RequestBody: &RequestBody{
				Required:    true,
				Description: "Refresh token to invalidate",
				Content: map[string]MediaType{
					"application/json": {Schema: &Schema{Ref: "#/components/schemas/RefreshInput"}},
				},
			},
			Responses: map[string]Response{
				"204": {Description: "Logged out successfully"},
				"400": {Description: "Invalid request", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
	}

	spec.Paths["/api/auth/me"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"auth"},
			Summary:     "Get current user",
			Description: "Get the currently authenticated user's information",
			OperationID: "getCurrentUser",
			Responses: map[string]Response{
				"200": {Description: "Current user", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/User"}}}},
				"401": {Description: "Not authenticated", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
	}

	spec.Components.Schemas["ProvidersResponse"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"providers": {Type: "array", Items: &Schema{Type: "string"}, Description: "List of enabled OAuth provider names"},
		},
		Required: []string{"providers"},
	}

	spec.Paths["/api/auth/providers"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"auth"},
			Summary:     "List OAuth providers",
			Description: "Get a list of enabled OAuth providers",
			OperationID: "listOAuthProviders",
			Security:    []SecurityRequirement{},
			Responses: map[string]Response{
				"200": {Description: "List of enabled OAuth providers", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/ProvidersResponse"}}}},
			},
		},
	}

	spec.Paths["/api/auth/oauth/{provider}"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"auth"},
			Summary:     "OAuth redirect",
			Description: "Initiates the OAuth flow by redirecting to the provider's authorization URL",
			OperationID: "oauthRedirect",
			Security:    []SecurityRequirement{},
			Parameters: []Parameter{
				{Name: "provider", In: "path", Required: true, Description: "OAuth provider name (e.g., github, google)", Schema: &Schema{Type: "string"}},
			},
			Responses: map[string]Response{
				"307": {Description: "Redirect to OAuth provider"},
				"400": {Description: "Provider name is required", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"404": {Description: "OAuth provider not found", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
	}

	spec.Paths["/api/auth/oauth/{provider}/callback"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"auth"},
			Summary:     "OAuth callback",
			Description: "Handles the OAuth callback from the provider and completes authentication",
			OperationID: "oauthCallback",
			Security:    []SecurityRequirement{},
			Parameters: []Parameter{
				{Name: "provider", In: "path", Required: true, Description: "OAuth provider name", Schema: &Schema{Type: "string"}},
				{Name: "code", In: "query", Required: true, Description: "Authorization code from provider", Schema: &Schema{Type: "string"}},
				{Name: "state", In: "query", Required: true, Description: "State parameter for CSRF protection", Schema: &Schema{Type: "string"}},
			},
			Responses: map[string]Response{
				"200": {Description: "OAuth login successful", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/AuthResponse"}}}},
				"400": {Description: "Invalid callback parameters or OAuth error", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"404": {Description: "OAuth provider not found", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"409": {Description: "OAuth account already linked to another user", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
	}
}

func intPtr(i int) *int {
	return &i
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

	setSchemaTypeAndFormat(f, s)
	applyFieldValidation(f, s)

	return s
}

func setSchemaTypeAndFormat(f *schema.Field, s *Schema) {
	switch f.Type {
	case schema.FieldTypeID:
		s.Type = typeString
		s.MinLength = intPtr(15)
		s.MaxLength = intPtr(15)
	case schema.FieldTypeUUID:
		s.Type = typeString
		s.Format = "uuid"
	case schema.FieldTypeString:
		s.Type = typeString
		s.MinLength = f.MinLength
		s.MaxLength = f.MaxLength
	case schema.FieldTypeText:
		s.Type = typeString
		s.MaxLength = f.MaxLength
	case schema.FieldTypeInt:
		s.Type = typeInteger
		s.Format = "int64"
	case schema.FieldTypeFloat:
		s.Type = typeNumber
		s.Format = "double"
	case schema.FieldTypeBool:
		s.Type = typeBoolean
	case schema.FieldTypeTimestamp:
		s.Type = typeString
		s.Format = "date-time"
	case schema.FieldTypeJSON:
		s.Type = typeObject
		s.AdditionalProperties = &Schema{}
	case schema.FieldTypeBlob:
		s.Type = typeString
		s.Format = "byte"
	default:
		s.Type = typeString
	}
}

func applyFieldValidation(f *schema.Field, s *Schema) {
	if f.Validate == nil {
		return
	}

	v := f.Validate
	if v.Pattern != "" {
		s.Pattern = v.Pattern
	}
	if len(v.Enum) > 0 {
		s.Enum = v.Enum
	}
	if v.Min != nil {
		s.Minimum = v.Min
	}
	if v.Max != nil {
		s.Maximum = v.Max
	}
	if v.MinLength != nil && s.MinLength == nil {
		s.MinLength = v.MinLength
	}
	if v.MaxLength != nil && s.MaxLength == nil {
		s.MaxLength = v.MaxLength
	}
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

func addFunctionEndpoints(spec *Spec) {
	spec.Tags = append(spec.Tags, Tag{
		Name:        "functions",
		Description: "Serverless function endpoints",
	})

	spec.Components.Schemas["FunctionInfo"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name":    {Type: "string"},
			"runtime": {Type: "string", Enum: []string{"node", "python", "go"}},
		},
		Required: []string{"name", "runtime"},
	}

	spec.Components.Schemas["FunctionInput"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"input": {Type: "object", AdditionalProperties: &Schema{}},
		},
	}

	spec.Components.Schemas["FunctionError"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"code":    {Type: "string"},
			"message": {Type: "string"},
			"details": {Type: "object", AdditionalProperties: &Schema{}},
		},
		Required: []string{"code", "message"},
	}

	spec.Components.Schemas["LogEntry"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"level":     {Type: "string", Enum: []string{"debug", "info", "warn", "error"}},
			"message":   {Type: "string"},
			"data":      {Type: "object", AdditionalProperties: &Schema{}},
			"timestamp": {Type: "string", Format: "date-time"},
		},
		Required: []string{"level", "message"},
	}

	spec.Components.Schemas["FunctionResponse"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"success":     {Type: "boolean"},
			"output":      {Type: "object", AdditionalProperties: &Schema{}},
			"error":       {Ref: "#/components/schemas/FunctionError"},
			"logs":        {Type: "array", Items: &Schema{Ref: "#/components/schemas/LogEntry"}},
			"duration_ms": {Type: "integer"},
		},
		Required: []string{"success", "duration_ms"},
	}

	spec.Components.Schemas["PoolStats"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"ready": {Type: "integer"},
			"busy":  {Type: "integer"},
			"total": {Type: "integer"},
		},
		Required: []string{"ready", "busy", "total"},
	}

	spec.Paths["/api/functions"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"functions"},
			Summary:     "List functions",
			Description: "List all discovered functions",
			OperationID: "listFunctions",
			Responses: map[string]Response{
				"200": {
					Description: "List of functions",
					Content: map[string]MediaType{
						"application/json": {Schema: &Schema{
							Type: "object",
							Properties: map[string]*Schema{
								"functions": {Type: "array", Items: &Schema{Ref: "#/components/schemas/FunctionInfo"}},
								"count":     {Type: "integer"},
							},
						}},
					},
				},
			},
		},
	}

	spec.Paths["/api/functions/stats"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"functions"},
			Summary:     "Get pool statistics",
			Description: "Get container pool statistics for all runtimes",
			OperationID: "getFunctionStats",
			Responses: map[string]Response{
				"200": {
					Description: "Pool statistics",
					Content: map[string]MediaType{
						"application/json": {Schema: &Schema{
							Type: "object",
							Properties: map[string]*Schema{
								"pools": {Type: "object", AdditionalProperties: &Schema{Ref: "#/components/schemas/PoolStats"}},
							},
						}},
					},
				},
			},
		},
	}

	spec.Paths["/api/functions/reload"] = &PathItem{
		Post: &Operation{
			Tags:        []string{"functions"},
			Summary:     "Reload functions",
			Description: "Rediscover and reload all functions",
			OperationID: "reloadFunctions",
			Responses: map[string]Response{
				"200": {
					Description: "Functions reloaded",
					Content: map[string]MediaType{
						"application/json": {Schema: &Schema{
							Type: "object",
							Properties: map[string]*Schema{
								"success": {Type: "boolean"},
								"count":   {Type: "integer"},
								"message": {Type: "string"},
							},
						}},
					},
				},
				"500": {Description: "Failed to reload functions", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
	}

	spec.Paths["/api/functions/{name}"] = &PathItem{
		Post: &Operation{
			Tags:        []string{"functions"},
			Summary:     "Invoke function",
			Description: "Invoke a serverless function by name",
			OperationID: "invokeFunction",
			Parameters: []Parameter{
				{Name: "name", In: "path", Required: true, Description: "Function name", Schema: &Schema{Type: "string"}},
			},
			RequestBody: &RequestBody{
				Description: "Function input data",
				Content: map[string]MediaType{
					"application/json": {Schema: &Schema{Ref: "#/components/schemas/FunctionInput"}},
				},
			},
			Responses: map[string]Response{
				"200": {Description: "Function executed", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/FunctionResponse"}}}},
				"400": {Description: "Invalid input", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"404": {Description: "Function not found", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"500": {Description: "Invocation error", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
	}
}

func (s *Spec) JSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

func addAdminEndpoints(spec *Spec) {
	spec.Tags = append(spec.Tags, Tag{
		Name:        "admin",
		Description: "Admin API endpoints (requires admin authentication)",
	})

	spec.Components.Schemas["AdminUser"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"id":         {Type: "string", Format: "uuid"},
			"email":      {Type: "string", Format: "email"},
			"verified":   {Type: "boolean"},
			"role":       {Type: "string", Enum: []string{"user", "admin"}},
			"created_at": {Type: "string", Format: "date-time"},
			"updated_at": {Type: "string", Format: "date-time"},
			"metadata":   {Type: "object", AdditionalProperties: &Schema{}},
		},
		Required: []string{"id", "email", "verified", "role", "created_at", "updated_at"},
	}

	spec.Components.Schemas["CreateUserInput"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"email":    {Type: "string", Format: "email"},
			"password": {Type: "string", MinLength: intPtr(defaultPasswordMinLength)},
			"verified": {Type: "boolean"},
			"role":     {Type: "string", Enum: []string{"user", "admin"}},
			"metadata": {Type: "object", AdditionalProperties: &Schema{}},
		},
		Required: []string{"email", "password"},
	}

	spec.Components.Schemas["UpdateUserInput"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"email":    {Type: "string", Format: "email"},
			"verified": {Type: "boolean"},
			"role":     {Type: "string", Enum: []string{"user", "admin"}},
			"metadata": {Type: "object", AdditionalProperties: &Schema{}},
		},
	}

	spec.Components.Schemas["SetPasswordInput"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"password": {Type: "string", MinLength: intPtr(defaultPasswordMinLength)},
		},
		Required: []string{"password"},
	}

	spec.Components.Schemas["UserListResponse"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"users": {Type: "array", Items: &Schema{Ref: "#/components/schemas/AdminUser"}},
			"total": {Type: "integer"},
		},
		Required: []string{"users", "total"},
	}

	spec.Paths["/api/admin/users"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"admin"},
			Summary:     "List users",
			Description: "Get a paginated list of all users",
			OperationID: "listUsers",
			Parameters: []Parameter{
				{Name: "limit", In: "query", Description: "Maximum users to return (default: 20, max: 100)", Schema: &Schema{Type: "integer"}},
				{Name: "offset", In: "query", Description: "Number of users to skip", Schema: &Schema{Type: "integer"}},
				{Name: "sort_by", In: "query", Description: "Field to sort by (id, email, verified, role, created_at, updated_at)", Schema: &Schema{Type: "string"}},
				{Name: "sort_dir", In: "query", Description: "Sort direction (asc, desc)", Schema: &Schema{Type: "string"}},
				{Name: "search", In: "query", Description: "Search in email", Schema: &Schema{Type: "string"}},
				{Name: "role", In: "query", Description: "Filter by role", Schema: &Schema{Type: "string"}},
			},
			Responses: map[string]Response{
				"200": {Description: "List of users", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/UserListResponse"}}}},
				"401": {Description: "Unauthorized", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
		Post: &Operation{
			Tags:        []string{"admin"},
			Summary:     "Create user",
			Description: "Create a new user with specified role",
			OperationID: "createUser",
			RequestBody: &RequestBody{
				Required:    true,
				Description: "User data",
				Content: map[string]MediaType{
					"application/json": {Schema: &Schema{Ref: "#/components/schemas/CreateUserInput"}},
				},
			},
			Responses: map[string]Response{
				"201": {Description: "User created", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/AdminUser"}}}},
				"400": {Description: "Invalid input", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"401": {Description: "Unauthorized", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"409": {Description: "User already exists", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
	}

	spec.Paths["/api/admin/users/{id}"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"admin"},
			Summary:     "Get user",
			Description: "Get a user by ID",
			OperationID: "getUser",
			Parameters: []Parameter{
				{Name: "id", In: "path", Required: true, Description: "User ID", Schema: &Schema{Type: "string", Format: "uuid"}},
			},
			Responses: map[string]Response{
				"200": {Description: "User details", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/AdminUser"}}}},
				"401": {Description: "Unauthorized", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"404": {Description: "User not found", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
		Patch: &Operation{
			Tags:        []string{"admin"},
			Summary:     "Update user",
			Description: "Update a user's information",
			OperationID: "updateUser",
			Parameters: []Parameter{
				{Name: "id", In: "path", Required: true, Description: "User ID", Schema: &Schema{Type: "string", Format: "uuid"}},
			},
			RequestBody: &RequestBody{
				Required:    true,
				Description: "Fields to update",
				Content: map[string]MediaType{
					"application/json": {Schema: &Schema{Ref: "#/components/schemas/UpdateUserInput"}},
				},
			},
			Responses: map[string]Response{
				"200": {Description: "User updated", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/AdminUser"}}}},
				"400": {Description: "Invalid input", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"401": {Description: "Unauthorized", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"404": {Description: "User not found", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"409": {Description: "Email already in use", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
		Delete: &Operation{
			Tags:        []string{"admin"},
			Summary:     "Delete user",
			Description: "Delete a user by ID",
			OperationID: "deleteUser",
			Parameters: []Parameter{
				{Name: "id", In: "path", Required: true, Description: "User ID", Schema: &Schema{Type: "string", Format: "uuid"}},
			},
			Responses: map[string]Response{
				"200": {Description: "User deleted"},
				"401": {Description: "Unauthorized", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"404": {Description: "User not found", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
	}

	spec.Paths["/api/admin/users/{id}/password"] = &PathItem{
		Post: &Operation{
			Tags:        []string{"admin"},
			Summary:     "Set user password",
			Description: "Set a new password for a user",
			OperationID: "setUserPassword",
			Parameters: []Parameter{
				{Name: "id", In: "path", Required: true, Description: "User ID", Schema: &Schema{Type: "string", Format: "uuid"}},
			},
			RequestBody: &RequestBody{
				Required:    true,
				Description: "New password",
				Content: map[string]MediaType{
					"application/json": {Schema: &Schema{Ref: "#/components/schemas/SetPasswordInput"}},
				},
			},
			Responses: map[string]Response{
				"200": {Description: "Password set"},
				"400": {Description: "Invalid password", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"401": {Description: "Unauthorized", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
				"404": {Description: "User not found", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/Error"}}}},
			},
		},
	}

	spec.Components.Schemas["RequestLogEntry"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"id":          {Type: "string", Description: "Request ID"},
			"timestamp":   {Type: "string", Format: "date-time", Description: "Request timestamp"},
			"method":      {Type: "string", Description: "HTTP method"},
			"path":        {Type: "string", Description: "Request path"},
			"query":       {Type: "string", Description: "Query string"},
			"status":      {Type: "integer", Description: "HTTP status code"},
			"duration":    {Type: "integer", Description: "Duration in nanoseconds"},
			"duration_ms": {Type: "number", Description: "Duration in milliseconds"},
			"bytes_in":    {Type: "integer", Description: "Request body size"},
			"bytes_out":   {Type: "integer", Description: "Response body size"},
			"client_ip":   {Type: "string", Description: "Client IP address"},
			"user_agent":  {Type: "string", Description: "User agent string"},
			"user_id":     {Type: "string", Description: "Authenticated user ID"},
			"error":       {Type: "string", Description: "Error message if any"},
			"error_code":  {Type: "string", Description: "Error code if any"},
		},
	}

	spec.Components.Schemas["RequestLogListResponse"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"entries": {Type: "array", Items: &Schema{Ref: "#/components/schemas/RequestLogEntry"}},
			"total":   {Type: "integer", Description: "Total matching entries"},
			"limit":   {Type: "integer", Description: "Page size"},
			"offset":  {Type: "integer", Description: "Page offset"},
		},
		Required: []string{"entries", "total", "limit", "offset"},
	}

	spec.Components.Schemas["RequestLogStats"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"capacity": {Type: "integer", Description: "Maximum log capacity"},
			"count":    {Type: "integer", Description: "Current entry count"},
		},
		Required: []string{"capacity", "count"},
	}

	spec.Paths["/api/admin/logs"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"admin"},
			Summary:     "List request logs",
			Description: "Get a paginated list of HTTP request logs",
			OperationID: "listRequestLogs",
			Parameters: []Parameter{
				{Name: "limit", In: "query", Description: "Maximum entries to return (default: 100, max: 1000)", Schema: &Schema{Type: "integer"}},
				{Name: "offset", In: "query", Description: "Number of entries to skip", Schema: &Schema{Type: "integer"}},
				{Name: "method", In: "query", Description: "Filter by HTTP method", Schema: &Schema{Type: "string"}},
				{Name: "path", In: "query", Description: "Filter by exact path", Schema: &Schema{Type: "string"}},
				{Name: "status", In: "query", Description: "Filter by exact status code", Schema: &Schema{Type: "integer"}},
				{Name: "min_status", In: "query", Description: "Filter by minimum status code", Schema: &Schema{Type: "integer"}},
				{Name: "max_status", In: "query", Description: "Filter by maximum status code", Schema: &Schema{Type: "integer"}},
				{Name: "user_id", In: "query", Description: "Filter by user ID", Schema: &Schema{Type: "string"}},
				{Name: "since", In: "query", Description: "Filter by start time (RFC3339)", Schema: &Schema{Type: "string", Format: "date-time"}},
				{Name: "until", In: "query", Description: "Filter by end time (RFC3339)", Schema: &Schema{Type: "string", Format: "date-time"}},
			},
			Responses: map[string]Response{
				"200": {Description: "List of request logs", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/RequestLogListResponse"}}}},
			},
		},
	}

	spec.Paths["/api/admin/logs/stats"] = &PathItem{
		Get: &Operation{
			Tags:        []string{"admin"},
			Summary:     "Get log store stats",
			Description: "Get statistics about the request log store",
			OperationID: "getRequestLogStats",
			Responses: map[string]Response{
				"200": {Description: "Log store statistics", Content: map[string]MediaType{"application/json": {Schema: &Schema{Ref: "#/components/schemas/RequestLogStats"}}}},
			},
		},
	}

	spec.Paths["/api/admin/logs/clear"] = &PathItem{
		Post: &Operation{
			Tags:        []string{"admin"},
			Summary:     "Clear request logs",
			Description: "Clear all request logs from the store",
			OperationID: "clearRequestLogs",
			Responses: map[string]Response{
				"200": {Description: "Logs cleared"},
			},
		},
	}
}
