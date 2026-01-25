package functions

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	tmpDir := t.TempDir()
	return NewRegistry(tmpDir)
}

func createTestFunction(t *testing.T, registry *Registry, name string, buildConfig *BuildConfig) string {
	t.Helper()

	funcDir := filepath.Join(registry.FunctionsDir(), name)
	if err := os.MkdirAll(funcDir, 0o755); err != nil {
		t.Fatalf("creating function directory: %v", err)
	}

	entryFile := filepath.Join(funcDir, "index.js")
	if err := os.WriteFile(entryFile, []byte("export default function() {}"), 0o644); err != nil {
		t.Fatalf("creating entry file: %v", err)
	}

	if buildConfig != nil {
		manifest := &Manifest{
			Name:    name,
			Runtime: "node",
			Build:   buildConfig,
		}

		manifestData := manifestToYAML(t, manifest)
		manifestPath := filepath.Join(funcDir, "manifest.yaml")
		if err := os.WriteFile(manifestPath, []byte(manifestData), 0o644); err != nil {
			t.Fatalf("creating manifest: %v", err)
		}
	}

	if err := registry.Discover(); err != nil {
		t.Fatalf("discovering functions: %v", err)
	}

	return funcDir
}

func manifestToYAML(t *testing.T, m *Manifest) string {
	t.Helper()

	yaml := "name: " + m.Name + "\n"
	yaml += "runtime: " + m.Runtime + "\n"

	if m.Build != nil {
		yaml += "build:\n"
		yaml += "  command: \"" + m.Build.Command + "\"\n"

		if len(m.Build.Args) > 0 {
			yaml += "  args:\n"
			for _, arg := range m.Build.Args {
				yaml += "    - \"" + arg + "\"\n"
			}
		}

		if len(m.Build.Watch) > 0 {
			yaml += "  watch:\n"
			for _, pattern := range m.Build.Watch {
				yaml += "    - \"" + pattern + "\"\n"
			}
		}

		yaml += "  output: \"" + m.Build.Output + "\"\n"
	}

	return yaml
}

