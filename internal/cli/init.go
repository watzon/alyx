package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	initTemplate string
	initForce    bool
)

var initCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a new Alyx project",
	Long: `Initialize a new Alyx project with a starter template.

Creates the project directory structure with:
  - alyx.yaml        Configuration file
  - schema.yaml      Schema definition
  - functions/       Serverless functions directory
  - migrations/      Database migrations directory

Templates:
  basic   Minimal starter with a single collection (default)
  blog    Blog application with posts, users, and comments
  saas    SaaS starter with organizations and members`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVarP(&initTemplate, "template", "t", "basic", "Project template (basic, blog, saas)")
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing files")

	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	projectDir := "."
	if len(args) > 0 {
		projectDir = args[0]
	}

	tmpl, err := validateTemplate(initTemplate)
	if err != nil {
		return err
	}

	if err := prepareProjectDir(projectDir, initForce); err != nil {
		return err
	}

	if err := createProjectStructure(projectDir); err != nil {
		return err
	}

	if err := writeTemplateFiles(projectDir, tmpl); err != nil {
		return err
	}

	if err := writeGitignore(projectDir); err != nil {
		return err
	}

	printSuccessMessage(projectDir, initTemplate)
	return nil
}

func validateTemplate(name string) (*Template, error) {
	templates := getTemplates()
	tmpl, ok := templates[name]
	if !ok {
		return nil, fmt.Errorf("unknown template: %s (available: basic, blog, saas)", name)
	}
	return tmpl, nil
}

func prepareProjectDir(projectDir string, force bool) error {
	if projectDir != "." {
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			return fmt.Errorf("creating project directory: %w", err)
		}
		log.Info().Str("directory", projectDir).Msg("Created project directory")
	}

	if !force {
		existingFiles := checkExistingFiles(projectDir)
		if len(existingFiles) > 0 {
			return fmt.Errorf("files already exist: %s (use --force to overwrite)", strings.Join(existingFiles, ", "))
		}
	}
	return nil
}

func createProjectStructure(projectDir string) error {
	dirs := []string{"data", "functions", "migrations"}
	for _, dir := range dirs {
		dirPath := filepath.Join(projectDir, dir)
		if err := os.MkdirAll(dirPath, 0o755); err != nil {
			return fmt.Errorf("creating %s directory: %w", dir, err)
		}
	}
	return nil
}

func writeTemplateFiles(projectDir string, tmpl *Template) error {
	for filename, content := range tmpl.Files {
		if err := writeTemplateFile(projectDir, filename, content); err != nil {
			return err
		}
		log.Info().Str("file", filename).Msg("Created")
	}
	return nil
}

func writeTemplateFile(projectDir, filename, content string) error {
	filePath := filepath.Join(projectDir, filename)

	if dir := filepath.Dir(filePath); dir != projectDir && dir != "." {
		if err := os.MkdirAll(filepath.Join(projectDir, dir), 0o755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", filename, err)
		}
	}

	return os.WriteFile(filePath, []byte(content), 0o600)
}

func writeGitignore(projectDir string) error {
	content := `# Alyx data
data/
*.db
*.db-wal
*.db-shm

# Generated
generated/

# Environment
.env
.env.local

# IDE
.idea/
.vscode/
*.swp
*.swo
`
	if err := os.WriteFile(filepath.Join(projectDir, ".gitignore"), []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing .gitignore: %w", err)
	}
	log.Info().Str("file", ".gitignore").Msg("Created")
	return nil
}

func printSuccessMessage(projectDir, templateName string) {
	fmt.Println()
	fmt.Printf("✓ Project initialized with %q template\n", templateName)
	fmt.Println()
	fmt.Println("Next steps:")
	if projectDir != "." {
		fmt.Printf("  cd %s\n", projectDir)
	}
	fmt.Println("  alyx dev    # Start the development server")
	fmt.Println()
}

func checkExistingFiles(dir string) []string {
	filesToCheck := []string{"alyx.yaml", "schema.yaml", "schema.yml"}
	var existing []string
	for _, f := range filesToCheck {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			existing = append(existing, f)
		}
	}
	return existing
}

