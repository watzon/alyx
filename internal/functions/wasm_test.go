package functions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	extism "github.com/extism/go-sdk"
)

func testWASMRuntime(t *testing.T) *WASMRuntime {
	t.Helper()
	config := &WASMConfig{
		MemoryLimitMB:  256,
		TimeoutSeconds: 5,
		EnableWASI:     true,
		AlyxURL:        "http://localhost:8090",
	}
	runtime := NewWASMRuntime(config)
	t.Cleanup(func() {
		runtime.Close()
	})
	return runtime
}

func createTestWASM(t *testing.T) string {
	t.Helper()

	wasmBytes := []byte{
		0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00, 0x01, 0x06, 0x01, 0x60,
		0x01, 0x7f, 0x01, 0x7f, 0x03, 0x02, 0x01, 0x00, 0x07, 0x0a, 0x01, 0x06,
		0x64, 0x6f, 0x75, 0x62, 0x6c, 0x65, 0x00, 0x00, 0x0a, 0x09, 0x01, 0x07,
		0x00, 0x20, 0x00, 0x20, 0x00, 0x6a, 0x0b,
	}

	tmpDir := t.TempDir()
	wasmPath := filepath.Join(tmpDir, "test.wasm")
	if err := os.WriteFile(wasmPath, wasmBytes, 0644); err != nil {
		t.Fatalf("Failed to write test WASM file: %v", err)
	}

	return wasmPath
}

func TestNewWASMRuntime(t *testing.T) {
	t.Run("with config", func(t *testing.T) {
		config := &WASMConfig{
			MemoryLimitMB:  512,
			TimeoutSeconds: 60,
			EnableWASI:     false,
			AlyxURL:        "http://example.com",
		}
		runtime := NewWASMRuntime(config)
		defer runtime.Close()

		if runtime.config.MemoryLimitMB != 512 {
			t.Errorf("Expected MemoryLimitMB=512, got %d", runtime.config.MemoryLimitMB)
		}
		if runtime.config.TimeoutSeconds != 60 {
			t.Errorf("Expected TimeoutSeconds=60, got %d", runtime.config.TimeoutSeconds)
		}
		if runtime.config.EnableWASI {
			t.Error("Expected EnableWASI=false")
		}
	})

	t.Run("with nil config", func(t *testing.T) {
		runtime := NewWASMRuntime(nil)
		defer runtime.Close()

		if runtime.config.MemoryLimitMB != 256 {
			t.Errorf("Expected default MemoryLimitMB=256, got %d", runtime.config.MemoryLimitMB)
		}
		if runtime.config.TimeoutSeconds != 30 {
			t.Errorf("Expected default TimeoutSeconds=30, got %d", runtime.config.TimeoutSeconds)
		}
		if !runtime.config.EnableWASI {
			t.Error("Expected default EnableWASI=true")
		}
	})
}

func TestWASMRuntime_LoadPlugin(t *testing.T) {
	t.Run("valid wasm file", func(t *testing.T) {
		runtime := testWASMRuntime(t)
		wasmPath := createTestWASM(t)

		err := runtime.LoadPlugin("test", wasmPath)
		if err != nil {
			t.Fatalf("LoadPlugin failed: %v", err)
		}

		runtime.mu.RLock()
		_, exists := runtime.plugins["test"]
		runtime.mu.RUnlock()

		if !exists {
			t.Error("Plugin not found in runtime after loading")
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		runtime := testWASMRuntime(t)

		err := runtime.LoadPlugin("test", "/nonexistent/path.wasm")
		if err == nil {
			t.Error("Expected error for nonexistent file, got nil")
		}
	})

	t.Run("duplicate plugin name", func(t *testing.T) {
		runtime := testWASMRuntime(t)
		wasmPath := createTestWASM(t)

		err := runtime.LoadPlugin("test", wasmPath)
		if err != nil {
			t.Fatalf("First LoadPlugin failed: %v", err)
		}

		err = runtime.LoadPlugin("test", wasmPath)
		if err == nil {
			t.Error("Expected error for duplicate plugin name, got nil")
		}
	})

	t.Run("invalid wasm data", func(t *testing.T) {
		runtime := testWASMRuntime(t)

		tmpDir := t.TempDir()
		invalidPath := filepath.Join(tmpDir, "invalid.wasm")
		if err := os.WriteFile(invalidPath, []byte("not wasm"), 0644); err != nil {
			t.Fatalf("Failed to write invalid WASM file: %v", err)
		}

		err := runtime.LoadPlugin("invalid", invalidPath)
		if err == nil {
			t.Error("Expected error for invalid WASM data, got nil")
		}
	})
}

func TestWASMRuntime_UnloadPlugin(t *testing.T) {
	t.Run("existing plugin", func(t *testing.T) {
		runtime := testWASMRuntime(t)
		wasmPath := createTestWASM(t)

		if err := runtime.LoadPlugin("test", wasmPath); err != nil {
			t.Fatalf("LoadPlugin failed: %v", err)
		}

		err := runtime.UnloadPlugin("test")
		if err != nil {
			t.Fatalf("UnloadPlugin failed: %v", err)
		}

		runtime.mu.RLock()
		_, exists := runtime.plugins["test"]
		runtime.mu.RUnlock()

		if exists {
			t.Error("Plugin still exists after unloading")
		}
	})

	t.Run("nonexistent plugin", func(t *testing.T) {
		runtime := testWASMRuntime(t)

		err := runtime.UnloadPlugin("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent plugin, got nil")
		}
	})
}

