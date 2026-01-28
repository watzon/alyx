package config

import (
	"fmt"
	"time"
)

// ConfigFieldType represents the type of a configuration field.
type ConfigFieldType string

const (
	FieldTypeString      ConfigFieldType = "string"
	FieldTypeInt         ConfigFieldType = "int"
	FieldTypeInt64       ConfigFieldType = "int64"
	FieldTypeBool        ConfigFieldType = "bool"
	FieldTypeDuration    ConfigFieldType = "duration"
	FieldTypeStringArray ConfigFieldType = "stringArray"
	FieldTypeStringMap   ConfigFieldType = "stringMap"
	FieldTypeObject      ConfigFieldType = "object"
	FieldTypeSecret      ConfigFieldType = "secret"
)

// ConfigFieldMeta holds metadata about a configuration field.
type ConfigFieldMeta struct {
	Type        ConfigFieldType `json:"type"`
	Description string          `json:"description,omitempty"`
	Default     any             `json:"default,omitempty"`
	Current     any             `json:"current,omitempty"`
	Sensitive   bool            `json:"sensitive,omitempty"`
	Required    bool            `json:"required,omitempty"`
	Options     []string        `json:"options,omitempty"`
	Fields      map[string]any  `json:"fields,omitempty"` // For nested objects
}

// ConfigSectionMeta holds metadata about a configuration section.
type ConfigSectionMeta struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Fields      map[string]any `json:"fields"`
}

