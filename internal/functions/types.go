// Package functions provides serverless function execution via containers.
package functions

import (
	"context"
	"time"
)

// Runtime represents a function runtime language.
type Runtime string

const (
	// RuntimeNode is the Node.js runtime.
	RuntimeNode Runtime = "node"
	// RuntimePython is the Python runtime.
	RuntimePython Runtime = "python"
	// RuntimeGo is the Go runtime.
	RuntimeGo Runtime = "go"
	// RuntimeWasm is the WebAssembly runtime.
	RuntimeWasm Runtime = "wasm"
)

// FunctionRequest represents a function invocation request.
type FunctionRequest struct {
	// RequestID is a unique identifier for this request.
	RequestID string `json:"request_id"`
	// Function is the name of the function to invoke.
	Function string `json:"function"`
	// Input is the function input data.
	Input map[string]any `json:"input"`
	// Context contains auth and environment information.
	Context *FunctionContext `json:"context"`
}

// FunctionContext contains the execution context passed to functions.
type FunctionContext struct {
	// Auth contains the authenticated user info (nil if unauthenticated).
	Auth *AuthContext `json:"auth,omitempty"`
	// Env contains environment variables available to the function.
	Env map[string]string `json:"env,omitempty"`
	// AlyxURL is the URL to reach the Alyx server from within the container.
	AlyxURL string `json:"alyx_url"`
	// InternalToken is a short-lived token for internal API calls.
	InternalToken string `json:"internal_token"`
}

// AuthContext contains authenticated user information.
type AuthContext struct {
	ID       string         `json:"id"`
	Email    string         `json:"email"`
	Role     string         `json:"role,omitempty"`
	Verified bool           `json:"verified"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// FunctionResponse represents a function invocation response.
type FunctionResponse struct {
	// RequestID echoes the request ID.
	RequestID string `json:"request_id"`
	// Success indicates whether the function executed successfully.
	Success bool `json:"success"`
	// Output contains the function return value on success.
	Output any `json:"output,omitempty"`
	// Error contains error details on failure.
	Error *FunctionError `json:"error,omitempty"`
	// Logs contains log entries from the function.
	Logs []LogEntry `json:"logs,omitempty"`
	// DurationMs is the execution time in milliseconds.
	DurationMs int64 `json:"duration_ms"`
}

// FunctionError contains function error details.
type FunctionError struct {
	// Code is a machine-readable error code.
	Code string `json:"code"`
	// Message is a human-readable error message.
	Message string `json:"message"`
	// Details contains additional error context.
	Details map[string]any `json:"details,omitempty"`
}

// LogEntry represents a log entry from a function.
type LogEntry struct {
	// Level is the log level (debug, info, warn, error).
	Level string `json:"level"`
	// Message is the log message.
	Message string `json:"message"`
	// Data contains structured log data.
	Data map[string]any `json:"data,omitempty"`
	// Timestamp is when the log was recorded.
	Timestamp time.Time `json:"timestamp"`
}

// Executor defines the interface for executing functions.
type Executor interface {
	// Execute invokes a function and returns the response.
	Execute(ctx context.Context, req *FunctionRequest) (*FunctionResponse, error)
	// Close shuts down the executor and releases resources.
	Close() error
}
