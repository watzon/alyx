package cli

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

var (
	migrateSchemaPath     string
	migrateMigrationsPath string
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migration commands",
	Long: `Database migration commands for Alyx.

Migrations allow you to evolve your database schema over time.
Alyx supports both automatic (safe) migrations from schema changes
and manual migration files for complex operations.

Examples:
  alyx migrate status        Show pending migrations
  alyx migrate apply         Apply pending migrations
  alyx migrate create name   Create new migration file`,
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	Long:  `Show the current migration status, including applied and pending migrations.`,
	RunE:  runMigrateStatus,
}

var migrateApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply pending migrations",
	Long: `Apply all pending migrations to the database.

This will:
  1. Apply any pending migration files from the migrations/ directory
  2. Apply safe schema changes from schema.yaml (additive only)

For destructive changes (removing fields, changing types), create a
migration file using 'alyx migrate create'.`,
	RunE: runMigrateApply,
}

var migrateRollbackCmd = &cobra.Command{
	Use:   "rollback [n]",
	Short: "Rollback migrations",
	Long: `Rollback the last n migrations (default: 1).

Note: Rollback is only supported for migrations that have a 'down' section.
Automatically generated schema migrations cannot be rolled back.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMigrateRollback,
}

var migrateCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new migration file",
	Long: `Create a new migration file with the given name.

The file will be created in the migrations/ directory with a version
number prefix. Edit the file to add your migration SQL.

Example:
  alyx migrate create add_user_roles
  # Creates: migrations/003_add_user_roles.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runMigrateCreate,
}

func init() {
	migrateCmd.PersistentFlags().StringVar(&migrateSchemaPath, "schema", "", "Path to schema file (default: schema.yaml)")
	migrateCmd.PersistentFlags().StringVar(&migrateMigrationsPath, "migrations", "migrations", "Path to migrations directory")

	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateApplyCmd)
	migrateCmd.AddCommand(migrateRollbackCmd)
	migrateCmd.AddCommand(migrateCreateCmd)

	rootCmd.AddCommand(migrateCmd)
}

func getMigrator() (*schema.Migrator, *database.DB, error) {
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		log.Warn().Err(err).Msg("No config file found, using defaults")
		cfg = config.Default()
	}

	db, err := database.Open(&cfg.Database)
	if err != nil {
		return nil, nil, fmt.Errorf("opening database: %w", err)
	}

	schemaPath := resolveSchemaPath(migrateSchemaPath)
	migrator := schema.NewMigrator(db.DB, schemaPath, migrateMigrationsPath)

	if err := migrator.Init(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("initializing migrator: %w", err)
	}

	return migrator, db, nil
}

func runMigrateStatus(cmd *cobra.Command, args []string) error {
	migrator, db, err := getMigrator()
	if err != nil {
		return err
	}
	defer db.Close()

	// Show applied migrations
	applied, err := migrator.AppliedMigrations()
	if err != nil {
		return fmt.Errorf("getting applied migrations: %w", err)
	}

	if len(applied) == 0 {
		fmt.Println("No migrations have been applied yet.")
	} else {
		fmt.Println("Applied migrations:")
		for _, m := range applied {
			fmt.Printf("  ✓ %s - %s (applied %s)\n",
				m.Version, m.Name, m.AppliedAt.Format("2006-01-02 15:04:05"))
		}
	}

	// Show pending migrations
	pending, err := migrator.PendingMigrations()
	if err != nil {
		return fmt.Errorf("getting pending migrations: %w", err)
	}

	fmt.Println()
	if len(pending) == 0 {
		fmt.Println("No pending migrations.")
	} else {
		fmt.Println("Pending migrations:")
		for _, m := range pending {
			fmt.Printf("  ○ %d - %s\n", m.Version, m.Name)
		}
	}

	// Check for schema changes
	schemaPath := resolveSchemaPath(migrateSchemaPath)
	if schemaPath != "" {
		schemaChanges, checkErr := checkSchemaChanges(db, schemaPath)
		if checkErr != nil {
			log.Warn().Err(checkErr).Msg("Could not check schema changes")
		} else if len(schemaChanges) > 0 {
			fmt.Println()
			fmt.Println("Schema changes detected:")
			for _, c := range schemaChanges {
				status := "⚠"
				if c.Safe {
					status = "✓"
				}
				fmt.Printf("  %s %s\n", status, c)
			}
			if hasUnsafeChanges(schemaChanges) {
				fmt.Println()
				fmt.Println("⚠ Some changes require a manual migration file.")
				fmt.Println("  Use 'alyx migrate create <name>' to create one.")
			}
		}
	}

	return nil
}

