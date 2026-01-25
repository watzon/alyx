package generate

import (
	"fmt"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/watzon/alyx/internal/openapi"
	"github.com/watzon/alyx/internal/schema"
	"github.com/watzon/alyx/internal/sdk/typescript"
)

// SDKCmd represents the sdk command.
var SDKCmd = newSDKCmd()

func newSDKCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sdk",
		Short: "Generate TypeScript SDK from schema",
		Long: `Generate a type-safe TypeScript SDK for your Alyx API.

The SDK includes:
  - Type definitions for collections, auth, functions, and events
  - Client methods for all API operations
  - Hook helpers with event types and payload types
  - Runtime context helpers for function development

Example:
  alyx generate sdk --lang typescript --output ./sdk`,
		RunE: runSDK,
	}

	cmd.Flags().StringVarP(&sdkLang, "lang", "l", "typescript", "SDK language (currently only typescript supported)")
	cmd.Flags().StringVarP(&sdkOutput, "output", "o", "./sdk", "Output directory for generated SDK")
	cmd.Flags().StringVarP(&sdkURL, "url", "u", "", "Server URL for client (default: http://localhost:8090)")

	return cmd
}

var (
	sdkLang   string
	sdkOutput string
	sdkURL    string
)

func runSDK(cmd *cobra.Command, args []string) error {
	// Validate language
	if sdkLang != "typescript" && sdkLang != "ts" {
		return fmt.Errorf("unsupported language: %s (only typescript is supported)", sdkLang)
	}

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

	// Determine server URL
	serverURL := sdkURL
	if serverURL == "" {
		host := viper.GetString("server.host")
		if host == "" {
			host = "localhost"
		}
		port := viper.GetInt("server.port")
		if port == 0 {
			port = 8090
		}
		serverURL = fmt.Sprintf("http://%s:%d", host, port)
	}

	// Generate OpenAPI spec
	spec := openapi.Generate(s, openapi.GeneratorConfig{
		Title:       "Alyx API",
		Description: "Generated API for Alyx Backend-as-a-Service",
		Version:     "1.0.0",
		ServerURL:   serverURL,
	})

	// Resolve output directory
	outputDir, err := filepath.Abs(sdkOutput)
	if err != nil {
		return fmt.Errorf("resolving output path: %w", err)
	}

	log.Info().
		Str("language", "typescript").
		Str("output", outputDir).
		Str("url", serverURL).
		Msg("Generating SDK")

	// Generate TypeScript SDK
	generator := typescript.NewGenerator(typescript.Config{
		OutputDir: outputDir,
		ServerURL: serverURL,
	})

	if err := generator.Generate(spec, s); err != nil {
		return fmt.Errorf("generating TypeScript SDK: %w", err)
	}

	log.Info().
		Str("path", outputDir).
		Msg("SDK generated successfully")

	log.Info().Msg("To use the SDK:")
	log.Info().Msgf("  cd %s", outputDir)
	log.Info().Msg("  npm install")
	log.Info().Msg("  npx tsc")

	return nil
}
