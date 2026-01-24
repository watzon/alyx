package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/watzon/alyx/internal/deploy"
)

// Table formatting constants.
const (
	historyTableWidth      = 80
	historyDescMaxLen      = 30
	historyDescTruncateLen = 27
	hashDisplayLen         = 12
)

var (
	deployURL      string
	deployToken    string
	deployDryRun   bool
	deployForce    bool
	deployRollback string
	deployHistory  bool
	deployDesc     string
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy to a remote Alyx instance",
	Long: `Deploy schema and functions to a remote Alyx server.

This command bundles your local schema.yaml and functions, computes
hashes, and synchronizes them with the remote server.

Examples:
  alyx deploy --url https://api.myapp.com --token <token>
  alyx deploy --url https://api.myapp.com --token <token> --dry-run
  alyx deploy --url https://api.myapp.com --token <token> --rollback v2
  alyx deploy --url https://api.myapp.com --token <token> --history

Environment Variables:
  ALYX_DEPLOY_URL    Default deployment URL
  ALYX_DEPLOY_TOKEN  Default deployment token`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().StringVar(&deployURL, "url", "", "Remote Alyx server URL (or ALYX_DEPLOY_URL)")
	deployCmd.Flags().StringVar(&deployToken, "token", "", "Admin token for authentication (or ALYX_DEPLOY_TOKEN)")
	deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "Show what would change without applying")
	deployCmd.Flags().BoolVar(&deployForce, "force", false, "Force deployment even with unsafe changes")
	deployCmd.Flags().StringVar(&deployRollback, "rollback", "", "Rollback to specified version")
	deployCmd.Flags().BoolVar(&deployHistory, "history", false, "Show deployment history")
	deployCmd.Flags().StringVar(&deployDesc, "description", "", "Deployment description")

	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, args []string) error {
	// Resolve URL and token from environment if not provided
	if deployURL == "" {
		deployURL = os.Getenv("ALYX_DEPLOY_URL")
	}
	if deployToken == "" {
		deployToken = os.Getenv("ALYX_DEPLOY_TOKEN")
	}

	if deployURL == "" {
		return fmt.Errorf("--url is required (or set ALYX_DEPLOY_URL)")
	}
	if deployToken == "" {
		return fmt.Errorf("--token is required (or set ALYX_DEPLOY_TOKEN)")
	}

	// Normalize URL
	deployURL = strings.TrimSuffix(deployURL, "/")

	client := &deployClient{
		baseURL: deployURL,
		token:   deployToken,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}

	// Handle history request
	if deployHistory {
		return showHistory(client)
	}

	// Handle rollback request
	if deployRollback != "" {
		return doRollback(client, deployRollback)
	}

	// Normal deployment
	return doDeploy(client)
}

type deployClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func (c *deployClient) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	return c.client.Do(req)
}

func doDeploy(client *deployClient) error {
	bundle, bundler, err := createDeployBundle()
	if err != nil {
		return err
	}

	printBundleInfo(bundle)

	ctx := context.Background()
	prepResp, err := prepareDeployment(ctx, client, bundle)
	if err != nil {
		return err
	}

	printVersionInfo(prepResp)

	if !prepResp.ChangesRequired {
		fmt.Println()
		fmt.Println("No changes detected. Already up to date.")
		return nil
	}

	printChanges(prepResp)

	if err := checkUnsafeChanges(prepResp); err != nil {
		return err
	}

	if deployDryRun {
		fmt.Println()
		fmt.Println("Dry run complete. No changes applied.")
		return nil
	}

	fmt.Println()
	if !confirmAction("Deploy these changes?") {
		fmt.Println("Deployment canceled.")
		return nil
	}

	return executeDeployment(ctx, client, bundle, bundler)
}

