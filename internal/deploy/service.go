package deploy

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/schema"
)

// Service provides deployment operations.
type Service struct {
	db            *sql.DB
	store         *Store
	schemaPath    string
	functionsPath string
	migrator      *schema.Migrator
}

// NewService creates a new deployment service.
func NewService(db *sql.DB, schemaPath, functionsPath, migrationsPath string) *Service {
	store := NewStore(db)
	migrator := schema.NewMigrator(db, schemaPath, migrationsPath)

	return &Service{
		db:            db,
		store:         store,
		schemaPath:    schemaPath,
		functionsPath: functionsPath,
		migrator:      migrator,
	}
}

// Init initializes the deployment store tables.
func (s *Service) Init() error {
	if err := s.store.Init(); err != nil {
		return fmt.Errorf("initializing deployment store: %w", err)
	}
	if err := s.migrator.Init(); err != nil {
		return fmt.Errorf("initializing migrator: %w", err)
	}
	return nil
}

// Store returns the deployment store.
func (s *Service) Store() *Store {
	return s.store
}

// Prepare analyzes incoming deployment and returns required changes.
func (s *Service) Prepare(req *PrepareRequest) (*PrepareResponse, error) {
	resp := &PrepareResponse{}

	current, err := s.store.GetCurrentDeployment()
	if err != nil {
		return nil, fmt.Errorf("getting current deployment: %w", err)
	}

	if current != nil {
		resp.CurrentVersion = current.Version
	}

	nextVersion, err := s.store.NextVersion()
	if err != nil {
		return nil, fmt.Errorf("getting next version: %w", err)
	}
	resp.NextVersion = nextVersion

	if s.noChangesRequired(current, req) {
		resp.ChangesRequired = false
		return resp, nil
	}

	resp.ChangesRequired = true
	s.analyzeSchemaChanges(current, resp)
	s.analyzeFunctionChanges(current, req.Functions, resp)

	return resp, nil
}

func (s *Service) noChangesRequired(current *Deployment, req *PrepareRequest) bool {
	if current == nil {
		return false
	}
	return current.SchemaHash == req.SchemaHash && current.FunctionsHash == req.FunctionsHash
}

func (s *Service) analyzeSchemaChanges(current *Deployment, resp *PrepareResponse) {
	if s.schemaPath == "" {
		return
	}

	newSchemaData, readErr := os.ReadFile(s.schemaPath)
	if readErr != nil {
		return
	}

	newSchema, parseErr := schema.Parse(newSchemaData)
	if parseErr != nil {
		return
	}

	currentSchema := s.getCurrentSchema(current)
	if currentSchema == nil {
		return
	}

	differ := schema.NewDiffer()
	changes := differ.Diff(currentSchema, newSchema)
	resp.SchemaChanges = changes
	resp.HasUnsafe = differ.HasUnsafeChanges(changes)

	for _, c := range changes {
		if !c.Safe {
			resp.UnsafeWarnings = append(resp.UnsafeWarnings, c.String())
		}
	}
}

func (s *Service) analyzeFunctionChanges(current *Deployment, functions []*FunctionInfo, resp *PrepareResponse) {
	if current == nil || current.FunctionsSnapshot == "" {
		return
	}

	currentFuncs, _ := DeserializeFunctions(current.FunctionsSnapshot)
	if currentFuncs == nil {
		return
	}

	funcChanges := DiffFunctions(functions, currentFuncs)
	resp.FunctionChanges = funcChanges

	for _, fc := range funcChanges {
		if !fc.Safe {
			resp.HasUnsafe = true
			resp.UnsafeWarnings = append(resp.UnsafeWarnings, fc.Reason)
		}
	}
}

