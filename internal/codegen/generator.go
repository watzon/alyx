// Package codegen provides client SDK generation for multiple languages.
package codegen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/watzon/alyx/internal/schema"
)

// Language represents a supported code generation language.
type Language string

const (
	// LanguageTypeScript generates TypeScript client code.
	LanguageTypeScript Language = "typescript"
	// LanguageGo generates Go client code.
	LanguageGo Language = "go"
	// LanguagePython generates Python client code.
	LanguagePython Language = "python"
)

// ParseLanguage parses a language string into a Language type.
func ParseLanguage(s string) (Language, error) {
	switch strings.ToLower(s) {
	case "typescript", "ts":
		return LanguageTypeScript, nil
	case "go", "golang":
		return LanguageGo, nil
	case "python", "py":
		return LanguagePython, nil
	default:
		return "", fmt.Errorf("unsupported language: %s", s)
	}
}

// Generator generates client SDK code for a specific language.
type Generator interface {
	// Language returns the target language.
	Language() Language

	// Generate produces client code from a schema.
	Generate(s *schema.Schema) ([]GeneratedFile, error)
}

// GeneratedFile represents a generated source file.
type GeneratedFile struct {
	// Path is the relative path for this file.
	Path string
	// Content is the file content.
	Content string
}

// Config holds code generation configuration.
type Config struct {
	// OutputDir is the base output directory.
	OutputDir string
	// Languages to generate.
	Languages []Language
	// ServerURL is the Alyx server URL for client initialization.
	ServerURL string
	// PackageName is used for Go package naming.
	PackageName string
}

// DefaultConfig returns a default configuration.
func DefaultConfig() *Config {
	return &Config{
		OutputDir:   "./generated",
		Languages:   []Language{LanguageTypeScript},
		ServerURL:   "http://localhost:8080",
		PackageName: "alyx",
	}
}

// GenerateAll generates client code for all configured languages.
func GenerateAll(cfg *Config, s *schema.Schema) error {
	for _, lang := range cfg.Languages {
		gen, err := NewGenerator(lang, cfg)
		if err != nil {
			return fmt.Errorf("creating generator for %s: %w", lang, err)
		}

		files, err := gen.Generate(s)
		if err != nil {
			return fmt.Errorf("generating %s code: %w", lang, err)
		}

		// Write files to output directory
		langDir := filepath.Join(cfg.OutputDir, string(lang))
		if err := os.MkdirAll(langDir, 0o755); err != nil {
			return fmt.Errorf("creating output directory %s: %w", langDir, err)
		}

		for _, f := range files {
			path := filepath.Join(langDir, f.Path)

			// Ensure parent directory exists
			if dir := filepath.Dir(path); dir != langDir {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("creating directory %s: %w", dir, err)
				}
			}

			if err := os.WriteFile(path, []byte(f.Content), 0o600); err != nil {
				return fmt.Errorf("writing file %s: %w", path, err)
			}
		}
	}

	return nil
}

// NewGenerator creates a generator for the specified language.
func NewGenerator(lang Language, cfg *Config) (Generator, error) {
	switch lang {
	case LanguageTypeScript:
		return NewTypeScriptGenerator(cfg), nil
	case LanguageGo:
		return NewGoGenerator(cfg), nil
	case LanguagePython:
		return NewPythonGenerator(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
}

// Helper functions for code generation.

// toPascalCase converts a string to PascalCase.
func toPascalCase(s string) string {
	words := splitWords(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
		}
	}
	return strings.Join(words, "")
}

// toCamelCase converts a string to camelCase.
func toCamelCase(s string) string {
	pascal := toPascalCase(s)
	if len(pascal) == 0 {
		return ""
	}
	return strings.ToLower(pascal[:1]) + pascal[1:]
}

// toSnakeCase converts a string to snake_case.
func toSnakeCase(s string) string {
	words := splitWords(s)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "_")
}

// splitWords splits a string into words by underscores, hyphens, or camelCase boundaries.
func splitWords(s string) []string {
	var words []string
	var current strings.Builder

	for i, r := range s {
		if r == '_' || r == '-' || r == ' ' {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			continue
		}

		// Check for camelCase boundary
		if i > 0 && isUpper(r) && (current.Len() > 0) {
			words = append(words, current.String())
			current.Reset()
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}

func isUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

// sortedCollectionNames returns collection names in sorted order for deterministic output.
func sortedCollectionNames(s *schema.Schema) []string {
	names := make([]string, 0, len(s.Collections))
	for name := range s.Collections {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