// Template represents a project template.
type Template struct {
	Name        string
	Description string
	Files       map[string]string
}

func getTemplates() map[string]*Template {
	return map[string]*Template{
		"basic": {
			Name:        "basic",
			Description: "Minimal starter with a single collection",
			Files: map[string]string{
				"alyx.yaml":   basicConfigYAML,
				"schema.yaml": basicSchemaYAML,
			},
		},
		"blog": {
			Name:        "blog",
			Description: "Blog application with posts, users, and comments",
			Files: map[string]string{
				"alyx.yaml":   blogConfigYAML,
				"schema.yaml": blogSchemaYAML,
			},
		},
		"saas": {
			Name:        "saas",
			Description: "SaaS starter with organizations and members",
			Files: map[string]string{
				"alyx.yaml":   saasConfigYAML,
				"schema.yaml": saasSchemaYAML,
			},
		},
	}
}

// Basic template files.
const basicConfigYAML = `# =============================================================================
# Alyx Configuration
# =============================================================================
# Full documentation: https://github.com/watzon/alyx/docs/configuration.md
#
# Environment variables can be used with ${VAR} or ${VAR:-default} syntax.
# Example: ${JWT_SECRET:-my-default-secret}
# =============================================================================

# -----------------------------------------------------------------------------
# Server Configuration
# -----------------------------------------------------------------------------
server:
  # Host to bind the server to
  host: localhost
  
  # Port to listen on
  port: 8090
  
  # CORS (Cross-Origin Resource Sharing) settings
  cors:
    enabled: true
    allowed_origins: ["*"]
    # allowed_methods: ["GET", "POST", "PATCH", "DELETE", "OPTIONS"]
    # allowed_headers: ["Content-Type", "Authorization"]
    # exposed_headers: []
    # allow_credentials: false
    # max_age: 86400  # 24 hours
  
  # Request timeouts
  # read_timeout: 30s
  # write_timeout: 30s
  # idle_timeout: 120s
  
  # Maximum request body size in bytes (default: 10MB)
  # max_body_size: 10485760
  
  # Enable embedded admin UI (coming soon)
  # admin_ui: true
  
  # TLS/HTTPS configuration (optional)
  # tls:
  #   enabled: true
  #   cert_file: /path/to/cert.pem
  #   key_file: /path/to/key.pem
  #   # Auto-TLS via Let's Encrypt
  #   auto_tls: false
  #   domain: example.com

# -----------------------------------------------------------------------------
# Database Configuration
# -----------------------------------------------------------------------------
database:
  # Path to SQLite database file
  path: ./data/alyx.db
  
  # Enable WAL mode for better concurrency (recommended)
  wal_mode: true
  
  # Enable foreign key constraints
  foreign_keys: true
  
  # Cache size in KB (negative) or pages (positive)
  # cache_size: -64000  # 64MB
  
  # Busy timeout in milliseconds
  # busy_timeout: 5000
  
  # Connection pool settings
  # max_open_conns: 25
  # max_idle_conns: 5
  # conn_max_lifetime: 5m
  
  # Turso (libSQL) for distributed deployments (optional)
  # turso:
  #   enabled: true
  #   url: libsql://your-database.turso.io
  #   auth_token: ${TURSO_AUTH_TOKEN}

# -----------------------------------------------------------------------------
# Authentication Configuration
# -----------------------------------------------------------------------------
auth:
  jwt:
    # Secret key for signing tokens (min 32 chars, CHANGE IN PRODUCTION!)
    secret: ${JWT_SECRET:-change-me-in-production-use-32-chars}
    
    # Token lifetimes (use Go duration format: s, m, h)
    access_ttl: 15m
    refresh_ttl: 168h  # 7 days
    
    # JWT claims
    # issuer: alyx
    # audience: []
  
  # Allow new user registration
  allow_registration: true
  
  # Require email verification before login
  # require_verification: false
  
  # Password requirements
  # password:
  #   min_length: 8
  #   require_uppercase: false
  #   require_lowercase: false
  #   require_number: false
  #   require_special: false
  
  # Rate limiting for auth endpoints
  # rate_limit:
  #   login:
  #     max: 5
  #     window: 1m
  #   register:
  #     max: 3
  #     window: 1m
  #   password_reset:
  #     max: 3
  #     window: 1h
  
  # OAuth providers (optional)
  # oauth:
  #   github:
  #     client_id: ${GITHUB_CLIENT_ID}
  #     client_secret: ${GITHUB_CLIENT_SECRET}
  #     scopes: [user:email]
  #   google:
  #     client_id: ${GOOGLE_CLIENT_ID}
  #     client_secret: ${GOOGLE_CLIENT_SECRET}
  #     scopes: [email, profile]
  #   # Custom OIDC provider
  #   custom:
  #     client_id: ${OIDC_CLIENT_ID}
  #     client_secret: ${OIDC_CLIENT_SECRET}
  #     auth_url: https://auth.example.com/authorize
  #     token_url: https://auth.example.com/token
  #     user_info_url: https://auth.example.com/userinfo
  #     scopes: [openid, email, profile]

# -----------------------------------------------------------------------------
# Serverless Functions Configuration
# -----------------------------------------------------------------------------
functions:
  enabled: true
  
  # Path to functions directory
  path: ./functions
  
  # Container runtime: docker or podman
  runtime: docker
  
  # Default execution timeout
  # timeout: 30s
  
  # Default resource limits
  # memory_limit: 256  # MB
  # cpu_limit: 1.0     # cores
  
  # Container pool settings per runtime
  # pools:
  #   node:
  #     min_warm: 1
  #     max_instances: 10
  #     idle_timeout: 60s
  #     image: ghcr.io/watzon/alyx-runtime-node:latest
  #   python:
  #     min_warm: 1
  #     max_instances: 10
  #     idle_timeout: 60s
  #     image: ghcr.io/watzon/alyx-runtime-python:latest
  
  # Environment variables passed to all functions
  # env:
  #   API_KEY: ${EXTERNAL_API_KEY}

# -----------------------------------------------------------------------------
# Real-time Subscriptions Configuration
# -----------------------------------------------------------------------------
realtime:
  enabled: true
  
  # How often to poll for changes (lower = faster updates, more CPU)
  poll_interval: 50ms
  
  # Maximum WebSocket connections
  # max_connections: 1000
  
  # Maximum subscriptions per client
  # max_subscriptions_per_client: 100
  
  # Change buffer size
  # change_buffer_size: 1000
  
  # Cleanup settings for processed changes
  # cleanup_interval: 5m
  # cleanup_age: 1h

# -----------------------------------------------------------------------------
# API Documentation Configuration
# -----------------------------------------------------------------------------
docs:
  enabled: true
  
  # UI style: scalar (recommended), swagger, or redoc
  ui: scalar
  
  # API info
  # title: My API
  # description: API documentation
  # version: 1.0.0

# -----------------------------------------------------------------------------
# Logging Configuration
# -----------------------------------------------------------------------------
logging:
  # Log level: debug, info, warn, error
  level: info
  
  # Log format: json or console
  format: console
  
  # Include caller info in logs
  # caller: false
  
  # Include timestamps
  # timestamp: true
  
  # Output file (empty for stdout)
  # output: ""

# -----------------------------------------------------------------------------
# Development Mode Configuration
# -----------------------------------------------------------------------------
dev:
  # Enable development mode features
  enabled: true
  
  # Watch for file changes
  watch: true
  
  # Auto-apply safe schema migrations
  auto_migrate: true
  
  # Auto-regenerate client SDKs on schema change
  auto_generate: false
  
  # Languages to generate (if auto_generate is true)
  # generate_languages: [typescript, go, python]
  
  # Output directory for generated clients
  # generate_output: ./generated
`

