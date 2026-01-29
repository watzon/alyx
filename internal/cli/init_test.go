package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateTemplate(t *testing.T) {
	tests := []struct {
		name        string
		templateStr string
		wantErr     bool
	}{
		{
			name:        "basic template",
			templateStr: "basic",
			wantErr:     false,
		},
		{
			name:        "blog template",
			templateStr: "blog",
			wantErr:     false,
		},
		{
			name:        "saas template",
			templateStr: "saas",
			wantErr:     false,
		},
		{
			name:        "unknown template",
			templateStr: "unknown",
			wantErr:     true,
		},
		{
			name:        "empty template",
			templateStr: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := validateTemplate(tt.templateStr)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateTemplate() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("validateTemplate() unexpected error: %v", err)
				return
			}
			if tmpl == nil {
				t.Errorf("validateTemplate() returned nil template")
			}
			if tmpl.Name != tt.templateStr {
				t.Errorf("validateTemplate() template name = %v, want %v", tmpl.Name, tt.templateStr)
			}
		})
	}
}

func TestPrepareProjectDir(t *testing.T) {
	tests := []struct {
		name       string
		setupFiles []string
		projectDir string
		force      bool
		wantErr    bool
	}{
		{
			name:       "new directory",
			projectDir: "newproject",
			force:      false,
			wantErr:    false,
		},
		{
			name:       "current directory empty",
			projectDir: ".",
			force:      false,
			wantErr:    false,
		},
		{
			name:       "existing alyx.yaml without force",
			projectDir: ".",
			setupFiles: []string{"alyx.yaml"},
			force:      false,
			wantErr:    true,
		},
		{
			name:       "existing schema.yaml without force",
			projectDir: ".",
			setupFiles: []string{"schema.yaml"},
			force:      false,
			wantErr:    true,
		},
		{
			name:       "existing files with force",
			projectDir: ".",
			setupFiles: []string{"alyx.yaml", "schema.yaml"},
			force:      true,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			defer os.Chdir(oldWd)

			if err := os.Chdir(tmpDir); err != nil {
				t.Fatal(err)
			}

			for _, file := range tt.setupFiles {
				if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			err = prepareProjectDir(tt.projectDir, tt.force)
			if tt.wantErr {
				if err == nil {
					t.Errorf("prepareProjectDir() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("prepareProjectDir() unexpected error: %v", err)
			}
		})
	}
}

func TestCreateProjectStructure(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "testproject")

	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := createProjectStructure(projectDir); err != nil {
		t.Fatalf("createProjectStructure() failed: %v", err)
	}

	expectedDirs := []string{"data", "functions", "migrations"}
	for _, dir := range expectedDirs {
		dirPath := filepath.Join(projectDir, dir)
		info, err := os.Stat(dirPath)
		if err != nil {
			t.Errorf("directory %s not created: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

func TestWriteTemplateFiles(t *testing.T) {
	tmpDir := t.TempDir()

	tmpl := &Template{
		Name: "test",
		Files: map[string]string{
			"alyx.yaml":   "test: config",
			"schema.yaml": "version: 1",
		},
	}

	if err := writeTemplateFiles(tmpDir, tmpl); err != nil {
		t.Fatalf("writeTemplateFiles() failed: %v", err)
	}

	for filename, expectedContent := range tmpl.Files {
		filePath := filepath.Join(tmpDir, filename)
		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("file %s not created: %v", filename, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("file %s content = %q, want %q", filename, string(content), expectedContent)
		}
	}
}

func TestWriteGitignore(t *testing.T) {
	tmpDir := t.TempDir()

	if err := writeGitignore(tmpDir); err != nil {
		t.Fatalf("writeGitignore() failed: %v", err)
	}

	gitignorePath := filepath.Join(tmpDir, ".gitignore")
	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}

	expectedPatterns := []string{"data/", "*.db", ".env", "generated/"}
	for _, pattern := range expectedPatterns {
		if !contains(string(content), pattern) {
			t.Errorf(".gitignore missing pattern: %s", pattern)
		}
	}
}

func TestCheckExistingFiles(t *testing.T) {
	tmpDir := t.TempDir()

	existing := checkExistingFiles(tmpDir)
	if len(existing) != 0 {
		t.Errorf("checkExistingFiles() on empty dir = %v, want []", existing)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "alyx.yaml"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	existing = checkExistingFiles(tmpDir)
	if len(existing) != 1 || existing[0] != "alyx.yaml" {
		t.Errorf("checkExistingFiles() = %v, want [alyx.yaml]", existing)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "schema.yaml"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	existing = checkExistingFiles(tmpDir)
	if len(existing) != 2 {
		t.Errorf("checkExistingFiles() returned %d files, want 2", len(existing))
	}
}

func TestGetTemplates(t *testing.T) {
	templates := getTemplates()

	expectedTemplates := []string{"basic", "blog", "saas"}
	for _, name := range expectedTemplates {
		tmpl, ok := templates[name]
		if !ok {
			t.Errorf("getTemplates() missing template: %s", name)
			continue
		}
		if tmpl.Name != name {
			t.Errorf("template %s has wrong name: %s", name, tmpl.Name)
		}
		if tmpl.Description == "" {
			t.Errorf("template %s has empty description", name)
		}
		if len(tmpl.Files) == 0 {
			t.Errorf("template %s has no files", name)
		}
		if _, ok := tmpl.Files["alyx.yaml"]; !ok {
			t.Errorf("template %s missing alyx.yaml", name)
		}
		if _, ok := tmpl.Files["schema.yaml"]; !ok {
			t.Errorf("template %s missing schema.yaml", name)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
