// Package config provides configuration management for Alyx.
package config

import (
	"time"
)

// Config is the root configuration structure for Alyx.
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Functions FunctionsConfig `mapstructure:"functions"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Dev       DevConfig       `mapstructure:"dev"`
	Docs      DocsConfig      `mapstructure:"docs"`
}

type DocsConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	UI          string `mapstructure:"ui"`
	Title       string `mapstructure:"title"`
	Description string `mapstructure:"description"`
	Version     string `mapstructure:"version"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	// Host to bind the server to
	Host string `mapstructure:"host"`

	// Port to listen on
	Port int `mapstructure:"port"`

	// Enable CORS
	CORS CORSConfig `mapstructure:"cors"`

	// Request timeout
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`

	// Maximum request body size in bytes
	MaxBodySize int64 `mapstructure:"max_body_size"`

	// Enable admin UI
	AdminUI bool `mapstructure:"admin_ui"`

	// TLS configuration (optional)
	TLS *TLSConfig `mapstructure:"tls"`
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	// Enable CORS
	Enabled bool `mapstructure:"enabled"`

	// Allowed origins (use ["*"] for all)
	AllowedOrigins []string `mapstructure:"allowed_origins"`

	// Allowed methods
	AllowedMethods []string `mapstructure:"allowed_methods"`

	// Allowed headers
	AllowedHeaders []string `mapstructure:"allowed_headers"`

	// Exposed headers
	ExposedHeaders []string `mapstructure:"exposed_headers"`

	// Allow credentials
	AllowCredentials bool `mapstructure:"allow_credentials"`

	// Max age for preflight cache
	MaxAge time.Duration `mapstructure:"max_age"`
}

// TLSConfig holds TLS settings.
type TLSConfig struct {
	// Enable TLS
	Enabled bool `mapstructure:"enabled"`

	// Path to certificate file
	CertFile string `mapstructure:"cert_file"`

	// Path to key file
	KeyFile string `mapstructure:"key_file"`

	// Enable auto TLS via Let's Encrypt
	AutoTLS bool `mapstructure:"auto_tls"`

	// Domain for auto TLS
	Domain string `mapstructure:"domain"`
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	// Path to SQLite database file
	Path string `mapstructure:"path"`

	// Enable WAL mode (recommended)
	WALMode bool `mapstructure:"wal_mode"`

	// Cache size in KB (negative for KB, positive for pages)
	CacheSize int `mapstructure:"cache_size"`

	// Busy timeout in milliseconds
	BusyTimeout time.Duration `mapstructure:"busy_timeout"`

	// Enable foreign keys
	ForeignKeys bool `mapstructure:"foreign_keys"`

	// Maximum open connections
	MaxOpenConns int `mapstructure:"max_open_conns"`

	// Maximum idle connections
	MaxIdleConns int `mapstructure:"max_idle_conns"`

	// Connection max lifetime
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`

	// Turso configuration (optional, for distributed deployments)
	Turso *TursoConfig `mapstructure:"turso"`
}

// TursoConfig holds Turso (libSQL) settings.
type TursoConfig struct {
	// Enable Turso
	Enabled bool `mapstructure:"enabled"`

	// Turso database URL
	URL string `mapstructure:"url"`

	// Auth token
	AuthToken string `mapstructure:"auth_token"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	// JWT configuration
	JWT JWTConfig `mapstructure:"jwt"`

	// Password requirements
	Password PasswordConfig `mapstructure:"password"`

	// OAuth providers
	OAuth map[string]OAuthProviderConfig `mapstructure:"oauth"`

	// Rate limiting
	RateLimit AuthRateLimitConfig `mapstructure:"rate_limit"`

	// Allow registration
	AllowRegistration bool `mapstructure:"allow_registration"`

	// Require email verification
	RequireVerification bool `mapstructure:"require_verification"`
}

// JWTConfig holds JWT settings.
type JWTConfig struct {
	// Secret key for signing tokens (required, min 32 chars)
	Secret string `mapstructure:"secret"`

	// Access token lifetime
	AccessTTL time.Duration `mapstructure:"access_ttl"`

	// Refresh token lifetime
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`

	// JWT issuer claim
	Issuer string `mapstructure:"issuer"`

	// JWT audience claim
	Audience []string `mapstructure:"audience"`
}

