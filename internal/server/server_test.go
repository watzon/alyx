package server

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:        "localhost",
			Port:        0,
			MaxBodySize: 1024 * 1024,
			CORS: config.CORSConfig{
				Enabled:        false,
				AllowedOrigins: []string{"*"},
			},
		},
		Database: config.DatabaseConfig{
			Path: dbPath,
		},
		Auth: config.AuthConfig{
			RateLimit: config.AuthRateLimitConfig{
				Login: config.RateLimitRule{
					Max:    5,
					Window: time.Minute,
				},
				Register: config.RateLimitRule{
					Max:    3,
					Window: time.Minute,
				},
			},
		},
		Realtime: config.RealtimeConfig{
			Enabled: false,
		},
		Functions: config.FunctionsConfig{
			Enabled: false,
		},
		AdminUI: config.AdminUIConfig{
			Enabled: false,
		},
		Docs: config.DocsConfig{
			Enabled: false,
		},
	}

	db, err := database.Open(&cfg.Database)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	schemaYAML := `
version: 1
collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      name:
        type: string
      email:
        type: string
        unique: true
`
	s, err := schema.Parse([]byte(schemaYAML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("execute DDL: %v", err)
		}
	}

	server := New(cfg, db, s)

	return server
}

func TestServer_New(t *testing.T) {
	server := setupTestServer(t)

	if server == nil {
		t.Fatal("expected server to be created")
	}

	if server.db == nil {
		t.Error("expected database to be initialized")
	}

	if server.schema == nil {
		t.Error("expected schema to be initialized")
	}

	if server.router == nil {
		t.Error("expected router to be initialized")
	}

	if server.httpServer == nil {
		t.Error("expected http server to be initialized")
	}

	if server.loginLimiter == nil {
		t.Error("expected login limiter to be initialized")
	}

	if server.registerLimiter == nil {
		t.Error("expected register limiter to be initialized")
	}
}

func TestServer_StartStop(t *testing.T) {
	server := setupTestServer(t)

	server.cfg.Server.Port = 0
	server.httpServer.Addr = server.cfg.Server.Address()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	time.Sleep(200 * time.Millisecond)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Errorf("shutdown failed: %v", err)
	}

	cancel()

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("unexpected server error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("server did not shut down in time")
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	server := setupTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())

	go server.Start(ctx)
	time.Sleep(100 * time.Millisecond)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()

	cancel()

	err := server.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("shutdown failed: %v", err)
	}
}

func TestServer_GetCollection(t *testing.T) {
	server := setupTestServer(t)

	coll, err := server.GetCollection("users")
	if err != nil {
		t.Fatalf("expected to get collection, got error: %v", err)
	}

	if coll == nil {
		t.Fatal("expected collection to be non-nil")
	}

	_, err = server.GetCollection("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent collection")
	}
}

func TestServer_UpdateSchema(t *testing.T) {
	server := setupTestServer(t)

	newSchemaYAML := `
version: 1
collections:
  posts:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      title:
        type: string
`
	newSchema, err := schema.Parse([]byte(newSchemaYAML))
	if err != nil {
		t.Fatalf("parse new schema: %v", err)
	}

	err = server.UpdateSchema(newSchema)
	if err != nil {
		t.Errorf("expected no error updating schema, got: %v", err)
	}

	if server.Schema() != newSchema {
		t.Error("schema was not updated")
	}
}

func TestServer_Accessors(t *testing.T) {
	server := setupTestServer(t)

	tests := []struct {
		name     string
		accessor func() interface{}
		wantNil  bool
	}{
		{"DB", func() interface{} { return server.DB() }, false},
		{"Schema", func() interface{} { return server.Schema() }, false},
		{"Config", func() interface{} { return server.Config() }, false},
		{"Broker", func() interface{} { return server.Broker() }, true},
		{"Rules", func() interface{} { return server.Rules() }, true},
		{"FuncService", func() interface{} { return server.FuncService() }, true},
		{"StorageService", func() interface{} { return server.StorageService() }, true},
		{"DeployService", func() interface{} { return server.DeployService() }, false},
		{"RequestLogs", func() interface{} { return server.RequestLogs() }, false},
		{"LoginLimiter", func() interface{} { return server.LoginLimiter() }, false},
		{"RegisterLimiter", func() interface{} { return server.RegisterLimiter() }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.accessor()
			if tt.wantNil {
				return
			}
			if result == nil {
				t.Errorf("%s should not be nil", tt.name)
			}
		})
	}
}

func TestServer_WithOptions(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:        "localhost",
			Port:        0,
			MaxBodySize: 1024 * 1024,
			CORS: config.CORSConfig{
				Enabled:        false,
				AllowedOrigins: []string{"*"},
			},
		},
		Database: config.DatabaseConfig{
			Path: dbPath,
		},
		Auth: config.AuthConfig{
			RateLimit: config.AuthRateLimitConfig{
				Login: config.RateLimitRule{
					Max:    5,
					Window: time.Minute,
				},
				Register: config.RateLimitRule{
					Max:    3,
					Window: time.Minute,
				},
			},
		},
		Realtime: config.RealtimeConfig{
			Enabled: false,
		},
		Functions: config.FunctionsConfig{
			Enabled: false,
		},
		AdminUI: config.AdminUIConfig{
			Enabled: false,
		},
	}

	db, err := database.Open(&cfg.Database)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	schemaYAML := `
version: 1
collections:
  test:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
`
	s, err := schema.Parse([]byte(schemaYAML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	customSchemaPath := "custom/schema.yaml"
	customConfigPath := "custom/config.yaml"

	server := New(
		cfg,
		db,
		s,
		WithSchemaPath(customSchemaPath),
		WithConfigPath(customConfigPath),
	)

	if server.SchemaPath() != customSchemaPath {
		t.Errorf("expected schema path %q, got %q", customSchemaPath, server.SchemaPath())
	}

	if server.ConfigPath() != customConfigPath {
		t.Errorf("expected config path %q, got %q", customConfigPath, server.ConfigPath())
	}
}
