package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/watzon/alyx/internal/codegen"
	"github.com/watzon/alyx/internal/schema"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate client SDKs from schema",
	Long: `Generate type-safe client libraries for your Alyx schema.

Supported languages:
  - typescript (or ts): TypeScript/JavaScript client
  - go (or golang): Go client  
  - python (or py): Python client

Examples:
  # Generate TypeScript client
  alyx generate --lang typescript

  # Generate multiple clients
  alyx generate --lang typescript,go,python

  # Generate to custom directory
  alyx generate --lang typescript --output ./src/lib/alyx`,
	RunE: runGenerate,
}

var (
	generateLangs  string
	generateOutput string
	generateURL    string
	generatePkg    string
)

func init() {
	generateCmd.Flags().StringVarP(&generateLangs, "lang", "l", "typescript", "Languages to generate (comma-separated)")
	generateCmd.Flags().StringVarP(&generateOutput, "output", "o", "", "Output directory (default: ./generated)")
	generateCmd.Flags().StringVarP(&generateURL, "url", "u", "", "Server URL for client (default: http://localhost:8080)")
	generateCmd.Flags().StringVar(&generatePkg, "package", "", "Package name for Go client (default: alyx)")

	AddCommand(generateCmd)
}

func runGenerate(cmd *cobra.Command, args []string) error {
	// Find and parse schema
	schemaPath := viper.GetString("schema")
	if schemaPath == "" {
		schemaPath = "schema.yaml"
	}

	absSchemaPath, err := filepath.Abs(schemaPath)
	if err != nil {
		return fmt.Errorf("resolving schema path: %w", err)
	}

	s, err := schema.ParseFile(absSchemaPath)
	if err != nil {
		return fmt.Errorf("parsing schema: %w", err)
	}

	// Parse languages
	languages, err := parseLanguages(generateLangs)
	if err != nil {
		return err
	}

	// Build config
	cfg := codegen.DefaultConfig()

	if generateOutput != "" {
		cfg.OutputDir = generateOutput
	} else if viper.IsSet("dev.generate_output") {
		cfg.OutputDir = viper.GetString("dev.generate_output")
	}

	if generateURL != "" {
		cfg.ServerURL = generateURL
	} else {
		host := viper.GetString("server.host")
		if host == "" {
			host = "localhost"
		}
		port := viper.GetInt("server.port")
		if port == 0 {
			port = 8080
		}
		cfg.ServerURL = fmt.Sprintf("http://%s:%d", host, port)
	}

	if generatePkg != "" {
		cfg.PackageName = generatePkg
	}

	cfg.Languages = languages

	log.Info().
		Strs("languages", languageStrings(languages)).
		Str("output", cfg.OutputDir).
		Str("url", cfg.ServerURL).
		Msg("Generating client SDKs")

	if err := codegen.GenerateAll(cfg, s); err != nil {
		return fmt.Errorf("generating code: %w", err)
	}

	for _, lang := range languages {
		langDir := filepath.Join(cfg.OutputDir, string(lang))
		log.Info().
			Str("language", string(lang)).
			Str("path", langDir).
			Msg("Generated client")
	}

	log.Info().Msg("Code generation complete")
	return nil
}

func parseLanguages(s string) ([]codegen.Language, error) {
	parts := strings.Split(s, ",")
	languages := make([]codegen.Language, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		lang, err := codegen.ParseLanguage(p)
		if err != nil {
			return nil, err
		}
		languages = append(languages, lang)
	}

	if len(languages) == 0 {
		return nil, fmt.Errorf("no languages specified")
	}

	return languages, nil
}

func languageStrings(langs []codegen.Language) []string {
	strs := make([]string, len(langs))
	for i, l := range langs {
		strs[i] = string(l)
	}
	return strs
}
