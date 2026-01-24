package deploy

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Store manages deployment state in the database.
type Store struct {
	db *sql.DB
}

// NewStore creates a new deployment store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Init creates the required tables for deployment tracking.
func (s *Store) Init() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS _alyx_deployments (
			id INTEGER PRIMARY KEY,
			version TEXT NOT NULL UNIQUE,
			schema_hash TEXT NOT NULL,
			functions_hash TEXT NOT NULL,
			schema_snapshot TEXT NOT NULL,
			functions_snapshot TEXT,
			deployed_at TEXT NOT NULL DEFAULT (datetime('now')),
			deployed_by TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			rollback_to TEXT,
			description TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_deployments_status ON _alyx_deployments(status);
		CREATE INDEX IF NOT EXISTS idx_deployments_deployed_at ON _alyx_deployments(deployed_at);

		CREATE TABLE IF NOT EXISTS _alyx_admin_tokens (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			token_hash TEXT NOT NULL,
			permissions TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			expires_at TEXT,
			last_used_at TEXT,
			created_by TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_admin_tokens_name ON _alyx_admin_tokens(name);
	`)
	return err
}

// GetCurrentDeployment returns the current active deployment or nil if none exists.
func (s *Store) GetCurrentDeployment() (*Deployment, error) {
	row := s.db.QueryRow(`
		SELECT id, version, schema_hash, functions_hash, schema_snapshot, 
		       functions_snapshot, deployed_at, deployed_by, status, rollback_to, description
		FROM _alyx_deployments
		WHERE status = ?
		ORDER BY deployed_at DESC
		LIMIT 1
	`, StatusActive)

	d, err := s.scanDeployment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil deployment is valid when none exists
	}
	return d, err
}

// GetDeployment returns a deployment by version or nil if not found.
func (s *Store) GetDeployment(version string) (*Deployment, error) {
	row := s.db.QueryRow(`
		SELECT id, version, schema_hash, functions_hash, schema_snapshot, 
		       functions_snapshot, deployed_at, deployed_by, status, rollback_to, description
		FROM _alyx_deployments
		WHERE version = ?
	`, version)

	d, err := s.scanDeployment(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil deployment is valid when version not found
	}
	return d, err
}

// ListDeployments returns deployment history.
func (s *Store) ListDeployments(limit int, status string) ([]*Deployment, error) {
	query := `
		SELECT id, version, schema_hash, functions_hash, schema_snapshot, 
		       functions_snapshot, deployed_at, deployed_by, status, rollback_to, description
		FROM _alyx_deployments
	`
	var args []any

	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}

	query += " ORDER BY deployed_at DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying deployments: %w", err)
	}
	defer rows.Close()

	var deployments []*Deployment
	for rows.Next() {
		d, err := s.scanDeploymentFromRows(rows)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, d)
	}

	return deployments, rows.Err()
}

// NextVersion returns the next deployment version number.
func (s *Store) NextVersion() (string, error) {
	var maxVersion sql.NullString
	err := s.db.QueryRow(`
		SELECT MAX(version) FROM _alyx_deployments
	`).Scan(&maxVersion)
	if err != nil {
		return "", fmt.Errorf("getting max version: %w", err)
	}

	if !maxVersion.Valid || maxVersion.String == "" {
		return "v1", nil
	}

	v := strings.TrimPrefix(maxVersion.String, "v")

	num, err := strconv.Atoi(v)
	if err != nil {
		return "", fmt.Errorf("parsing version number: %w", err)
	}

	return fmt.Sprintf("v%d", num+1), nil
}

// CreateDeployment creates a new deployment record.
func (s *Store) CreateDeployment(d *Deployment) error {
	_, err := s.db.Exec(`
		INSERT INTO _alyx_deployments (
			version, schema_hash, functions_hash, schema_snapshot, 
			functions_snapshot, deployed_by, status, description
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, d.Version, d.SchemaHash, d.FunctionsHash, d.SchemaSnapshot,
		d.FunctionsSnapshot, d.DeployedBy, d.Status, d.Description)

	if err != nil {
		return fmt.Errorf("creating deployment: %w", err)
	}
	return nil
}

// UpdateDeploymentStatus updates the status of a deployment.
func (s *Store) UpdateDeploymentStatus(version string, status DeploymentStatus, rollbackTo string) error {
	_, err := s.db.Exec(`
		UPDATE _alyx_deployments 
		SET status = ?, rollback_to = ?
		WHERE version = ?
	`, status, rollbackTo, version)

	if err != nil {
		return fmt.Errorf("updating deployment status: %w", err)
	}
	return nil
}

// DeactivateAllDeployments marks all active deployments as rolled back.
func (s *Store) DeactivateAllDeployments() error {
	_, err := s.db.Exec(`
		UPDATE _alyx_deployments 
		SET status = ?
		WHERE status = ?
	`, StatusRolledBack, StatusActive)
	return err
}

func (s *Store) scanDeployment(row *sql.Row) (*Deployment, error) {
	var d Deployment
	var deployedAt, expiresAt, rollbackTo, description sql.NullString

	err := row.Scan(
		&d.ID, &d.Version, &d.SchemaHash, &d.FunctionsHash,
		&d.SchemaSnapshot, &d.FunctionsSnapshot, &deployedAt,
		&d.DeployedBy, &d.Status, &rollbackTo, &description,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("scanning deployment: %w", err)
	}

	if deployedAt.Valid {
		if t, err := time.Parse(time.RFC3339, deployedAt.String); err == nil {
			d.DeployedAt = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", deployedAt.String); err == nil {
			d.DeployedAt = t
		}
	}
	if rollbackTo.Valid {
		d.RollbackTo = rollbackTo.String
	}
	if description.Valid {
		d.Description = description.String
	}
	_ = expiresAt // unused but needed for future

	return &d, nil
}

func (s *Store) scanDeploymentFromRows(rows *sql.Rows) (*Deployment, error) {
	var d Deployment
	var deployedAt, rollbackTo, description sql.NullString

	err := rows.Scan(
		&d.ID, &d.Version, &d.SchemaHash, &d.FunctionsHash,
		&d.SchemaSnapshot, &d.FunctionsSnapshot, &deployedAt,
		&d.DeployedBy, &d.Status, &rollbackTo, &description,
	)
	if err != nil {
		return nil, fmt.Errorf("scanning deployment row: %w", err)
	}

	if deployedAt.Valid {
		if t, err := time.Parse(time.RFC3339, deployedAt.String); err == nil {
			d.DeployedAt = t
		} else if t, err := time.Parse("2006-01-02 15:04:05", deployedAt.String); err == nil {
			d.DeployedAt = t
		}
	}
	if rollbackTo.Valid {
		d.RollbackTo = rollbackTo.String
	}
	if description.Valid {
		d.Description = description.String
	}

	return &d, nil
}

// Token management methods.

// CreateToken creates a new admin token.
func (s *Store) CreateToken(name string, permissions []string, expiresAt *time.Time, createdBy string) (string, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	// Hash the token for storage
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing token: %w", err)
	}

	permsStr := strings.Join(permissions, ",")

	var expiresAtStr *string
	if expiresAt != nil {
		s := expiresAt.Format(time.RFC3339)
		expiresAtStr = &s
	}

	_, err = s.db.Exec(`
		INSERT INTO _alyx_admin_tokens (name, token_hash, permissions, expires_at, created_by)
		VALUES (?, ?, ?, ?, ?)
	`, name, string(hash), permsStr, expiresAtStr, createdBy)

	if err != nil {
		return "", fmt.Errorf("creating token: %w", err)
	}

	return token, nil
}

