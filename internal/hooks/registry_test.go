package hooks

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
)

func testDB(t *testing.T) *database.DB {
	t.Helper()

	tmpDir := t.TempDir()
	cfg := &config.DatabaseConfig{
		Path: filepath.Join(tmpDir, "test.db"),
	}

	db, err := database.Open(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestRegistry_Register(t *testing.T) {
	db := testDB(t)
	registry, err := NewRegistry(db)
	require.NoError(t, err)

	ctx := context.Background()

	hook := &Hook{
		Name:        "test-hook",
		FunctionID:  "func-123",
		EventType:   "http",
		EventSource: "api",
		EventAction: "create",
		Mode:        HookModeAsync,
		Priority:    10,
		Enabled:     true,
		Config: HookConfig{
			OnFailure: "continue",
			Timeout:   5 * time.Second,
		},
	}

	err = registry.Register(ctx, hook)
	require.NoError(t, err)
	require.NotEmpty(t, hook.ID)

	// Verify hook is in cache
	retrieved, err := registry.Get(ctx, hook.ID)
	require.NoError(t, err)
	require.Equal(t, hook.Name, retrieved.Name)
	require.Equal(t, hook.FunctionID, retrieved.FunctionID)
	require.Equal(t, hook.EventType, retrieved.EventType)
}

func TestRegistry_Unregister(t *testing.T) {
	db := testDB(t)
	registry, err := NewRegistry(db)
	require.NoError(t, err)

	ctx := context.Background()

	hook := &Hook{
		Name:        "test-hook",
		FunctionID:  "func-123",
		EventType:   "http",
		EventSource: "api",
		EventAction: "create",
		Mode:        HookModeAsync,
		Priority:    10,
		Enabled:     true,
	}

	err = registry.Register(ctx, hook)
	require.NoError(t, err)

	// Unregister hook
	err = registry.Unregister(ctx, hook.ID)
	require.NoError(t, err)

	// Verify hook is removed
	_, err = registry.Get(ctx, hook.ID)
	require.Error(t, err)
}

func TestRegistry_FindByEvent(t *testing.T) {
	db := testDB(t)
	registry, err := NewRegistry(db)
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name          string
		hook          *Hook
		queryType     string
		querySource   string
		queryAction   string
		shouldMatch   bool
		expectedCount int
	}{
		{
			name: "exact match",
			hook: &Hook{
				Name:        "exact-match",
				FunctionID:  "func-1",
				EventType:   "http",
				EventSource: "api",
				EventAction: "create",
				Mode:        HookModeAsync,
				Priority:    10,
				Enabled:     true,
			},
			queryType:     "http",
			querySource:   "api",
			queryAction:   "create",
			shouldMatch:   true,
			expectedCount: 1,
		},
		{
			name: "wildcard source",
			hook: &Hook{
				Name:        "wildcard-source",
				FunctionID:  "func-2",
				EventType:   "http",
				EventSource: "*",
				EventAction: "create",
				Mode:        HookModeAsync,
				Priority:    5,
				Enabled:     true,
			},
			queryType:     "http",
			querySource:   "any-source",
			queryAction:   "create",
			shouldMatch:   true,
			expectedCount: 1,
		},
		{
			name: "wildcard action",
			hook: &Hook{
				Name:        "wildcard-action",
				FunctionID:  "func-3",
				EventType:   "http",
				EventSource: "api",
				EventAction: "*",
				Mode:        HookModeAsync,
				Priority:    8,
				Enabled:     true,
			},
			queryType:     "http",
			querySource:   "api",
			queryAction:   "any-action",
			shouldMatch:   true,
			expectedCount: 1,
		},
		{
			name: "wildcard both",
			hook: &Hook{
				Name:        "wildcard-both",
				FunctionID:  "func-4",
				EventType:   "http",
				EventSource: "*",
				EventAction: "*",
				Mode:        HookModeAsync,
				Priority:    3,
				Enabled:     true,
			},
			queryType:     "http",
			querySource:   "any-source",
			queryAction:   "any-action",
			shouldMatch:   true,
			expectedCount: 1,
		},
		{
			name: "type mismatch",
			hook: &Hook{
				Name:        "type-mismatch",
				FunctionID:  "func-5",
				EventType:   "database",
				EventSource: "api",
				EventAction: "create",
				Mode:        HookModeAsync,
				Priority:    10,
				Enabled:     true,
			},
			queryType:     "http",
			querySource:   "api",
			queryAction:   "create",
			shouldMatch:   false,
			expectedCount: 0,
		},
		{
			name: "disabled hook",
			hook: &Hook{
				Name:        "disabled",
				FunctionID:  "func-6",
				EventType:   "http",
				EventSource: "api",
				EventAction: "create",
				Mode:        HookModeAsync,
				Priority:    10,
				Enabled:     false,
			},
			queryType:     "http",
			querySource:   "api",
			queryAction:   "create",
			shouldMatch:   false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register hook
			err := registry.Register(ctx, tt.hook)
			require.NoError(t, err)

			// Find by event
			matches, err := registry.FindByEvent(ctx, tt.queryType, tt.querySource, tt.queryAction)
			require.NoError(t, err)

			if tt.shouldMatch {
				require.GreaterOrEqual(t, len(matches), tt.expectedCount)
				// Verify the hook we just registered is in the matches
				found := false
				for _, match := range matches {
					if match.ID == tt.hook.ID {
						found = true
						break
					}
				}
				require.True(t, found, "Expected hook not found in matches")
			} else {
				// Verify the hook we just registered is NOT in the matches
				for _, match := range matches {
					require.NotEqual(t, tt.hook.ID, match.ID, "Unexpected hook found in matches")
				}
			}

			// Cleanup
			err = registry.Unregister(ctx, tt.hook.ID)
			require.NoError(t, err)
		})
	}
}

