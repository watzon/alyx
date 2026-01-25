// Package functions provides serverless function execution via subprocesses.
package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	absPath, err := filepath.Abs(entrypoint)
	if err != nil {
		return nil, fmt.Errorf("resolving entrypoint path: %w", err)
	}
	funcDir := filepath.Dir(absPath)
	funcFile := filepath.Base(absPath)

	var cmd *exec.Cmd

	if r.runtime == RuntimeBinary {
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("binary not found: %s", absPath)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return nil, fmt.Errorf("checking binary: %w", err)
		}
		if info.Mode()&0111 == 0 {
			return nil, fmt.Errorf("file is not executable: %s", absPath)
		}

		// #nosec G204 - entrypoint is validated during function discovery and comes from trusted manifest files
		cmd = exec.CommandContext(ctx, absPath)
		cmd.Dir = funcDir
	} else {
		args := make([]string, 0, len(r.config.Args)+1)
		args = append(args, r.config.Args...)
		args = append(args, funcFile)

		// #nosec G204 - entrypoint is validated during function discovery and comes from trusted manifest files
		cmd = exec.CommandContext(ctx, r.config.Command, args...)
		cmd.Dir = funcDir
	}

	inputJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling function request: %w", err)
	}

	cmd.Stdin = bytes.NewReader(inputJSON)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("function %s timed out: %w", name, ctx.Err())
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
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