const basicSchemaYAML = `# =============================================================================
# Alyx Schema Definition
# =============================================================================
# Full documentation: https://github.com/watzon/alyx/docs/schema-reference.md
#
# This file defines your data model. Alyx will automatically:
#   - Create database tables from your collections
#   - Generate REST API endpoints
#   - Apply access control rules (CEL expressions)
#   - Generate type-safe client SDKs
# =============================================================================

version: 1

collections:
  # ---------------------------------------------------------------------------
  # Items Collection - A simple example to get started
  # ---------------------------------------------------------------------------
  items:
    fields:
      # Primary key - auto-generated 15-character ID
      id:
        type: id
        primary: true
        default: auto
      
      # Required string field with max length
      name:
        type: string
        maxLength: 200
      
      # Optional text field (no length limit)
      description:
        type: text
        nullable: true
      
      # Auto-managed timestamps
      created_at:
        type: timestamp
        default: now
      updated_at:
        type: timestamp
        default: now
        onUpdate: now

    # Access control rules using CEL (Common Expression Language)
    # Available variables:
    #   auth     - Current user (null if unauthenticated)
    #   auth.id  - User ID
    #   auth.role - User role
    #   doc      - Document being accessed
    #   doc.<field> - Any field from the document
    rules:
      create: "true"           # Anyone can create
      read: "true"             # Anyone can read
      update: "true"           # Anyone can update
      delete: "true"           # Anyone can delete

  # ===========================================================================
  # EXAMPLE COLLECTIONS (commented out)
  # Uncomment and modify these to add more collections to your schema
  # ===========================================================================

  # ---------------------------------------------------------------------------
  # Users Collection Example
  # ---------------------------------------------------------------------------
  # users:
  #   fields:
  #     id:
  #       type: id
  #       primary: true
  #       default: auto
  #     email:
  #       type: string
  #       unique: true              # Enforces uniqueness
  #       index: true               # Creates database index for faster queries
  #       validate:
  #         format: email           # Built-in email validation
  #     name:
  #       type: string
  #       maxLength: 100
  #       nullable: true
  #     avatar_url:
  #       type: string
  #       nullable: true
  #     role:
  #       type: string
  #       default: "user"           # Default value for new records
  #       validate:
  #         enum: [user, admin]     # Only allow these values
  #     metadata:
  #       type: json                # Flexible JSON data
  #       nullable: true
  #     created_at:
  #       type: timestamp
  #       default: now
  #     updated_at:
  #       type: timestamp
  #       default: now
  #       onUpdate: now
  #
  #   rules:
  #     create: "true"
  #     read: "auth.id == doc.id || auth.role == 'admin'"
  #     update: "auth.id == doc.id"
  #     delete: "auth.role == 'admin'"

  # ---------------------------------------------------------------------------
  # Posts Collection Example (with foreign key relationship)
  # ---------------------------------------------------------------------------
  # posts:
  #   fields:
  #     id:
  #       type: id
  #       primary: true
  #       default: auto
  #     title:
  #       type: string
  #       minLength: 1
  #       maxLength: 200
  #     slug:
  #       type: string
  #       unique: true
  #       index: true
  #       validate:
  #         pattern: "^[a-z0-9-]+$"   # Regex validation
  #     content:
  #       type: text
  #     author_id:
  #       type: uuid
  #       references: users.id        # Foreign key to users table
  #       onDelete: cascade           # Delete posts when user is deleted
  #       index: true
  #     published:
  #       type: bool
  #       default: false
  #     published_at:
  #       type: timestamp
  #       nullable: true
  #     view_count:
  #       type: int
  #       default: 0
  #     tags:
  #       type: json                  # Store as JSON array: ["tech", "news"]
  #       nullable: true
  #     created_at:
  #       type: timestamp
  #       default: now
  #     updated_at:
  #       type: timestamp
  #       default: now
  #       onUpdate: now
  #
  #   # Composite indexes for common query patterns
  #   indexes:
  #     - name: idx_posts_published
  #       fields: [published, published_at]
  #       order: desc
  #     - name: idx_posts_author
  #       fields: [author_id, created_at]
  #       order: desc
  #
  #   rules:
  #     create: "auth.id != null"
  #     read: "doc.published == true || auth.id == doc.author_id"
  #     update: "auth.id == doc.author_id"
  #     delete: "auth.id == doc.author_id"

  # ---------------------------------------------------------------------------
  # Field Types Reference
  # ---------------------------------------------------------------------------
  # type: id          - 15-character alphanumeric ID (recommended for primary keys)
  # type: uuid        - Full UUID string (36 characters)
  # type: string      - Text with optional length limits
  # type: text        - Unlimited text (for long content)
  # type: int         - Integer number
  # type: float       - Decimal number
  # type: bool        - Boolean (true/false)
  # type: timestamp   - Date/time (stored as ISO8601)
  # type: json        - JSON data (arrays, objects)
  # type: blob        - Binary data

  # ---------------------------------------------------------------------------
  # Field Options Reference
  # ---------------------------------------------------------------------------
  # primary: true     - Mark as primary key
  # unique: true      - Enforce uniqueness
  # nullable: true    - Allow NULL values
  # index: true       - Create database index
  # default: value    - Default value (use "auto" for IDs/UUIDs, "now" for timestamps)
  # references: tbl.col - Foreign key reference
  # onDelete: cascade/restrict/set null - Foreign key action
  # onUpdate: now     - Auto-update timestamp on changes
  # internal: true    - Hide from API responses
  #
  # validate:
  #   minLength: 1    - Minimum string length
  #   maxLength: 200  - Maximum string length
  #   min: 0          - Minimum numeric value
  #   max: 100        - Maximum numeric value
  #   format: email   - Built-in format (email, url, uuid)
  #   pattern: "^..."  - Regex pattern
  #   enum: [a, b, c] - Allowed values

  # ---------------------------------------------------------------------------
  # CEL Rules Reference
  # ---------------------------------------------------------------------------
  # Available in rules expressions:
  #   auth              - Current authenticated user (null if anonymous)
  #   auth.id           - User's ID
  #   auth.email        - User's email
  #   auth.role         - User's role
  #   auth.verified     - Whether email is verified
  #   auth.metadata     - Custom user metadata
  #   doc               - The document being accessed
  #   doc.<field>       - Any field from the document
  #
  # Common patterns:
  #   "true"                              - Allow everyone
  #   "false"                             - Deny everyone
  #   "auth.id != null"                   - Require authentication
  #   "auth.id == doc.author_id"          - Owner only
  #   "auth.role == 'admin'"              - Admin only
  #   "auth.role in ['admin', 'mod']"     - Multiple roles
  #   "doc.published == true || auth.id == doc.author_id"  - Public or owner
`

