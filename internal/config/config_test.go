package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Server.Port != DefaultPort {
		t.Errorf("expected port %d, got %d", DefaultPort, cfg.Server.Port)
	}

	if cfg.Database.Path != DefaultDBPath {
		t.Errorf("expected db path %s, got %s", DefaultDBPath, cfg.Database.Path)
	}

	if cfg.Auth.JWT.AccessTTL != DefaultAccessTTL {
		t.Errorf("expected access TTL %v, got %v", DefaultAccessTTL, cfg.Auth.JWT.AccessTTL)
	}

	if !cfg.Functions.Enabled {
		t.Error("expected functions to be enabled by default")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := Default()
	if err := Validate(cfg); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := Default()
	cfg.Server.Port = 0

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for invalid port")
	}

	errs, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	found := false
	for _, e := range errs {
		if e.Field == "server.port" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for server.port field")
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := Default()
	cfg.Logging.Level = "invalid"

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for invalid log level")
	}
}

func TestValidate_TLSWithoutCert(t *testing.T) {
	cfg := Default()
	cfg.Server.TLS = &TLSConfig{
		Enabled: true,
		AutoTLS: false,
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for TLS without cert")
	}
}

func TestValidate_InvalidFunctionRuntime(t *testing.T) {
	cfg := Default()
	cfg.Functions.Runtime = "invalid"

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for invalid runtime")
	}
}

func TestValidateJWTSecret(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{"empty", "", true},
		{"too short", "short", true},
		{"valid", "this-is-a-very-long-secret-key-for-jwt-signing", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJWTSecret(tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJWTSecret() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "alyx.yaml")

	content := `
server:
  port: 9000
  host: "0.0.0.0"
database:
  path: "test.db"
logging:
  level: "debug"
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Server.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Server.Port)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Server.Host)
	}

	if cfg.Database.Path != "test.db" {
		t.Errorf("expected db path test.db, got %s", cfg.Database.Path)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level debug, got %s", cfg.Logging.Level)
	}
}

func TestLoadWithEnvOverride(t *testing.T) {
	t.Setenv("ALYX_SERVER_PORT", "7777")
	t.Setenv("ALYX_DATABASE_PATH", "env-test.db")

	cfg, err := LoadWithDefaults()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Server.Port != 7777 {
		t.Errorf("expected port 7777 from env, got %d", cfg.Server.Port)
	}

	if cfg.Database.Path != "env-test.db" {
		t.Errorf("expected db path env-test.db from env, got %s", cfg.Database.Path)
	}
}

func TestServerAddress(t *testing.T) {
	cfg := &ServerConfig{Host: "localhost", Port: 8090}
	if addr := cfg.Address(); addr != "localhost:8090" {
		t.Errorf("expected localhost:8090, got %s", addr)
	}
}

func TestValidate_OAuthProvider(t *testing.T) {
	cfg := Default()
	cfg.Auth.OAuth["github"] = OAuthProviderConfig{
		ClientID:     "",
		ClientSecret: "secret",
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for missing OAuth client_id")
	}
}

func TestValidate_FunctionPoolConfig(t *testing.T) {
	cfg := Default()
	cfg.Functions.Pools["test"] = PoolConfig{
		MinWarm:      5,
		MaxInstances: 2,
		IdleTimeout:  time.Minute,
		Image:        "test:latest",
	}

	err := Validate(cfg)
	if err == nil {
		t.Error("expected validation error for min_warm > max_instances")
	}
}
