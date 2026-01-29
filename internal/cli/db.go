package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

var (
	dbFormat string
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database utilities",
	Long: `Database utilities for Alyx.

Commands for seeding, dumping, and resetting the database.

Examples:
  alyx db seed data.json      Seed database from JSON file
  alyx db dump output.json    Export database to JSON file
  alyx db reset               Reset database (development only!)`,
}

var dbSeedCmd = &cobra.Command{
	Use:   "seed <file>",
	Short: "Seed database from file",
	Long: `Seed the database with data from a JSON or YAML file.

The file should contain a map of collection names to arrays of documents.

Example JSON:
  {
    "users": [
      {"id": "user_1", "name": "Alice", "email": "alice@example.com"}
    ],
    "posts": [
      {"id": "post_1", "title": "Hello", "author_id": "user_1"}
    ]
  }`,
	Args: cobra.ExactArgs(1),
	RunE: runDBSeed,
}

var dbDumpCmd = &cobra.Command{
	Use:   "dump <file>",
	Short: "Dump database to file",
	Long: `Export all collection data to a JSON or YAML file.

Use the --format flag to specify output format (default: json).`,
	Args: cobra.ExactArgs(1),
	RunE: runDBDump,
}

var dbResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset database (development only!)",
	Long: `Drop all tables and recreate the schema.

⚠️  WARNING: This will DELETE ALL DATA. Only use in development!

This command will:
  1. Drop all user-defined tables
  2. Drop all Alyx internal tables
  3. Recreate the schema from schema.yaml`,
	RunE: runDBReset,
}

func init() {
	dbDumpCmd.Flags().StringVarP(&dbFormat, "format", "f", "json", "Output format (json, yaml)")

	dbCmd.AddCommand(dbSeedCmd)
	dbCmd.AddCommand(dbDumpCmd)
	dbCmd.AddCommand(dbResetCmd)

	rootCmd.AddCommand(dbCmd)
}

func runDBSeed(cmd *cobra.Command, args []string) error {
	seedFile := args[0]

	data, err := os.ReadFile(seedFile)
	if err != nil {
		return fmt.Errorf("reading seed file: %w", err)
	}

	seedData, err := parseSeedData(seedFile, data)
	if err != nil {
		return err
	}

	// Load config and schema
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		log.Warn().Err(err).Msg("No config file found, using defaults")
		cfg = config.Default()
	}

	schemaPath := resolveSchemaPath("")
	if schemaPath == "" {
		return fmt.Errorf("no schema file found")
	}

	s, err := schema.ParseFile(schemaPath)
	if err != nil {
		return fmt.Errorf("parsing schema: %w", err)
	}

	// Open database
	db, err := database.Open(&cfg.Database)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	// Seed each collection
	totalInserted := 0
	for collectionName, documents := range seedData {
		// Check if collection exists in schema
		col, ok := s.Collections[collectionName]
		if !ok {
			log.Warn().Str("collection", collectionName).Msg("Collection not found in schema, skipping")
			continue
		}

		inserted, seedErr := seedCollection(db, col, documents)
		if seedErr != nil {
			return fmt.Errorf("seeding %s: %w", collectionName, seedErr)
		}

		totalInserted += inserted
		log.Info().
			Str("collection", collectionName).
			Int("count", inserted).
			Msg("Seeded collection")
	}

	fmt.Printf("✓ Seeded %d documents across %d collections\n", totalInserted, len(seedData))
	return nil
}

func seedCollection(db *database.DB, col *schema.Collection, documents []map[string]any) (int, error) {
	if len(documents) == 0 {
		return 0, nil
	}

	inserted := 0
	for _, doc := range documents {
		// Build insert query
		var columns []string
		var placeholders []string
		var values []any

		for _, field := range col.OrderedFields() {
			if val, ok := doc[field.Name]; ok {
				columns = append(columns, field.Name)
				placeholders = append(placeholders, "?")

				// Handle JSON fields
				if field.Type == schema.FieldTypeJSON {
					jsonBytes, err := json.Marshal(val)
					if err != nil {
						return inserted, fmt.Errorf("marshaling JSON for %s: %w", field.Name, err)
					}
					values = append(values, string(jsonBytes))
				} else {
					values = append(values, val)
				}
			}
		}

		if len(columns) == 0 {
			continue
		}

		query := fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)",
			col.Name,
			joinStrings(columns, ", "),
			joinStrings(placeholders, ", "),
		)

		if _, err := db.Exec(query, values...); err != nil {
			return inserted, fmt.Errorf("inserting document: %w", err)
		}
		inserted++
	}

	return inserted, nil
}

