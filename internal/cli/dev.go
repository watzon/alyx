package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
	"github.com/watzon/alyx/internal/server"
)

var (
	devPort       int
	devHost       string
	devSchemaPath string
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start the development server",
	Long: `Start the Alyx development server with hot reload.

The development server will:
  - Load schema from schema.yaml
  - Create/update database tables
  - Start the HTTP server
  - Watch for file changes (schema, functions)`,
	RunE: runDev,
}

func init() {
	devCmd.Flags().IntVarP(&devPort, "port", "p", 8090, "Port to listen on")
	devCmd.Flags().StringVar(&devHost, "host", "localhost", "Host to bind to")
	devCmd.Flags().StringVar(&devSchemaPath, "schema", "", "Path to schema file (default: schema.yaml or schema.yml)")

	rootCmd.AddCommand(devCmd)
}

func runDev(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		log.Warn().Err(err).Msg("No config file found, using defaults")
		cfg = config.Default()
	}

	if cmd.Flags().Changed("port") {
		cfg.Server.Port = devPort
	}
	if cmd.Flags().Changed("host") {
		cfg.Server.Host = devHost
	}
	cfg.Dev.Enabled = true

	schemaPath := resolveSchemaPath(devSchemaPath)
	if schemaPath == "" {
		log.Fatal().Msg("No schema file found. Create schema.yaml or schema.yml, or specify --schema path")
	}

	log.Info().
		Str("schema", schemaPath).
		Str("addr", cfg.Server.Address()).
		Msg("Starting development server")

	s, err := schema.ParseFile(schemaPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse schema")
	}

	log.Info().
		Int("collections", len(s.Collections)).
		Msg("Schema loaded")

	db, err := database.Open(&cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open database")
	}
	defer db.Close()

	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		if _, err := db.Exec(stmt); err != nil {
			log.Debug().Str("sql", stmt).Msg("Executing SQL")
			log.Fatal().Err(err).Msg("Failed to execute schema SQL")
		}
	}
	log.Info().Msg("Database schema applied")

	srv := server.New(cfg, db, s)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info().Msg("Shutdown signal received")
		cancel()
		_ = srv.Shutdown(context.Background())
	}()

	log.Info().
		Str("url", "http://"+cfg.Server.Address()).
		Msg("Server started")

	for name := range s.Collections {
		log.Info().
			Str("collection", name).
			Str("endpoint", "http://"+cfg.Server.Address()+"/api/collections/"+name).
			Msg("Collection endpoint")
	}

	if cfg.Docs.Enabled {
		log.Info().
			Str("docs", "http://"+cfg.Server.Address()+"/api/docs").
			Str("openapi", "http://"+cfg.Server.Address()+"/api/openapi.json").
			Str("ui", cfg.Docs.UI).
			Msg("API documentation")
	}

	if cfg.Realtime.Enabled {
		log.Info().
			Str("ws", "ws://"+cfg.Server.Address()+"/api/realtime").
			Msg("Realtime WebSocket endpoint")
	}

	if err := srv.Start(ctx); err != nil {
		log.Error().Err(err).Msg("Server error")
		return err
	}

	<-ctx.Done()
	return nil
}

func resolveSchemaPath(explicit string) string {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit
		}
		return ""
	}

	candidates := []string{"schema.yaml", "schema.yml"}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
