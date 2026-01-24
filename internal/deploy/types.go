// Package deploy provides deployment management for Alyx.
package deploy

import (
	"time"

	"github.com/watzon/alyx/internal/schema"
)

// DeploymentStatus represents the status of a deployment.
type DeploymentStatus string

const (
	// StatusActive indicates the deployment is currently active.
	StatusActive DeploymentStatus = "active"
	// StatusRolledBack indicates the deployment was rolled back.
	StatusRolledBack DeploymentStatus = "rolled_back"
	// StatusFailed indicates the deployment failed.
	StatusFailed DeploymentStatus = "failed"
)

// Deployment represents a deployment record.
type Deployment struct {
	ID                int64            `json:"id"`
	Version           string           `json:"version"`
	SchemaHash        string           `json:"schema_hash"`
	FunctionsHash     string           `json:"functions_hash"`
	SchemaSnapshot    string           `json:"schema_snapshot"`
	FunctionsSnapshot string           `json:"functions_snapshot,omitempty"`
	DeployedAt        time.Time        `json:"deployed_at"`
	DeployedBy        string           `json:"deployed_by,omitempty"`
	Status            DeploymentStatus `json:"status"`
	RollbackTo        string           `json:"rollback_to,omitempty"`
	Description       string           `json:"description,omitempty"`
}

// FunctionInfo represents function metadata for deployment.
type FunctionInfo struct {
	Name     string `json:"name"`
	Runtime  string `json:"runtime"`
	Hash     string `json:"hash"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

// Bundle represents a deployment bundle containing schema and functions.
type Bundle struct {
	Schema        *schema.Schema  `json:"schema"`
	SchemaRaw     string          `json:"schema_raw"`
	SchemaHash    string          `json:"schema_hash"`
	Functions     []*FunctionInfo `json:"functions"`
	FunctionsHash string          `json:"functions_hash"`
}

// PrepareRequest is the request payload for deployment preparation.
type PrepareRequest struct {
	SchemaHash    string          `json:"schema_hash"`
	FunctionsHash string          `json:"functions_hash"`
	Functions     []*FunctionInfo `json:"functions,omitempty"`
}

// PrepareResponse is the response from deployment preparation.
type PrepareResponse struct {
	ChangesRequired bool              `json:"changes_required"`
	SchemaChanges   []*schema.Change  `json:"schema_changes,omitempty"`
	FunctionChanges []*FunctionChange `json:"function_changes,omitempty"`
	CurrentVersion  string            `json:"current_version,omitempty"`
	NextVersion     string            `json:"next_version"`
	HasUnsafe       bool              `json:"has_unsafe"`
	UnsafeWarnings  []string          `json:"unsafe_warnings,omitempty"`
}

// FunctionChange represents a change to a function.
type FunctionChange struct {
	Type    FunctionChangeType `json:"type"`
	Name    string             `json:"name"`
	Runtime string             `json:"runtime,omitempty"`
	OldHash string             `json:"old_hash,omitempty"`
	NewHash string             `json:"new_hash,omitempty"`
	Safe    bool               `json:"safe"`
	Reason  string             `json:"reason,omitempty"`
}

// FunctionChangeType represents the type of function change.
type FunctionChangeType string

const (
	// FunctionAdd indicates a new function was added.
	FunctionAdd FunctionChangeType = "add"
	// FunctionRemove indicates a function was removed.
	FunctionRemove FunctionChangeType = "remove"
	// FunctionModify indicates a function was modified.
	FunctionModify FunctionChangeType = "modify"
)

// ExecuteRequest is the request payload for deployment execution.
type ExecuteRequest struct {
	Schema        string            `json:"schema"`
	SchemaHash    string            `json:"schema_hash"`
	Functions     []*FunctionInfo   `json:"functions"`
	FunctionsHash string            `json:"functions_hash"`
	FunctionFiles map[string][]byte `json:"function_files,omitempty"`
	Description   string            `json:"description,omitempty"`
	Force         bool              `json:"force,omitempty"`
}

// ExecuteResponse is the response from deployment execution.
type ExecuteResponse struct {
	Success     bool   `json:"success"`
	Version     string `json:"version"`
	Message     string `json:"message,omitempty"`
	RollbackCmd string `json:"rollback_cmd,omitempty"`
}

// RollbackRequest is the request payload for rollback.
type RollbackRequest struct {
	ToVersion string `json:"to_version,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// RollbackResponse is the response from rollback.
type RollbackResponse struct {
	Success        bool   `json:"success"`
	RolledBackFrom string `json:"rolled_back_from"`
	RolledBackTo   string `json:"rolled_back_to"`
	Message        string `json:"message,omitempty"`
}

// HistoryRequest is the request for deployment history.
type HistoryRequest struct {
	Limit  int    `json:"limit,omitempty"`
	Status string `json:"status,omitempty"`
}

// HistoryResponse is the response containing deployment history.
type HistoryResponse struct {
	Deployments []*Deployment `json:"deployments"`
	Total       int           `json:"total"`
}

// AdminToken represents an admin token for deployment authentication.
type AdminToken struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	TokenHash   string     `json:"-"`
	Permissions []string   `json:"permissions"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedBy   string     `json:"created_by,omitempty"`
}

// TokenPermission represents permissions for admin tokens.
type TokenPermission string

const (
	// PermissionDeploy allows deployment operations.
	PermissionDeploy TokenPermission = "deploy"
	// PermissionRollback allows rollback operations.
	PermissionRollback TokenPermission = "rollback"
	// PermissionAdmin allows full admin access.
	PermissionAdmin TokenPermission = "admin"
)

// CreateTokenRequest is the request to create an admin token.
type CreateTokenRequest struct {
	Name        string     `json:"name"`
	Permissions []string   `json:"permissions"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// CreateTokenResponse is the response after creating an admin token.
type CreateTokenResponse struct {
	Token       string     `json:"token"`
	Name        string     `json:"name"`
	Permissions []string   `json:"permissions"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Message     string     `json:"message"`
}
