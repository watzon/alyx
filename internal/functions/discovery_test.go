package functions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/watzon/alyx/internal/schema"
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

func TestNewRegistryFromSchema(t *testing.T) {
	dir := t.TempDir()

	createFunctionDir(t, dir, "hello", "index.js", "module.exports = {}")
	createFunctionDir(t, dir, "greet", "main.py", "def handler(): pass")

	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"hello": {
				Runtime:    "node",
				Entrypoint: "index.js",
			},
			"greet": {
				Runtime:    "python",
				Entrypoint: "main.py",
			},
		},
	}

	registry, err := NewRegistryFromSchema(s, dir, nil)
	if err != nil {
		t.Fatalf("NewRegistryFromSchema failed: %v", err)
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
}

func TestNewRegistryFromSchema_EmptySchema(t *testing.T) {
	dir := t.TempDir()

	s := &schema.Schema{
		Functions: nil,
	}

	registry, err := NewRegistryFromSchema(s, dir, nil)
	if err != nil {
		t.Fatalf("NewRegistryFromSchema failed: %v", err)
	}

	if registry.Count() != 0 {
		t.Errorf("expected 0 functions, got %d", registry.Count())
	}
}

func TestNewRegistryFromSchema_MissingEntrypoint(t *testing.T) {
	dir := t.TempDir()

	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"missing": {
				Runtime:    "node",
				Entrypoint: "index.js",
			},
		},
	}

	_, err := NewRegistryFromSchema(s, dir, nil)
	if err == nil {
		t.Error("expected error for missing entrypoint")
	}
}

func TestNewRegistryFromSchema_WithBunRuntime(t *testing.T) {
	dir := t.TempDir()

	createFunctionDir(t, dir, "bun-func", "index.ts", "export default () => {}")

	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"bun-func": {
				Runtime:    "bun",
				Entrypoint: "index.ts",
			},
		},
	}

	registry, err := NewRegistryFromSchema(s, dir, nil)
	if err != nil {
		t.Fatalf("NewRegistryFromSchema failed: %v", err)
	}

	fn, ok := registry.Get("bun-func")
	if !ok {
		t.Error("expected to find 'bun-func' function")
	} else if fn.Runtime != RuntimeBun {
		t.Errorf("expected runtime bun, got %s", fn.Runtime)
	}
}

func TestNewRegistryFromSchema_WithCustomPath(t *testing.T) {
	dir := t.TempDir()
	customDir := filepath.Join(dir, "custom-location")

	createFunctionDir(t, customDir, "", "handler.js", "module.exports = {}")

	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"custom": {
				Runtime:    "node",
				Entrypoint: "handler.js",
				Path:       customDir,
			},
		},
	}

	registry, err := NewRegistryFromSchema(s, dir, nil)
	if err != nil {
		t.Fatalf("NewRegistryFromSchema failed: %v", err)
	}

	fn, ok := registry.Get("custom")
	if !ok {
		t.Error("expected to find 'custom' function")
	}
	if fn.Path != filepath.Join(customDir, "handler.js") {
		t.Errorf("expected custom path, got %s", fn.Path)
	}
}

func TestRegistry_Get(t *testing.T) {
	dir := t.TempDir()
	createFunctionDir(t, dir, "test-func", "index.js", "module.exports = {}")

	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"test-func": {
				Runtime:    "node",
				Entrypoint: "index.js",
			},
		},
	}

	registry, err := NewRegistryFromSchema(s, dir, nil)
	if err != nil {
		t.Fatalf("NewRegistryFromSchema failed: %v", err)
	}

	fn, ok := registry.Get("test-func")
	if !ok {
		t.Error("expected to find function")
	}
	if fn.Name != "test-func" {
		t.Errorf("expected name 'test-func', got %s", fn.Name)
	}

	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("should not find nonexistent function")
	}
}

func TestRegistry_List(t *testing.T) {
	dir := t.TempDir()
	createFunctionDir(t, dir, "func1", "index.js", "module.exports = {}")
	createFunctionDir(t, dir, "func2", "main.py", "def handler(): pass")

	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"func1": {
				Runtime:    "node",
				Entrypoint: "index.js",
			},
			"func2": {
				Runtime:    "python",
				Entrypoint: "main.py",
			},
		},
	}

	registry, err := NewRegistryFromSchema(s, dir, nil)
	if err != nil {
		t.Fatalf("NewRegistryFromSchema failed: %v", err)
	}

	funcs := registry.List()
	if len(funcs) != 2 {
		t.Errorf("expected 2 functions, got %d", len(funcs))
	}
}

func TestRegistry_GetByRuntime(t *testing.T) {
	dir := t.TempDir()
	createFunctionDir(t, dir, "node1", "index.js", "module.exports = {}")
	createFunctionDir(t, dir, "node2", "index.js", "module.exports = {}")
	createFunctionDir(t, dir, "python1", "main.py", "def handler(): pass")

	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"node1": {
				Runtime:    "node",
				Entrypoint: "index.js",
			},
			"node2": {
				Runtime:    "node",
				Entrypoint: "index.js",
			},
			"python1": {
				Runtime:    "python",
				Entrypoint: "main.py",
			},
		},
	}

	registry, err := NewRegistryFromSchema(s, dir, nil)
	if err != nil {
		t.Fatalf("NewRegistryFromSchema failed: %v", err)
	}

	nodeFuncs := registry.GetByRuntime(RuntimeNode)
	if len(nodeFuncs) != 2 {
		t.Errorf("expected 2 node functions, got %d", len(nodeFuncs))
	}

	pythonFuncs := registry.GetByRuntime(RuntimePython)
	if len(pythonFuncs) != 1 {
		t.Errorf("expected 1 python function, got %d", len(pythonFuncs))
	}

	goFuncs := registry.GetByRuntime(RuntimeGo)
	if len(goFuncs) != 0 {
		t.Errorf("expected 0 go functions, got %d", len(goFuncs))
	}
}

func TestFunctionDef_GetEntrypoint(t *testing.T) {
	fn := &FunctionDef{
		Name:       "test",
		Path:       "/src/index.ts",
		OutputPath: "/dist/index.js",
		HasBuild:   true,
	}

	if ep := fn.GetEntrypoint(true); ep != "/src/index.ts" {
		t.Errorf("dev mode should return source path, got %s", ep)
	}

	if ep := fn.GetEntrypoint(false); ep != "/dist/index.js" {
		t.Errorf("prod mode with build should return output path, got %s", ep)
	}

	fnNoBuild := &FunctionDef{
		Name:     "test",
		Path:     "/src/index.ts",
		HasBuild: false,
	}

	if ep := fnNoBuild.GetEntrypoint(false); ep != "/src/index.ts" {
		t.Errorf("prod mode without build should return source path, got %s", ep)
	}
}
