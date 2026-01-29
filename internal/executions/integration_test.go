package executions

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/functions"
)

func testDBIntegration(t *testing.T) *database.DB {
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

func TestIntegration_ExecutionFlow(t *testing.T) {
	db := testDBIntegration(t)
	logger := NewExecutionLogger(db, 30*24*time.Hour)
	ctx := context.Background()

	logger.Start(ctx)
	defer logger.Stop()

	t.Run("successful execution", func(t *testing.T) {
		input := map[string]any{"test": "data"}
		resp := &functions.FunctionResponse{
			RequestID:  "req-success",
			Success:    true,
			Output:     map[string]any{"result": "ok"},
			DurationMs: 100,
			Logs: []functions.LogEntry{
				{
					Level:     "info",
					Message:   "processing",
					Timestamp: time.Now(),
				},
			},
		}

		result, err := logger.WrapExecution(
			ctx,
			"test-func",
			"req-success",
			"http",
			"",
			input,
			func() (*functions.FunctionResponse, error) {
				time.Sleep(10 * time.Millisecond)
				return resp, nil
			},
		)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.Success)

		time.Sleep(50 * time.Millisecond)

		logs, err := logger.store.List(ctx, map[string]any{"request_id": "req-success"}, 0, 0)
		require.NoError(t, err)
		require.Len(t, logs, 1)

		execLog := logs[0]
		require.Equal(t, "test-func", execLog.FunctionID)
		require.Equal(t, "http", execLog.TriggerType)
		require.Equal(t, ExecutionStatusSuccess, execLog.Status)
		require.NotNil(t, execLog.CompletedAt)
		require.Greater(t, execLog.DurationMs, 0)
		require.Contains(t, execLog.Input, "test")
		require.Contains(t, execLog.Output, "result")
		require.Contains(t, execLog.Logs, "processing")
	})

	t.Run("failed execution", func(t *testing.T) {
		input := map[string]any{"test": "data"}
		resp := &functions.FunctionResponse{
			RequestID: "req-failed",
			Success:   false,
			Error: &functions.FunctionError{
				Code:    "VALIDATION_ERROR",
				Message: "invalid input",
			},
			DurationMs: 50,
		}

		result, err := logger.WrapExecution(
			ctx,
			"test-func",
			"req-failed",
			"webhook",
			"hook-1",
			input,
			func() (*functions.FunctionResponse, error) {
				return resp, nil
			},
		)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.False(t, result.Success)

		time.Sleep(50 * time.Millisecond)

		logs, err := logger.store.List(ctx, map[string]any{
			"request_id":   "req-failed",
			"trigger_type": "webhook",
		}, 0, 0)
		require.NoError(t, err)
		require.Len(t, logs, 1)

		execLog := logs[0]
		require.Equal(t, "test-func", execLog.FunctionID)
		require.Equal(t, "webhook", execLog.TriggerType)
		require.Equal(t, "hook-1", execLog.TriggerID)
		require.Equal(t, ExecutionStatusFailed, execLog.Status)
		require.Contains(t, execLog.Error, "invalid input")
	})

	t.Run("execution with different triggers", func(t *testing.T) {
		triggers := []struct {
			triggerType string
			triggerID   string
		}{
			{"http", ""},
			{"webhook", "webhook-1"},
			{"schedule", "schedule-1"},
			{"database", "users"},
			{"auth", "signup"},
		}

		for _, trigger := range triggers {
			input := map[string]any{"trigger": trigger.triggerType}
			resp := &functions.FunctionResponse{
				RequestID:  "req-" + trigger.triggerType,
				Success:    true,
				Output:     map[string]any{"trigger": trigger.triggerType},
				DurationMs: 10,
			}

			_, err := logger.WrapExecution(
				ctx,
				"test-func",
				"req-"+trigger.triggerType,
				trigger.triggerType,
				trigger.triggerID,
				input,
				func() (*functions.FunctionResponse, error) {
					return resp, nil
				},
			)

			require.NoError(t, err)
		}

		time.Sleep(100 * time.Millisecond)

		logs, err := logger.store.List(ctx, map[string]any{}, 0, 0)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(logs), 5)

		for _, trigger := range triggers {
			found := false
			for _, log := range logs {
				if log.TriggerType == trigger.triggerType && log.TriggerID == trigger.triggerID {
					found = true
					break
				}
			}
			require.True(t, found, "trigger type %s not found", trigger.triggerType)
		}
	})
}

