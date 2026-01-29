package hooks

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
)

type mockFunctionChecker struct {
	functions map[string]bool
}

func (m *mockFunctionChecker) FunctionExists(name string) bool {
	if m.functions == nil {
		return false
	}
	return m.functions[name]
}

func setupTestDB(t *testing.T) *database.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.DatabaseConfig{
		Path: dbPath,
	}

	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	return db
}

func TestRegistry_Create(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	funcChecker := &mockFunctionChecker{
		functions: map[string]bool{"test-function": true},
	}
	registry := NewRegistry(db, funcChecker)

	tests := []struct {
		name    string
		hook    *Hook
		wantErr bool
	}{
		{
			name: "valid database hook",
			hook: &Hook{
				Type:         HookTypeDatabase,
				Source:       "users",
				Action:       "insert",
				FunctionName: "test-function",
				Mode:         HookModeAsync,
				Enabled:      true,
			},
			wantErr: false,
		},
		{
			name: "valid webhook hook",
			hook: &Hook{
				Type:         HookTypeWebhook,
				Source:       "/webhook/test",
				FunctionName: "test-function",
				Mode:         HookModeSync,
				Enabled:      true,
			},
			wantErr: false,
		},
		{
			name: "invalid - missing action for database hook",
			hook: &Hook{
				Type:         HookTypeDatabase,
				Source:       "users",
				FunctionName: "test-function",
				Mode:         HookModeAsync,
				Enabled:      true,
			},
			wantErr: true,
		},
		{
			name: "invalid - webhook with action",
			hook: &Hook{
				Type:         HookTypeWebhook,
				Source:       "/webhook/test",
				Action:       "insert",
				FunctionName: "test-function",
				Mode:         HookModeAsync,
				Enabled:      true,
			},
			wantErr: true,
		},
		{
			name: "invalid - function not found",
			hook: &Hook{
				Type:         HookTypeDatabase,
				Source:       "users",
				Action:       "insert",
				FunctionName: "nonexistent-function",
				Mode:         HookModeAsync,
				Enabled:      true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.Create(ctx, tt.hook)
			if (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if tt.hook.ID == "" {
					t.Error("Expected hook ID to be generated")
				}
				if tt.hook.CreatedAt.IsZero() {
					t.Error("Expected CreatedAt to be set")
				}
			}
		})
	}
}