// Execute performs the deployment.
func (s *Service) Execute(req *ExecuteRequest, deployedBy string) (*ExecuteResponse, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	current, err := s.store.GetCurrentDeployment()
	if err != nil {
		return nil, fmt.Errorf("getting current deployment: %w", err)
	}

	nextVersion, err := s.store.NextVersion()
	if err != nil {
		return nil, fmt.Errorf("getting next version: %w", err)
	}

	newSchema, err := schema.Parse([]byte(req.Schema))
	if err != nil {
		return nil, fmt.Errorf("parsing schema: %w", err)
	}

	if applyErr := s.applySchemaChanges(current, newSchema); applyErr != nil {
		return nil, fmt.Errorf("applying schema changes: %w", applyErr)
	}

	if funcErr := s.applyFunctionChanges(req.Functions, req.FunctionFiles); funcErr != nil {
		return nil, fmt.Errorf("applying function changes: %w", funcErr)
	}

	// Serialize functions for storage
	funcsSnapshot, err := SerializeFunctions(req.Functions)
	if err != nil {
		return nil, fmt.Errorf("serializing functions: %w", err)
	}

	// Deactivate current deployment
	if current != nil {
		if err := s.store.UpdateDeploymentStatus(current.Version, StatusRolledBack, nextVersion); err != nil {
			return nil, fmt.Errorf("deactivating current deployment: %w", err)
		}
	}

	// Create new deployment record
	deployment := &Deployment{
		Version:           nextVersion,
		SchemaHash:        req.SchemaHash,
		FunctionsHash:     req.FunctionsHash,
		SchemaSnapshot:    req.Schema,
		FunctionsSnapshot: funcsSnapshot,
		DeployedBy:        deployedBy,
		Status:            StatusActive,
		Description:       req.Description,
	}

	if err := s.store.CreateDeployment(deployment); err != nil {
		return nil, fmt.Errorf("creating deployment record: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing deployment: %w", err)
	}

	log.Info().
		Str("version", nextVersion).
		Str("deployed_by", deployedBy).
		Msg("Deployment completed successfully")

	return &ExecuteResponse{
		Success:     true,
		Version:     nextVersion,
		Message:     fmt.Sprintf("Deployed version %s successfully", nextVersion),
		RollbackCmd: fmt.Sprintf("alyx deploy --rollback %s", nextVersion),
	}, nil
}

// Rollback reverts to a previous deployment.
func (s *Service) Rollback(req *RollbackRequest, rolledBackBy string) (*RollbackResponse, error) {
	// Get current deployment
	current, err := s.store.GetCurrentDeployment()
	if err != nil {
		return nil, fmt.Errorf("getting current deployment: %w", err)
	}
	if current == nil {
		return nil, fmt.Errorf("no active deployment to rollback")
	}

	targetVersion := req.ToVersion
	if targetVersion == "" {
		deployments, listErr := s.store.ListDeployments(2, "")
		if listErr != nil {
			return nil, fmt.Errorf("listing deployments: %w", listErr)
		}
		if len(deployments) < 2 {
			return nil, fmt.Errorf("no previous deployment to rollback to")
		}
		targetVersion = deployments[1].Version
	}

	// Get target deployment
	target, err := s.store.GetDeployment(targetVersion)
	if err != nil {
		return nil, fmt.Errorf("getting target deployment: %w", err)
	}
	if target == nil {
		return nil, fmt.Errorf("target deployment %s not found", targetVersion)
	}

	// Start transaction
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Parse target schema
	targetSchema, err := schema.Parse([]byte(target.SchemaSnapshot))
	if err != nil {
		return nil, fmt.Errorf("parsing target schema: %w", err)
	}

	if applyErr := s.applySchemaChanges(current, targetSchema); applyErr != nil {
		return nil, fmt.Errorf("applying schema rollback: %w", applyErr)
	}

	if statusErr := s.store.UpdateDeploymentStatus(current.Version, StatusRolledBack, targetVersion); statusErr != nil {
		return nil, fmt.Errorf("marking current as rolled back: %w", statusErr)
	}

	// Create new deployment pointing to rolled-back state
	nextVersion, err := s.store.NextVersion()
	if err != nil {
		return nil, fmt.Errorf("getting next version: %w", err)
	}

	rollbackDeployment := &Deployment{
		Version:           nextVersion,
		SchemaHash:        target.SchemaHash,
		FunctionsHash:     target.FunctionsHash,
		SchemaSnapshot:    target.SchemaSnapshot,
		FunctionsSnapshot: target.FunctionsSnapshot,
		DeployedBy:        rolledBackBy,
		Status:            StatusActive,
		RollbackTo:        targetVersion,
		Description:       fmt.Sprintf("Rollback to %s: %s", targetVersion, req.Reason),
	}

	if err := s.store.CreateDeployment(rollbackDeployment); err != nil {
		return nil, fmt.Errorf("creating rollback deployment: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing rollback: %w", err)
	}

	log.Info().
		Str("from_version", current.Version).
		Str("to_version", targetVersion).
		Str("new_version", nextVersion).
		Str("rolled_back_by", rolledBackBy).
		Msg("Rollback completed successfully")

	return &RollbackResponse{
		Success:        true,
		RolledBackFrom: current.Version,
		RolledBackTo:   targetVersion,
		Message:        fmt.Sprintf("Rolled back from %s to %s (new version: %s)", current.Version, targetVersion, nextVersion),
	}, nil
}