// Blog template files.
const blogConfigYAML = `# =============================================================================
# Alyx Configuration - Blog Template
# =============================================================================
# See full options in: https://github.com/watzon/alyx/docs/configuration.md
# =============================================================================

server:
  host: localhost
  port: 8090
  cors:
    enabled: true
    allowed_origins: ["*"]

database:
  path: ./data/alyx.db
  wal_mode: true
  foreign_keys: true

auth:
  jwt:
    secret: ${JWT_SECRET:-change-me-in-production-use-32-chars}
    access_ttl: 15m
    refresh_ttl: 168h  # 7 days
  allow_registration: true
  
  # Uncomment to enable OAuth providers
  # oauth:
  #   github:
  #     client_id: ${GITHUB_CLIENT_ID}
  #     client_secret: ${GITHUB_CLIENT_SECRET}
  #     scopes: [user:email]

functions:
  enabled: true
  path: ./functions
  runtime: docker

realtime:
  enabled: true
  poll_interval: 50ms

docs:
  enabled: true
  ui: scalar
  title: Blog API
  description: Blog backend powered by Alyx

dev:
  enabled: true
  watch: true
  auto_migrate: true
`

const blogSchemaYAML = `# =============================================================================
# Alyx Schema Definition - Blog Template
# =============================================================================
# A complete blog schema with users, posts, and comments.
# Includes examples of relationships, validation, and access control.
# =============================================================================

version: 1

collections:
  # ---------------------------------------------------------------------------
  # Users - Blog authors and readers
  # ---------------------------------------------------------------------------
  users:
    fields:
      id:
        type: id
        primary: true
        default: auto
      email:
        type: string
        unique: true
        index: true
        validate:
          format: email
      name:
        type: string
        maxLength: 100
        nullable: true
      avatar_url:
        type: string
        nullable: true
      role:
        type: string
        default: "user"
        validate:
          enum: [user, author, admin]
      created_at:
        type: timestamp
        default: now
      updated_at:
        type: timestamp
        default: now
        onUpdate: now

    # Access rules:
    # - Anyone can register (create)
    # - Users can only read their own profile (or admins can read all)
    # - Users can only update their own profile
    # - Only admins can delete users
    rules:
      create: "true"
      read: "auth.id == doc.id || auth.role == 'admin'"
      update: "auth.id == doc.id"
      delete: "auth.role == 'admin'"

  # ---------------------------------------------------------------------------
  # Posts - Blog articles
  # ---------------------------------------------------------------------------
  posts:
    fields:
      id:
        type: id
        primary: true
        default: auto
      title:
        type: string
        minLength: 1
        maxLength: 200
      slug:
        type: string
        unique: true
        index: true
      content:
        type: text
      excerpt:
        type: string
        maxLength: 500
        nullable: true
      author_id:
        type: uuid
        references: users.id
        onDelete: cascade
        index: true
      published:
        type: bool
        default: false
      published_at:
        type: timestamp
        nullable: true
      tags:
        type: json
        nullable: true
      view_count:
        type: int
        default: 0
      created_at:
        type: timestamp
        default: now
      updated_at:
        type: timestamp
        default: now
        onUpdate: now

    # Composite indexes for common query patterns
    indexes:
      - name: idx_posts_published_date
        fields: [published, published_at]
        order: desc
      - name: idx_posts_author_date
        fields: [author_id, created_at]
        order: desc

    # Access rules:
    # - Only authenticated users can create posts
    # - Published posts are public; drafts only visible to author/admin
    # - Only author or admin can update/delete
    rules:
      create: "auth.id != null"
      read: "doc.published == true || auth.id == doc.author_id || auth.role == 'admin'"
      update: "auth.id == doc.author_id || auth.role == 'admin'"
      delete: "auth.id == doc.author_id || auth.role == 'admin'"

  # ---------------------------------------------------------------------------
  # Comments - User comments on posts
  # ---------------------------------------------------------------------------
  comments:
    fields:
      id:
        type: id
        primary: true
        default: auto
      post_id:
        type: uuid
        references: posts.id
        onDelete: cascade
        index: true
      author_id:
        type: uuid
        references: users.id
        onDelete: cascade
      content:
        type: text
        maxLength: 5000
      created_at:
        type: timestamp
        default: now

    # Access rules:
    # - Only authenticated users can comment
    # - All comments are public
    # - Only the author can edit their comment
    # - Author or admin can delete
    rules:
      create: "auth.id != null"
      read: "true"
      update: "auth.id == doc.author_id"
      delete: "auth.id == doc.author_id || auth.role == 'admin'"

# =============================================================================
# Additional Collections You Might Add
# =============================================================================
# 
# categories:
#   fields:
#     id: { type: id, primary: true, default: auto }
#     name: { type: string, maxLength: 50 }
#     slug: { type: string, unique: true, index: true }
#     description: { type: text, nullable: true }
#   rules:
#     create: "auth.role == 'admin'"
#     read: "true"
#     update: "auth.role == 'admin'"
#     delete: "auth.role == 'admin'"
#
# tags:
#   fields:
#     id: { type: id, primary: true, default: auto }
#     name: { type: string, unique: true, maxLength: 30 }
#   rules:
#     create: "auth.id != null"
#     read: "true"
#     update: "auth.role == 'admin'"
#     delete: "auth.role == 'admin'"
#
# media:
#   fields:
#     id: { type: id, primary: true, default: auto }
#     filename: { type: string }
#     url: { type: string }
#     mime_type: { type: string }
#     size: { type: int }
#     uploaded_by: { type: uuid, references: users.id, onDelete: cascade }
#     created_at: { type: timestamp, default: now }
#   rules:
#     create: "auth.id != null"
#     read: "true"
#     update: "auth.id == doc.uploaded_by || auth.role == 'admin'"
#     delete: "auth.id == doc.uploaded_by || auth.role == 'admin'"
`

