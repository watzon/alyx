// Package functions provides serverless function execution via subprocesses.
package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// SubprocessRuntime executes functions by spawning subprocesses.
// It communicates with functions via JSON over stdin/stdout.
type SubprocessRuntime struct {
	runtime Runtime
	config  RuntimeConfig
}

// NewSubprocessRuntime creates a new subprocess runtime for the given runtime type.
// It validates that the runtime binary exists on the system.
func NewSubprocessRuntime(runtime Runtime) (*SubprocessRuntime, error) {
	config, ok := defaultRuntimes[runtime]
	if !ok {
		return nil, fmt.Errorf("unsupported runtime: %s", runtime)
	}

	// Check if the runtime binary exists
	_, err := exec.LookPath(config.Command)
	if err != nil {
		return nil, fmt.Errorf("runtime binary not found: %s (install %s to use this runtime)", config.Command, runtime)
	}

	return &SubprocessRuntime{
		runtime: runtime,
		config:  config,
	}, nil
}

// Call executes a function by spawning a subprocess and communicating via JSON.
// The function receives a FunctionRequest on stdin and returns a FunctionResponse on stdout.
func (r *SubprocessRuntime) Call(ctx context.Context, name, entrypoint string, req *FunctionRequest) (*FunctionResponse, error) {
	// Build command arguments
	args := append(r.config.Args, entrypoint)
	cmd := exec.CommandContext(ctx, r.config.Command, args...)

	// Marshal request to JSON
	inputJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling function request: %w", err)
	}

	// Set up stdin with JSON input
	cmd.Stdin = bytes.NewReader(inputJSON)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute the command
	err = cmd.Run()

	// Handle execution errors
	if err != nil {
		// Check if it's a context cancellation (timeout)
		if ctx.Err() != nil {
			return nil, fmt.Errorf("function %s timed out: %w", name, ctx.Err())
		}

		// Check if it's an exit error
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderrStr := strings.TrimSpace(stderr.String())
			return nil, fmt.Errorf("function %s exited with code %d: %s", name, exitErr.ExitCode(), stderrStr)
		}

		return nil, fmt.Errorf("executing function %s: %w", name, err)
	}

	// Parse stdout JSON response
	var response FunctionResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		// Include stderr in error for debugging
		stderrStr := strings.TrimSpace(stderr.String())
		stdoutStr := strings.TrimSpace(stdout.String())
		return nil, fmt.Errorf("parsing function response: %w (stdout: %s, stderr: %s)", err, stdoutStr, stderrStr)
	}

	return &response, nil
}

// Runtime returns the runtime type.
func (r *SubprocessRuntime) Runtime() Runtime {
	return r.runtime
}

// Config returns the runtime configuration.
func (r *SubprocessRuntime) Config() RuntimeConfig {
	return r.config
}
