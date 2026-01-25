package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	extism "github.com/extism/go-sdk"
	"github.com/rs/zerolog/log"
)

// WASMConfig contains configuration for the WASM runtime.
type WASMConfig struct {
	// MemoryLimitMB is the maximum memory a plugin can use in megabytes.
	MemoryLimitMB int64
	// TimeoutSeconds is the maximum execution time for a function call.
	TimeoutSeconds int
	// EnableWASI enables WASI support for HTTP access.
	EnableWASI bool
	// AlyxURL is the URL to reach the Alyx server from within WASM.
	AlyxURL string
}

// DefaultWASMConfig returns the default WASM configuration.
func DefaultWASMConfig() *WASMConfig {
	return &WASMConfig{
		MemoryLimitMB:  256,
		TimeoutSeconds: 30,
		EnableWASI:     true,
		AlyxURL:        "http://localhost:8090",
	}
}

// WASMRuntime manages WASM plugin execution using Extism.
type WASMRuntime struct {
	mu      sync.RWMutex
	plugins map[string]*extism.Plugin
	config  *WASMConfig
}

// NewWASMRuntime creates a new WASM runtime with the given configuration.
func NewWASMRuntime(config *WASMConfig) *WASMRuntime {
	if config == nil {
		config = DefaultWASMConfig()
	}
	return &WASMRuntime{
		plugins: make(map[string]*extism.Plugin),
		config:  config,
	}
}

// LoadPlugin loads a WASM plugin from the given file path.
func (w *WASMRuntime) LoadPlugin(name string, wasmPath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if plugin already loaded
	if _, exists := w.plugins[name]; exists {
		return fmt.Errorf("plugin %s already loaded", name)
	}

	return w.loadPluginLocked(name, wasmPath)
}

// loadPluginLocked loads a plugin without acquiring the lock (internal use).
func (w *WASMRuntime) loadPluginLocked(name string, wasmPath string) error {
	// Read WASM file
	wasmData, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("reading WASM file: %w", err)
	}

	// Create manifest
	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmData{
				Data: wasmData,
			},
		},
		AllowedHosts: []string{"*"}, // Allow all hosts for HTTP access
		Config:       map[string]string{},
	}

	config := extism.PluginConfig{
		EnableWasi: w.config.EnableWASI,
	}

	// Create context with timeout for plugin creation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create host functions
	hostFunctions := w.createHostFunctions()

	// Create plugin
	plugin, err := extism.NewPlugin(ctx, manifest, config, hostFunctions)
	if err != nil {
		return fmt.Errorf("creating plugin: %w", err)
	}

	w.plugins[name] = plugin

	log.Debug().
		Str("name", name).
		Str("path", wasmPath).
		Int64("memory_limit_mb", w.config.MemoryLimitMB).
		Msg("Loaded WASM plugin")

	return nil
}

// UnloadPlugin unloads a WASM plugin by name.
func (w *WASMRuntime) UnloadPlugin(name string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	plugin, exists := w.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	plugin.Close(context.Background())
	delete(w.plugins, name)

	log.Debug().Str("name", name).Msg("Unloaded WASM plugin")

	return nil
}

// Reload reloads a WASM plugin by unloading and reloading it.
func (w *WASMRuntime) Reload(name string, wasmPath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Unload existing plugin if it exists
	if plugin, ok := w.plugins[name]; ok {
		plugin.Close(context.Background())
		delete(w.plugins, name)
		log.Debug().Str("name", name).Msg("Unloaded plugin for reload")
	}

	// Load the new plugin
	if err := w.loadPluginLocked(name, wasmPath); err != nil {
		return fmt.Errorf("reloading plugin: %w", err)
	}

	log.Info().Str("name", name).Str("path", wasmPath).Msg("Reloaded WASM plugin")

	return nil
}

// Call invokes a function in a loaded WASM plugin.
func (w *WASMRuntime) Call(name string, function string, input []byte) ([]byte, error) {
	w.mu.RLock()
	plugin, exists := w.plugins[name]
	w.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("plugin %s not found", name)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(w.config.TimeoutSeconds)*time.Second)
	defer cancel()

	exitCode, output, err := plugin.CallWithContext(ctx, function, input)
	if err != nil {
		return nil, fmt.Errorf("calling function %s: %w", function, err)
	}

	if exitCode != 0 {
		return nil, fmt.Errorf("function %s exited with code %d", function, exitCode)
	}

	return output, nil
}

// Close shuts down the runtime and unloads all plugins.
func (w *WASMRuntime) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	for name, plugin := range w.plugins {
		plugin.Close(context.Background())
		log.Debug().Str("name", name).Msg("Closed WASM plugin")
	}

	w.plugins = make(map[string]*extism.Plugin)

	return nil
}

// createHostFunctions creates host functions for Alyx API access.
func (w *WASMRuntime) createHostFunctions() []extism.HostFunction {
	// HTTP request host function
	httpRequest := extism.NewHostFunctionWithStack(
		"alyx_http_request",
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
			offset := stack[0]
			inputData, err := p.ReadBytes(offset)
			if err != nil {
				log.Error().Err(err).Msg("Failed to read HTTP request input from WASM memory")
				stack[0] = 0
				return
			}

			// Parse request
			var req struct {
				Method  string            `json:"method"`
				URL     string            `json:"url"`
				Headers map[string]string `json:"headers"`
				Body    string            `json:"body"`
			}
			err = json.Unmarshal(inputData, &req)
			if err != nil {
				log.Error().Err(err).Msg("Failed to parse HTTP request")
				stack[0] = 0
				return
			}

			// Make HTTP request
			httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, nil)
			if err != nil {
				log.Error().Err(err).Msg("Failed to create HTTP request")
				stack[0] = 0
				return
			}

			// Add headers
			for key, value := range req.Headers {
				httpReq.Header.Set(key, value)
			}

			// Execute request
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Do(httpReq)
			if err != nil {
				log.Error().Err(err).Msg("HTTP request failed")
				stack[0] = 0
				return
			}
			defer resp.Body.Close()

			// Read response body
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Error().Err(err).Msg("Failed to read HTTP response body")
				stack[0] = 0
				return
			}

			// Build response
			response := struct {
				Status     int               `json:"status"`
				StatusText string            `json:"status_text"`
				Headers    map[string]string `json:"headers"`
				Body       string            `json:"body"`
			}{
				Status:     resp.StatusCode,
				StatusText: resp.Status,
				Headers:    make(map[string]string),
				Body:       string(body),
			}

			// Copy headers
			for key, values := range resp.Header {
				if len(values) > 0 {
					response.Headers[key] = values[0]
				}
			}

			// Marshal response
			responseData, err := json.Marshal(response)
			if err != nil {
				log.Error().Err(err).Msg("Failed to marshal HTTP response")
				stack[0] = 0
				return
			}

			responseOffset, err := p.WriteBytes(responseData)
			if err != nil {
				log.Error().Err(err).Msg("Failed to write HTTP response to WASM memory")
				stack[0] = 0
				return
			}

			stack[0] = responseOffset
		},
		[]extism.ValueType{extism.ValueTypePTR, extism.ValueTypeI64}, // input offset, input length
		[]extism.ValueType{extism.ValueTypePTR},                      // output offset
	)

	return []extism.HostFunction{httpRequest}
}