// SaaS template files.
const saasConfigYAML = `# =============================================================================
# Alyx Configuration - SaaS Template
# =============================================================================
# See full options in: https://github.com/watzon/alyx/docs/configuration.md
# =============================================================================

server:
  host: localhost
  port: 8090
  cors:
    enabled: true
    allowed_origins: ["*"]

database:
  path: ./data/alyx.db
  wal_mode: true
  foreign_keys: true

auth:
  jwt:
    secret: ${JWT_SECRET:-change-me-in-production-use-32-chars}
    access_ttl: 15m
    refresh_ttl: 168h  # 7 days
  allow_registration: true
  
  # Uncomment to enable OAuth providers (recommended for SaaS)
  # oauth:
  #   github:
  #     client_id: ${GITHUB_CLIENT_ID}
  #     client_secret: ${GITHUB_CLIENT_SECRET}
  #     scopes: [user:email]
  #   google:
  #     client_id: ${GOOGLE_CLIENT_ID}
  #     client_secret: ${GOOGLE_CLIENT_SECRET}
  #     scopes: [email, profile]

functions:
  enabled: true
  path: ./functions
  runtime: docker

realtime:
  enabled: true
  poll_interval: 50ms

docs:
  enabled: true
  ui: scalar
  title: SaaS API
  description: SaaS backend powered by Alyx

dev:
  enabled: true
  watch: true
  auto_migrate: true
`