func runDBDump(cmd *cobra.Command, args []string) error {
	outputFile := args[0]

	// Load config and schema
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		log.Warn().Err(err).Msg("No config file found, using defaults")
		cfg = config.Default()
	}

	schemaPath := resolveSchemaPath("")
	if schemaPath == "" {
		return fmt.Errorf("no schema file found")
	}

	s, err := schema.ParseFile(schemaPath)
	if err != nil {
		return fmt.Errorf("parsing schema: %w", err)
	}

	// Open database
	db, err := database.Open(&cfg.Database)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	// Dump each collection
	dump := make(map[string][]database.Row)
	totalDocuments := 0

	for collectionName := range s.Collections {
		rows, queryErr := db.Query(fmt.Sprintf("SELECT * FROM %s", collectionName))
		if queryErr != nil {
			log.Warn().Err(queryErr).Str("collection", collectionName).Msg("Error querying collection")
			continue
		}
		defer rows.Close()

		documents, scanErr := database.ScanRows(rows)
		if scanErr != nil {
			return fmt.Errorf("scanning %s: %w", collectionName, scanErr)
		}

		if len(documents) > 0 {
			dump[collectionName] = documents
			totalDocuments += len(documents)
		}
	}

	// Write output
	var output []byte
	if dbFormat == "yaml" {
		output, err = yaml.Marshal(dump)
	} else {
		output, err = json.MarshalIndent(dump, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("marshaling output: %w", err)
	}

	if err := os.WriteFile(outputFile, output, 0o600); err != nil {
		return fmt.Errorf("writing output file: %w", err)
	}

	fmt.Printf("✓ Dumped %d documents from %d collections to %s\n",
		totalDocuments, len(dump), outputFile)
	return nil
}

func runDBReset(cmd *cobra.Command, args []string) error {
	if !confirmReset() {
		fmt.Println("Aborted.")
		return nil
	}

	cfg, s, err := loadConfigAndSchema()
	if err != nil {
		return err
	}

	db, err := database.Open(&cfg.Database)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	tables, err := listAllTables(db)
	if err != nil {
		return err
	}

	if err := dropAllTables(db, tables); err != nil {
		return err
	}

	if err := recreateSchema(db, s); err != nil {
		return err
	}

	fmt.Printf("✓ Database reset complete. Dropped %d tables and recreated schema.\n", len(tables))
	return nil
}

func confirmReset() bool {
	fmt.Println("⚠️  WARNING: This will DELETE ALL DATA in the database!")
	fmt.Println("Type 'yes' to confirm: ")

	var confirm string
	if _, err := fmt.Scanln(&confirm); err != nil {
		return false
	}
	return confirm == "yes"
}

func loadConfigAndSchema() (*config.Config, *schema.Schema, error) {
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		log.Warn().Err(err).Msg("No config file found, using defaults")
		cfg = config.Default()
	}

	schemaPath := resolveSchemaPath("")
	if schemaPath == "" {
		return nil, nil, fmt.Errorf("no schema file found")
	}

	s, err := schema.ParseFile(schemaPath)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing schema: %w", err)
	}

	return cfg, s, nil
}

func listAllTables(db *database.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT name FROM sqlite_master 
		WHERE type='table' AND name NOT LIKE 'sqlite_%'
	`)
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("scanning table name: %w", err)
		}
		tables = append(tables, tableName)
	}
	return tables, rows.Err()
}

func dropAllTables(db *database.DB, tables []string) error {
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("disabling foreign keys: %w", err)
	}

	for _, table := range tables {
		log.Info().Str("table", table).Msg("Dropping table")
		if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
			return fmt.Errorf("dropping table %s: %w", table, err)
		}
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("enabling foreign keys: %w", err)
	}
	return nil
}

func recreateSchema(db *database.DB, s *schema.Schema) error {
	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		if _, err := db.Exec(stmt); err != nil {
			log.Debug().Str("sql", stmt).Msg("Executing SQL")
			return fmt.Errorf("executing schema SQL: %w", err)
		}
	}
	return nil
}

func parseSeedData(filename string, data []byte) (map[string][]map[string]any, error) {
	var seedData map[string][]map[string]any
	if isYAML(filename) {
		if err := yaml.Unmarshal(data, &seedData); err != nil {
			return nil, fmt.Errorf("parsing YAML: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &seedData); err != nil {
			return nil, fmt.Errorf("parsing JSON: %w", err)
		}
	}
	return seedData, nil
}

func isYAML(filename string) bool {
	return hasExtension(filename, ".yaml") || hasExtension(filename, ".yml")
}

func hasExtension(filename, ext string) bool {
	return len(filename) > len(ext) &&
		filename[len(filename)-len(ext):] == ext
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
