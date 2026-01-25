package typescript

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/watzon/alyx/internal/openapi"
	"github.com/watzon/alyx/internal/schema"
)

// Config holds configuration for TypeScript SDK generation.
type Config struct {
	OutputDir string
	ServerURL string
}

// Generator generates TypeScript SDK from OpenAPI spec and schema.
type Generator struct {
	config Config
}

// NewGenerator creates a new TypeScript SDK generator.
func NewGenerator(cfg Config) *Generator {
	return &Generator{
		config: cfg,
	}
}

// Generate generates the complete TypeScript SDK.
func (g *Generator) Generate(spec *openapi.Spec, s *schema.Schema) error {
	// Create output directory structure
	if err := g.createDirectories(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	// Extract collection names
	collections := g.extractCollections(s)

	// Generate package.json and tsconfig.json
	if err := g.generatePackageJSON(); err != nil {
		return fmt.Errorf("generating package.json: %w", err)
	}

	if err := g.generateTSConfig(); err != nil {
		return fmt.Errorf("generating tsconfig.json: %w", err)
	}

	// Generate types
	if err := g.generateTypes(spec, collections); err != nil {
		return fmt.Errorf("generating types: %w", err)
	}

	// Generate resources
	if err := g.generateResources(collections); err != nil {
		return fmt.Errorf("generating resources: %w", err)
	}

	// Generate client
	if err := g.generateClient(collections); err != nil {
		return fmt.Errorf("generating client: %w", err)
	}

	// Generate context helper
	if err := g.generateContext(); err != nil {
		return fmt.Errorf("generating context: %w", err)
	}

	// Generate index
	if err := g.generateIndex(); err != nil {
		return fmt.Errorf("generating index: %w", err)
	}

	return nil
}

func (g *Generator) createDirectories() error {
	dirs := []string{
		g.config.OutputDir,
		filepath.Join(g.config.OutputDir, "types"),
		filepath.Join(g.config.OutputDir, "resources"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	return nil
}

func (g *Generator) extractCollections(s *schema.Schema) []string {
	collections := make([]string, 0, len(s.Collections))
	for name := range s.Collections {
		collections = append(collections, name)
	}
	sort.Strings(collections)
	return collections
}

func (g *Generator) generatePackageJSON() error {
	content := `{
  "name": "alyx-sdk",
  "version": "1.0.0",
  "description": "TypeScript SDK for Alyx Backend-as-a-Service",
  "main": "index.ts",
  "types": "index.ts",
  "scripts": {
    "build": "tsc"
  },
  "dependencies": {},
  "devDependencies": {
    "@types/node": "^20.0.0",
    "typescript": "^5.3.0"
  }
}
`
	return os.WriteFile(filepath.Join(g.config.OutputDir, "package.json"), []byte(content), 0600)
}

func (g *Generator) generateTSConfig() error {
	content := `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "lib": ["ES2020", "DOM"],
    "declaration": true,
    "outDir": "./dist",
    "rootDir": "./",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["**/*.ts"],
  "exclude": ["node_modules", "dist"]
}
`
	return os.WriteFile(filepath.Join(g.config.OutputDir, "tsconfig.json"), []byte(content), 0600)
}

func (g *Generator) generateTypes(spec *openapi.Spec, collections []string) error {
	// Generate collection types
	if err := g.generateCollectionTypes(spec, collections); err != nil {
		return err
	}

	// Generate auth types
	if err := g.generateAuthTypes(); err != nil {
		return err
	}

	// Generate function types
	if err := g.generateFunctionTypes(); err != nil {
		return err
	}

	// Generate event types
	return g.generateEventTypes()
}

func (g *Generator) generateCollectionTypes(spec *openapi.Spec, collections []string) error {
	var sb strings.Builder

	sb.WriteString("// Auto-generated collection types\n\n")

	// Generate types for each collection
	for _, name := range collections {
		// Find schema in spec
		collectionSchema := spec.Components.Schemas[name]
		inputSchema := spec.Components.Schemas[name+"Input"]

		if collectionSchema != nil {
			sb.WriteString(fmt.Sprintf("export interface %s {\n", capitalize(name)))
			g.writeSchemaProperties(&sb, collectionSchema, "  ")
			sb.WriteString("}\n\n")
		}

		if inputSchema != nil {
			sb.WriteString(fmt.Sprintf("export interface %sInput {\n", capitalize(name)))
			g.writeSchemaProperties(&sb, inputSchema, "  ")
			sb.WriteString("}\n\n")
		}
	}

	// Add list response type
	sb.WriteString("export interface ListResponse<T> {\n")
	sb.WriteString("  docs: T[];\n")
	sb.WriteString("  total: number;\n")
	sb.WriteString("  limit: number;\n")
	sb.WriteString("  offset: number;\n")
	sb.WriteString("}\n")

	return os.WriteFile(filepath.Join(g.config.OutputDir, "types", "collections.ts"), []byte(sb.String()), 0600)
}

func (g *Generator) writeSchemaProperties(sb *strings.Builder, s *openapi.Schema, indent string) {
	if s.Properties == nil {
		return
	}

	// Sort properties for consistent output
	props := make([]string, 0, len(s.Properties))
	for name := range s.Properties {
		props = append(props, name)
	}
	sort.Strings(props)

	for _, name := range props {
		prop := s.Properties[name]
		optional := !contains(s.Required, name)

		tsType := g.schemaToTSType(prop)
		optionalMarker := ""
		if optional {
			optionalMarker = "?"
		}

		sb.WriteString(fmt.Sprintf("%s%s%s: %s;\n", indent, name, optionalMarker, tsType))
	}
}

const (
	tsTypeNumber = "number"
)

func (g *Generator) schemaToTSType(s *openapi.Schema) string {
	if s.Ref != "" {
		// Extract type name from $ref
		parts := strings.Split(s.Ref, "/")
		return parts[len(parts)-1]
	}

	switch s.Type {
	case "string":
		if len(s.Enum) > 0 {
			return strings.Join(quoteStrings(s.Enum), " | ")
		}
		return "string"
	case "integer":
		return tsTypeNumber
	case "number":
		return tsTypeNumber
	case "boolean":
		return "boolean"
	case "array":
		if s.Items != nil {
			return g.schemaToTSType(s.Items) + "[]"
		}
		return "any[]"
	case "object":
		if s.AdditionalProperties != nil {
			return "Record<string, any>"
		}
		return "object"
	default:
		return "any"
	}
}

func (g *Generator) generateAuthTypes() error {
	content := `// Auto-generated auth types

export interface User {
  id: string;
  email: string;
  verified: boolean;
  role: 'user' | 'admin';
  created_at: string;
  updated_at: string;
  metadata?: Record<string, any>;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  expires_at: string;
  token_type: string;
}

export interface AuthResponse {
  user: User;
  tokens: TokenPair;
}

export interface RegisterInput {
  email: string;
  password: string;
  metadata?: Record<string, any>;
}

export interface LoginInput {
  email: string;
  password: string;
}

export interface RefreshInput {
  refresh_token: string;
}
`
	return os.WriteFile(filepath.Join(g.config.OutputDir, "types", "auth.ts"), []byte(content), 0600)
}

func (g *Generator) generateFunctionTypes() error {
	content := `// Auto-generated function types

export interface FunctionInfo {
  name: string;
  runtime: 'node' | 'python' | 'go';
}

export interface FunctionInput {
  input?: Record<string, any>;
}

export interface FunctionError {
  code: string;
  message: string;
  details?: Record<string, any>;
}

export interface LogEntry {
  level: 'debug' | 'info' | 'warn' | 'error';
  message: string;
  data?: Record<string, any>;
  timestamp?: string;
}

export interface FunctionResponse {
  success: boolean;
  output?: Record<string, any>;
  error?: FunctionError;
  logs?: LogEntry[];
  duration_ms: number;
}
`
	return os.WriteFile(filepath.Join(g.config.OutputDir, "types", "functions.ts"), []byte(content), 0600)
}

func (g *Generator) generateEventTypes() error {
	content := `// Auto-generated event types

export type EventType = 'http' | 'database' | 'auth' | 'schedule' | 'webhook' | 'custom';

export interface EventPayload {
  [key: string]: any;
}

export interface EventMetadata {
  user_id?: string;
  ip_address?: string;
  user_agent?: string;
  extra?: Record<string, any>;
}

export interface Event {
  id: string;
  type: EventType;
  source: string;
  action: string;
  payload: EventPayload;
  metadata?: EventMetadata;
  status: 'pending' | 'processing' | 'completed' | 'failed';
  created_at: string;
  process_at?: string;
  processed_at?: string;
}

// Hook event payload types
export interface DatabaseEventPayload {
  document: Record<string, any>;
  previous_document?: Record<string, any>;
  action: 'insert' | 'update' | 'delete';
  collection: string;
  changed_fields?: string[];
}

export interface AuthEventPayload {
  user: {
    id: string;
    email: string;
    verified: boolean;
    role: string;
    created_at: string;
  };
  action: 'signup' | 'login' | 'logout' | 'password_reset' | 'email_verify';
  metadata?: {
    ip_address?: string;
    user_agent?: string;
  };
}

export interface WebhookEventPayload {
  method: string;
  path: string;
  headers: Record<string, string>;
  body: string;
  query: Record<string, string>;
  verified: boolean;
  webhook_id: string;
  verification_error?: string;
}

export interface ScheduleEventPayload {
  schedule_id: string;
  schedule_name: string;
  function_id: string;
  input?: Record<string, any>;
}
`
	return os.WriteFile(filepath.Join(g.config.OutputDir, "types", "events.ts"), []byte(content), 0600)
}

func (g *Generator) generateResources(collections []string) error {
	// Generate collections resource
	if err := g.generateCollectionsResource(collections); err != nil {
		return err
	}

	// Generate auth resource
	if err := g.generateAuthResource(); err != nil {
		return err
	}

	// Generate functions resource
	if err := g.generateFunctionsResource(); err != nil {
		return err
	}

	// Generate events resource
	return g.generateEventsResource()
}

func (g *Generator) generateCollectionsResource(_ []string) error {
	var sb strings.Builder

	sb.WriteString("// Auto-generated collections resource\n\n")
	sb.WriteString("import { ListResponse } from '../types/collections';\n\n")

	sb.WriteString("export class CollectionClient<T, TInput = Partial<T>> {\n")
	sb.WriteString("  constructor(\n")
	sb.WriteString("    private baseURL: string,\n")
	sb.WriteString("    private collectionName: string,\n")
	sb.WriteString("    private getHeaders: () => Record<string, string>\n")
	sb.WriteString("  ) {}\n\n")

	sb.WriteString("  async list(params?: {\n")
	sb.WriteString("    limit?: number;\n")
	sb.WriteString("    offset?: number;\n")
	sb.WriteString("    sort?: string;\n")
	sb.WriteString("    filter?: string[];\n")
	sb.WriteString("  }): Promise<ListResponse<T>> {\n")
	sb.WriteString("    const query = new URLSearchParams();\n")
	sb.WriteString("    if (params?.limit) query.set('limit', params.limit.toString());\n")
	sb.WriteString("    if (params?.offset) query.set('offset', params.offset.toString());\n")
	sb.WriteString("    if (params?.sort) query.set('sort', params.sort);\n")
	sb.WriteString("    if (params?.filter) params.filter.forEach(f => query.append('filter', f));\n\n")
	sb.WriteString("    const response = await fetch(\n")
	sb.WriteString("      `${this.baseURL}/api/collections/${this.collectionName}?${query}`,\n")
	sb.WriteString("      { headers: this.getHeaders() }\n")
	sb.WriteString("    );\n")
	sb.WriteString("    if (!response.ok) throw new Error(`HTTP ${response.status}: ${await response.text()}`);\n")
	sb.WriteString("    return response.json();\n")
	sb.WriteString("  }\n\n")

	sb.WriteString("  async get(id: string): Promise<T> {\n")
	sb.WriteString("    const response = await fetch(\n")
	sb.WriteString("      `${this.baseURL}/api/collections/${this.collectionName}/${id}`,\n")
	sb.WriteString("      { headers: this.getHeaders() }\n")
	sb.WriteString("    );\n")
	sb.WriteString("    if (!response.ok) throw new Error(`HTTP ${response.status}: ${await response.text()}`);\n")
	sb.WriteString("    return response.json();\n")
	sb.WriteString("  }\n\n")

	sb.WriteString("  async create(data: TInput): Promise<T> {\n")
	sb.WriteString("    const response = await fetch(\n")
	sb.WriteString("      `${this.baseURL}/api/collections/${this.collectionName}`,\n")
	sb.WriteString("      {\n")
	sb.WriteString("        method: 'POST',\n")
	sb.WriteString("        headers: { ...this.getHeaders(), 'Content-Type': 'application/json' },\n")
	sb.WriteString("        body: JSON.stringify(data),\n")
	sb.WriteString("      }\n")
	sb.WriteString("    );\n")
	sb.WriteString("    if (!response.ok) throw new Error(`HTTP ${response.status}: ${await response.text()}`);\n")
	sb.WriteString("    return response.json();\n")
	sb.WriteString("  }\n\n")

	sb.WriteString("  async update(id: string, data: TInput): Promise<T> {\n")
	sb.WriteString("    const response = await fetch(\n")
	sb.WriteString("      `${this.baseURL}/api/collections/${this.collectionName}/${id}`,\n")
	sb.WriteString("      {\n")
	sb.WriteString("        method: 'PATCH',\n")
	sb.WriteString("        headers: { ...this.getHeaders(), 'Content-Type': 'application/json' },\n")
	sb.WriteString("        body: JSON.stringify(data),\n")
	sb.WriteString("      }\n")
	sb.WriteString("    );\n")
	sb.WriteString("    if (!response.ok) throw new Error(`HTTP ${response.status}: ${await response.text()}`);\n")
	sb.WriteString("    return response.json();\n")
	sb.WriteString("  }\n\n")

	sb.WriteString("  async delete(id: string): Promise<void> {\n")
	sb.WriteString("    const response = await fetch(\n")
	sb.WriteString("      `${this.baseURL}/api/collections/${this.collectionName}/${id}`,\n")
	sb.WriteString("      { method: 'DELETE', headers: this.getHeaders() }\n")
	sb.WriteString("    );\n")
	sb.WriteString("    if (!response.ok) throw new Error(`HTTP ${response.status}: ${await response.text()}`);\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n")

	return os.WriteFile(filepath.Join(g.config.OutputDir, "resources", "collections.ts"), []byte(sb.String()), 0600)
}

func (g *Generator) generateAuthResource() error {
	content := `// Auto-generated auth resource

import { User, AuthResponse, RegisterInput, LoginInput, RefreshInput } from '../types/auth';

export class AuthClient {
  constructor(
    private baseURL: string,
    private getHeaders: () => Record<string, string>
  ) {}

  async register(input: RegisterInput): Promise<AuthResponse> {
    const response = await fetch(` + "`${this.baseURL}/api/auth/register`" + `, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
    });
    if (!response.ok) throw new Error(` + "`HTTP ${response.status}: ${await response.text()}`" + `);
    return response.json();
  }

  async login(input: LoginInput): Promise<AuthResponse> {
    const response = await fetch(` + "`${this.baseURL}/api/auth/login`" + `, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
    });
    if (!response.ok) throw new Error(` + "`HTTP ${response.status}: ${await response.text()}`" + `);
    return response.json();
  }

  async refresh(input: RefreshInput): Promise<AuthResponse> {
    const response = await fetch(` + "`${this.baseURL}/api/auth/refresh`" + `, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
    });
    if (!response.ok) throw new Error(` + "`HTTP ${response.status}: ${await response.text()}`" + `);
    return response.json();
  }

  async logout(refreshToken: string): Promise<void> {
    const response = await fetch(` + "`${this.baseURL}/api/auth/logout`" + `, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    if (!response.ok) throw new Error(` + "`HTTP ${response.status}: ${await response.text()}`" + `);
  }

  async me(): Promise<User> {
    const response = await fetch(` + "`${this.baseURL}/api/auth/me`" + `, {
      headers: this.getHeaders(),
    });
    if (!response.ok) throw new Error(` + "`HTTP ${response.status}: ${await response.text()}`" + `);
    return response.json();
  }

  async listProviders(): Promise<{ providers: string[] }> {
    const response = await fetch(` + "`${this.baseURL}/api/auth/providers`" + `);
    if (!response.ok) throw new Error(` + "`HTTP ${response.status}: ${await response.text()}`" + `);
    return response.json();
  }
}
`
	return os.WriteFile(filepath.Join(g.config.OutputDir, "resources", "auth.ts"), []byte(content), 0600)
}

func (g *Generator) generateFunctionsResource() error {
	content := `// Auto-generated functions resource

import { FunctionInfo, FunctionInput, FunctionResponse } from '../types/functions';

export class FunctionsClient {
  constructor(
    private baseURL: string,
    private getHeaders: () => Record<string, string>
  ) {}

  async list(): Promise<{ functions: FunctionInfo[]; count: number }> {
    const response = await fetch(` + "`${this.baseURL}/api/functions`" + `, {
      headers: this.getHeaders(),
    });
    if (!response.ok) throw new Error(` + "`HTTP ${response.status}: ${await response.text()}`" + `);
    return response.json();
  }

  async invoke(name: string, input?: FunctionInput): Promise<FunctionResponse> {
    const response = await fetch(` + "`${this.baseURL}/api/functions/${name}`" + `, {
      method: 'POST',
      headers: { ...this.getHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(input || {}),
    });
    if (!response.ok) throw new Error(` + "`HTTP ${response.status}: ${await response.text()}`" + `);
    return response.json();
  }

  async stats(): Promise<{ pools: Record<string, { ready: number; busy: number; total: number }> }> {
    const response = await fetch(` + "`${this.baseURL}/api/functions/stats`" + `, {
      headers: this.getHeaders(),
    });
    if (!response.ok) throw new Error(` + "`HTTP ${response.status}: ${await response.text()}`" + `);
    return response.json();
  }

  async reload(): Promise<{ success: boolean; count: number; message: string }> {
    const response = await fetch(` + "`${this.baseURL}/api/functions/reload`" + `, {
      method: 'POST',
      headers: this.getHeaders(),
    });
    if (!response.ok) throw new Error(` + "`HTTP ${response.status}: ${await response.text()}`" + `);
    return response.json();
  }
}
`
	return os.WriteFile(filepath.Join(g.config.OutputDir, "resources", "functions.ts"), []byte(content), 0600)
}

func (g *Generator) generateEventsResource() error {
	content := `// Auto-generated events resource

import { Event, EventType, EventPayload, EventMetadata } from '../types/events';

export class EventsClient {
  constructor(
    private baseURL: string,
    private getHeaders: () => Record<string, string>
  ) {}

  async publish(event: {
    type: EventType;
    source: string;
    action: string;
    payload: EventPayload;
    metadata?: EventMetadata;
    process_at?: string;
  }): Promise<Event> {
    const response = await fetch(` + "`${this.baseURL}/api/events`" + `, {
      method: 'POST',
      headers: { ...this.getHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(event),
    });
    if (!response.ok) throw new Error(` + "`HTTP ${response.status}: ${await response.text()}`" + `);
    return response.json();
  }
}
`
	return os.WriteFile(filepath.Join(g.config.OutputDir, "resources", "events.ts"), []byte(content), 0600)
}

func (g *Generator) generateClient(collections []string) error {
	var sb strings.Builder

	sb.WriteString("// Auto-generated Alyx client\n\n")
	sb.WriteString("import { CollectionClient } from './resources/collections';\n")
	sb.WriteString("import { AuthClient } from './resources/auth';\n")
	sb.WriteString("import { FunctionsClient } from './resources/functions';\n")
	sb.WriteString("import { EventsClient } from './resources/events';\n")

	// Import collection types
	for _, name := range collections {
		sb.WriteString(fmt.Sprintf("import { %s, %sInput } from './types/collections';\n", capitalize(name), capitalize(name)))
	}

	sb.WriteString("\nexport interface AlyxConfig {\n")
	sb.WriteString("  url: string;\n")
	sb.WriteString("  token?: string;\n")
	sb.WriteString("}\n\n")

	sb.WriteString("export class AlyxClient {\n")
	sb.WriteString("  private config: AlyxConfig;\n")
	sb.WriteString("  public collections: {\n")
	for _, name := range collections {
		sb.WriteString(fmt.Sprintf("    %s: CollectionClient<%s, %sInput>;\n", name, capitalize(name), capitalize(name)))
	}
	sb.WriteString("  };\n")
	sb.WriteString("  public auth: AuthClient;\n")
	sb.WriteString("  public functions: FunctionsClient;\n")
	sb.WriteString("  public events: EventsClient;\n\n")

	sb.WriteString("  constructor(config: AlyxConfig) {\n")
	sb.WriteString("    this.config = config;\n\n")

	// Initialize collection clients
	sb.WriteString("    this.collections = {\n")
	for i, name := range collections {
		comma := ","
		if i == len(collections)-1 {
			comma = ""
		}
		sb.WriteString(fmt.Sprintf("      %s: new CollectionClient<%s, %sInput>(this.config.url, '%s', () => this.getHeaders())%s\n",
			name, capitalize(name), capitalize(name), name, comma))
	}
	sb.WriteString("    };\n\n")

	// Initialize other clients
	sb.WriteString("    this.auth = new AuthClient(this.config.url, () => this.getHeaders());\n")
	sb.WriteString("    this.functions = new FunctionsClient(this.config.url, () => this.getHeaders());\n")
	sb.WriteString("    this.events = new EventsClient(this.config.url, () => this.getHeaders());\n")
	sb.WriteString("  }\n\n")

	sb.WriteString("  private getHeaders(): Record<string, string> {\n")
	sb.WriteString("    const headers: Record<string, string> = {};\n")
	sb.WriteString("    if (this.config.token) {\n")
	sb.WriteString("      headers['Authorization'] = `Bearer ${this.config.token}`;\n")
	sb.WriteString("    }\n")
	sb.WriteString("    return headers;\n")
	sb.WriteString("  }\n")
	sb.WriteString("}\n")

	return os.WriteFile(filepath.Join(g.config.OutputDir, "client.ts"), []byte(sb.String()), 0600)
}

func (g *Generator) generateContext() error {
	content := `// Auto-generated context helper for function runtime

import { AlyxClient, AlyxConfig } from './client';
import { User } from './types/auth';

export interface FunctionContext {
  alyx: AlyxClient;
  auth: User | null;
  env: Record<string, string | undefined>;
}

export function getContext(): FunctionContext {
  const config: AlyxConfig = {
    url: process.env.ALYX_URL || 'http://localhost:8090',
    token: process.env.ALYX_INTERNAL_TOKEN,
  };

  let auth: User | null = null;
  if (process.env.ALYX_AUTH) {
    try {
      auth = JSON.parse(process.env.ALYX_AUTH);
    } catch (e) {
      console.error('Failed to parse ALYX_AUTH:', e);
    }
  }

  return {
    alyx: new AlyxClient(config),
    auth,
    env: process.env as Record<string, string | undefined>,
  };
}
`
	return os.WriteFile(filepath.Join(g.config.OutputDir, "context.ts"), []byte(content), 0600)
}

func (g *Generator) generateIndex() error {
	content := `// Auto-generated SDK exports

export * from './client';
export * from './context';
export * from './types/collections';
export * from './types/auth';
export * from './types/functions';
export * from './types/events';
export * from './resources/collections';
export * from './resources/auth';
export * from './resources/functions';
export * from './resources/events';
`
	return os.WriteFile(filepath.Join(g.config.OutputDir, "index.ts"), []byte(content), 0600)
}

// Helper functions

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func quoteStrings(strs []string) []string {
	quoted := make([]string, len(strs))
	for i, s := range strs {
		quoted[i] = "'" + s + "'"
	}
	return quoted
}