const saasSchemaYAML = `# =============================================================================
# Alyx Schema Definition - SaaS Template
# =============================================================================
# A multi-tenant SaaS schema with organizations, members, and invitations.
# Demonstrates team-based access control patterns.
# =============================================================================

version: 1

collections:
  # ---------------------------------------------------------------------------
  # Users - Individual user accounts
  # ---------------------------------------------------------------------------
  users:
    fields:
      id:
        type: id
        primary: true
        default: auto
      email:
        type: string
        unique: true
        index: true
        validate:
          format: email
      name:
        type: string
        maxLength: 100
        nullable: true
      avatar_url:
        type: string
        nullable: true
      created_at:
        type: timestamp
        default: now
      updated_at:
        type: timestamp
        default: now
        onUpdate: now

    # Users can only access their own data
    rules:
      create: "true"
      read: "auth.id == doc.id"
      update: "auth.id == doc.id"
      delete: "false"  # Prevent self-deletion; use account deletion flow

  # ---------------------------------------------------------------------------
  # Organizations - Teams/companies (tenants)
  # ---------------------------------------------------------------------------
  organizations:
    fields:
      id:
        type: id
        primary: true
        default: auto
      name:
        type: string
        maxLength: 100
      slug:
        type: string
        unique: true
        index: true
      plan:
        type: string
        default: "free"
        validate:
          enum: [free, starter, pro, enterprise]
      owner_id:
        type: uuid
        references: users.id
        onDelete: restrict
      settings:
        type: json
        nullable: true
      created_at:
        type: timestamp
        default: now
      updated_at:
        type: timestamp
        default: now
        onUpdate: now

    # Only owner can manage organization
    # Note: In production, you'd check membership for read access
    rules:
      create: "auth.id != null"
      read: "auth.id == doc.owner_id"
      update: "auth.id == doc.owner_id"
      delete: "auth.id == doc.owner_id"

  # ---------------------------------------------------------------------------
  # Members - Organization memberships (many-to-many)
  # ---------------------------------------------------------------------------
  members:
    fields:
      id:
        type: id
        primary: true
        default: auto
      org_id:
        type: uuid
        references: organizations.id
        onDelete: cascade
        index: true
      user_id:
        type: uuid
        references: users.id
        onDelete: cascade
        index: true
      role:
        type: string
        default: "member"
        validate:
          enum: [member, admin, owner]
      invited_by:
        type: uuid
        references: users.id
        onDelete: set null
        nullable: true
      created_at:
        type: timestamp
        default: now

    # Unique constraint: user can only be member once per org
    indexes:
      - name: idx_members_org_user
        fields: [org_id, user_id]
        unique: true

    # Members can view their own memberships
    # Note: Org admins would need a function to manage members
    rules:
      create: "auth.id != null"
      read: "auth.id == doc.user_id"
      update: "false"  # Role changes via functions only
      delete: "auth.id == doc.user_id"

  # ---------------------------------------------------------------------------
  # Invitations - Pending team invites
  # ---------------------------------------------------------------------------
  invitations:
    fields:
      id:
        type: id
        primary: true
        default: auto
      org_id:
        type: uuid
        references: organizations.id
        onDelete: cascade
        index: true
      email:
        type: string
        validate:
          format: email
      role:
        type: string
        default: "member"
        validate:
          enum: [member, admin]
      token:
        type: string
        unique: true
      invited_by:
        type: uuid
        references: users.id
        onDelete: cascade
      expires_at:
        type: timestamp
      created_at:
        type: timestamp
        default: now

    # Invitations are private - only the inviter can manage
    # Accept flow should be handled via functions
    rules:
      create: "auth.id != null"
      read: "false"   # Read via token-based accept endpoint
      update: "false"
      delete: "auth.id == doc.invited_by"

# =============================================================================
# Additional Collections You Might Add
# =============================================================================
#
# projects:
#   fields:
#     id: { type: id, primary: true, default: auto }
#     org_id: { type: uuid, references: organizations.id, onDelete: cascade, index: true }
#     name: { type: string, maxLength: 100 }
#     description: { type: text, nullable: true }
#     created_by: { type: uuid, references: users.id, onDelete: set null, nullable: true }
#     created_at: { type: timestamp, default: now }
#     updated_at: { type: timestamp, default: now, onUpdate: now }
#   rules:
#     create: "auth.id != null"  # Check org membership in function
#     read: "auth.id != null"    # Check org membership in function
#     update: "auth.id != null"
#     delete: "auth.id != null"
#
# api_keys:
#   fields:
#     id: { type: id, primary: true, default: auto }
#     org_id: { type: uuid, references: organizations.id, onDelete: cascade, index: true }
#     name: { type: string, maxLength: 50 }
#     key_hash: { type: string, internal: true }  # Never expose
#     last_used_at: { type: timestamp, nullable: true }
#     expires_at: { type: timestamp, nullable: true }
#     created_by: { type: uuid, references: users.id, onDelete: set null, nullable: true }
#     created_at: { type: timestamp, default: now }
#   rules:
#     create: "auth.id != null"
#     read: "auth.id != null"
#     update: "false"
#     delete: "auth.id != null"
#
# audit_logs:
#   fields:
#     id: { type: id, primary: true, default: auto }
#     org_id: { type: uuid, references: organizations.id, onDelete: cascade, index: true }
#     user_id: { type: uuid, references: users.id, onDelete: set null, nullable: true }
#     action: { type: string }
#     resource_type: { type: string }
#     resource_id: { type: string, nullable: true }
#     metadata: { type: json, nullable: true }
#     ip_address: { type: string, nullable: true }
#     created_at: { type: timestamp, default: now }
#   indexes:
#     - name: idx_audit_org_created
#       fields: [org_id, created_at]
#       order: desc
#   rules:
#     create: "false"  # Created via internal functions only
#     read: "auth.id != null"  # Check org admin role
#     update: "false"
#     delete: "false"  # Audit logs are immutable
#
# subscriptions:
#   fields:
#     id: { type: id, primary: true, default: auto }
#     org_id: { type: uuid, references: organizations.id, onDelete: cascade, unique: true }
#     plan: { type: string, validate: { enum: [free, starter, pro, enterprise] } }
#     status: { type: string, validate: { enum: [active, past_due, canceled, trialing] } }
#     stripe_subscription_id: { type: string, nullable: true, internal: true }
#     stripe_customer_id: { type: string, nullable: true, internal: true }
#     current_period_start: { type: timestamp, nullable: true }
#     current_period_end: { type: timestamp, nullable: true }
#     canceled_at: { type: timestamp, nullable: true }
#     created_at: { type: timestamp, default: now }
#     updated_at: { type: timestamp, default: now, onUpdate: now }
#   rules:
#     create: "false"  # Created via billing functions
#     read: "auth.id != null"
#     update: "false"  # Updated via billing functions
#     delete: "false"
`
