package executions

import "time"

// ExecutionStatus represents the status of a function execution.
type ExecutionStatus string

const (
	// ExecutionStatusPending indicates the execution is queued but not started.
	ExecutionStatusPending ExecutionStatus = "pending"
	// ExecutionStatusRunning indicates the execution is currently in progress.
	ExecutionStatusRunning ExecutionStatus = "running"
	// ExecutionStatusSuccess indicates the execution completed successfully.
	ExecutionStatusSuccess ExecutionStatus = "success"
	// ExecutionStatusFailed indicates the execution failed with an error.
	ExecutionStatusFailed ExecutionStatus = "failed"
	// ExecutionStatusTimedOut indicates the execution exceeded its timeout.
	ExecutionStatusTimedOut ExecutionStatus = "timed_out"
	// ExecutionStatusCanceled indicates the execution was canceled.
	ExecutionStatusCanceled ExecutionStatus = "canceled"
)

// ExecutionLog represents a function execution log entry.
type ExecutionLog struct {
	ID          string          // Unique execution ID
	FunctionID  string          // Function that was executed
	RequestID   string          // Request ID for tracing
	TriggerType string          // Trigger type (http, webhook, database, auth, schedule, custom)
	TriggerID   string          // Trigger ID (hook ID, schedule ID, etc.)
	Status      ExecutionStatus // Execution status
	StartedAt   time.Time       // When execution started
	CompletedAt *time.Time      // When execution completed (nil if still running)
	DurationMs  int             // Execution duration in milliseconds
	Input       string          // Function input (JSON string)
	Output      string          // Function output (JSON string)
	Error       string          // Error message if failed
	Logs        string          // Function logs
}
