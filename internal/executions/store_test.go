package executions

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
)

func testDBExec(t *testing.T) *database.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.DatabaseConfig{
		Path: dbPath,
	}

	db, err := database.Open(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestStore_Create(t *testing.T) {
	db := testDBExec(t)
	store := NewStore(db)
	ctx := context.Background()

	now := time.Now().UTC()
	log := &ExecutionLog{
		ID:          "exec-1",
		FunctionID:  "test-func",
		RequestID:   "req-1",
		TriggerType: "http",
		TriggerID:   "",
		Status:      ExecutionStatusPending,
		StartedAt:   now,
		Input:       `{"key":"value"}`,
	}

	err := store.Create(ctx, log)
	require.NoError(t, err)

	retrieved, err := store.Get(ctx, "exec-1")
	require.NoError(t, err)
	require.Equal(t, "exec-1", retrieved.ID)
	require.Equal(t, "test-func", retrieved.FunctionID)
	require.Equal(t, "req-1", retrieved.RequestID)
	require.Equal(t, "http", retrieved.TriggerType)
	require.Equal(t, ExecutionStatusPending, retrieved.Status)
	require.Equal(t, `{"key":"value"}`, retrieved.Input)
	require.Nil(t, retrieved.CompletedAt)
}

func TestStore_Update(t *testing.T) {
	db := testDBExec(t)
	store := NewStore(db)
	ctx := context.Background()

	now := time.Now().UTC()
	log := &ExecutionLog{
		ID:          "exec-2",
		FunctionID:  "test-func",
		RequestID:   "req-2",
		TriggerType: "webhook",
		TriggerID:   "hook-1",
		Status:      ExecutionStatusRunning,
		StartedAt:   now,
		Input:       `{"data":"test"}`,
	}

	err := store.Create(ctx, log)
	require.NoError(t, err)

	completedAt := now.Add(100 * time.Millisecond)
	log.Status = ExecutionStatusSuccess
	log.CompletedAt = &completedAt
	log.DurationMs = 100
	log.Output = `{"result":"ok"}`
	log.Logs = `[{"level":"info","message":"test"}]`

	err = store.Update(ctx, log)
	require.NoError(t, err)

	retrieved, err := store.Get(ctx, "exec-2")
	require.NoError(t, err)
	require.Equal(t, ExecutionStatusSuccess, retrieved.Status)
	require.NotNil(t, retrieved.CompletedAt)
	require.Equal(t, 100, retrieved.DurationMs)
	require.Equal(t, `{"result":"ok"}`, retrieved.Output)
	require.Equal(t, `[{"level":"info","message":"test"}]`, retrieved.Logs)
}

func TestStore_Get_NotFound(t *testing.T) {
	db := testDBExec(t)
	store := NewStore(db)
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestStore_List(t *testing.T) {
	db := testDBExec(t)
	store := NewStore(db)
	ctx := context.Background()

	now := time.Now().UTC()

	logs := []*ExecutionLog{
		{
			ID:          "exec-3",
			FunctionID:  "func-a",
			RequestID:   "req-3",
			TriggerType: "http",
			Status:      ExecutionStatusSuccess,
			StartedAt:   now.Add(-3 * time.Minute),
			Input:       "{}",
		},
		{
			ID:          "exec-4",
			FunctionID:  "func-b",
			RequestID:   "req-4",
			TriggerType: "schedule",
			TriggerID:   "sched-1",
			Status:      ExecutionStatusFailed,
			StartedAt:   now.Add(-2 * time.Minute),
			Input:       "{}",
		},
		{
			ID:          "exec-5",
			FunctionID:  "func-a",
			RequestID:   "req-5",
			TriggerType: "http",
			Status:      ExecutionStatusSuccess,
			StartedAt:   now.Add(-1 * time.Minute),
			Input:       "{}",
		},
	}

	for _, log := range logs {
		err := store.Create(ctx, log)
		require.NoError(t, err)
	}

	t.Run("list all", func(t *testing.T) {
		results, err := store.List(ctx, map[string]any{}, 0, 0)
		require.NoError(t, err)
		require.Len(t, results, 3)
		require.Equal(t, "exec-5", results[0].ID)
		require.Equal(t, "exec-4", results[1].ID)
		require.Equal(t, "exec-3", results[2].ID)
	})

	t.Run("filter by function_id", func(t *testing.T) {
		results, err := store.List(ctx, map[string]any{"function_id": "func-a"}, 0, 0)
		require.NoError(t, err)
		require.Len(t, results, 2)
		require.Equal(t, "exec-5", results[0].ID)
		require.Equal(t, "exec-3", results[1].ID)
	})

	t.Run("filter by status", func(t *testing.T) {
		results, err := store.List(ctx, map[string]any{"status": "failed"}, 0, 0)
		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Equal(t, "exec-4", results[0].ID)
	})

	t.Run("filter by trigger_type", func(t *testing.T) {
		results, err := store.List(ctx, map[string]any{"trigger_type": "schedule"}, 0, 0)
		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Equal(t, "exec-4", results[0].ID)
	})

	t.Run("limit and offset", func(t *testing.T) {
		results, err := store.List(ctx, map[string]any{}, 2, 1)
		require.NoError(t, err)
		require.Len(t, results, 2)
		require.Equal(t, "exec-4", results[0].ID)
		require.Equal(t, "exec-3", results[1].ID)
	})
}

func TestStore_DeleteOlderThan(t *testing.T) {
	db := testDBExec(t)
	store := NewStore(db)
	ctx := context.Background()

	now := time.Now().UTC()

	logs := []*ExecutionLog{
		{
			ID:          "exec-6",
			FunctionID:  "func-a",
			RequestID:   "req-6",
			TriggerType: "http",
			Status:      ExecutionStatusSuccess,
			StartedAt:   now.Add(-48 * time.Hour),
			Input:       "{}",
		},
		{
			ID:          "exec-7",
			FunctionID:  "func-a",
			RequestID:   "req-7",
			TriggerType: "http",
			Status:      ExecutionStatusFailed,
			StartedAt:   now.Add(-25 * time.Hour),
			Input:       "{}",
		},
		{
			ID:          "exec-8",
			FunctionID:  "func-a",
			RequestID:   "req-8",
			TriggerType: "http",
			Status:      ExecutionStatusSuccess,
			StartedAt:   now.Add(-1 * time.Hour),
			Input:       "{}",
		},
		{
			ID:          "exec-9",
			FunctionID:  "func-a",
			RequestID:   "req-9",
			TriggerType: "http",
			Status:      ExecutionStatusRunning,
			StartedAt:   now.Add(-48 * time.Hour),
			Input:       "{}",
		},
	}

	for _, log := range logs {
		err := store.Create(ctx, log)
		require.NoError(t, err)
	}

	err := store.DeleteOlderThan(ctx, 24*time.Hour)
	require.NoError(t, err)

	results, err := store.List(ctx, map[string]any{}, 0, 0)
	require.NoError(t, err)
	require.Len(t, results, 2)

	ids := []string{results[0].ID, results[1].ID}
	require.Contains(t, ids, "exec-8")
	require.Contains(t, ids, "exec-9")
}
