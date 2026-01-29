package cli

import (
	"testing"

	"github.com/watzon/alyx/internal/codegen"
)

func TestParseLanguages(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []codegen.Language
		wantErr  bool
	}{
		{
			name:     "single language",
			input:    "typescript",
			expected: []codegen.Language{codegen.LanguageTypeScript},
			wantErr:  false,
		},
		{
			name:     "multiple languages",
			input:    "typescript,go,python",
			expected: []codegen.Language{codegen.LanguageTypeScript, codegen.LanguageGo, codegen.LanguagePython},
			wantErr:  false,
		},
		{
			name:     "with spaces",
			input:    "typescript, go, python",
			expected: []codegen.Language{codegen.LanguageTypeScript, codegen.LanguageGo, codegen.LanguagePython},
			wantErr:  false,
		},
		{
			name:     "language alias",
			input:    "ts",
			expected: []codegen.Language{codegen.LanguageTypeScript},
			wantErr:  false,
		},
		{
			name:     "multiple aliases",
			input:    "ts,golang,py",
			expected: []codegen.Language{codegen.LanguageTypeScript, codegen.LanguageGo, codegen.LanguagePython},
			wantErr:  false,
		},
		{
			name:    "unknown language",
			input:   "ruby",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "only spaces",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "mixed valid and invalid",
			input:   "typescript,invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseLanguages(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseLanguages(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("parseLanguages(%q) unexpected error: %v", tt.input, err)
				return
			}
			if len(result) != len(tt.expected) {
				t.Errorf("parseLanguages(%q) returned %d languages, want %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i, lang := range result {
				if lang != tt.expected[i] {
					t.Errorf("parseLanguages(%q)[%d] = %v, want %v", tt.input, i, lang, tt.expected[i])
				}
			}
		})
	}
}

func TestLanguageStrings(t *testing.T) {
	tests := []struct {
		name     string
		langs    []codegen.Language
		expected []string
	}{
		{
			name:     "empty",
			langs:    []codegen.Language{},
			expected: []string{},
		},
		{
			name:     "single",
			langs:    []codegen.Language{codegen.LanguageTypeScript},
			expected: []string{"typescript"},
		},
		{
			name:     "multiple",
			langs:    []codegen.Language{codegen.LanguageTypeScript, codegen.LanguageGo, codegen.LanguagePython},
			expected: []string{"typescript", "go", "python"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := languageStrings(tt.langs)
			if len(result) != len(tt.expected) {
				t.Errorf("languageStrings() returned %d strings, want %d", len(result), len(tt.expected))
				return
			}
			for i, s := range result {
				if s != tt.expected[i] {
					t.Errorf("languageStrings()[%d] = %q, want %q", i, s, tt.expected[i])
				}
			}
		})
	}
}
