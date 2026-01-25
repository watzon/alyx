package functions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func createFunctionDir(t *testing.T, baseDir, name, entryFile, code string) {
	t.Helper()
	funcDir := filepath.Join(baseDir, name)
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(funcDir, entryFile), []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
}

func createFunctionDirWithManifest(t *testing.T, baseDir, name, entryFile, code, manifest string) {
	t.Helper()
	createFunctionDir(t, baseDir, name, entryFile, code)
	funcDir := filepath.Join(baseDir, name)
	if err := os.WriteFile(filepath.Join(funcDir, "manifest.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRegistry_Discover(t *testing.T) {
	dir := t.TempDir()

	createFunctionDir(t, dir, "hello", "index.js", "module.exports = {}")
	createFunctionDir(t, dir, "greet", "index.py", "default = {}")

	if err := os.MkdirAll(filepath.Join(dir, "_shared"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".hidden"), 0755); err != nil {
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
		t.Error("should not discover directories starting with underscore")
	}
	if _, ok := registry.Get(".hidden"); ok {
		t.Error("should not discover hidden directories")
	}
}

func TestRegistry_DiscoverWithManifest(t *testing.T) {
	dir := t.TempDir()

	manifest := `
name: compute
runtime: node
timeout: 60s
memory: 512mb
env:
  API_KEY: test-key
`
	createFunctionDirWithManifest(t, dir, "compute", "index.js", "module.exports = {}", manifest)

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

	createFunctionDir(t, dir, "a", "index.js", "")
	createFunctionDir(t, dir, "b", "index.js", "")
	createFunctionDir(t, dir, "c", "index.py", "")

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

type mockRegistrar struct {
	hooks     map[string][]HookConfig
	schedules map[string][]ScheduleConfig
	webhooks  map[string][]HookConfig
}

func newMockRegistrar() *mockRegistrar {
	return &mockRegistrar{
		hooks:     make(map[string][]HookConfig),
		schedules: make(map[string][]ScheduleConfig),
		webhooks:  make(map[string][]HookConfig),
	}
}

func (m *mockRegistrar) RegisterHooks(ctx context.Context, functionID string, hooks []HookConfig) error {
	m.hooks[functionID] = hooks
	return nil
}

func (m *mockRegistrar) RegisterSchedules(ctx context.Context, functionID string, schedules []ScheduleConfig) error {
	m.schedules[functionID] = schedules
	return nil
}

func (m *mockRegistrar) RegisterWebhooks(ctx context.Context, functionID string, hooks []HookConfig) error {
	m.webhooks[functionID] = hooks
	return nil
}

func TestRegistry_AutoRegistration(t *testing.T) {
	tmpDir := t.TempDir()

	manifestYAML := `
name: test-function
runtime: node
timeout: 30s
memory: 256mb

routes:
  - path: /api/test
    methods: [GET, POST]

hooks:
  - type: database
    source: users
    action: insert
    mode: async
    
  - type: webhook
    verification:
      type: hmac-sha256
      header: X-Signature
      secret: secret123

schedules:
  - name: daily-job
    type: cron
    expression: "0 0 * * *"
    timezone: UTC
`

	createFunctionDirWithManifest(t, tmpDir, "test-function", "index.js", "export default function() {}", manifestYAML)

	registry := NewRegistry(tmpDir)
	registrar := newMockRegistrar()
	registry.SetRegistrar(registrar)

	if err := registry.Discover(); err != nil {
		t.Fatalf("discovery failed: %v", err)
	}

	fn, ok := registry.Get("test-function")
	if !ok {
		t.Fatal("function not found")
	}

	if len(fn.Routes) != 1 {
		t.Errorf("expected 1 route, got %d", len(fn.Routes))
	}

	if len(fn.Hooks) != 2 {
		t.Errorf("expected 2 hooks, got %d", len(fn.Hooks))
	}

	if len(fn.Schedules) != 1 {
		t.Errorf("expected 1 schedule, got %d", len(fn.Schedules))
	}

	hooks, ok := registrar.hooks["test-function"]
	if !ok {
		t.Fatal("hooks not registered")
	}
	if len(hooks) != 2 {
		t.Errorf("expected 2 hooks registered, got %d", len(hooks))
	}

	schedules, ok := registrar.schedules["test-function"]
	if !ok {
		t.Fatal("schedules not registered")
	}
	if len(schedules) != 1 {
		t.Errorf("expected 1 schedule registered, got %d", len(schedules))
	}

	webhooks, ok := registrar.webhooks["test-function"]
	if !ok {
		t.Fatal("webhooks not registered")
	}
	if len(webhooks) != 1 {
		t.Errorf("expected 1 webhook registered, got %d", len(webhooks))
	}
}

func TestRegistry_BackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()

	legacyManifestYAML := `
name: legacy-function
runtime: python
timeout: 60s
memory: 512mb
env:
  API_KEY: secret
`

	createFunctionDirWithManifest(t, tmpDir, "legacy-function", "index.py", "def handler(req, res): pass", legacyManifestYAML)

	registry := NewRegistry(tmpDir)
	registrar := newMockRegistrar()
	registry.SetRegistrar(registrar)

	if err := registry.Discover(); err != nil {
		t.Fatalf("discovery failed: %v", err)
	}

	fn, ok := registry.Get("legacy-function")
	if !ok {
		t.Fatal("function not found")
	}

	if fn.Runtime != RuntimePython {
		t.Errorf("expected runtime python, got %s", fn.Runtime)
	}

	if fn.Timeout != 60 {
		t.Errorf("expected timeout 60, got %d", fn.Timeout)
	}

	if fn.Memory != 512 {
		t.Errorf("expected memory 512, got %d", fn.Memory)
	}

	if len(fn.Routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(fn.Routes))
	}

	if len(fn.Hooks) != 0 {
		t.Errorf("expected 0 hooks, got %d", len(fn.Hooks))
	}

	if len(fn.Schedules) != 0 {
		t.Errorf("expected 0 schedules, got %d", len(fn.Schedules))
	}

	if len(registrar.hooks) != 0 {
		t.Errorf("expected no hooks registered, got %d", len(registrar.hooks))
	}

	if len(registrar.schedules) != 0 {
		t.Errorf("expected no schedules registered, got %d", len(registrar.schedules))
	}

	if len(registrar.webhooks) != 0 {
		t.Errorf("expected no webhooks registered, got %d", len(registrar.webhooks))
	}
}

func TestRegistry_NoRegistrar(t *testing.T) {
	tmpDir := t.TempDir()

	manifestYAML := `
name: test-function
runtime: node

hooks:
  - type: database
    source: users
    action: insert
    mode: async
`

	createFunctionDirWithManifest(t, tmpDir, "test-function", "index.js", "export default function() {}", manifestYAML)

	registry := NewRegistry(tmpDir)

	if err := registry.Discover(); err != nil {
		t.Fatalf("discovery failed: %v", err)
	}

	fn, ok := registry.Get("test-function")
	if !ok {
		t.Fatal("function not found")
	}

	if len(fn.Hooks) != 1 {
		t.Errorf("expected 1 hook in function def, got %d", len(fn.Hooks))
	}
}

func TestRegistry_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()

	invalidManifestYAML := `
name: invalid-function
runtime: invalid-runtime
`

	createFunctionDirWithManifest(t, tmpDir, "invalid-function", "index.js", "export default function() {}", invalidManifestYAML)

	registry := NewRegistry(tmpDir)

	if err := registry.Discover(); err != nil {
		t.Fatalf("discovery failed: %v", err)
	}

	if _, ok := registry.Get("invalid-function"); ok {
		t.Error("expected invalid function to be skipped, but it was registered")
	}
}

