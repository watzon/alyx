package config

import (
	"fmt"
	"strings"
	"time"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("configuration validation failed:\n")
	for _, err := range e {
		sb.WriteString("  - ")
		sb.WriteString(err.Error())
		sb.WriteString("\n")
	}
	return sb.String()
}

func Validate(cfg *Config) error {
	var errs ValidationErrors

	errs = append(errs, validateServer(&cfg.Server)...)
	errs = append(errs, validateDatabase(&cfg.Database)...)
	errs = append(errs, validateAuth(&cfg.Auth)...)
	errs = append(errs, validateFunctions(&cfg.Functions)...)
	errs = append(errs, validateLogging(&cfg.Logging)...)
	errs = append(errs, validateDocs(&cfg.Docs)...)
	errs = append(errs, validateRealtime(&cfg.Realtime)...)
	errs = append(errs, validateAdminUI(&cfg.AdminUI)...)
	errs = append(errs, validateStorage(&cfg.Storage)...)

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func validateServer(cfg *ServerConfig) ValidationErrors {
	var errs ValidationErrors

	if cfg.Port < 1 || cfg.Port > 65535 {
		errs = append(errs, ValidationError{
			Field:   "server.port",
			Message: "must be between 1 and 65535",
		})
	}

	if cfg.ReadTimeout < 0 {
		errs = append(errs, ValidationError{
			Field:   "server.read_timeout",
			Message: "must be non-negative",
		})
	}

	if cfg.ReadTimeout > 0 && cfg.ReadTimeout < time.Second {
		errs = append(errs, ValidationError{
			Field:   "server.read_timeout",
			Message: "warning: values below 1s may cause legitimate requests to timeout",
		})
	}

	if cfg.WriteTimeout < 0 {
		errs = append(errs, ValidationError{
			Field:   "server.write_timeout",
			Message: "must be non-negative",
		})
	}

	if cfg.WriteTimeout > 0 && cfg.WriteTimeout < time.Second {
		errs = append(errs, ValidationError{
			Field:   "server.write_timeout",
			Message: "warning: values below 1s may cause legitimate requests to timeout",
		})
	}

	if cfg.MaxBodySize < 0 {
		errs = append(errs, ValidationError{
			Field:   "server.max_body_size",
			Message: "must be non-negative",
		})
	}

	if cfg.CORS.Enabled && cfg.CORS.AllowCredentials {
		for _, origin := range cfg.CORS.AllowedOrigins {
			if origin == "*" {
				errs = append(errs, ValidationError{
					Field:   "server.cors",
					Message: "security: allow_credentials=true with allowed_origins=[\"*\"] is insecure",
				})
				break
			}
		}
	}

	if cfg.TLS != nil && cfg.TLS.Enabled {
		if !cfg.TLS.AutoTLS {
			if cfg.TLS.CertFile == "" {
				errs = append(errs, ValidationError{
					Field:   "server.tls.cert_file",
					Message: "required when TLS is enabled without auto_tls",
				})
			}
			if cfg.TLS.KeyFile == "" {
				errs = append(errs, ValidationError{
					Field:   "server.tls.key_file",
					Message: "required when TLS is enabled without auto_tls",
				})
			}
		} else if cfg.TLS.Domain == "" {
			errs = append(errs, ValidationError{
				Field:   "server.tls.domain",
				Message: "required when auto_tls is enabled",
			})
		}
	}

	return errs
}

func validateDatabase(cfg *DatabaseConfig) ValidationErrors {
	var errs ValidationErrors

	if cfg.Path == "" {
		errs = append(errs, ValidationError{
			Field:   "database.path",
			Message: "required",
		})
	}

	// Database connection settings are hard-coded (see DatabaseConfig methods)

	if cfg.Turso != nil && cfg.Turso.Enabled {
		if cfg.Turso.URL == "" {
			errs = append(errs, ValidationError{
				Field:   "database.turso.url",
				Message: "required when Turso is enabled",
			})
		}
		if cfg.Turso.AuthToken == "" {
			errs = append(errs, ValidationError{
				Field:   "database.turso.auth_token",
				Message: "required when Turso is enabled",
			})
		}
	}

	return errs
}

func validateAuth(cfg *AuthConfig) ValidationErrors {
	var errs ValidationErrors

	if cfg.JWT.AccessTTL < time.Second {
		errs = append(errs, ValidationError{
			Field:   "auth.jwt.access_ttl",
			Message: "must be at least 1 second",
		})
	}

	if cfg.JWT.RefreshTTL < cfg.JWT.AccessTTL {
		errs = append(errs, ValidationError{
			Field:   "auth.jwt.refresh_ttl",
			Message: "must be greater than or equal to access_ttl",
		})
	}

	if cfg.Password.MinLength < 8 {
		errs = append(errs, ValidationError{
			Field:   "auth.password.min_length",
			Message: "must be at least 8 for security",
		})
	}

	if cfg.RateLimit.Login.Max < 1 {
		errs = append(errs, ValidationError{
			Field:   "auth.rate_limit.login.max",
			Message: "must be at least 1",
		})
	}

	for name, provider := range cfg.OAuth {
		if provider.ClientID == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("auth.oauth.%s.client_id", name),
				Message: "required",
			})
		}
		if provider.ClientSecret == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("auth.oauth.%s.client_secret", name),
				Message: "required",
			})
		}
	}

	return errs
}

