package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/deploy"
)

// Table formatting constants.
const (
	tokensTableWidth    = 90
	permissionsMaxLen   = 18
	permissionsTruncLen = 15
)

var (
	adminTokenExpiry string
	adminTokenPerms  []string
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Admin utilities",
	Long: `Administrative utilities for Alyx.

Commands:
  create-token  Create an admin token for deployment
  list-tokens   List all admin tokens
  revoke-token  Revoke an admin token`,
}

var createTokenCmd = &cobra.Command{
	Use:   "create-token <name>",
	Short: "Create an admin token",
	Long: `Create an admin token for deployment authentication.

The token will be displayed once after creation. Store it securely
as it cannot be retrieved again.

Examples:
  alyx admin create-token deploy-ci
  alyx admin create-token deploy-ci --permissions deploy,rollback
  alyx admin create-token deploy-ci --expires 30d`,
	Args: cobra.ExactArgs(1),
	RunE: runCreateToken,
}

var listTokensCmd = &cobra.Command{
	Use:   "list-tokens",
	Short: "List admin tokens",
	Long:  `List all admin tokens (token values are not shown).`,
	RunE:  runListTokens,
}

var revokeTokenCmd = &cobra.Command{
	Use:   "revoke-token <name>",
	Short: "Revoke an admin token",
	Long:  `Revoke (delete) an admin token by name.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runRevokeToken,
}

func init() {
	createTokenCmd.Flags().StringVar(&adminTokenExpiry, "expires", "", "Token expiry duration (e.g., 30d, 1y)")
	createTokenCmd.Flags().StringSliceVar(&adminTokenPerms, "permissions", []string{"deploy", "rollback"}, "Token permissions (deploy, rollback, admin)")

	adminCmd.AddCommand(createTokenCmd)
	adminCmd.AddCommand(listTokensCmd)
	adminCmd.AddCommand(revokeTokenCmd)

	rootCmd.AddCommand(adminCmd)
}

func getDeployService() (*deploy.Service, *database.DB, error) {
	cfg, err := config.LoadWithDefaults()
	if err != nil {
		cfg = config.Default()
	}

	db, err := database.Open(&cfg.Database)
	if err != nil {
		return nil, nil, fmt.Errorf("opening database: %w", err)
	}

	svc := deploy.NewService(db.DB, "schema.yaml", "functions", "migrations")
	if err := svc.Init(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("initializing deploy service: %w", err)
	}

	return svc, db, nil
}

func runCreateToken(cmd *cobra.Command, args []string) error {
	name := args[0]

	svc, db, err := getDeployService()
	if err != nil {
		return err
	}
	defer db.Close()

	var expiresAt *time.Time
	if adminTokenExpiry != "" {
		duration, parseErr := parseDuration(adminTokenExpiry)
		if parseErr != nil {
			return fmt.Errorf("invalid expiry format: %w", parseErr)
		}
		t := time.Now().Add(duration)
		expiresAt = &t
	}

	req := &deploy.CreateTokenRequest{
		Name:        name,
		Permissions: adminTokenPerms,
		ExpiresAt:   expiresAt,
	}

	resp, err := svc.CreateToken(req, "cli")
	if err != nil {
		return fmt.Errorf("creating token: %w", err)
	}

	fmt.Println("Admin token created successfully!")
	fmt.Println()
	fmt.Printf("Name:        %s\n", resp.Name)
	fmt.Printf("Permissions: %s\n", strings.Join(resp.Permissions, ", "))
	if resp.ExpiresAt != nil {
		fmt.Printf("Expires:     %s\n", resp.ExpiresAt.Format("2006-01-02 15:04:05"))
	} else {
		fmt.Println("Expires:     never")
	}
	fmt.Println()
	fmt.Println("Token (store securely - shown only once):")
	fmt.Printf("  %s\n", resp.Token)
	fmt.Println()
	fmt.Println("Use with deploy command:")
	fmt.Printf("  alyx deploy --url <url> --token %s\n", resp.Token)
	fmt.Println()
	fmt.Println("Or set as environment variable:")
	fmt.Printf("  export ALYX_DEPLOY_TOKEN=%s\n", resp.Token)

	return nil
}

func runListTokens(cmd *cobra.Command, args []string) error {
	svc, db, err := getDeployService()
	if err != nil {
		return err
	}
	defer db.Close()

	tokens, err := svc.ListTokens()
	if err != nil {
		return fmt.Errorf("listing tokens: %w", err)
	}

	if len(tokens) == 0 {
		fmt.Println("No admin tokens found.")
		fmt.Println()
		fmt.Println("Create one with:")
		fmt.Println("  alyx admin create-token <name>")
		return nil
	}

	fmt.Println("Admin Tokens:")
	fmt.Println()
	fmt.Printf("%-20s %-20s %-25s %-20s\n", "NAME", "PERMISSIONS", "CREATED", "LAST USED")
	fmt.Println(strings.Repeat("-", tokensTableWidth))

	for _, t := range tokens {
		perms := strings.Join(t.Permissions, ",")
		if len(perms) > permissionsMaxLen {
			perms = perms[:permissionsTruncLen] + "..."
		}

		created := t.CreatedAt.Format("2006-01-02 15:04")
		lastUsed := "never"
		if t.LastUsedAt != nil {
			lastUsed = t.LastUsedAt.Format("2006-01-02 15:04")
		}

		fmt.Printf("%-20s %-20s %-25s %-20s\n", t.Name, perms, created, lastUsed)
	}

	return nil
}

func runRevokeToken(cmd *cobra.Command, args []string) error {
	name := args[0]

	svc, db, err := getDeployService()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := svc.DeleteToken(name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("token %q not found", name)
		}
		return fmt.Errorf("revoking token: %w", err)
	}

	fmt.Printf("Token %q revoked successfully.\n", name)
	return nil
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	var multiplier time.Duration
	var numStr string

	switch {
	case strings.HasSuffix(s, "d"):
		numStr = strings.TrimSuffix(s, "d")
		multiplier = 24 * time.Hour
	case strings.HasSuffix(s, "w"):
		numStr = strings.TrimSuffix(s, "w")
		multiplier = 7 * 24 * time.Hour
	case strings.HasSuffix(s, "m"):
		numStr = strings.TrimSuffix(s, "m")
		multiplier = 30 * 24 * time.Hour
	case strings.HasSuffix(s, "y"):
		numStr = strings.TrimSuffix(s, "y")
		multiplier = 365 * 24 * time.Hour
	default:
		return time.ParseDuration(s)
	}

	var num int
	if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
		return 0, fmt.Errorf("invalid number: %s", numStr)
	}

	return time.Duration(num) * multiplier, nil
}