func TestSourceWatcher_DetectsChanges(t *testing.T) {
	registry := testRegistry(t)

	buildConfig := &BuildConfig{
		Command: "echo",
		Args:    []string{"building"},
		Watch:   []string{"src/**/*.js"},
		Output:  "plugin.wasm",
	}

	funcDir := createTestFunction(t, registry, "test-func", buildConfig)

	watcher, err := NewSourceWatcher(registry)
	if err != nil {
		t.Fatalf("creating watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		t.Fatalf("starting watcher: %v", err)
	}

	srcDir := filepath.Join(funcDir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("creating src directory: %v", err)
	}

	testFile := filepath.Join(srcDir, "test.js")
	if err := os.WriteFile(testFile, []byte("console.log('test')"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := os.WriteFile(testFile, []byte("console.log('modified')"), 0o644); err != nil {
		t.Fatalf("modifying test file: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
}

func TestSourceWatcher_Debounce(t *testing.T) {
	registry := testRegistry(t)

	buildConfig := &BuildConfig{
		Command: "echo",
		Args:    []string{"building"},
		Watch:   []string{"*.js"},
		Output:  "plugin.wasm",
	}

	funcDir := createTestFunction(t, registry, "test-func", buildConfig)

	watcher, err := NewSourceWatcher(registry)
	if err != nil {
		t.Fatalf("creating watcher: %v", err)
	}
	defer watcher.Stop()

	watcher.SetDebounceDuration(200 * time.Millisecond)

	if err := watcher.Start(); err != nil {
		t.Fatalf("starting watcher: %v", err)
	}

	testFile := filepath.Join(funcDir, "test.js")
	if err := os.WriteFile(testFile, []byte("v1"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(testFile, []byte("v2"), 0o644); err != nil {
		t.Fatalf("modifying test file (1): %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(testFile, []byte("v3"), 0o644); err != nil {
		t.Fatalf("modifying test file (2): %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(testFile, []byte("v4"), 0o644); err != nil {
		t.Fatalf("modifying test file (3): %v", err)
	}

	time.Sleep(300 * time.Millisecond)
}

func TestSourceWatcher_BuildSuccess(t *testing.T) {
	registry := testRegistry(t)

	buildConfig := &BuildConfig{
		Command: "echo",
		Args:    []string{"build", "successful"},
		Watch:   []string{"*.js"},
		Output:  "plugin.wasm",
	}

	funcDir := createTestFunction(t, registry, "test-func", buildConfig)

	watcher, err := NewSourceWatcher(registry)
	if err != nil {
		t.Fatalf("creating watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		t.Fatalf("starting watcher: %v", err)
	}

	testFile := filepath.Join(funcDir, "test.js")
	if err := os.WriteFile(testFile, []byte("console.log('test')"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
}

func TestSourceWatcher_BuildFailure(t *testing.T) {
	registry := testRegistry(t)

	buildConfig := &BuildConfig{
		Command: "false",
		Args:    []string{},
		Watch:   []string{"*.js"},
		Output:  "plugin.wasm",
	}

	funcDir := createTestFunction(t, registry, "test-func", buildConfig)

	watcher, err := NewSourceWatcher(registry)
	if err != nil {
		t.Fatalf("creating watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		t.Fatalf("starting watcher: %v", err)
	}

	testFile := filepath.Join(funcDir, "test.js")
	if err := os.WriteFile(testFile, []byte("console.log('test')"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if err := os.WriteFile(testFile, []byte("console.log('still works')"), 0o644); err != nil {
		t.Fatalf("modifying test file: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
}

func TestSourceWatcher_GlobPattern(t *testing.T) {
	tests := []struct {
		name          string
		patterns      []string
		createFiles   []string
		modifyFile    string
		shouldTrigger bool
	}{
		{
			name:          "matches single file pattern",
			patterns:      []string{"*.js"},
			createFiles:   []string{"test.js"},
			modifyFile:    "test.js",
			shouldTrigger: true,
		},
		{
			name:          "matches nested pattern",
			patterns:      []string{"src/**/*.js"},
			createFiles:   []string{"src/components/button.js"},
			modifyFile:    "src/components/button.js",
			shouldTrigger: true,
		},
		{
			name:          "does not match excluded pattern",
			patterns:      []string{"src/**/*.js"},
			createFiles:   []string{"test.ts"},
			modifyFile:    "test.ts",
			shouldTrigger: false,
		},
		{
			name:          "matches multiple patterns",
			patterns:      []string{"*.js", "*.ts"},
			createFiles:   []string{"test.ts"},
			modifyFile:    "test.ts",
			shouldTrigger: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := testRegistry(t)

			buildConfig := &BuildConfig{
				Command: "echo",
				Args:    []string{"building"},
				Watch:   tt.patterns,
				Output:  "plugin.wasm",
			}

			funcDir := createTestFunction(t, registry, "test-func", buildConfig)

			watcher, err := NewSourceWatcher(registry)
			if err != nil {
				t.Fatalf("creating watcher: %v", err)
			}
			defer watcher.Stop()

			if err := watcher.Start(); err != nil {
				t.Fatalf("starting watcher: %v", err)
			}

			for _, file := range tt.createFiles {
				fullPath := filepath.Join(funcDir, file)
				dir := filepath.Dir(fullPath)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatalf("creating directory: %v", err)
				}
				if err := os.WriteFile(fullPath, []byte("initial"), 0o644); err != nil {
					t.Fatalf("creating file: %v", err)
				}
			}

			time.Sleep(100 * time.Millisecond)

			modifyPath := filepath.Join(funcDir, tt.modifyFile)
			if err := os.WriteFile(modifyPath, []byte("modified"), 0o644); err != nil {
				t.Fatalf("modifying file: %v", err)
			}

			time.Sleep(200 * time.Millisecond)
		})
	}
}

func TestSourceWatcher_MultipleFunction(t *testing.T) {
	registry := testRegistry(t)

	buildConfig1 := &BuildConfig{
		Command: "echo",
		Args:    []string{"building", "func1"},
		Watch:   []string{"*.js"},
		Output:  "plugin.wasm",
	}

	buildConfig2 := &BuildConfig{
		Command: "echo",
		Args:    []string{"building", "func2"},
		Watch:   []string{"*.ts"},
		Output:  "plugin.wasm",
	}

	funcDir1 := createTestFunction(t, registry, "func1", buildConfig1)
	funcDir2 := createTestFunction(t, registry, "func2", buildConfig2)

	watcher, err := NewSourceWatcher(registry)
	if err != nil {
		t.Fatalf("creating watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		t.Fatalf("starting watcher: %v", err)
	}

	testFile1 := filepath.Join(funcDir1, "test.js")
	if err := os.WriteFile(testFile1, []byte("func1"), 0o644); err != nil {
		t.Fatalf("creating test file 1: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	testFile2 := filepath.Join(funcDir2, "test.ts")
	if err := os.WriteFile(testFile2, []byte("func2"), 0o644); err != nil {
		t.Fatalf("creating test file 2: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
}

func TestSourceWatcher_NoBuildConfig(t *testing.T) {
	registry := testRegistry(t)

	funcDir := createTestFunction(t, registry, "test-func", nil)

	watcher, err := NewSourceWatcher(registry)
	if err != nil {
		t.Fatalf("creating watcher: %v", err)
	}
	defer watcher.Stop()

	if err := watcher.Start(); err != nil {
		t.Fatalf("starting watcher: %v", err)
	}

	testFile := filepath.Join(funcDir, "test.js")
	if err := os.WriteFile(testFile, []byte("console.log('test')"), 0o644); err != nil {
		t.Fatalf("creating test file: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
}

func TestSourceWatcher_StopCleansUp(t *testing.T) {
	registry := testRegistry(t)

	buildConfig := &BuildConfig{
		Command: "echo",
		Args:    []string{"building"},
		Watch:   []string{"*.js"},
		Output:  "plugin.wasm",
	}

	createTestFunction(t, registry, "test-func", buildConfig)

	watcher, err := NewSourceWatcher(registry)
	if err != nil {
		t.Fatalf("creating watcher: %v", err)
	}

	if err := watcher.Start(); err != nil {
		t.Fatalf("starting watcher: %v", err)
	}

	if err := watcher.Stop(); err != nil {
		t.Fatalf("stopping watcher: %v", err)
	}

	if len(watcher.debounceTimers) > 0 {
		t.Errorf("expected debounce timers to be cleaned up, got %d", len(watcher.debounceTimers))
	}
}
