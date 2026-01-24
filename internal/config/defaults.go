package config

import "time"

// Default configuration values.
const (
	// Server defaults.
	DefaultHost         = "localhost"
	DefaultPort         = 8090
	DefaultReadTimeout  = 30 * time.Second
	DefaultWriteTimeout = 30 * time.Second
	DefaultIdleTimeout  = 120 * time.Second
	DefaultMaxBodySize  = 10 * 1024 * 1024 // 10MB

	// Database defaults.
	DefaultDBPath       = "alyx.db"
	DefaultCacheSize    = -64000 // 64MB
	DefaultBusyTimeout  = 5 * time.Second
	DefaultMaxOpenConns = 1 // SQLite works best with single writer
	DefaultMaxIdleConns = 1

	// Auth defaults.
	DefaultAccessTTL      = 15 * time.Minute
	DefaultRefreshTTL     = 7 * 24 * time.Hour // 7 days
	DefaultJWTIssuer      = "alyx"
	DefaultMinPassword    = 8
	DefaultLoginRateLimit = 5
	DefaultLoginWindow    = time.Minute

	// Functions defaults.
	DefaultFunctionsPath   = "functions"
	DefaultFunctionTimeout = 30 * time.Second
	DefaultMemoryLimit     = 256 // MB
	DefaultCPULimit        = 1.0
	DefaultMinWarm         = 1
	DefaultMaxInstances    = 10
	DefaultIdlePoolTimeout = 60 * time.Second

	// Logging defaults.
	DefaultLogLevel  = "info"
	DefaultLogFormat = "console"

	// Realtime defaults.
	DefaultPollInterval              = 50 * time.Millisecond
	DefaultMaxConnections            = 1000
	DefaultMaxSubscriptionsPerClient = 100
	DefaultChangeBufferSize          = 1000
	DefaultCleanupInterval           = 5 * time.Minute
	DefaultCleanupAge                = time.Hour
)

// Default returns a Config with sensible defaults.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         DefaultHost,
			Port:         DefaultPort,
			ReadTimeout:  DefaultReadTimeout,
			WriteTimeout: DefaultWriteTimeout,
			IdleTimeout:  DefaultIdleTimeout,
			MaxBodySize:  DefaultMaxBodySize,
			AdminUI:      true,
			CORS: CORSConfig{
				Enabled:          true,
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
				AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
				ExposedHeaders:   []string{"X-Request-ID"},
				AllowCredentials: false,
				MaxAge:           12 * time.Hour,
			},
		},
		Database: DatabaseConfig{
			Path:            DefaultDBPath,
			WALMode:         true,
			CacheSize:       DefaultCacheSize,
			BusyTimeout:     DefaultBusyTimeout,
			ForeignKeys:     true,
			MaxOpenConns:    DefaultMaxOpenConns,
			MaxIdleConns:    DefaultMaxIdleConns,
			ConnMaxLifetime: 0, // No limit
		},
		Auth: AuthConfig{
			JWT: JWTConfig{
				AccessTTL:  DefaultAccessTTL,
				RefreshTTL: DefaultRefreshTTL,
				Issuer:     DefaultJWTIssuer,
			},
			Password: PasswordConfig{
				MinLength:        DefaultMinPassword,
				RequireUppercase: false,
				RequireLowercase: false,
				RequireNumber:    false,
				RequireSpecial:   false,
			},
			RateLimit: AuthRateLimitConfig{
				Login: RateLimitRule{
					Max:    DefaultLoginRateLimit,
					Window: DefaultLoginWindow,
				},
				Register: RateLimitRule{
					Max:    3,
					Window: time.Minute,
				},
				PasswordReset: RateLimitRule{
					Max:    3,
					Window: time.Hour,
				},
			},
			AllowRegistration:   true,
			RequireVerification: false,
			OAuth:               make(map[string]OAuthProviderConfig),
		},
		Functions: FunctionsConfig{
			Enabled:     true,
			Path:        DefaultFunctionsPath,
			Runtime:     "docker",
			Timeout:     DefaultFunctionTimeout,
			MemoryLimit: DefaultMemoryLimit,
			CPULimit:    DefaultCPULimit,
			Pools: map[string]PoolConfig{
				"node": {
					MinWarm:      DefaultMinWarm,
					MaxInstances: DefaultMaxInstances,
					IdleTimeout:  DefaultIdlePoolTimeout,
					Image:        "ghcr.io/watzon/alyx-runtime-node:latest",
				},
				"python": {
					MinWarm:      DefaultMinWarm,
					MaxInstances: DefaultMaxInstances,
					IdleTimeout:  DefaultIdlePoolTimeout,
					Image:        "ghcr.io/watzon/alyx-runtime-python:latest",
				},
				"go": {
					MinWarm:      0, // Go compiles, so no warm pool needed
					MaxInstances: DefaultMaxInstances,
					IdleTimeout:  DefaultIdlePoolTimeout,
					Image:        "ghcr.io/watzon/alyx-runtime-go:latest",
				},
			},
			Env: make(map[string]string),
		},
		Logging: LoggingConfig{
			Level:     DefaultLogLevel,
			Format:    DefaultLogFormat,
			Caller:    false,
			Timestamp: true,
		},
		Dev: DevConfig{
			Enabled:           false,
			Watch:             true,
			AutoMigrate:       true,
			AutoGenerate:      true,
			GenerateLanguages: []string{"typescript"},
			GenerateOutput:    "generated",
		},
		Docs: DocsConfig{
			Enabled:     true,
			UI:          "scalar",
			Title:       "Alyx API",
			Description: "Auto-generated API documentation",
			Version:     "1.0.0",
		},
		Realtime: RealtimeConfig{
			Enabled:                   true,
			PollInterval:              DefaultPollInterval,
			MaxConnections:            DefaultMaxConnections,
			MaxSubscriptionsPerClient: DefaultMaxSubscriptionsPerClient,
			ChangeBufferSize:          DefaultChangeBufferSize,
			CleanupInterval:           DefaultCleanupInterval,
			CleanupAge:                DefaultCleanupAge,
		},
		AdminUI: AdminUIConfig{
			Enabled: true,
			Path:    "/_admin",
		},
	}
}
