package executions

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/functions"
)

func testDBLogger(t *testing.T) *database.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.DatabaseConfig{
		Path:            dbPath,
		WALMode:         true,
		ForeignKeys:     true,
		BusyTimeout:     5 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 1 * time.Hour,
		CacheSize:       -2000,
	}

	db, err := database.Open(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestExecutionLogger_LogExecution(t *testing.T) {
	db := testDBLogger(t)
	logger := NewExecutionLogger(db, 30*24*time.Hour)
	ctx := context.Background()

	execLog := &ExecutionLog{
		ID:          "exec-log-1",
		FunctionID:  "test-func",
		RequestID:   "req-1",
		TriggerType: "http",
		TriggerID:   "",
		Status:      ExecutionStatusPending,
		StartedAt:   time.Now().UTC(),
		Input:       `{"test":"data"}`,
	}

	err := logger.LogExecution(ctx, execLog)
	require.NoError(t, err)

	retrieved, err := logger.store.Get(ctx, "exec-log-1")
	require.NoError(t, err)
	require.Equal(t, "exec-log-1", retrieved.ID)
	require.Equal(t, "test-func", retrieved.FunctionID)
	require.Equal(t, ExecutionStatusPending, retrieved.Status)
}

func TestExecutionLogger_UpdateStatus(t *testing.T) {
	db := testDBLogger(t)
	logger := NewExecutionLogger(db, 30*24*time.Hour)
	ctx := context.Background()

	execLog := &ExecutionLog{
		ID:          "exec-log-2",
		FunctionID:  "test-func",
		RequestID:   "req-2",
		TriggerType: "webhook",
		TriggerID:   "hook-1",
		Status:      ExecutionStatusRunning,
		StartedAt:   time.Now().UTC(),
		Input:       `{}`,
	}

	err := logger.LogExecution(ctx, execLog)
	require.NoError(t, err)

	err = logger.UpdateStatus(ctx, "exec-log-2", ExecutionStatusSuccess, `{"result":"ok"}`, "", "[]", 150)
	require.NoError(t, err)

	retrieved, err := logger.store.Get(ctx, "exec-log-2")
	require.NoError(t, err)
	require.Equal(t, ExecutionStatusSuccess, retrieved.Status)
	require.Equal(t, `{"result":"ok"}`, retrieved.Output)
	require.Equal(t, "", retrieved.Error)
	require.Equal(t, 150, retrieved.DurationMs)
	require.NotNil(t, retrieved.CompletedAt)
}

func TestExecutionLogger_WrapExecution_Success(t *testing.T) {
	db := testDBLogger(t)
	logger := NewExecutionLogger(db, 30*24*time.Hour)
	ctx := context.Background()

	input := map[string]any{"key": "value"}
	resp := &functions.FunctionResponse{
		RequestID:  "req-3",
		Success:    true,
		Output:     map[string]any{"result": "success"},
		DurationMs: 100,
		Logs: []functions.LogEntry{
			{
				Level:     "info",
				Message:   "test log",
				Timestamp: time.Now(),
			},
		},
	}

	result, err := logger.WrapExecution(
		ctx,
		"test-func",
		"req-3",
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

	logs, err := logger.store.List(ctx, map[string]any{"function_id": "test-func"}, 0, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, ExecutionStatusSuccess, logs[0].Status)
	require.Contains(t, logs[0].Output, "success")
	require.NotEmpty(t, logs[0].Logs)
}

func TestExecutionLogger_WrapExecution_Failure(t *testing.T) {
	db := testDBLogger(t)
	logger := NewExecutionLogger(db, 30*24*time.Hour)
	ctx := context.Background()

	input := map[string]any{"key": "value"}
	expectedErr := errors.New("execution failed")

	result, err := logger.WrapExecution(
		ctx,
		"test-func",
		"req-4",
		"schedule",
		"sched-1",
		input,
		func() (*functions.FunctionResponse, error) {
			return nil, expectedErr
		},
	)

	require.Error(t, err)
	require.Equal(t, expectedErr, err)
	require.Nil(t, result)

	time.Sleep(50 * time.Millisecond)

	logs, err := logger.store.List(ctx, map[string]any{"function_id": "test-func"}, 0, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, ExecutionStatusFailed, logs[0].Status)
	require.Contains(t, logs[0].Error, "execution failed")
}

func TestExecutionLogger_WrapExecution_FunctionError(t *testing.T) {
	db := testDBLogger(t)
	logger := NewExecutionLogger(db, 30*24*time.Hour)
	ctx := context.Background()

	input := map[string]any{"key": "value"}
	resp := &functions.FunctionResponse{
		RequestID: "req-5",
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
		"req-5",
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

	logs, err := logger.store.List(ctx, map[string]any{"function_id": "test-func"}, 0, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, ExecutionStatusFailed, logs[0].Status)
	require.Contains(t, logs[0].Error, "invalid input")
}

func TestExecutionLogger_Cleanup(t *testing.T) {
	db := testDBLogger(t)
	logger := NewExecutionLogger(db, 1*time.Hour)
	ctx := context.Background()

	now := time.Now().UTC()

	oldLog := &ExecutionLog{
		ID:          "exec-old",
		FunctionID:  "test-func",
		RequestID:   "req-old",
		TriggerType: "http",
		Status:      ExecutionStatusSuccess,
		StartedAt:   now.Add(-2 * time.Hour),
		Input:       "{}",
	}
	completedAt := now.Add(-2 * time.Hour)
	oldLog.CompletedAt = &completedAt

	recentLog := &ExecutionLog{
		ID:          "exec-recent",
		FunctionID:  "test-func",
		RequestID:   "req-recent",
		TriggerType: "http",
		Status:      ExecutionStatusSuccess,
		StartedAt:   now.Add(-30 * time.Minute),
		Input:       "{}",
	}

	err := logger.LogExecution(ctx, oldLog)
	require.NoError(t, err)
	err = logger.LogExecution(ctx, recentLog)
	require.NoError(t, err)

	err = logger.store.DeleteOlderThan(ctx, 1*time.Hour)
	require.NoError(t, err)

	logs, err := logger.store.List(ctx, map[string]any{}, 0, 0)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "exec-recent", logs[0].ID)
}

func TestExecutionLogger_StartStop(t *testing.T) {
	db := testDBLogger(t)
	logger := NewExecutionLogger(db, 1*time.Hour)
	ctx := context.Background()

	logger.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	logger.Stop()
}