func createDeployBundle() (*deploy.Bundle, *deploy.Bundler, error) {
	schemaPath := resolveSchemaPath("")
	if schemaPath == "" {
		return nil, nil, fmt.Errorf("schema.yaml not found")
	}

	bundler := deploy.NewBundler(schemaPath, "functions")
	bundle, err := bundler.CreateBundle()
	if err != nil {
		return nil, nil, fmt.Errorf("creating bundle: %w", err)
	}

	return bundle, bundler, nil
}

func printBundleInfo(bundle *deploy.Bundle) {
	fmt.Println("Preparing deployment...")
	fmt.Printf("  Schema hash: %s\n", truncateHash(bundle.SchemaHash))
	fmt.Printf("  Functions:   %d\n", len(bundle.Functions))
	if bundle.FunctionsHash != "" {
		fmt.Printf("  Functions hash: %s\n", truncateHash(bundle.FunctionsHash))
	}
}

func prepareDeployment(ctx context.Context, client *deployClient, bundle *deploy.Bundle) (*deploy.PrepareResponse, error) {
	prepReq := &deploy.PrepareRequest{
		SchemaHash:    bundle.SchemaHash,
		FunctionsHash: bundle.FunctionsHash,
		Functions:     bundle.Functions,
	}

	resp, err := client.doRequest(ctx, "POST", "/api/admin/deploy/prepare", prepReq)
	if err != nil {
		return nil, fmt.Errorf("prepare request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, handleErrorResponse(resp)
	}

	var prepResp deploy.PrepareResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&prepResp); decodeErr != nil {
		return nil, fmt.Errorf("parsing prepare response: %w", decodeErr)
	}

	return &prepResp, nil
}

func printVersionInfo(prepResp *deploy.PrepareResponse) {
	fmt.Println()
	if prepResp.CurrentVersion != "" {
		fmt.Printf("Current version: %s\n", prepResp.CurrentVersion)
	} else {
		fmt.Println("Current version: (none)")
	}
	fmt.Printf("Next version:    %s\n", prepResp.NextVersion)
}

func printChanges(prepResp *deploy.PrepareResponse) {
	fmt.Println()
	fmt.Println("Changes to apply:")

	if len(prepResp.SchemaChanges) > 0 {
		fmt.Println("  Schema changes:")
		for _, c := range prepResp.SchemaChanges {
			status := "  "
			if !c.Safe {
				status = "! "
			}
			fmt.Printf("    %s%s\n", status, c)
		}
	}

	if len(prepResp.FunctionChanges) > 0 {
		fmt.Println("  Function changes:")
		for _, c := range prepResp.FunctionChanges {
			status := "  "
			if !c.Safe {
				status = "! "
			}
			fmt.Printf("    %s%s %s (%s)\n", status, functionChangeAction(c.Type), c.Name, c.Runtime)
		}
	}
}

func functionChangeAction(t deploy.FunctionChangeType) string {
	switch t {
	case deploy.FunctionAdd:
		return "add"
	case deploy.FunctionRemove:
		return "remove"
	case deploy.FunctionModify:
		return "modify"
	default:
		return "unknown"
	}
}

func checkUnsafeChanges(prepResp *deploy.PrepareResponse) error {
	if !prepResp.HasUnsafe {
		return nil
	}

	fmt.Println()
	fmt.Println("WARNING: This deployment contains unsafe changes:")
	for _, warning := range prepResp.UnsafeWarnings {
		fmt.Printf("  ! %s\n", warning)
	}

	if !deployForce {
		fmt.Println()
		fmt.Println("Use --force to proceed with unsafe changes.")
		return fmt.Errorf("deployment aborted due to unsafe changes")
	}

	return nil
}