func TestRegistry_Update(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	funcChecker := &mockFunctionChecker{
		functions: map[string]bool{
			"test-function":  true,
			"other-function": true,
		},
	}
	registry := NewRegistry(db, funcChecker)

	hook := &Hook{
		Type:         HookTypeDatabase,
		Source:       "users",
		Action:       "insert",
		FunctionName: "test-function",
		Mode:         HookModeAsync,
		Enabled:      true,
	}

	if err := registry.Create(ctx, hook); err != nil {
		t.Fatalf("Failed to create hook: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	hook.Mode = HookModeSync
	hook.FunctionName = "other-function"

	if err := registry.Update(ctx, hook.ID, hook); err != nil {
		t.Fatalf("Failed to update hook: %v", err)
	}

	retrieved, err := registry.Get(ctx, hook.ID)
	if err != nil {
		t.Fatalf("Failed to get hook: %v", err)
	}

	if retrieved.Mode != HookModeSync {
		t.Errorf("Expected mode to be sync, got %s", retrieved.Mode)
	}
	if retrieved.FunctionName != "other-function" {
		t.Errorf("Expected function_name to be other-function, got %s", retrieved.FunctionName)
	}
	if retrieved.UpdatedAt.Before(retrieved.CreatedAt) {
		t.Error("Expected UpdatedAt to be equal or after CreatedAt")
	}
}

func TestRegistry_Delete(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	funcChecker := &mockFunctionChecker{
		functions: map[string]bool{"test-function": true},
	}
	registry := NewRegistry(db, funcChecker)

	hook := &Hook{
		Type:         HookTypeDatabase,
		Source:       "users",
		Action:       "insert",
		FunctionName: "test-function",
		Mode:         HookModeAsync,
		Enabled:      true,
	}

	if err := registry.Create(ctx, hook); err != nil {
		t.Fatalf("Failed to create hook: %v", err)
	}

	if err := registry.Delete(ctx, hook.ID); err != nil {
		t.Fatalf("Failed to delete hook: %v", err)
	}

	_, err := registry.Get(ctx, hook.ID)
	if err == nil {
		t.Error("Expected error when getting deleted hook")
	}
}

func TestRegistry_EnableDisable(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	funcChecker := &mockFunctionChecker{
		functions: map[string]bool{"test-function": true},
	}
	registry := NewRegistry(db, funcChecker)

	hook := &Hook{
		Type:         HookTypeDatabase,
		Source:       "users",
		Action:       "insert",
		FunctionName: "test-function",
		Mode:         HookModeAsync,
		Enabled:      true,
	}

	if err := registry.Create(ctx, hook); err != nil {
		t.Fatalf("Failed to create hook: %v", err)
	}

	if err := registry.Disable(ctx, hook.ID); err != nil {
		t.Fatalf("Failed to disable hook: %v", err)
	}

	retrieved, err := registry.Get(ctx, hook.ID)
	if err != nil {
		t.Fatalf("Failed to get hook: %v", err)
	}
	if retrieved.Enabled {
		t.Error("Expected hook to be disabled")
	}

	err = registry.Enable(ctx, hook.ID)
	if err != nil {
		t.Fatalf("Failed to enable hook: %v", err)
	}

	retrieved, err = registry.Get(ctx, hook.ID)
	if err != nil {
		t.Fatalf("Failed to get hook: %v", err)
	}
	if !retrieved.Enabled {
		t.Error("Expected hook to be enabled")
	}
}

func TestRegistry_List(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	funcChecker := &mockFunctionChecker{
		functions: map[string]bool{"test-function": true},
	}
	registry := NewRegistry(db, funcChecker)

	hooks := []*Hook{
		{
			Type:         HookTypeDatabase,
			Source:       "users",
			Action:       "insert",
			FunctionName: "test-function",
			Mode:         HookModeAsync,
			Enabled:      true,
		},
		{
			Type:         HookTypeWebhook,
			Source:       "/webhook/test",
			FunctionName: "test-function",
			Mode:         HookModeSync,
			Enabled:      true,
		},
		{
			Type:         HookTypeSchedule,
			Source:       "daily-cleanup",
			FunctionName: "test-function",
			Mode:         HookModeAsync,
			Enabled:      false,
		},
	}

	for _, hook := range hooks {
		if err := registry.Create(ctx, hook); err != nil {
			t.Fatalf("Failed to create hook: %v", err)
		}
	}

	list, err := registry.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list hooks: %v", err)
	}

	if len(list) != 3 {
		t.Errorf("Expected 3 hooks, got %d", len(list))
	}
}

func TestRegistry_LoadFromDatabase(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	funcChecker := &mockFunctionChecker{
		functions: map[string]bool{"test-function": true},
	}
	registry := NewRegistry(db, funcChecker)

	hook := &Hook{
		Type:         HookTypeDatabase,
		Source:       "users",
		Action:       "insert",
		FunctionName: "test-function",
		Mode:         HookModeAsync,
		Enabled:      true,
	}

	if err := registry.Create(ctx, hook); err != nil {
		t.Fatalf("Failed to create hook: %v", err)
	}

	newRegistry := NewRegistry(db, funcChecker)
	if err := newRegistry.LoadFromDatabase(ctx); err != nil {
		t.Fatalf("Failed to load hooks: %v", err)
	}

	list, err := newRegistry.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list hooks: %v", err)
	}

	if len(list) != 1 {
		t.Errorf("Expected 1 hook after loading, got %d", len(list))
	}
}