// GetConfigSchema returns the full configuration schema with metadata and current values.
func GetConfigSchema(current *Config, configPath string) map[string]any {
	defaults := Default()

	sections := map[string]ConfigSectionMeta{
		"server": {
			Name:        "Server",
			Description: "HTTP server settings",
			Fields: map[string]any{
				"host": ConfigFieldMeta{
					Type:        FieldTypeString,
					Description: "Host to bind the server to",
					Default:     defaults.Server.Host,
					Current:     current.Server.Host,
				},
				"port": ConfigFieldMeta{
					Type:        FieldTypeInt,
					Description: "Port to listen on",
					Default:     defaults.Server.Port,
					Current:     current.Server.Port,
				},
				"read_timeout": ConfigFieldMeta{
					Type:        FieldTypeDuration,
					Description: "Request read timeout",
					Default:     formatDuration(defaults.Server.ReadTimeout),
					Current:     formatDuration(current.Server.ReadTimeout),
				},
				"write_timeout": ConfigFieldMeta{
					Type:        FieldTypeDuration,
					Description: "Request write timeout",
					Default:     formatDuration(defaults.Server.WriteTimeout),
					Current:     formatDuration(current.Server.WriteTimeout),
				},
				"idle_timeout": ConfigFieldMeta{
					Type:        FieldTypeDuration,
					Description: "Connection idle timeout",
					Default:     formatDuration(defaults.Server.IdleTimeout),
					Current:     formatDuration(current.Server.IdleTimeout),
				},
				"max_body_size": ConfigFieldMeta{
					Type:        FieldTypeInt64,
					Description: "Maximum request body size in bytes",
					Default:     defaults.Server.MaxBodySize,
					Current:     current.Server.MaxBodySize,
				},
				"cors": ConfigFieldMeta{
					Type:        FieldTypeObject,
					Description: "CORS settings",
					Fields: map[string]any{
						"enabled": ConfigFieldMeta{
							Type:        FieldTypeBool,
							Description: "Enable CORS",
							Default:     defaults.Server.CORS.Enabled,
							Current:     current.Server.CORS.Enabled,
						},
						"allowed_origins": ConfigFieldMeta{
							Type:        FieldTypeStringArray,
							Description: "Allowed origins (use [\"*\"] for all)",
							Default:     defaults.Server.CORS.AllowedOrigins,
							Current:     current.Server.CORS.AllowedOrigins,
						},
						"exposed_headers": ConfigFieldMeta{
							Type:        FieldTypeStringArray,
							Description: "Exposed headers",
							Default:     defaults.Server.CORS.ExposedHeaders,
							Current:     current.Server.CORS.ExposedHeaders,
						},
						"allow_credentials": ConfigFieldMeta{
							Type:        FieldTypeBool,
							Description: "Allow credentials",
							Default:     defaults.Server.CORS.AllowCredentials,
							Current:     current.Server.CORS.AllowCredentials,
						},
						"max_age": ConfigFieldMeta{
							Type:        FieldTypeDuration,
							Description: "Max age for preflight cache",
							Default:     formatDuration(defaults.Server.CORS.MaxAge),
							Current:     formatDuration(current.Server.CORS.MaxAge),
						},
					},
				},
				"tls": ConfigFieldMeta{
					Type:        FieldTypeObject,
					Description: "TLS configuration (optional)",
					Fields: map[string]any{
						"enabled": ConfigFieldMeta{
							Type:        FieldTypeBool,
							Description: "Enable TLS",
							Default:     false,
							Current:     current.Server.TLS != nil && current.Server.TLS.Enabled,
						},
						"cert_file": ConfigFieldMeta{
							Type:        FieldTypeString,
							Description: "Path to certificate file",
							Default:     "",
							Current:     getStringFromPtr(current.Server.TLS, func(t *TLSConfig) string { return t.CertFile }),
						},
						"key_file": ConfigFieldMeta{
							Type:        FieldTypeString,
							Description: "Path to key file",
							Default:     "",
							Current:     getStringFromPtr(current.Server.TLS, func(t *TLSConfig) string { return t.KeyFile }),
						},
						"auto_tls": ConfigFieldMeta{
							Type:        FieldTypeBool,
							Description: "Enable auto TLS via Let's Encrypt",
							Default:     false,
							Current:     current.Server.TLS != nil && current.Server.TLS.AutoTLS,
						},
						"domain": ConfigFieldMeta{
							Type:        FieldTypeString,
							Description: "Domain for auto TLS",
							Default:     "",
							Current:     getStringFromPtr(current.Server.TLS, func(t *TLSConfig) string { return t.Domain }),
						},
					},
				},
			},
		},
		"database": {
			Name:        "Database",
			Description: "Database settings",
			Fields: map[string]any{
				"path": ConfigFieldMeta{
					Type:        FieldTypeString,
					Description: "Path to SQLite database file",
					Default:     defaults.Database.Path,
					Current:     current.Database.Path,
				},
				"turso": ConfigFieldMeta{
					Type:        FieldTypeObject,
					Description: "Turso configuration (optional, for distributed deployments)",
					Fields: map[string]any{
						"enabled": ConfigFieldMeta{
							Type:        FieldTypeBool,
							Description: "Enable Turso",
							Default:     false,
							Current:     current.Database.Turso != nil && current.Database.Turso.Enabled,
						},
						"url": ConfigFieldMeta{
							Type:        FieldTypeString,
							Description: "Turso database URL",
							Default:     "",
							Current:     getStringFromPtr(current.Database.Turso, func(t *TursoConfig) string { return t.URL }),
						},
						"auth_token": ConfigFieldMeta{
							Type:        FieldTypeSecret,
							Description: "Auth token",
							Sensitive:   true,
							Default:     "",
							Current:     isSecretSet(getStringFromPtr(current.Database.Turso, func(t *TursoConfig) string { return t.AuthToken })),
						},
					},
				},
			},
		},
		"auth": {
			Name:        "Authentication",
			Description: "Authentication and authorization settings",
			Fields: map[string]any{
				"allow_registration": ConfigFieldMeta{
					Type:        FieldTypeBool,
					Description: "Allow user registration",
					Default:     defaults.Auth.AllowRegistration,
					Current:     current.Auth.AllowRegistration,
				},
				"require_verification": ConfigFieldMeta{
					Type:        FieldTypeBool,
					Description: "Require email verification",
					Default:     defaults.Auth.RequireVerification,
					Current:     current.Auth.RequireVerification,
				},
				"jwt": ConfigFieldMeta{
					Type:        FieldTypeObject,
					Description: "JWT configuration",
					Fields: map[string]any{
						"secret": ConfigFieldMeta{
							Type:        FieldTypeSecret,
							Description: "JWT signing secret (required, min 32 chars)",
							Sensitive:   true,
							Required:    true,
							Default:     "",
							Current:     isSecretSet(current.Auth.JWT.Secret),
						},
						"access_ttl": ConfigFieldMeta{
							Type:        FieldTypeDuration,
							Description: "Access token lifetime",
							Default:     formatDuration(defaults.Auth.JWT.AccessTTL),
							Current:     formatDuration(current.Auth.JWT.AccessTTL),
						},
						"refresh_ttl": ConfigFieldMeta{
							Type:        FieldTypeDuration,
							Description: "Refresh token lifetime",
							Default:     formatDuration(defaults.Auth.JWT.RefreshTTL),
							Current:     formatDuration(current.Auth.JWT.RefreshTTL),
						},
						"issuer": ConfigFieldMeta{
							Type:        FieldTypeString,
							Description: "JWT issuer claim",
							Default:     defaults.Auth.JWT.Issuer,
							Current:     current.Auth.JWT.Issuer,
						},
						"audience": ConfigFieldMeta{
							Type:        FieldTypeStringArray,
							Description: "JWT audience claim",
							Default:     defaults.Auth.JWT.Audience,
							Current:     current.Auth.JWT.Audience,
						},
					},
				},
				"password": ConfigFieldMeta{
					Type:        FieldTypeObject,
					Description: "Password requirements",
					Fields: map[string]any{
						"min_length": ConfigFieldMeta{
							Type:        FieldTypeInt,
							Description: "Minimum password length",
							Default:     defaults.Auth.Password.MinLength,
							Current:     current.Auth.Password.MinLength,
						},
						"require_uppercase": ConfigFieldMeta{
							Type:        FieldTypeBool,
							Description: "Require uppercase letter",
							Default:     defaults.Auth.Password.RequireUppercase,
							Current:     current.Auth.Password.RequireUppercase,
						},
						"require_lowercase": ConfigFieldMeta{
							Type:        FieldTypeBool,
							Description: "Require lowercase letter",
							Default:     defaults.Auth.Password.RequireLowercase,
							Current:     current.Auth.Password.RequireLowercase,
						},
						"require_number": ConfigFieldMeta{
							Type:        FieldTypeBool,
							Description: "Require number",
							Default:     defaults.Auth.Password.RequireNumber,
							Current:     current.Auth.Password.RequireNumber,
						},
						"require_special": ConfigFieldMeta{
							Type:        FieldTypeBool,
							Description: "Require special character",
							Default:     defaults.Auth.Password.RequireSpecial,
							Current:     current.Auth.Password.RequireSpecial,
						},
					},
				},
				"rate_limit": ConfigFieldMeta{
					Type:        FieldTypeObject,
					Description: "Rate limiting settings for auth endpoints",
					Fields: map[string]any{
						"login":          buildRateLimitFields(defaults.Auth.RateLimit.Login, current.Auth.RateLimit.Login),
						"register":       buildRateLimitFields(defaults.Auth.RateLimit.Register, current.Auth.RateLimit.Register),
						"password_reset": buildRateLimitFields(defaults.Auth.RateLimit.PasswordReset, current.Auth.RateLimit.PasswordReset),
					},
				},
				"oauth": ConfigFieldMeta{
					Type:        FieldTypeStringMap,
					Description: "OAuth providers (map of provider name to config)",
					Fields:      buildOAuthSchemaTemplate(),
					Current:     buildOAuthCurrentValues(current.Auth.OAuth),
				},
			},
		},
		"functions": {
			Name:        "Functions",
			Description: "Serverless functions settings",
			Fields: map[string]any{
				"enabled": ConfigFieldMeta{
					Type:        FieldTypeBool,
					Description: "Enable functions",
					Default:     defaults.Functions.Enabled,
					Current:     current.Functions.Enabled,
				},
				"path": ConfigFieldMeta{
					Type:        FieldTypeString,
					Description: "Path to functions directory",
					Default:     defaults.Functions.Path,
					Current:     current.Functions.Path,
				},
				"timeout": ConfigFieldMeta{
					Type:        FieldTypeDuration,
					Description: "Default execution timeout",
					Default:     formatDuration(defaults.Functions.Timeout),
					Current:     formatDuration(current.Functions.Timeout),
				},
				"env": ConfigFieldMeta{
					Type:        FieldTypeStringMap,
					Description: "Environment variables to pass to functions",
					Default:     defaults.Functions.Env,
					Current:     current.Functions.Env,
				},
			},
		},
		"realtime": {
			Name:        "Realtime",
			Description: "Real-time subscription settings",
			Fields: map[string]any{
				"enabled": ConfigFieldMeta{
					Type:        FieldTypeBool,
					Description: "Enable real-time subscriptions",
					Default:     defaults.Realtime.Enabled,
					Current:     current.Realtime.Enabled,
				},
				"poll_interval": ConfigFieldMeta{
					Type:        FieldTypeDuration,
					Description: "Polling interval for changes",
					Default:     formatDuration(defaults.Realtime.PollInterval),
					Current:     formatDuration(current.Realtime.PollInterval),
				},
				"max_connections": ConfigFieldMeta{
					Type:        FieldTypeInt,
					Description: "Maximum concurrent connections",
					Default:     defaults.Realtime.MaxConnections,
					Current:     current.Realtime.MaxConnections,
				},
				"max_subscriptions_per_client": ConfigFieldMeta{
					Type:        FieldTypeInt,
					Description: "Maximum subscriptions per client",
					Default:     defaults.Realtime.MaxSubscriptionsPerClient,
					Current:     current.Realtime.MaxSubscriptionsPerClient,
				},
				"change_buffer_size": ConfigFieldMeta{
					Type:        FieldTypeInt,
					Description: "Change buffer size",
					Default:     defaults.Realtime.ChangeBufferSize,
					Current:     current.Realtime.ChangeBufferSize,
				},
				"cleanup_interval": ConfigFieldMeta{
					Type:        FieldTypeDuration,
					Description: "Cleanup interval for stale connections",
					Default:     formatDuration(defaults.Realtime.CleanupInterval),
					Current:     formatDuration(current.Realtime.CleanupInterval),
				},
				"cleanup_age": ConfigFieldMeta{
					Type:        FieldTypeDuration,
					Description: "Age threshold for cleanup",
					Default:     formatDuration(defaults.Realtime.CleanupAge),
					Current:     formatDuration(current.Realtime.CleanupAge),
				},
			},
		},
		"logging": {
			Name:        "Logging",
			Description: "Logging settings",
			Fields: map[string]any{
				"level": ConfigFieldMeta{
					Type:        FieldTypeString,
					Description: "Log level",
					Default:     defaults.Logging.Level,
					Current:     current.Logging.Level,
					Options:     []string{"debug", "info", "warn", "error"},
				},
				"format": ConfigFieldMeta{
					Type:        FieldTypeString,
					Description: "Log format",
					Default:     defaults.Logging.Format,
					Current:     current.Logging.Format,
					Options:     []string{"json", "console"},
				},
				"caller": ConfigFieldMeta{
					Type:        FieldTypeBool,
					Description: "Include caller info",
					Default:     defaults.Logging.Caller,
					Current:     current.Logging.Caller,
				},
				"timestamp": ConfigFieldMeta{
					Type:        FieldTypeBool,
					Description: "Include timestamp",
					Default:     defaults.Logging.Timestamp,
					Current:     current.Logging.Timestamp,
				},
				"output": ConfigFieldMeta{
					Type:        FieldTypeString,
					Description: "Output file (empty for stdout)",
					Default:     defaults.Logging.Output,
					Current:     current.Logging.Output,
				},
			},
		},
		"dev": {
			Name:        "Development",
			Description: "Development mode settings",
			Fields: map[string]any{
				"enabled": ConfigFieldMeta{
					Type:        FieldTypeBool,
					Description: "Enable development mode",
					Default:     defaults.Dev.Enabled,
					Current:     current.Dev.Enabled,
				},
				"watch": ConfigFieldMeta{
					Type:        FieldTypeBool,
					Description: "Watch for file changes",
					Default:     defaults.Dev.Watch,
					Current:     current.Dev.Watch,
				},
				"auto_migrate": ConfigFieldMeta{
					Type:        FieldTypeBool,
					Description: "Auto-run migrations on schema changes",
					Default:     defaults.Dev.AutoMigrate,
					Current:     current.Dev.AutoMigrate,
				},
				"auto_generate": ConfigFieldMeta{
					Type:        FieldTypeBool,
					Description: "Auto-generate SDKs on schema changes",
					Default:     defaults.Dev.AutoGenerate,
					Current:     current.Dev.AutoGenerate,
				},
				"generate_languages": ConfigFieldMeta{
					Type:        FieldTypeStringArray,
					Description: "Languages to generate SDKs for",
					Default:     defaults.Dev.GenerateLanguages,
					Current:     current.Dev.GenerateLanguages,
				},
				"generate_output": ConfigFieldMeta{
					Type:        FieldTypeString,
					Description: "Output directory for generated SDKs",
					Default:     defaults.Dev.GenerateOutput,
					Current:     current.Dev.GenerateOutput,
				},
			},
		},
		"docs": {
			Name:        "Documentation",
			Description: "API documentation settings",
			Fields: map[string]any{
				"enabled": ConfigFieldMeta{
					Type:        FieldTypeBool,
					Description: "Enable API documentation",
					Default:     defaults.Docs.Enabled,
					Current:     current.Docs.Enabled,
				},
				"ui": ConfigFieldMeta{
					Type:        FieldTypeString,
					Description: "Documentation UI",
					Default:     defaults.Docs.UI,
					Current:     current.Docs.UI,
					Options:     []string{"scalar", "swagger", "redoc"},
				},
				"title": ConfigFieldMeta{
					Type:        FieldTypeString,
					Description: "API title",
					Default:     defaults.Docs.Title,
					Current:     current.Docs.Title,
				},
				"description": ConfigFieldMeta{
					Type:        FieldTypeString,
					Description: "API description",
					Default:     defaults.Docs.Description,
					Current:     current.Docs.Description,
				},
				"version": ConfigFieldMeta{
					Type:        FieldTypeString,
					Description: "API version",
					Default:     defaults.Docs.Version,
					Current:     current.Docs.Version,
				},
			},
		},
		"admin_ui": {
			Name:        "Admin UI",
			Description: "Admin UI settings",
			Fields: map[string]any{
				"enabled": ConfigFieldMeta{
					Type:        FieldTypeBool,
					Description: "Enable admin UI",
					Default:     defaults.AdminUI.Enabled,
					Current:     current.AdminUI.Enabled,
				},
				"path": ConfigFieldMeta{
					Type:        FieldTypeString,
					Description: "Admin UI path",
					Default:     defaults.AdminUI.Path,
					Current:     current.AdminUI.Path,
				},
			},
		},
		"storage": {
			Name:        "Storage",
			Description: "Storage backend settings",
			Fields: map[string]any{
				"backends": ConfigFieldMeta{
					Type:        FieldTypeStringMap,
					Description: "Named backend configurations (map of backend name to config)",
					Fields:      buildStorageSchemaTemplate(),
					Current:     buildStorageCurrentValues(current.Storage.Backends),
				},
			},
		},
	}

	return map[string]any{
		"sections": sections,
		"path":     configPath,
	}
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	// Convert to human-readable format
	if d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", d/(24*time.Hour))
	}
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", d/time.Hour)
	}
	if d%time.Minute == 0 {
		return fmt.Sprintf("%dm", d/time.Minute)
	}
	if d%time.Second == 0 {
		return fmt.Sprintf("%ds", d/time.Second)
	}
	if d%time.Millisecond == 0 {
		return fmt.Sprintf("%dms", d/time.Millisecond)
	}
	return d.String()
}