// History returns deployment history.
func (s *Service) History(req *HistoryRequest) (*HistoryResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	deployments, err := s.store.ListDeployments(limit, req.Status)
	if err != nil {
		return nil, fmt.Errorf("listing deployments: %w", err)
	}

	return &HistoryResponse{
		Deployments: deployments,
		Total:       len(deployments),
	}, nil
}

// getCurrentSchema parses the current deployment's schema.
func (s *Service) getCurrentSchema(current *Deployment) *schema.Schema {
	if current == nil || current.SchemaSnapshot == "" {
		return nil
	}

	parsed, err := schema.Parse([]byte(current.SchemaSnapshot))
	if err != nil {
		return nil
	}
	return parsed
}

func (s *Service) applySchemaChanges(current *Deployment, newSchema *schema.Schema) error {
	if current == nil {
		return s.migrator.ApplySchema(newSchema)
	}

	currentSchema := s.getCurrentSchema(current)
	if currentSchema == nil {
		return s.migrator.ApplySchema(newSchema)
	}

	differ := schema.NewDiffer()
	changes := differ.Diff(currentSchema, newSchema)

	safeChanges := differ.SafeChanges(changes)
	if len(safeChanges) > 0 {
		if err := s.migrator.ApplySafeChanges(safeChanges); err != nil {
			return fmt.Errorf("applying safe changes: %w", err)
		}
	}

	return nil
}

// applyFunctionChanges deploys function files.
func (s *Service) applyFunctionChanges(functions []*FunctionInfo, files map[string][]byte) error {
	if s.functionsPath == "" || len(files) == 0 {
		return nil
	}

	// Ensure functions directory exists
	if err := os.MkdirAll(s.functionsPath, 0o755); err != nil {
		return fmt.Errorf("creating functions directory: %w", err)
	}

	// Write function files
	for name, content := range files {
		// Find corresponding function info to get extension
		var ext string
		for _, f := range functions {
			if f.Name == name {
				switch f.Runtime {
				case "node":
					ext = ".js"
				case "python":
					ext = ".py"
				case "go":
					ext = ".go"
				}
				break
			}
		}
		if ext == "" {
			continue
		}

		path := filepath.Join(s.functionsPath, name+ext)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			return fmt.Errorf("writing function %s: %w", name, err)
		}

		log.Debug().Str("function", name).Str("path", path).Msg("Deployed function")
	}

	return nil
}

// ValidateToken validates an admin token and returns its info.
func (s *Service) ValidateToken(token string) (*AdminToken, error) {
	return s.store.ValidateToken(token)
}

// CreateToken creates a new admin token.
func (s *Service) CreateToken(req *CreateTokenRequest, createdBy string) (*CreateTokenResponse, error) {
	token, err := s.store.CreateToken(req.Name, req.Permissions, req.ExpiresAt, createdBy)
	if err != nil {
		return nil, fmt.Errorf("creating token: %w", err)
	}

	return &CreateTokenResponse{
		Token:       token,
		Name:        req.Name,
		Permissions: req.Permissions,
		ExpiresAt:   req.ExpiresAt,
		Message:     "Token created successfully. Store it securely - it cannot be retrieved again.",
	}, nil
}

// ListTokens returns all admin tokens.
func (s *Service) ListTokens() ([]*AdminToken, error) {
	return s.store.ListTokens()
}

// DeleteToken deletes an admin token.
func (s *Service) DeleteToken(name string) error {
	return s.store.DeleteToken(name)
}