// ValidateToken validates an admin token and returns its permissions.
func (s *Store) ValidateToken(token string) (*AdminToken, error) {
	rows, err := s.db.Query(`
		SELECT id, name, token_hash, permissions, created_at, expires_at, last_used_at, created_by
		FROM _alyx_admin_tokens
	`)
	if err != nil {
		return nil, fmt.Errorf("querying tokens: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var t AdminToken
		var permsStr string
		var createdAt, expiresAt, lastUsedAt sql.NullString

		scanErr := rows.Scan(
			&t.ID, &t.Name, &t.TokenHash, &permsStr,
			&createdAt, &expiresAt, &lastUsedAt, &t.CreatedBy,
		)
		if scanErr != nil {
			continue
		}

		// Check if token matches
		if bcrypt.CompareHashAndPassword([]byte(t.TokenHash), []byte(token)) != nil {
			continue
		}

		// Parse permissions
		if permsStr != "" {
			t.Permissions = strings.Split(permsStr, ",")
		}

		// Parse timestamps
		if createdAt.Valid {
			if parsed, parseErr := time.Parse(time.RFC3339, createdAt.String); parseErr == nil {
				t.CreatedAt = parsed
			}
		}
		if expiresAt.Valid {
			if parsed, parseErr := time.Parse(time.RFC3339, expiresAt.String); parseErr == nil {
				t.ExpiresAt = &parsed
			}
		}
		if lastUsedAt.Valid {
			if parsed, parseErr := time.Parse(time.RFC3339, lastUsedAt.String); parseErr == nil {
				t.LastUsedAt = &parsed
			}
		}

		// Check expiration
		if t.ExpiresAt != nil && t.ExpiresAt.Before(time.Now()) {
			return nil, fmt.Errorf("token expired")
		}

		// Update last used
		_, _ = s.db.Exec(`
			UPDATE _alyx_admin_tokens SET last_used_at = datetime('now') WHERE id = ?
		`, t.ID)

		return &t, nil
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating tokens: %w", err)
	}

	return nil, fmt.Errorf("invalid token")
}

// ListTokens returns all admin tokens (without the actual token values).
func (s *Store) ListTokens() ([]*AdminToken, error) {
	rows, err := s.db.Query(`
		SELECT id, name, permissions, created_at, expires_at, last_used_at, created_by
		FROM _alyx_admin_tokens
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*AdminToken
	for rows.Next() {
		var t AdminToken
		var permsStr string
		var createdAt, expiresAt, lastUsedAt sql.NullString

		err := rows.Scan(
			&t.ID, &t.Name, &permsStr,
			&createdAt, &expiresAt, &lastUsedAt, &t.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning token: %w", err)
		}

		if permsStr != "" {
			t.Permissions = strings.Split(permsStr, ",")
		}
		if createdAt.Valid {
			if parsed, err := time.Parse(time.RFC3339, createdAt.String); err == nil {
				t.CreatedAt = parsed
			}
		}
		if expiresAt.Valid {
			if parsed, err := time.Parse(time.RFC3339, expiresAt.String); err == nil {
				t.ExpiresAt = &parsed
			}
		}
		if lastUsedAt.Valid {
			if parsed, err := time.Parse(time.RFC3339, lastUsedAt.String); err == nil {
				t.LastUsedAt = &parsed
			}
		}

		tokens = append(tokens, &t)
	}

	return tokens, rows.Err()
}

// DeleteToken deletes an admin token by name.
func (s *Store) DeleteToken(name string) error {
	result, err := s.db.Exec(`DELETE FROM _alyx_admin_tokens WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("deleting token: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("token not found")
	}

	return nil
}

// HasPermission checks if a token has a specific permission.
func (t *AdminToken) HasPermission(perm TokenPermission) bool {
	for _, p := range t.Permissions {
		if p == string(PermissionAdmin) || p == string(perm) {
			return true
		}
	}
	return false
}

// hashString returns a SHA256 hash of the input string.
func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