// PasswordConfig holds password requirements.
type PasswordConfig struct {
	// Minimum password length
	MinLength int `mapstructure:"min_length"`

	// Require uppercase letter
	RequireUppercase bool `mapstructure:"require_uppercase"`

	// Require lowercase letter
	RequireLowercase bool `mapstructure:"require_lowercase"`

	// Require number
	RequireNumber bool `mapstructure:"require_number"`

	// Require special character
	RequireSpecial bool `mapstructure:"require_special"`
}

// OAuthProviderConfig holds OAuth provider settings.
type OAuthProviderConfig struct {
	// Client ID
	ClientID string `mapstructure:"client_id"`

	// Client secret
	ClientSecret string `mapstructure:"client_secret"`

	// OAuth scopes
	Scopes []string `mapstructure:"scopes"`

	// Custom authorization URL (for custom OIDC)
	AuthURL string `mapstructure:"auth_url"`

	// Custom token URL (for custom OIDC)
	TokenURL string `mapstructure:"token_url"`

	// Custom user info URL (for custom OIDC)
	UserInfoURL string `mapstructure:"user_info_url"`
}

// AuthRateLimitConfig holds rate limiting settings for auth endpoints.
type AuthRateLimitConfig struct {
	// Login attempts per minute
	Login RateLimitRule `mapstructure:"login"`

	// Registration attempts per minute
	Register RateLimitRule `mapstructure:"register"`

	// Password reset attempts per hour
	PasswordReset RateLimitRule `mapstructure:"password_reset"`
}

// RateLimitRule defines a rate limit rule.
type RateLimitRule struct {
	// Maximum requests
	Max int `mapstructure:"max"`

	// Time window
	Window time.Duration `mapstructure:"window"`
}

// FunctionsConfig holds serverless functions settings.
type FunctionsConfig struct {
	// Enable functions
	Enabled bool `mapstructure:"enabled"`

	// Path to functions directory
	Path string `mapstructure:"path"`

	// Container runtime (docker or podman)
	Runtime string `mapstructure:"runtime"`

	// Default execution timeout
	Timeout time.Duration `mapstructure:"timeout"`

	// Default memory limit in MB
	MemoryLimit int `mapstructure:"memory_limit"`

	// Default CPU limit (cores)
	CPULimit float64 `mapstructure:"cpu_limit"`

	// Container pool settings per runtime
	Pools map[string]PoolConfig `mapstructure:"pools"`

	// Environment variables to pass to functions
	Env map[string]string `mapstructure:"env"`
}

// PoolConfig holds container pool settings.
type PoolConfig struct {
	// Minimum warm instances
	MinWarm int `mapstructure:"min_warm"`

	// Maximum concurrent instances
	MaxInstances int `mapstructure:"max_instances"`

	// Idle timeout before scaling down
	IdleTimeout time.Duration `mapstructure:"idle_timeout"`

	// Container image to use
	Image string `mapstructure:"image"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	// Log level (debug, info, warn, error)
	Level string `mapstructure:"level"`

	// Log format (json, console)
	Format string `mapstructure:"format"`

	// Include caller info
	Caller bool `mapstructure:"caller"`

	// Include timestamp
	Timestamp bool `mapstructure:"timestamp"`

	// Output file (empty for stdout)
	Output string `mapstructure:"output"`
}

// DevConfig holds development mode settings.
type DevConfig struct {
	// Enable development mode
	Enabled bool `mapstructure:"enabled"`

	// Watch for file changes
	Watch bool `mapstructure:"watch"`

	// Auto-apply safe migrations
	AutoMigrate bool `mapstructure:"auto_migrate"`

	// Auto-regenerate client SDKs
	AutoGenerate bool `mapstructure:"auto_generate"`

	// Languages to generate clients for
	GenerateLanguages []string `mapstructure:"generate_languages"`

	// Output directory for generated clients
	GenerateOutput string `mapstructure:"generate_output"`
}

// Address returns the server address in host:port format.
func (s *ServerConfig) Address() string {
	return s.Host + ":" + itoa(s.Port)
}

// itoa converts int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	negative := i < 0
	if negative {
		i = -i
	}
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	if negative {
		n--
		b[n] = '-'
	}
	return string(b[n:])
}