func validateFunctions(cfg *FunctionsConfig) ValidationErrors {
	var errs ValidationErrors

	if !cfg.Enabled {
		return errs
	}

	if cfg.Path == "" {
		errs = append(errs, ValidationError{
			Field:   "functions.path",
			Message: "required when functions are enabled",
		})
	}

	validRuntimes := map[string]bool{"docker": true, "podman": true}
	if !validRuntimes[cfg.Runtime] {
		errs = append(errs, ValidationError{
			Field:   "functions.runtime",
			Message: "must be 'docker' or 'podman'",
		})
	}

	if cfg.Timeout < time.Second {
		errs = append(errs, ValidationError{
			Field:   "functions.timeout",
			Message: "must be at least 1 second",
		})
	}

	if cfg.MemoryLimit < 64 {
		errs = append(errs, ValidationError{
			Field:   "functions.memory_limit",
			Message: "must be at least 64 MB",
		})
	}

	if cfg.CPULimit < 0.1 {
		errs = append(errs, ValidationError{
			Field:   "functions.cpu_limit",
			Message: "must be at least 0.1 cores",
		})
	}

	for name, pool := range cfg.Pools {
		if pool.MaxInstances < 1 {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("functions.pools.%s.max_instances", name),
				Message: "must be at least 1",
			})
		}
		if pool.MinWarm < 0 {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("functions.pools.%s.min_warm", name),
				Message: "must be non-negative",
			})
		}
		if pool.MinWarm > pool.MaxInstances {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("functions.pools.%s.min_warm", name),
				Message: "must not exceed max_instances",
			})
		}
		if pool.Image == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("functions.pools.%s.image", name),
				Message: "required",
			})
		}
	}

	return errs
}

func validateLogging(cfg *LoggingConfig) ValidationErrors {
	var errs ValidationErrors

	validLevels := map[string]bool{
		"trace": true, "debug": true, "info": true,
		"warn": true, "error": true, "fatal": true, "panic": true,
	}
	if !validLevels[cfg.Level] {
		errs = append(errs, ValidationError{
			Field:   "logging.level",
			Message: "must be one of: trace, debug, info, warn, error, fatal, panic",
		})
	}

	validFormats := map[string]bool{"json": true, "console": true}
	if !validFormats[cfg.Format] {
		errs = append(errs, ValidationError{
			Field:   "logging.format",
			Message: "must be 'json' or 'console'",
		})
	}

	return errs
}

func validateDocs(cfg *DocsConfig) ValidationErrors {
	var errs ValidationErrors

	if !cfg.Enabled {
		return errs
	}

	validUIs := map[string]bool{
		"scalar": true, "swagger": true, "redoc": true, "stoplight": true,
	}
	if !validUIs[cfg.UI] {
		errs = append(errs, ValidationError{
			Field:   "docs.ui",
			Message: "must be one of: scalar, swagger, redoc, stoplight",
		})
	}

	return errs
}