func TestRegistry_FindByEvent_Priority(t *testing.T) {
	db := testDB(t)
	registry, err := NewRegistry(db)
	require.NoError(t, err)

	ctx := context.Background()

	// Register hooks with different priorities
	hooks := []*Hook{
		{
			Name:        "low-priority",
			FunctionID:  "func-1",
			EventType:   "http",
			EventSource: "api",
			EventAction: "create",
			Mode:        HookModeAsync,
			Priority:    1,
			Enabled:     true,
		},
		{
			Name:        "high-priority",
			FunctionID:  "func-2",
			EventType:   "http",
			EventSource: "api",
			EventAction: "create",
			Mode:        HookModeAsync,
			Priority:    100,
			Enabled:     true,
		},
		{
			Name:        "medium-priority",
			FunctionID:  "func-3",
			EventType:   "http",
			EventSource: "api",
			EventAction: "create",
			Mode:        HookModeAsync,
			Priority:    50,
			Enabled:     true,
		},
	}

	for _, hook := range hooks {
		regErr := registry.Register(ctx, hook)
		require.NoError(t, regErr)
	}

	// Find by event
	matches, err := registry.FindByEvent(ctx, "http", "api", "create")
	require.NoError(t, err)
	require.Len(t, matches, 3)

	// Verify priority order (higher priority first)
	require.Equal(t, 100, matches[0].Priority)
	require.Equal(t, 50, matches[1].Priority)
	require.Equal(t, 1, matches[2].Priority)

	// Cleanup
	for _, hook := range hooks {
		err := registry.Unregister(ctx, hook.ID)
		require.NoError(t, err)
	}
}

func TestRegistry_FindByFunction(t *testing.T) {
	db := testDB(t)
	registry, err := NewRegistry(db)
	require.NoError(t, err)

	ctx := context.Background()

	functionID := "func-123"

	hooks := []*Hook{
		{
			Name:        "hook-1",
			FunctionID:  functionID,
			EventType:   "http",
			EventSource: "api",
			EventAction: "create",
			Mode:        HookModeAsync,
			Priority:    10,
			Enabled:     true,
		},
		{
			Name:        "hook-2",
			FunctionID:  functionID,
			EventType:   "database",
			EventSource: "users",
			EventAction: "update",
			Mode:        HookModeSync,
			Priority:    20,
			Enabled:     true,
		},
		{
			Name:        "hook-3",
			FunctionID:  "other-func",
			EventType:   "http",
			EventSource: "api",
			EventAction: "delete",
			Mode:        HookModeAsync,
			Priority:    5,
			Enabled:     true,
		},
	}

	for _, hook := range hooks {
		regErr := registry.Register(ctx, hook)
		require.NoError(t, regErr)
	}

	// Find by function
	matches, err := registry.FindByFunction(ctx, functionID)
	require.NoError(t, err)
	require.Len(t, matches, 2)

	// Verify both hooks belong to the function
	for _, match := range matches {
		require.Equal(t, functionID, match.FunctionID)
	}

	// Verify priority order
	require.Equal(t, 20, matches[0].Priority)
	require.Equal(t, 10, matches[1].Priority)

	// Cleanup
	for _, hook := range hooks {
		err := registry.Unregister(ctx, hook.ID)
		require.NoError(t, err)
	}
}

func TestRegistry_List(t *testing.T) {
	db := testDB(t)
	registry, err := NewRegistry(db)
	require.NoError(t, err)

	ctx := context.Background()

	hooks := []*Hook{
		{
			Name:        "hook-1",
			FunctionID:  "func-1",
			EventType:   "http",
			EventSource: "api",
			EventAction: "create",
			Mode:        HookModeAsync,
			Priority:    10,
			Enabled:     true,
		},
		{
			Name:        "hook-2",
			FunctionID:  "func-2",
			EventType:   "database",
			EventSource: "users",
			EventAction: "update",
			Mode:        HookModeSync,
			Priority:    20,
			Enabled:     true,
		},
	}

	for _, hook := range hooks {
		regErr := registry.Register(ctx, hook)
		require.NoError(t, regErr)
	}

	// List all hooks
	all, err := registry.List(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(all), 2)

	// Cleanup
	for _, hook := range hooks {
		err := registry.Unregister(ctx, hook.ID)
		require.NoError(t, err)
	}
}

func TestRegistry_Reload(t *testing.T) {
	db := testDB(t)
	registry, err := NewRegistry(db)
	require.NoError(t, err)

	ctx := context.Background()

	hook := &Hook{
		Name:        "test-hook",
		FunctionID:  "func-123",
		EventType:   "http",
		EventSource: "api",
		EventAction: "create",
		Mode:        HookModeAsync,
		Priority:    10,
		Enabled:     true,
	}

	err = registry.Register(ctx, hook)
	require.NoError(t, err)

	// Invalidate cache
	registry.invalidateCache()

	// Verify cache is empty
	_, err = registry.Get(ctx, hook.ID)
	require.Error(t, err)

	// Reload cache
	err = registry.Reload(ctx)
	require.NoError(t, err)

	// Verify hook is back in cache
	retrieved, err := registry.Get(ctx, hook.ID)
	require.NoError(t, err)
	require.Equal(t, hook.Name, retrieved.Name)
}