func TestIntegration_StatusTransitions(t *testing.T) {
	db := testDBIntegration(t)
	logger := NewExecutionLogger(db, 30*24*time.Hour)
	ctx := context.Background()

	execLog := &ExecutionLog{
		ID:          "exec-transitions",
		FunctionID:  "test-func",
		RequestID:   "req-transitions",
		TriggerType: "http",
		Status:      ExecutionStatusPending,
		StartedAt:   time.Now().UTC(),
		Input:       "{}",
	}

	err := logger.LogExecution(ctx, execLog)
	require.NoError(t, err)

	retrieved, err := logger.store.Get(ctx, "exec-transitions")
	require.NoError(t, err)
	require.Equal(t, ExecutionStatusPending, retrieved.Status)
	require.Nil(t, retrieved.CompletedAt)

	execLog.Status = ExecutionStatusRunning
	err = logger.store.Update(ctx, execLog)
	require.NoError(t, err)

	retrieved, err = logger.store.Get(ctx, "exec-transitions")
	require.NoError(t, err)
	require.Equal(t, ExecutionStatusRunning, retrieved.Status)

	err = logger.UpdateStatus(ctx, "exec-transitions", ExecutionStatusSuccess, `{"result":"ok"}`, "", "[]", 200)
	require.NoError(t, err)

	retrieved, err = logger.store.Get(ctx, "exec-transitions")
	require.NoError(t, err)
	require.Equal(t, ExecutionStatusSuccess, retrieved.Status)
	require.NotNil(t, retrieved.CompletedAt)
	require.Equal(t, 200, retrieved.DurationMs)
	require.Equal(t, `{"result":"ok"}`, retrieved.Output)
}

func TestIntegration_Filtering(t *testing.T) {
	db := testDBIntegration(t)
	logger := NewExecutionLogger(db, 30*24*time.Hour)
	ctx := context.Background()

	executions := []*ExecutionLog{
		{
			ID:          "exec-filter-1",
			FunctionID:  "func-a",
			RequestID:   "req-1",
			TriggerType: "http",
			Status:      ExecutionStatusSuccess,
			StartedAt:   time.Now().UTC(),
			Input:       "{}",
		},
		{
			ID:          "exec-filter-2",
			FunctionID:  "func-b",
			RequestID:   "req-2",
			TriggerType: "webhook",
			TriggerID:   "hook-1",
			Status:      ExecutionStatusFailed,
			StartedAt:   time.Now().UTC(),
			Input:       "{}",
		},
		{
			ID:          "exec-filter-3",
			FunctionID:  "func-a",
			RequestID:   "req-3",
			TriggerType: "schedule",
			TriggerID:   "sched-1",
			Status:      ExecutionStatusSuccess,
			StartedAt:   time.Now().UTC(),
			Input:       "{}",
		},
	}

	for _, exec := range executions {
		err := logger.LogExecution(ctx, exec)
		require.NoError(t, err)
	}

	t.Run("filter by function_id", func(t *testing.T) {
		logs, err := logger.store.List(ctx, map[string]any{"function_id": "func-a"}, 0, 0)
		require.NoError(t, err)
		require.Len(t, logs, 2)
	})

	t.Run("filter by status", func(t *testing.T) {
		logs, err := logger.store.List(ctx, map[string]any{"status": string(ExecutionStatusFailed)}, 0, 0)
		require.NoError(t, err)
		require.Len(t, logs, 1)
		require.Equal(t, "exec-filter-2", logs[0].ID)
	})

	t.Run("filter by trigger_type", func(t *testing.T) {
		logs, err := logger.store.List(ctx, map[string]any{"trigger_type": "webhook"}, 0, 0)
		require.NoError(t, err)
		require.Len(t, logs, 1)
		require.Equal(t, "exec-filter-2", logs[0].ID)
	})

	t.Run("filter by trigger_id", func(t *testing.T) {
		logs, err := logger.store.List(ctx, map[string]any{"trigger_id": "sched-1"}, 0, 0)
		require.NoError(t, err)
		require.Len(t, logs, 1)
		require.Equal(t, "exec-filter-3", logs[0].ID)
	})
}
