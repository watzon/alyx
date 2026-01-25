package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/watzon/alyx/internal/codegen"
	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
	"github.com/watzon/alyx/internal/server"
)

var (
	devPort       int
	devHost       string
	devSchemaPath string
	devNoWatch    bool
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Start the development server",
	Long: `Start the Alyx development server with hot reload.

The development server will:
  - Load schema from schema.yaml
  - Create/update database tables
  - Start the HTTP server
  - Watch for file changes (schema, functions)
  
Use --no-watch to disable file watching.`,
	RunE: runDev,
}

func init() {
	devCmd.Flags().IntVarP(&devPort, "port", "p", 8090, "Port to listen on")
	devCmd.Flags().StringVar(&devHost, "host", "localhost", "Host to bind to")
	devCmd.Flags().StringVar(&devSchemaPath, "schema", "", "Path to schema file (default: schema.yaml or schema.yml)")
	devCmd.Flags().BoolVar(&devNoWatch, "no-watch", false, "Disable file watching")

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

	if err := applySchema(db, s); err != nil {
		return err
	}

	configPath, _ := config.ConfigFilePath("")
	srv := server.New(cfg, db, s,
		server.WithSchemaPath(schemaPath),
		server.WithConfigPath(configPath),
	)

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

	if !devNoWatch && cfg.Dev.Watch {
		watcher, watchErr := setupDevWatcher(ctx, schemaPath, cfg.Functions.Path, db, srv, cfg)
		if watchErr != nil {
			log.Warn().Err(watchErr).Msg("Failed to set up file watcher, continuing without hot-reload")
		} else {
			defer func() { _ = watcher.Stop() }()
			log.Info().Msg("File watching enabled")
		}
	}

	logServerInfo(cfg, s)

	if err := srv.Start(ctx); err != nil {
		log.Error().Err(err).Msg("Server error")
		return err
	}

	<-ctx.Done()
	return nil
}

func applySchema(db *database.DB, s *schema.Schema) error {
	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		if _, err := db.Exec(stmt); err != nil {
			log.Debug().Str("sql", stmt).Msg("Executing SQL")
			log.Error().Err(err).Msg("Failed to execute schema SQL")
			return err
		}
	}

	log.Info().Msg("Database schema applied")
	return nil
}

func logServerInfo(cfg *config.Config, s *schema.Schema) {
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

	if cfg.Functions.Enabled {
		log.Info().
			Str("functions", cfg.Functions.Path).
			Msg("Functions directory")
	}

	if cfg.AdminUI.Enabled {
		log.Info().
			Str("admin", "http://"+cfg.Server.Address()+cfg.AdminUI.Path).
			Msg("Admin UI")
	}
}

func setupDevWatcher(ctx context.Context, schemaPath, functionsPath string, db *database.DB, srv *server.Server, cfg *config.Config) (*DevWatcher, error) {
	absSchemaPath, _ := filepath.Abs(schemaPath)
	absFunctionsPath := ""
	if functionsPath != "" {
		absFunctionsPath, _ = filepath.Abs(functionsPath)
	}

	watcher, err := NewDevWatcher(DevWatcherConfig{
		SchemaPath:    absSchemaPath,
		FunctionsPath: absFunctionsPath,
		OnSchemaChange: func(path string) {
			handleSchemaChange(path, db, srv, cfg)
		},
		OnFunctionChange: func(path string, eventType EventType) {
			handleFunctionChange(path, eventType, srv)
		},
	})
	if err != nil {
		return nil, err
	}

	watcher.Start(ctx)
	return watcher, nil
}

func handleSchemaChange(path string, db *database.DB, srv *server.Server, cfg *config.Config) {
	log.Info().Str("path", path).Msg("Schema file changed")

	newSchema, err := schema.ParseFile(path)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse updated schema")
		return
	}

	currentSchema := srv.Schema()
	differ := schema.NewDiffer()
	changes := differ.Diff(currentSchema, newSchema)

	if len(changes) == 0 {
		log.Debug().Msg("No schema changes detected")
		return
	}

	safeChanges := differ.SafeChanges(changes)
	unsafeChanges := differ.UnsafeChanges(changes)

	for _, c := range safeChanges {
		log.Info().Str("change", c.String()).Msg("Applying schema change")
	}

	if len(safeChanges) > 0 {
		migrator := schema.NewMigrator(db.DB, path, "migrations")
		if err := migrator.ApplySafeChanges(safeChanges); err != nil {
			log.Error().Err(err).Msg("Failed to apply schema changes")
			return
		}
	}

	if err := srv.UpdateSchema(newSchema); err != nil {
		log.Error().Err(err).Msg("Failed to update server schema")
		return
	}

	log.Info().
		Int("applied", len(safeChanges)).
		Int("pending", len(unsafeChanges)).
		Msg("Schema changes applied")

	if len(unsafeChanges) > 0 {
		log.Warn().Msg("Some changes require manual migration:")
		for _, c := range unsafeChanges {
			log.Warn().Str("change", c.String()).Msg("  Requires migration")
		}
	}

	// Auto-regenerate client SDKs if configured
	if cfg.Dev.AutoGenerate && len(cfg.Dev.GenerateLanguages) > 0 {
		regenerateClients(newSchema, cfg)
	}
}

func regenerateClients(s *schema.Schema, cfg *config.Config) {
	languages := make([]codegen.Language, 0, len(cfg.Dev.GenerateLanguages))
	for _, langStr := range cfg.Dev.GenerateLanguages {
		lang, err := codegen.ParseLanguage(langStr)
		if err != nil {
			log.Warn().Str("language", langStr).Msg("Unknown language, skipping")
			continue
		}
		languages = append(languages, lang)
	}

	if len(languages) == 0 {
		return
	}

	genCfg := &codegen.Config{
		OutputDir:   cfg.Dev.GenerateOutput,
		Languages:   languages,
		ServerURL:   fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port),
		PackageName: "alyx",
	}

	if genCfg.OutputDir == "" {
		genCfg.OutputDir = "./generated"
	}

	if err := codegen.GenerateAll(genCfg, s); err != nil {
		log.Error().Err(err).Msg("Failed to regenerate client SDKs")
		return
	}

	log.Info().
		Strs("languages", cfg.Dev.GenerateLanguages).
		Str("output", genCfg.OutputDir).
		Msg("Client SDKs regenerated")
}

func handleFunctionChange(path string, eventType EventType, srv *server.Server) {
	log.Info().
		Str("path", path).
		Str("event", eventType.String()).
		Msg("Function file changed")

	if err := srv.ReloadFunctions(); err != nil {
		log.Error().Err(err).Msg("Failed to reload functions")
		return
	}

	log.Info().Msg("Functions reloaded successfully")
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