func runMigrateApply(cmd *cobra.Command, args []string) error {
	migrator, db, err := getMigrator()
	if err != nil {
		return err
	}
	defer db.Close()

	// Apply pending file migrations
	pending, err := migrator.PendingMigrations()
	if err != nil {
		return fmt.Errorf("getting pending migrations: %w", err)
	}

	for _, m := range pending {
		fmt.Printf("Applying migration %d - %s...\n", m.Version, m.Name)
		if applyErr := migrator.Apply(m); applyErr != nil {
			return fmt.Errorf("applying migration %d: %w", m.Version, applyErr)
		}
		fmt.Printf("  ✓ Applied\n")
	}

	// Apply schema changes
	schemaPath := resolveSchemaPath(migrateSchemaPath)
	if schemaPath == "" {
		if len(pending) == 0 {
			fmt.Println("No migrations to apply.")
		}
		return nil
	}

	schemaChanges, checkErr := checkSchemaChanges(db, schemaPath)
	if checkErr != nil {
		return fmt.Errorf("checking schema changes: %w", checkErr)
	}

	safeChanges := filterSafeChanges(schemaChanges)
	if len(safeChanges) == 0 {
		if len(pending) == 0 {
			fmt.Println("No migrations to apply.")
		}
		return nil
	}

	fmt.Println("Applying schema changes...")
	for _, c := range safeChanges {
		fmt.Printf("  ✓ %s\n", c)
	}

	if err := migrator.ApplySafeChanges(safeChanges); err != nil {
		return fmt.Errorf("applying schema changes: %w", err)
	}

	unsafeChanges := filterUnsafeChanges(schemaChanges)
	if len(unsafeChanges) > 0 {
		fmt.Println()
		fmt.Println("⚠ The following changes require a manual migration:")
		for _, c := range unsafeChanges {
			fmt.Printf("  • %s\n", c)
		}
		fmt.Println("  Use 'alyx migrate create <name>' to create a migration file.")
	}

	return nil
}

func runMigrateRollback(cmd *cobra.Command, args []string) error {
	return fmt.Errorf("rollback is not yet implemented - SQLite has limited ALTER TABLE support")
}

func runMigrateCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	migrator, db, err := getMigrator()
	if err != nil {
		return err
	}
	defer db.Close()

	version, err := migrator.NextVersion()
	if err != nil {
		return fmt.Errorf("getting next version: %w", err)
	}

	path, err := migrator.CreateMigrationFile(name, version)
	if err != nil {
		return fmt.Errorf("creating migration file: %w", err)
	}

	fmt.Printf("Created migration file: %s\n", path)
	fmt.Println()
	fmt.Println("Edit the file to add your migration SQL, then run:")
	fmt.Println("  alyx migrate apply")

	return nil
}

func checkSchemaChanges(db *database.DB, schemaPath string) ([]*schema.Change, error) {
	newSchema, err := schema.ParseFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("parsing schema: %w", err)
	}

	// Get current schema from database
	currentSchema, err := getSchemaFromDB(db)
	if err != nil {
		// If tables don't exist yet, treat as all new
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, fmt.Errorf("getting current schema: %w", err)
	}

	differ := schema.NewDiffer()
	return differ.Diff(currentSchema, newSchema), nil
}

func getSchemaFromDB(db *database.DB) (*schema.Schema, error) {
	// Query SQLite for table information
	rows, err := db.Query(`
		SELECT name FROM sqlite_master 
		WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '_alyx_%'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	s := &schema.Schema{
		Version:     1,
		Collections: make(map[string]*schema.Collection),
	}

	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}

		col, err := getCollectionFromDB(db, tableName)
		if err != nil {
			return nil, fmt.Errorf("getting collection %s: %w", tableName, err)
		}
		s.Collections[tableName] = col
	}

	return s, rows.Err()
}

func getCollectionFromDB(db *database.DB, tableName string) (*schema.Collection, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	col := &schema.Collection{
		Name:   tableName,
		Fields: make(map[string]*schema.Field),
	}

	var fieldOrder []string
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var defaultValue any

		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}

		field := &schema.Field{
			Name:     name,
			Type:     sqliteTypeToFieldType(colType),
			Primary:  pk == 1,
			Nullable: notNull == 0,
		}

		col.Fields[name] = field
		fieldOrder = append(fieldOrder, name)
	}

	col.SetFieldOrder(fieldOrder)
	return col, rows.Err()
}

func sqliteTypeToFieldType(sqlType string) schema.FieldType {
	switch strings.ToUpper(sqlType) {
	case "INTEGER":
		return schema.FieldTypeInt
	case "REAL":
		return schema.FieldTypeFloat
	case "BLOB":
		return schema.FieldTypeBlob
	default:
		return schema.FieldTypeString
	}
}

func filterSafeChanges(changes []*schema.Change) []*schema.Change {
	var safe []*schema.Change
	for _, c := range changes {
		if c.Safe {
			safe = append(safe, c)
		}
	}
	return safe
}

func filterUnsafeChanges(changes []*schema.Change) []*schema.Change {
	var unsafe []*schema.Change
	for _, c := range changes {
		if !c.Safe {
			unsafe = append(unsafe, c)
		}
	}
	return unsafe
}

func hasUnsafeChanges(changes []*schema.Change) bool {
	for _, c := range changes {
		if !c.Safe {
			return true
		}
	}
	return false
}
