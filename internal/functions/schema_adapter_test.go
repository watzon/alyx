package functions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/watzon/alyx/internal/schema"
)

func TestSchemaToFunctionDefs_Empty(t *testing.T) {
	s := &schema.Schema{}
	defs, err := SchemaToFunctionDefs(s, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(defs) != 0 {
		t.Errorf("expected 0 defs, got %d", len(defs))
	}
}

func TestSchemaToFunctionDefs_Minimal(t *testing.T) {
	tmpDir := t.TempDir()
	funcDir := filepath.Join(tmpDir, "hello")
	if err := os.Mkdir(funcDir, 0755); err != nil {
		t.Fatal(err)
	}
	entrypoint := filepath.Join(funcDir, "index.js")
	if err := os.WriteFile(entrypoint, []byte("export default () => {}"), 0644); err != nil {
		t.Fatal(err)
	}

	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"hello": {
				Runtime:    "node",
				Entrypoint: "index.js",
			},
		},
	}

	defs, err := SchemaToFunctionDefs(s, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}

	def := defs[0]
	if def.Name != "hello" {
		t.Errorf("expected name 'hello', got %q", def.Name)
	}
	if def.Runtime != RuntimeNode {
		t.Errorf("expected runtime 'node', got %q", def.Runtime)
	}
	if def.Path != funcDir {
		t.Errorf("expected path %q, got %q", funcDir, def.Path)
	}
	if def.Timeout != 30 {
		t.Errorf("expected timeout 30, got %d", def.Timeout)
	}
	if def.Memory != 128 {
		t.Errorf("expected memory 128, got %d", def.Memory)
	}
}

func TestSchemaToFunctionDefs_FullConfig(t *testing.T) {
	tmpDir := t.TempDir()
	funcDir := filepath.Join(tmpDir, "full")
	if err := os.Mkdir(funcDir, 0755); err != nil {
		t.Fatal(err)
	}
	entrypoint := filepath.Join(funcDir, "main.py")
	if err := os.WriteFile(entrypoint, []byte("def handler(): pass"), 0644); err != nil {
		t.Fatal(err)
	}

	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"full": {
				Runtime:    "python",
				Entrypoint: "main.py",
				Timeout:    "5m",
				Memory:     "512mb",
				Env: map[string]string{
					"API_KEY": "secret",
				},
				Dependencies: []string{"requests"},
				Hooks: []schema.FunctionHook{
					{
						Type:   "database",
						Source: "users",
						Action: "insert",
						Mode:   "async",
					},
				},
				Schedules: []schema.FunctionSchedule{
					{
						Name:       "daily",
						Type:       "cron",
						Expression: "0 0 * * *",
						Timezone:   "UTC",
						Config: map[string]interface{}{
							"enabled": true,
						},
						Input: map[string]interface{}{
							"task": "cleanup",
						},
					},
				},
				Routes: []schema.FunctionRoute{
					{
						Path:    "/api/full",
						Methods: []string{"GET", "POST"},
					},
				},
				Build: &schema.FunctionBuild{
					Command: "python",
					Args:    []string{"-m", "build"},
					Watch:   []string{"*.py"},
					Output:  "dist/main.py",
				},
			},
		},
	}

	defs, err := SchemaToFunctionDefs(s, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}

	def := defs[0]
	if def.Timeout != 300 {
		t.Errorf("expected timeout 300, got %d", def.Timeout)
	}
	if def.Memory != 512 {
		t.Errorf("expected memory 512, got %d", def.Memory)
	}
	if def.Env["API_KEY"] != "secret" {
		t.Errorf("expected env API_KEY=secret, got %q", def.Env["API_KEY"])
	}
	if len(def.Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(def.Hooks))
	}
	if def.Hooks[0].Type != "database" {
		t.Errorf("expected hook type 'database', got %q", def.Hooks[0].Type)
	}
	if len(def.Schedules) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(def.Schedules))
	}
	if def.Schedules[0].Name != "daily" {
		t.Errorf("expected schedule name 'daily', got %q", def.Schedules[0].Name)
	}
	if def.Schedules[0].Input["task"] != "cleanup" {
		t.Errorf("expected schedule input task=cleanup, got %v", def.Schedules[0].Input["task"])
	}
	if len(def.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(def.Routes))
	}
	if def.Routes[0].Path != "/api/full" {
		t.Errorf("expected route path '/api/full', got %q", def.Routes[0].Path)
	}
	if !def.HasBuild {
		t.Error("expected HasBuild to be true")
	}
	expectedOutput := filepath.Join(funcDir, "dist/main.py")
	if def.OutputPath != expectedOutput {
		t.Errorf("expected output path %q, got %q", expectedOutput, def.OutputPath)
	}
}

