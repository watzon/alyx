package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

var (
	ErrConfigNotFound  = errors.New("config file not found")
	ErrInvalidConfig   = errors.New("invalid configuration")
	ErrMissingRequired = errors.New("missing required configuration")
)

type LoadOptions struct {
	ConfigFile string
	EnvPrefix  string
	Defaults   *Config
}

func Load(opts LoadOptions) (*Config, error) {
	v := viper.New()

	defaults := opts.Defaults
	if defaults == nil {
		defaults = Default()
	}
	setViperDefaults(v, defaults)

	if opts.EnvPrefix == "" {
		opts.EnvPrefix = "ALYX"
	}
	v.SetEnvPrefix(opts.EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if opts.ConfigFile != "" {
		v.SetConfigFile(opts.ConfigFile)
	} else {
		v.SetConfigName("alyx")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.config/alyx")
		v.AddConfigPath("/etc/alyx")
	}

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	expandEnvInConfig(v)

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func LoadFromFile(path string) (*Config, error) {
	return Load(LoadOptions{ConfigFile: path})
}

func LoadWithDefaults() (*Config, error) {
	return Load(LoadOptions{})
}

func setViperDefaults(v *viper.Viper, cfg *Config) {
	v.SetDefault("server.host", cfg.Server.Host)
	v.SetDefault("server.port", cfg.Server.Port)
	v.SetDefault("server.read_timeout", cfg.Server.ReadTimeout)
	v.SetDefault("server.write_timeout", cfg.Server.WriteTimeout)
	v.SetDefault("server.idle_timeout", cfg.Server.IdleTimeout)
	v.SetDefault("server.max_body_size", cfg.Server.MaxBodySize)

	v.SetDefault("server.cors.enabled", cfg.Server.CORS.Enabled)
	v.SetDefault("server.cors.allowed_origins", cfg.Server.CORS.AllowedOrigins)
	v.SetDefault("server.cors.exposed_headers", cfg.Server.CORS.ExposedHeaders)
	// CORS methods and headers are hard-coded (see CORSConfig methods)
	v.SetDefault("server.cors.allow_credentials", cfg.Server.CORS.AllowCredentials)
	v.SetDefault("server.cors.max_age", cfg.Server.CORS.MaxAge)

	v.SetDefault("database.path", cfg.Database.Path)
	// Database connection settings are hard-coded (see DatabaseConfig methods)

	v.SetDefault("auth.jwt.access_ttl", cfg.Auth.JWT.AccessTTL)
	v.SetDefault("auth.jwt.refresh_ttl", cfg.Auth.JWT.RefreshTTL)
	v.SetDefault("auth.jwt.issuer", cfg.Auth.JWT.Issuer)
	v.SetDefault("auth.password.min_length", cfg.Auth.Password.MinLength)
	v.SetDefault("auth.password.require_uppercase", cfg.Auth.Password.RequireUppercase)
	v.SetDefault("auth.password.require_lowercase", cfg.Auth.Password.RequireLowercase)
	v.SetDefault("auth.password.require_number", cfg.Auth.Password.RequireNumber)
	v.SetDefault("auth.password.require_special", cfg.Auth.Password.RequireSpecial)
	v.SetDefault("auth.rate_limit.login.max", cfg.Auth.RateLimit.Login.Max)
	v.SetDefault("auth.rate_limit.login.window", cfg.Auth.RateLimit.Login.Window)
	v.SetDefault("auth.rate_limit.register.max", cfg.Auth.RateLimit.Register.Max)
	v.SetDefault("auth.rate_limit.register.window", cfg.Auth.RateLimit.Register.Window)
	v.SetDefault("auth.rate_limit.password_reset.max", cfg.Auth.RateLimit.PasswordReset.Max)
	v.SetDefault("auth.rate_limit.password_reset.window", cfg.Auth.RateLimit.PasswordReset.Window)
	v.SetDefault("auth.allow_registration", cfg.Auth.AllowRegistration)
	v.SetDefault("auth.require_verification", cfg.Auth.RequireVerification)

	v.SetDefault("functions.enabled", cfg.Functions.Enabled)
	v.SetDefault("functions.path", cfg.Functions.Path)
	v.SetDefault("functions.runtime", cfg.Functions.Runtime)
	v.SetDefault("functions.timeout", cfg.Functions.Timeout)
	v.SetDefault("functions.memory_limit", cfg.Functions.MemoryLimit)
	v.SetDefault("functions.cpu_limit", cfg.Functions.CPULimit)

	v.SetDefault("logging.level", cfg.Logging.Level)
	v.SetDefault("logging.format", cfg.Logging.Format)
	v.SetDefault("logging.caller", cfg.Logging.Caller)
	v.SetDefault("logging.timestamp", cfg.Logging.Timestamp)

	v.SetDefault("dev.enabled", cfg.Dev.Enabled)
	v.SetDefault("dev.watch", cfg.Dev.Watch)
	v.SetDefault("dev.auto_migrate", cfg.Dev.AutoMigrate)
	v.SetDefault("dev.auto_generate", cfg.Dev.AutoGenerate)
	v.SetDefault("dev.generate_languages", cfg.Dev.GenerateLanguages)
	v.SetDefault("dev.generate_output", cfg.Dev.GenerateOutput)

	v.SetDefault("docs.enabled", cfg.Docs.Enabled)
	v.SetDefault("docs.ui", cfg.Docs.UI)
	v.SetDefault("docs.title", cfg.Docs.Title)
	v.SetDefault("docs.description", cfg.Docs.Description)
	v.SetDefault("docs.version", cfg.Docs.Version)

	v.SetDefault("admin_ui.enabled", cfg.AdminUI.Enabled)
	v.SetDefault("admin_ui.path", cfg.AdminUI.Path)
}

func expandEnvInConfig(v *viper.Viper) {
	for _, key := range v.AllKeys() {
		val := v.GetString(key)
		if strings.HasPrefix(val, "${") && strings.HasSuffix(val, "}") {
			envVar := val[2 : len(val)-1]
			if envVal := os.Getenv(envVar); envVal != "" {
				v.Set(key, envVal)
			}
		}
	}
}

func ConfigFilePath(customPath string) (string, error) {
	if customPath != "" {
		absPath, err := filepath.Abs(customPath)
		if err != nil {
			return "", fmt.Errorf("resolving config path: %w", err)
		}
		if _, err := os.Stat(absPath); err != nil {
			return "", fmt.Errorf("config file not found: %s", absPath)
		}
		return absPath, nil
	}

	searchPaths := []string{
		"alyx.yaml",
		"alyx.yml",
		filepath.Join(os.Getenv("HOME"), ".config", "alyx", "alyx.yaml"),
		"/etc/alyx/alyx.yaml",
	}

	for _, p := range searchPaths {
		if _, err := os.Stat(p); err == nil {
			return filepath.Abs(p)
		}
	}

	return "", ErrConfigNotFound
}
