package functions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry_Discover(t *testing.T) {
	dir := t.TempDir()

	// Create test function files
	if err := os.WriteFile(filepath.Join(dir, "hello.js"), []byte("module.exports = {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "greet.py"), []byte("default = {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_shared.js"), []byte("// shared"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden.js"), []byte("// hidden"), 0644); err != nil {
		t.Fatal(err)
	}

	registry := NewRegistry(dir)
	if err := registry.Discover(); err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if registry.Count() != 2 {
		t.Errorf("expected 2 functions, got %d", registry.Count())
	}

	fn, ok := registry.Get("hello")
	if !ok {
		t.Error("expected to find 'hello' function")
	} else if fn.Runtime != RuntimeNode {
		t.Errorf("expected runtime node, got %s", fn.Runtime)
	}

	fn, ok = registry.Get("greet")
	if !ok {
		t.Error("expected to find 'greet' function")
	} else if fn.Runtime != RuntimePython {
		t.Errorf("expected runtime python, got %s", fn.Runtime)
	}

	if _, ok := registry.Get("_shared"); ok {
		t.Error("should not discover files starting with underscore")
	}
	if _, ok := registry.Get(".hidden"); ok {
		t.Error("should not discover hidden files")
	}
}

func TestRegistry_DiscoverWithManifest(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "compute.js"), []byte("module.exports = {}"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest := `
name: compute
runtime: node
timeout: 60s
memory: 512mb
env:
  API_KEY: test-key
`
	if err := os.WriteFile(filepath.Join(dir, "compute.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	registry := NewRegistry(dir)
	if err := registry.Discover(); err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	fn, ok := registry.Get("compute")
	if !ok {
		t.Fatal("expected to find 'compute' function")
	}

	if !fn.HasManifest {
		t.Error("expected HasManifest to be true")
	}
	if fn.Timeout != 60 {
		t.Errorf("expected timeout 60, got %d", fn.Timeout)
	}
	if fn.Memory != 512 {
		t.Errorf("expected memory 512, got %d", fn.Memory)
	}
	if fn.Env["API_KEY"] != "test-key" {
		t.Errorf("expected API_KEY=test-key, got %s", fn.Env["API_KEY"])
	}
}

func TestRegistry_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	registry := NewRegistry(dir)
	if err := registry.Discover(); err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if registry.Count() != 0 {
		t.Errorf("expected 0 functions, got %d", registry.Count())
	}
}

func TestRegistry_NonExistentDirectory(t *testing.T) {
	registry := NewRegistry("/nonexistent/path")
	if err := registry.Discover(); err != nil {
		t.Fatalf("Discover should not fail for nonexistent directory: %v", err)
	}

	if registry.Count() != 0 {
		t.Errorf("expected 0 functions, got %d", registry.Count())
	}
}

func TestRegistry_GetByRuntime(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "a.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.js"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c.py"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	registry := NewRegistry(dir)
	if err := registry.Discover(); err != nil {
		t.Fatal(err)
	}

	nodeFuncs := registry.GetByRuntime(RuntimeNode)
	if len(nodeFuncs) != 2 {
		t.Errorf("expected 2 node functions, got %d", len(nodeFuncs))
	}

	pythonFuncs := registry.GetByRuntime(RuntimePython)
	if len(pythonFuncs) != 1 {
		t.Errorf("expected 1 python function, got %d", len(pythonFuncs))
	}
}

func TestParseTimeoutSeconds(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"30", 30},
		{"30s", 30},
		{"5m", 300},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		result := parseTimeoutSeconds(tt.input)
		if result != tt.expected {
			t.Errorf("parseTimeoutSeconds(%q) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestParseMemoryMB(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"256", 256},
		{"256mb", 256},
		{"256MB", 256},
		{"1gb", 1024},
		{"1GB", 1024},
		{"512m", 512},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		result := parseMemoryMB(tt.input)
		if result != tt.expected {
			t.Errorf("parseMemoryMB(%q) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestDetectRuntime(t *testing.T) {
	tests := []struct {
		ext      string
		expected Runtime
	}{
		{".js", RuntimeNode},
		{".mjs", RuntimeNode},
		{".cjs", RuntimeNode},
		{".py", RuntimePython},
		{".go", RuntimeGo},
		{".txt", ""},
		{".rs", ""},
	}

	for _, tt := range tests {
		result := detectRuntime(tt.ext)
		if result != tt.expected {
			t.Errorf("detectRuntime(%q) = %q, expected %q", tt.ext, result, tt.expected)
		}
	}
}
