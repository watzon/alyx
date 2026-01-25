package webhooks

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/watzon/alyx/internal/database"
)

// Store handles database operations for webhook endpoints.
type Store struct {
	db *database.DB
}

// NewStore creates a new webhook store.
func NewStore(db *database.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new webhook endpoint.
func (s *Store) Create(ctx context.Context, endpoint *WebhookEndpoint) error {
	if endpoint.ID == "" {
		endpoint.ID = uuid.New().String()
	}
	if endpoint.CreatedAt.IsZero() {
		endpoint.CreatedAt = time.Now().UTC()
	}

	// Serialize methods
	methodsJSON, err := json.Marshal(endpoint.Methods)
	if err != nil {
		return fmt.Errorf("marshaling methods: %w", err)
	}

	// Serialize verification config
	var verificationJSON []byte
	if endpoint.Verification != nil {
		verificationJSON, err = json.Marshal(endpoint.Verification)
		if err != nil {
			return fmt.Errorf("marshaling verification: %w", err)
		}
	}

	query := `
		INSERT INTO webhook_endpoints (id, path, function_id, methods, verification, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = s.db.ExecContext(ctx, query,
		endpoint.ID,
		endpoint.Path,
		endpoint.FunctionID,
		string(methodsJSON),
		string(verificationJSON),
		endpoint.Enabled,
		endpoint.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting webhook endpoint: %w", err)
	}

	return nil
}

// Update updates an existing webhook endpoint.
func (s *Store) Update(ctx context.Context, endpoint *WebhookEndpoint) error {
	// Serialize methods
	methodsJSON, err := json.Marshal(endpoint.Methods)
	if err != nil {
		return fmt.Errorf("marshaling methods: %w", err)
	}

	// Serialize verification config
	var verificationJSON []byte
	if endpoint.Verification != nil {
		verificationJSON, err = json.Marshal(endpoint.Verification)
		if err != nil {
			return fmt.Errorf("marshaling verification: %w", err)
		}
	}

	query := `
		UPDATE webhook_endpoints
		SET path = ?, function_id = ?, methods = ?, verification = ?, enabled = ?
		WHERE id = ?
	`

	_, err = s.db.ExecContext(ctx, query,
		endpoint.Path,
		endpoint.FunctionID,
		string(methodsJSON),
		string(verificationJSON),
		endpoint.Enabled,
		endpoint.ID,
	)
	if err != nil {
		return fmt.Errorf("updating webhook endpoint: %w", err)
	}

	return nil
}

// Delete removes a webhook endpoint.
func (s *Store) Delete(ctx context.Context, endpointID string) error {
	query := `DELETE FROM webhook_endpoints WHERE id = ?`

	_, err := s.db.ExecContext(ctx, query, endpointID)
	if err != nil {
		return fmt.Errorf("deleting webhook endpoint: %w", err)
	}

	return nil
}

// Get retrieves a webhook endpoint by ID.
func (s *Store) Get(ctx context.Context, endpointID string) (*WebhookEndpoint, error) {
	query := `
		SELECT id, path, function_id, methods, verification, enabled, created_at
		FROM webhook_endpoints
		WHERE id = ?
	`

	row := s.db.QueryRowContext(ctx, query, endpointID)

	endpoint, err := s.scanEndpoint(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("webhook endpoint not found: %s", endpointID)
		}
		return nil, fmt.Errorf("getting webhook endpoint: %w", err)
	}

	return endpoint, nil
}

// GetByPath retrieves a webhook endpoint by path.
func (s *Store) GetByPath(ctx context.Context, path string) (*WebhookEndpoint, error) {
	query := `
		SELECT id, path, function_id, methods, verification, enabled, created_at
		FROM webhook_endpoints
		WHERE path = ? AND enabled = 1
	`

	row := s.db.QueryRowContext(ctx, query, path)

	endpoint, err := s.scanEndpoint(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("webhook endpoint not found: %s", path)
		}
		return nil, fmt.Errorf("getting webhook endpoint by path: %w", err)
	}

	return endpoint, nil
}

// List retrieves all webhook endpoints.
func (s *Store) List(ctx context.Context) ([]*WebhookEndpoint, error) {
	query := `
		SELECT id, path, function_id, methods, verification, enabled, created_at
		FROM webhook_endpoints
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying webhook endpoints: %w", err)
	}
	defer rows.Close()

	return s.scanEndpoints(rows)
}

// scanEndpoint scans a single row into a WebhookEndpoint struct.
func (s *Store) scanEndpoint(row *sql.Row) (*WebhookEndpoint, error) {
	var endpoint WebhookEndpoint
	var methodsJSON string
	var verificationJSON sql.NullString
	var createdAt string
	var enabled int

	err := row.Scan(
		&endpoint.ID,
		&endpoint.Path,
		&endpoint.FunctionID,
		&methodsJSON,
		&verificationJSON,
		&enabled,
		&createdAt,
	)
	if err != nil {
		return nil, err
	}

	// Deserialize methods
	if unmarshalErr := json.Unmarshal([]byte(methodsJSON), &endpoint.Methods); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshaling methods: %w", unmarshalErr)
	}

	// Deserialize verification config
	if verificationJSON.Valid && verificationJSON.String != "" {
		var verification WebhookVerification
		if unmarshalErr := json.Unmarshal([]byte(verificationJSON.String), &verification); unmarshalErr != nil {
			return nil, fmt.Errorf("unmarshaling verification: %w", unmarshalErr)
		}
		endpoint.Verification = &verification
	}

	// Parse enabled
	endpoint.Enabled = enabled == 1

	// Parse timestamp
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parsing created_at: %w", err)
	}
	endpoint.CreatedAt = t

	return &endpoint, nil
}

// scanEndpoints scans rows into WebhookEndpoint structs.
func (s *Store) scanEndpoints(rows *sql.Rows) ([]*WebhookEndpoint, error) {
	var endpoints []*WebhookEndpoint

	for rows.Next() {
		var endpoint WebhookEndpoint
		var methodsJSON string
		var verificationJSON sql.NullString
		var createdAt string
		var enabled int

		err := rows.Scan(
			&endpoint.ID,
			&endpoint.Path,
			&endpoint.FunctionID,
			&methodsJSON,
			&verificationJSON,
			&enabled,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning webhook endpoint row: %w", err)
		}

		// Deserialize methods
		if unmarshalErr := json.Unmarshal([]byte(methodsJSON), &endpoint.Methods); unmarshalErr != nil {
			return nil, fmt.Errorf("unmarshaling methods: %w", unmarshalErr)
		}

		// Deserialize verification config
		if verificationJSON.Valid && verificationJSON.String != "" {
			var verification WebhookVerification
			if unmarshalErr := json.Unmarshal([]byte(verificationJSON.String), &verification); unmarshalErr != nil {
				return nil, fmt.Errorf("unmarshaling verification: %w", unmarshalErr)
			}
			endpoint.Verification = &verification
		}

		// Parse enabled
		endpoint.Enabled = enabled == 1

		// Parse timestamp
		t, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parsing created_at: %w", err)
		}
		endpoint.CreatedAt = t

		endpoints = append(endpoints, &endpoint)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating webhook endpoint rows: %w", err)
	}

	return endpoints, nil
}