func executeDeployment(ctx context.Context, client *deployClient, bundle *deploy.Bundle, bundler *deploy.Bundler) error {
	var funcFiles map[string][]byte
	var err error
	if len(bundle.Functions) > 0 {
		funcFiles, err = bundler.ReadFunctionFiles(bundle.Functions)
		if err != nil {
			return fmt.Errorf("reading function files: %w", err)
		}
	}

	execReq := &deploy.ExecuteRequest{
		Schema:        bundle.SchemaRaw,
		SchemaHash:    bundle.SchemaHash,
		Functions:     bundle.Functions,
		FunctionsHash: bundle.FunctionsHash,
		FunctionFiles: funcFiles,
		Description:   deployDesc,
		Force:         deployForce,
	}

	fmt.Println()
	fmt.Println("Deploying...")

	resp, err := client.doRequest(ctx, "POST", "/api/admin/deploy/execute", execReq)
	if err != nil {
		return fmt.Errorf("execute request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return handleErrorResponse(resp)
	}

	var execResp deploy.ExecuteResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&execResp); decodeErr != nil {
		return fmt.Errorf("parsing execute response: %w", decodeErr)
	}

	if !execResp.Success {
		return fmt.Errorf("deployment failed: %s", execResp.Message)
	}

	fmt.Printf("\n%s\n", execResp.Message)
	fmt.Printf("Rollback command: %s\n", execResp.RollbackCmd)

	return nil
}

func doRollback(client *deployClient, toVersion string) error {
	fmt.Printf("Rolling back to version %s...\n", toVersion)

	if !confirmAction("Are you sure you want to rollback?") {
		fmt.Println("Rollback canceled.")
		return nil
	}

	req := &deploy.RollbackRequest{
		ToVersion: toVersion,
		Reason:    "CLI rollback",
	}

	ctx := context.Background()
	resp, err := client.doRequest(ctx, "POST", "/api/admin/deploy/rollback", req)
	if err != nil {
		return fmt.Errorf("rollback request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return handleErrorResponse(resp)
	}

	var rollbackResp deploy.RollbackResponse
	if err := json.NewDecoder(resp.Body).Decode(&rollbackResp); err != nil {
		return fmt.Errorf("parsing rollback response: %w", err)
	}

	if !rollbackResp.Success {
		return fmt.Errorf("rollback failed: %s", rollbackResp.Message)
	}

	fmt.Printf("\n%s\n", rollbackResp.Message)

	return nil
}

func showHistory(client *deployClient) error {
	ctx := context.Background()
	resp, err := client.doRequest(ctx, "GET", "/api/admin/deploy/history?limit=20", nil)
	if err != nil {
		return fmt.Errorf("history request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return handleErrorResponse(resp)
	}

	var historyResp deploy.HistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&historyResp); err != nil {
		return fmt.Errorf("parsing history response: %w", err)
	}

	if len(historyResp.Deployments) == 0 {
		fmt.Println("No deployments yet.")
		return nil
	}

	fmt.Println("Deployment History:")
	fmt.Println()
	fmt.Printf("%-8s %-12s %-20s %-15s %s\n", "VERSION", "STATUS", "DEPLOYED AT", "DEPLOYED BY", "DESCRIPTION")
	fmt.Println(strings.Repeat("-", historyTableWidth))

	for _, d := range historyResp.Deployments {
		status := string(d.Status)
		if d.RollbackTo != "" {
			status += " -> " + d.RollbackTo
		}

		desc := d.Description
		if len(desc) > historyDescMaxLen {
			desc = desc[:historyDescTruncateLen] + "..."
		}

		fmt.Printf("%-8s %-12s %-20s %-15s %s\n",
			d.Version,
			status,
			d.DeployedAt.Format("2006-01-02 15:04:05"),
			d.DeployedBy,
			desc,
		)
	}

	return nil
}

func handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Code    string `json:"code"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil {
		if errResp.Message != "" {
			return fmt.Errorf("server error: %s", errResp.Message)
		}
		if errResp.Error != "" {
			return fmt.Errorf("server error: %s", errResp.Error)
		}
	}

	return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
}

func confirmAction(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func truncateHash(hash string) string {
	if len(hash) > hashDisplayLen {
		return hash[:hashDisplayLen] + "..."
	}
	return hash
}