func getStringFromPtr[T any](ptr *T, getter func(*T) string) string {
	if ptr == nil {
		return ""
	}
	return getter(ptr)
}

func isSecretSet(secret string) any {
	if secret == "" {
		return ""
	}
	return "***SET***"
}

func buildRateLimitFields(defaultRule, currentRule RateLimitRule) map[string]any {
	return map[string]any{
		"max": ConfigFieldMeta{
			Type:        FieldTypeInt,
			Description: "Maximum requests",
			Default:     defaultRule.Max,
			Current:     currentRule.Max,
		},
		"window": ConfigFieldMeta{
			Type:        FieldTypeDuration,
			Description: "Time window",
			Default:     formatDuration(defaultRule.Window),
			Current:     formatDuration(currentRule.Window),
		},
	}
}

func buildOAuthSchemaTemplate() map[string]any {
	return map[string]any{
		"client_id": ConfigFieldMeta{
			Type:        FieldTypeString,
			Description: "Client ID",
			Default:     "",
		},
		"client_secret": ConfigFieldMeta{
			Type:        FieldTypeSecret,
			Description: "Client secret",
			Sensitive:   true,
			Default:     "",
		},
		"scopes": ConfigFieldMeta{
			Type:        FieldTypeStringArray,
			Description: "OAuth scopes",
			Default:     []string{},
		},
		"auth_url": ConfigFieldMeta{
			Type:        FieldTypeString,
			Description: "Custom authorization URL (for custom OIDC)",
			Default:     "",
		},
		"token_url": ConfigFieldMeta{
			Type:        FieldTypeString,
			Description: "Custom token URL (for custom OIDC)",
			Default:     "",
		},
		"user_info_url": ConfigFieldMeta{
			Type:        FieldTypeString,
			Description: "Custom user info URL (for custom OIDC)",
			Default:     "",
		},
	}
}