func TestSchemaToFunctionDefs_CustomPath(t *testing.T) {
	tmpDir := t.TempDir()
	customDir := filepath.Join(tmpDir, "custom", "location")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatal(err)
	}
	entrypoint := filepath.Join(customDir, "index.ts")
	if err := os.WriteFile(entrypoint, []byte("export default () => {}"), 0644); err != nil {
		t.Fatal(err)
	}

	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"custom": {
				Runtime:    "deno",
				Entrypoint: "index.ts",
				Path:       customDir,
			},
		},
	}

	defs, err := SchemaToFunctionDefs(s, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}

	if defs[0].Path != customDir {
		t.Errorf("expected path %q, got %q", customDir, defs[0].Path)
	}
}

func TestSchemaToFunctionDefs_DirectoryNotExist(t *testing.T) {
	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"missing": {
				Runtime:    "node",
				Entrypoint: "index.js",
			},
		},
	}

	_, err := SchemaToFunctionDefs(s, "/nonexistent")
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
	if !contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %v", err)
	}
}

func TestSchemaToFunctionDefs_EntrypointNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	funcDir := filepath.Join(tmpDir, "noentry")
	if err := os.Mkdir(funcDir, 0755); err != nil {
		t.Fatal(err)
	}

	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"noentry": {
				Runtime:    "node",
				Entrypoint: "missing.js",
			},
		},
	}

	_, err := SchemaToFunctionDefs(s, tmpDir)
	if err == nil {
		t.Fatal("expected error for missing entrypoint")
	}
	if !contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %v", err)
	}
}

func TestParseTimeout(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"", 30, false},
		{"30", 30, false},
		{"60", 60, false},
		{"30s", 30, false},
		{"5m", 300, false},
		{"10m", 600, false},
		{"invalid", 0, true},
		{"30x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseTimeout(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("parseTimeout(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseMemory(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"", 128, false},
		{"128", 128, false},
		{"256", 256, false},
		{"128mb", 128, false},
		{"256MB", 256, false},
		{"1gb", 1024, false},
		{"2GB", 2048, false},
		{"invalid", 0, true},
		{"128x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseMemory(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("parseMemory(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSchemaToFunctionDefs_WebhookVerification(t *testing.T) {
	tmpDir := t.TempDir()
	funcDir := filepath.Join(tmpDir, "webhook")
	if err := os.Mkdir(funcDir, 0755); err != nil {
		t.Fatal(err)
	}
	entrypoint := filepath.Join(funcDir, "index.js")
	if err := os.WriteFile(entrypoint, []byte("export default () => {}"), 0644); err != nil {
		t.Fatal(err)
	}

	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"webhook": {
				Runtime:    "node",
				Entrypoint: "index.js",
				Hooks: []schema.FunctionHook{
					{
						Type: "webhook",
						Verification: &schema.FunctionWebhookVerification{
							Type:   "hmac-sha256",
							Header: "X-Hub-Signature",
							Secret: "secret123",
						},
					},
				},
			},
		},
	}

	defs, err := SchemaToFunctionDefs(s, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}

	if len(defs[0].Hooks) != 1 {
		t.Fatalf("expected 1 hook, got %d", len(defs[0].Hooks))
	}

	hook := defs[0].Hooks[0]
	if hook.Verification == nil {
		t.Fatal("expected verification config")
	}
	if hook.Verification.Type != "hmac-sha256" {
		t.Errorf("expected verification type 'hmac-sha256', got %q", hook.Verification.Type)
	}
	if hook.Verification.Header != "X-Hub-Signature" {
		t.Errorf("expected header 'X-Hub-Signature', got %q", hook.Verification.Header)
	}
	if hook.Verification.Secret != "secret123" {
		t.Errorf("expected secret 'secret123', got %q", hook.Verification.Secret)
	}
}

func TestSchemaToFunctionDefs_MultipleFunctions(t *testing.T) {
	tmpDir := t.TempDir()

	for _, name := range []string{"func1", "func2", "func3"} {
		funcDir := filepath.Join(tmpDir, name)
		if err := os.Mkdir(funcDir, 0755); err != nil {
			t.Fatal(err)
		}
		entrypoint := filepath.Join(funcDir, "index.js")
		if err := os.WriteFile(entrypoint, []byte("export default () => {}"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	s := &schema.Schema{
		Functions: map[string]*schema.Function{
			"func1": {Runtime: "node", Entrypoint: "index.js"},
			"func2": {Runtime: "deno", Entrypoint: "index.js"},
			"func3": {Runtime: "bun", Entrypoint: "index.js"},
		},
	}

	defs, err := SchemaToFunctionDefs(s, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(defs) != 3 {
		t.Fatalf("expected 3 defs, got %d", len(defs))
	}

	names := make(map[string]bool)
	for _, def := range defs {
		names[def.Name] = true
	}

	for _, expected := range []string{"func1", "func2", "func3"} {
		if !names[expected] {
			t.Errorf("expected function %q not found", expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