func validateRealtime(cfg *RealtimeConfig) ValidationErrors {
	var errs ValidationErrors

	if !cfg.Enabled {
		return errs
	}

	if cfg.PollInterval < 10*time.Millisecond {
		errs = append(errs, ValidationError{
			Field:   "realtime.poll_interval",
			Message: "must be at least 10ms to prevent high CPU usage",
		})
	}

	if cfg.PollInterval < 50*time.Millisecond {
		errs = append(errs, ValidationError{
			Field:   "realtime.poll_interval",
			Message: "warning: values below 50ms may cause elevated CPU usage",
		})
	}

	if cfg.MaxConnections < 1 {
		errs = append(errs, ValidationError{
			Field:   "realtime.max_connections",
			Message: "must be at least 1",
		})
	}

	if cfg.MaxSubscriptionsPerClient < 1 {
		errs = append(errs, ValidationError{
			Field:   "realtime.max_subscriptions_per_client",
			Message: "must be at least 1",
		})
	}

	if cfg.ChangeBufferSize < 1 {
		errs = append(errs, ValidationError{
			Field:   "realtime.change_buffer_size",
			Message: "must be at least 1",
		})
	}

	if cfg.CleanupInterval < time.Minute {
		errs = append(errs, ValidationError{
			Field:   "realtime.cleanup_interval",
			Message: "must be at least 1 minute",
		})
	}

	if cfg.CleanupAge < time.Minute {
		errs = append(errs, ValidationError{
			Field:   "realtime.cleanup_age",
			Message: "must be at least 1 minute",
		})
	}

	return errs
}

func validateAdminUI(cfg *AdminUIConfig) ValidationErrors {
	var errs ValidationErrors

	if !cfg.Enabled {
		return errs
	}

	if cfg.Path == "" {
		errs = append(errs, ValidationError{
			Field:   "admin_ui.path",
			Message: "required when admin UI is enabled",
		})
	}

	if cfg.Path != "" && cfg.Path[0] != '/' {
		errs = append(errs, ValidationError{
			Field:   "admin_ui.path",
			Message: "must start with /",
		})
	}

	reservedPaths := map[string]bool{
		"/api": true, "/auth": true, "/functions": true,
		"/docs": true, "/health": true, "/metrics": true,
	}
	if reservedPaths[cfg.Path] {
		errs = append(errs, ValidationError{
			Field:   "admin_ui.path",
			Message: fmt.Sprintf("path '%s' is reserved for API routes", cfg.Path),
		})
	}

	return errs
}

func validateStorage(cfg *StorageConfig) ValidationErrors {
	var errs ValidationErrors

	for name, backend := range cfg.Backends {
		if backend.Type == "" {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("storage.backends.%s.type", name),
				Message: "required (must be 'filesystem' or 's3')",
			})
			continue
		}

		validTypes := map[string]bool{"filesystem": true, "s3": true}
		if !validTypes[backend.Type] {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("storage.backends.%s.type", name),
				Message: "must be 'filesystem' or 's3'",
			})
		}

		switch backend.Type {
		case "filesystem":
			if backend.Filesystem == nil {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("storage.backends.%s.filesystem", name),
					Message: "required when type is 'filesystem'",
				})
				break
			}

			if backend.Filesystem.Path == "" {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("storage.backends.%s.filesystem.path", name),
					Message: "required",
				})
			}

			if strings.Contains(backend.Filesystem.Path, "..") {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("storage.backends.%s.filesystem.path", name),
					Message: "path traversal (..) not allowed",
				})
			}

			if strings.Contains(backend.Filesystem.BasePath, "..") || strings.Contains(backend.Filesystem.BasePath, "/") {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("storage.backends.%s.filesystem.base_path", name),
					Message: "must not contain path separators or traversal (..)",
				})
			}

		case "s3":
			if backend.S3 == nil {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("storage.backends.%s.s3", name),
					Message: "required when type is 's3'",
				})
				break
			}

			if backend.S3.Region == "" {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("storage.backends.%s.s3.region", name),
					Message: "required",
				})
			}

			if backend.S3.AccessKeyID == "" {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("storage.backends.%s.s3.access_key_id", name),
					Message: "required",
				})
			}

			if backend.S3.SecretAccessKey == "" {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("storage.backends.%s.s3.secret_access_key", name),
					Message: "required",
				})
			}

			if strings.Contains(backend.S3.BucketPrefix, "/") {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("storage.backends.%s.s3.bucket_prefix", name),
					Message: "must not contain path separators",
				})
			}
		}
	}

	return errs
}

func ValidateJWTSecret(secret string) error {
	if secret == "" {
		return &ValidationError{
			Field:   "auth.jwt.secret",
			Message: "required for production use",
		}
	}
	if len(secret) < 32 {
		return &ValidationError{
			Field:   "auth.jwt.secret",
			Message: "must be at least 32 characters",
		}
	}
	return nil
}