func buildOAuthCurrentValues(providers map[string]OAuthProviderConfig) map[string]any {
	if len(providers) == 0 {
		return map[string]any{}
	}

	result := make(map[string]any)
	for name, provider := range providers {
		result[name] = map[string]any{
			"client_id":     provider.ClientID,
			"client_secret": isSecretSet(provider.ClientSecret),
			"scopes":        provider.Scopes,
			"auth_url":      provider.AuthURL,
			"token_url":     provider.TokenURL,
			"user_info_url": provider.UserInfoURL,
		}
	}
	return result
}

func buildStorageSchemaTemplate() map[string]any {
	return map[string]any{
		"type": ConfigFieldMeta{
			Type:        FieldTypeString,
			Description: "Backend type",
			Default:     "",
			Options:     []string{"filesystem", "s3"},
		},
		"path": ConfigFieldMeta{
			Type:        FieldTypeString,
			Description: "Base path for storage (filesystem only)",
			Default:     "",
		},
		"base_path": ConfigFieldMeta{
			Type:        FieldTypeString,
			Description: "Base path prefix for all buckets (optional)",
			Default:     "",
		},
		"endpoint": ConfigFieldMeta{
			Type:        FieldTypeString,
			Description: "Custom endpoint for S3-compatible services (MinIO, R2, etc.)",
			Default:     "",
		},
		"region": ConfigFieldMeta{
			Type:        FieldTypeString,
			Description: "AWS region (e.g., us-east-1)",
			Default:     "",
		},
		"access_key_id": ConfigFieldMeta{
			Type:        FieldTypeSecret,
			Description: "Access key ID",
			Sensitive:   true,
			Default:     "",
		},
		"secret_access_key": ConfigFieldMeta{
			Type:        FieldTypeSecret,
			Description: "Secret access key",
			Sensitive:   true,
			Default:     "",
		},
		"bucket_prefix": ConfigFieldMeta{
			Type:        FieldTypeString,
			Description: "Bucket prefix for all buckets (optional)",
			Default:     "",
		},
		"force_path_style": ConfigFieldMeta{
			Type:        FieldTypeBool,
			Description: "Force path-style addressing (required for MinIO)",
			Default:     false,
		},
	}
}

func buildStorageCurrentValues(backends map[string]StorageBackendConfig) map[string]any {
	if len(backends) == 0 {
		return map[string]any{}
	}

	result := make(map[string]any)
	for name, backend := range backends {
		backendValues := map[string]any{
			"type":              backend.Type,
			"path":              "",
			"base_path":         "",
			"endpoint":          "",
			"region":            "",
			"access_key_id":     "",
			"secret_access_key": "",
			"bucket_prefix":     "",
			"force_path_style":  false,
		}

		if backend.Filesystem != nil {
			backendValues["path"] = backend.Filesystem.Path
			backendValues["base_path"] = backend.Filesystem.BasePath
		}

		if backend.S3 != nil {
			backendValues["endpoint"] = backend.S3.Endpoint
			backendValues["region"] = backend.S3.Region
			backendValues["access_key_id"] = isSecretSet(backend.S3.AccessKeyID)
			backendValues["secret_access_key"] = isSecretSet(backend.S3.SecretAccessKey)
			backendValues["bucket_prefix"] = backend.S3.BucketPrefix
			backendValues["force_path_style"] = backend.S3.ForcePathStyle
		}

		result[name] = backendValues
	}
	return result
}