func TestRegistry_MultipleHookTypes(t *testing.T) {
	tmpDir := t.TempDir()

	manifestYAML := `
name: multi-hook
runtime: node

hooks:
  - type: database
    source: users
    action: insert
    mode: async
    
  - type: auth
    source: signup
    action: after
    mode: sync
    
  - type: webhook
    verification:
      type: hmac-sha256
      header: X-Signature
      secret: secret
`

	createFunctionDirWithManifest(t, tmpDir, "multi-hook", "index.js", "export default function() {}", manifestYAML)

	registry := NewRegistry(tmpDir)
	registrar := newMockRegistrar()
	registry.SetRegistrar(registrar)

	if err := registry.Discover(); err != nil {
		t.Fatalf("discovery failed: %v", err)
	}

	hooks, ok := registrar.hooks["multi-hook"]
	if !ok {
		t.Fatal("hooks not registered")
	}
	if len(hooks) != 3 {
		t.Errorf("expected 3 hooks registered, got %d", len(hooks))
	}

	webhooks, ok := registrar.webhooks["multi-hook"]
	if !ok {
		t.Fatal("webhooks not registered")
	}
	if len(webhooks) != 1 {
		t.Errorf("expected 1 webhook registered, got %d", len(webhooks))
	}

	if webhooks[0].Type != "webhook" {
		t.Errorf("expected webhook type, got %s", webhooks[0].Type)
	}
}

func TestRegistry_DirectoryWithNoEntryFile(t *testing.T) {
	tmpDir := t.TempDir()

	funcDir := filepath.Join(tmpDir, "no-entry")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(funcDir, "readme.txt"), []byte("not a function"), 0644); err != nil {
		t.Fatal(err)
	}

	registry := NewRegistry(tmpDir)
	if err := registry.Discover(); err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if registry.Count() != 0 {
		t.Errorf("expected 0 functions, got %d", registry.Count())
	}
}
