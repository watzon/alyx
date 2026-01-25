package executions

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/functions"
)

// ExecutionLogger manages execution logging.
type ExecutionLogger struct {
	db        *database.DB
	store     *Store
	retention time.Duration
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewExecutionLogger creates a new execution logger.
func NewExecutionLogger(db *database.DB, retention time.Duration) *ExecutionLogger {
	if retention == 0 {
		retention = 30 * 24 * time.Hour // Default 30 days
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ExecutionLogger{
		db:        db,
		store:     NewStore(db),
		retention: retention,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins background cleanup.
func (l *ExecutionLogger) Start(ctx context.Context) {
	l.wg.Add(1)
	go l.cleanupLoop(l.ctx, 1*time.Hour)
}

// Stop gracefully shuts down the logger.
func (l *ExecutionLogger) Stop() {
	l.cancel()
	l.wg.Wait()
}

// LogExecution logs a function execution.
func (l *ExecutionLogger) LogExecution(ctx context.Context, execLog *ExecutionLog) error {
	if err := l.store.Create(ctx, execLog); err != nil {
		return fmt.Errorf("creating execution log: %w", err)
	}

	log.Debug().
		Str("execution_id", execLog.ID).
		Str("function_id", execLog.FunctionID).
		Str("status", string(execLog.Status)).
		Msg("Execution logged")

	return nil
}

// UpdateStatus updates execution status.
func (l *ExecutionLogger) UpdateStatus(ctx context.Context, id string, status ExecutionStatus, output, errorMsg, logs string, duration int) error {
	// Get existing log
	execLog, err := l.store.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("getting execution log: %w", err)
	}

	// Update fields
	execLog.Status = status
	execLog.Output = output
	execLog.Error = errorMsg
	execLog.Logs = logs
	execLog.DurationMs = duration
	now := time.Now().UTC()
	execLog.CompletedAt = &now

	// Save
	if err := l.store.Update(ctx, execLog); err != nil {
		return fmt.Errorf("updating execution log: %w", err)
	}

	log.Debug().
		Str("execution_id", id).
		Str("status", string(status)).
		Int("duration_ms", duration).
		Msg("Execution status updated")

	return nil
}

// cleanupLoop periodically removes old execution logs.
func (l *ExecutionLogger) cleanupLoop(ctx context.Context, interval time.Duration) {
	defer l.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := l.store.DeleteOlderThan(ctx, l.retention); err != nil {
				log.Error().Err(err).Msg("Failed to cleanup old execution logs")
			}
		}
	}
}

// WrapExecution wraps a function execution with logging.
func (l *ExecutionLogger) WrapExecution(
	ctx context.Context,
	functionID string,
	requestID string,
	triggerType string,
	triggerID string,
	input map[string]any,
	execute func() (*functions.FunctionResponse, error),
) (*functions.FunctionResponse, error) {
	// Create execution log
	execID := uuid.New().String()
	inputJSON, err := json.Marshal(input)
	if err != nil {
		inputJSON = []byte("{}")
	}

	execLog := &ExecutionLog{
		ID:          execID,
		FunctionID:  functionID,
		RequestID:   requestID,
		TriggerType: triggerType,
		TriggerID:   triggerID,
		Status:      ExecutionStatusPending,
		StartedAt:   time.Now().UTC(),
		Input:       string(inputJSON),
	}

	// Log pending execution
	if err := l.LogExecution(ctx, execLog); err != nil {
		log.Error().Err(err).Msg("Failed to log pending execution")
		// Continue execution even if logging fails
	}

	// Update to running
	execLog.Status = ExecutionStatusRunning
	if err := l.store.Update(ctx, execLog); err != nil {
		log.Error().Err(err).Msg("Failed to update execution to running")
	}

	// Execute function
	startTime := time.Now()
	resp, execErr := execute()
	duration := int(time.Since(startTime).Milliseconds())

	// Determine status
	status := ExecutionStatusSuccess
	var outputJSON string
	var errorMsg string
	var logsJSON string = "[]"

	if execErr != nil {
		status = ExecutionStatusFailed
		errorMsg = execErr.Error()
	} else if resp != nil {
		if !resp.Success {
			status = ExecutionStatusFailed
			if resp.Error != nil {
				errorMsg = resp.Error.Message
			}
		}

		// Serialize output
		if resp.Output != nil {
			outputBytes, err := json.Marshal(resp.Output)
			if err != nil {
				outputJSON = "{}"
			} else {
				outputJSON = string(outputBytes)
			}
		}

		// Serialize logs
		if len(resp.Logs) > 0 {
			logsBytes, marshalErr := json.Marshal(resp.Logs)
			if marshalErr != nil {
				logsJSON = "[]"
			} else {
				logsJSON = string(logsBytes)
			}
		}
	}

	// Update final status
	if err := l.UpdateStatus(ctx, execID, status, outputJSON, errorMsg, logsJSON, duration); err != nil {
		log.Error().Err(err).Msg("Failed to update execution status")
	}

	return resp, execErr
}