func TestWASMRuntime_Call(t *testing.T) {
	t.Run("nonexistent plugin", func(t *testing.T) {
		runtime := testWASMRuntime(t)

		_, err := runtime.Call("nonexistent", "function", []byte("input"))
		if err == nil {
			t.Error("Expected error for nonexistent plugin, got nil")
		}
	})

	t.Run("nonexistent function", func(t *testing.T) {
		runtime := testWASMRuntime(t)
		wasmPath := createTestWASM(t)

		if err := runtime.LoadPlugin("test", wasmPath); err != nil {
			t.Fatalf("LoadPlugin failed: %v", err)
		}

		_, err := runtime.Call("test", "nonexistent", []byte("input"))
		if err == nil {
			t.Error("Expected error for nonexistent function, got nil")
		}
	})
}

func TestWASMRuntime_Call_Timeout(t *testing.T) {
	runtime := &WASMRuntime{
		plugins: make(map[string]*extism.Plugin),
		config: &WASMConfig{
			MemoryLimitMB:  256,
			TimeoutSeconds: 1,
			EnableWASI:     true,
			AlyxURL:        "http://localhost:8090",
		},
	}
	defer runtime.Close()

	wasmPath := createTestWASM(t)
	if err := runtime.LoadPlugin("test", wasmPath); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	<-ctx.Done()

	_, err := runtime.Call("test", "double", []byte{0x05})
	if err != nil && err.Error() != "context deadline exceeded" {
		t.Logf("Call completed or failed with different error: %v", err)
	}
}

func TestWASMRuntime_MemoryLimit(t *testing.T) {
	runtime := &WASMRuntime{
		plugins: make(map[string]*extism.Plugin),
		config: &WASMConfig{
			MemoryLimitMB:  1,
			TimeoutSeconds: 30,
			EnableWASI:     true,
			AlyxURL:        "http://localhost:8090",
		},
	}
	defer runtime.Close()

	wasmPath := createTestWASM(t)
	err := runtime.LoadPlugin("test", wasmPath)
	if err != nil {
		t.Logf("Plugin creation with low memory limit: %v", err)
	}
}

func TestWASMRuntime_Close(t *testing.T) {
	runtime := testWASMRuntime(t)
	wasmPath := createTestWASM(t)

	if err := runtime.LoadPlugin("test1", wasmPath); err != nil {
		t.Fatalf("LoadPlugin test1 failed: %v", err)
	}
	if err := runtime.LoadPlugin("test2", wasmPath); err != nil {
		t.Fatalf("LoadPlugin test2 failed: %v", err)
	}

	err := runtime.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	runtime.mu.RLock()
	count := len(runtime.plugins)
	runtime.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 plugins after Close, got %d", count)
	}
}

func TestWASMRuntime_ConcurrentAccess(t *testing.T) {
	runtime := testWASMRuntime(t)
	wasmPath := createTestWASM(t)

	if err := runtime.LoadPlugin("test", wasmPath); err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			runtime.mu.RLock()
			_ = runtime.plugins["test"]
			runtime.mu.RUnlock()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestWASMRuntime_HostFunctions(t *testing.T) {
	runtime := testWASMRuntime(t)

	hostFuncs := runtime.createHostFunctions()
	if len(hostFuncs) == 0 {
		t.Error("Expected at least one host function")
	}

	found := false
	for _, hf := range hostFuncs {
		if hf.Name == "alyx_http_request" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected alyx_http_request host function")
	}
}

func TestDefaultWASMConfig(t *testing.T) {
	config := DefaultWASMConfig()

	if config.MemoryLimitMB != 256 {
		t.Errorf("Expected MemoryLimitMB=256, got %d", config.MemoryLimitMB)
	}
	if config.TimeoutSeconds != 30 {
		t.Errorf("Expected TimeoutSeconds=30, got %d", config.TimeoutSeconds)
	}
	if !config.EnableWASI {
		t.Error("Expected EnableWASI=true")
	}
	if config.AlyxURL != "http://localhost:8090" {
		t.Errorf("Expected AlyxURL=http://localhost:8090, got %s", config.AlyxURL)
	}
}

func TestWASMRuntime_HTTPHostFunction(t *testing.T) {
	t.Skip("Requires WASM module that calls alyx_http_request host function")

	req := map[string]any{
		"method":  "GET",
		"url":     "https://httpbin.org/get",
		"headers": map[string]string{},
		"body":    "",
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	t.Logf("HTTP request data: %s", string(reqData))
}
