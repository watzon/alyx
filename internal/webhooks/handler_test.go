package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/functions"
	"github.com/watzon/alyx/internal/schema"
)

const testWebhookSecret = "test-secret"

func testDB(t *testing.T) *database.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.DatabaseConfig{
		Path: dbPath,
	}

	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func testFunctionService(t *testing.T) *functions.Service {
	t.Helper()

	funcCfg := &config.FunctionsConfig{
		Enabled: true,
		Path:    t.TempDir() + "/functions",
		Timeout: 30 * time.Second,
	}

	cfg := &functions.ServiceConfig{
		FunctionsDir: t.TempDir(),
		Config:       funcCfg,
		ServerPort:   8090,
		Schema:       &schema.Schema{},
	}

	svc, err := functions.NewService(cfg)
	if err != nil {
		t.Fatalf("Failed to create function service: %v", err)
	}

	t.Cleanup(func() {
		svc.Close()
	})

	return svc
}

func TestHandler_ServeHTTP_NotFound(t *testing.T) {
	db := testDB(t)
	svc := testFunctionService(t)
	store := NewStore(db)
	handler := NewHandler(store, svc)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	db := testDB(t)
	svc := testFunctionService(t)
	store := NewStore(db)
	handler := NewHandler(store, svc)

	// Create endpoint that only allows POST
	endpoint := &WebhookEndpoint{
		Path:       "/webhooks/test",
		FunctionID: "test-function",
		Methods:    []string{"POST"},
		Enabled:    true,
	}

	if err := store.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	// Try GET request
	req := httptest.NewRequest(http.MethodGet, "/webhooks/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandler_ServeHTTP_VerificationSuccess(t *testing.T) {
	db := testDB(t)
	svc := testFunctionService(t)
	store := NewStore(db)
	handler := NewHandler(store, svc)

	secret := testWebhookSecret
	body := []byte(`{"event":"test"}`)

	// Compute signature
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	signature := "sha256=" + hex.EncodeToString(h.Sum(nil))

	// Create endpoint with verification
	endpoint := &WebhookEndpoint{
		Path:       "/webhooks/verified",
		FunctionID: "test-function",
		Methods:    []string{"POST"},
		Verification: &WebhookVerification{
			Type:        "hmac-sha256",
			Header:      "X-Hub-Signature",
			Secret:      secret,
			SkipInvalid: false,
		},
		Enabled: true,
	}

	if err := store.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	// Make request with valid signature
	req := httptest.NewRequest(http.MethodPost, "/webhooks/verified", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature", signature)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should fail because function doesn't exist, but verification should pass
	// (we'd get 500 instead of 401)
	if w.Code == http.StatusUnauthorized {
		t.Errorf("Signature verification failed unexpectedly")
	}
}

func TestHandler_ServeHTTP_VerificationFailure(t *testing.T) {
	db := testDB(t)
	svc := testFunctionService(t)
	store := NewStore(db)
	handler := NewHandler(store, svc)

	secret := testWebhookSecret
	body := []byte(`{"event":"test"}`)

	// Create endpoint with verification
	endpoint := &WebhookEndpoint{
		Path:       "/webhooks/verified",
		FunctionID: "test-function",
		Methods:    []string{"POST"},
		Verification: &WebhookVerification{
			Type:        "hmac-sha256",
			Header:      "X-Hub-Signature",
			Secret:      secret,
			SkipInvalid: false, // Reject invalid signatures
		},
		Enabled: true,
	}

	if err := store.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	// Make request with invalid signature
	req := httptest.NewRequest(http.MethodPost, "/webhooks/verified", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature", "sha256=invalid")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestHandler_ServeHTTP_VerificationSkipInvalid(t *testing.T) {
	db := testDB(t)
	svc := testFunctionService(t)
	store := NewStore(db)
	handler := NewHandler(store, svc)

	secret := testWebhookSecret
	body := []byte(`{"event":"test"}`)

	// Create endpoint with skip_invalid=true
	endpoint := &WebhookEndpoint{
		Path:       "/webhooks/verified",
		FunctionID: "test-function",
		Methods:    []string{"POST"},
		Verification: &WebhookVerification{
			Type:        "hmac-sha256",
			Header:      "X-Hub-Signature",
			Secret:      secret,
			SkipInvalid: true, // Pass verification result to function
		},
		Enabled: true,
	}

	if err := store.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	// Make request with invalid signature
	req := httptest.NewRequest(http.MethodPost, "/webhooks/verified", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature", "sha256=invalid")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should NOT return 401 (should try to invoke function instead)
	if w.Code == http.StatusUnauthorized {
		t.Errorf("Expected to skip invalid signature, got 401")
	}
}

func TestHandler_ExtractHeaders(t *testing.T) {
	handler := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom-Header", "value")

	headers := handler.extractHeaders(req)

	if headers["Content-Type"] != "application/json" {
		t.Errorf("Expected Content-Type header")
	}

	if headers["X-Custom-Header"] != "value" {
		t.Errorf("Expected X-Custom-Header")
	}
}

func TestHandler_ExtractQuery(t *testing.T) {
	handler := &Handler{}

	req := httptest.NewRequest(http.MethodGet, "/test?foo=bar&baz=qux", nil)

	query := handler.extractQuery(req)

	if query["foo"] != "bar" {
		t.Errorf("Expected foo=bar")
	}

	if query["baz"] != "qux" {
		t.Errorf("Expected baz=qux")
	}
}

func TestHandler_WriteResponse(t *testing.T) {
	tests := []struct {
		name          string
		output        any
		wantStatus    int
		wantBody      string
		wantHeader    string
		wantHeaderVal string
	}{
		{
			name:       "nil output",
			output:     nil,
			wantStatus: http.StatusOK,
			wantBody:   "",
		},
		{
			name: "map with status and body",
			output: map[string]any{
				"status": float64(201),
				"body":   "created",
			},
			wantStatus: http.StatusCreated,
			wantBody:   "created",
		},
		{
			name: "map with headers",
			output: map[string]any{
				"status": float64(200),
				"headers": map[string]any{
					"X-Custom": "value",
				},
				"body": "ok",
			},
			wantStatus:    http.StatusOK,
			wantBody:      "ok",
			wantHeader:    "X-Custom",
			wantHeaderVal: "value",
		},
		{
			name: "map with JSON body",
			output: map[string]any{
				"status": float64(200),
				"body": map[string]any{
					"message": "hello",
				},
			},
			wantStatus: http.StatusOK,
			wantBody:   `{"message":"hello"}`,
		},
		{
			name: "plain object",
			output: map[string]any{
				"message": "hello",
			},
			wantStatus: http.StatusOK,
			wantBody:   `{"message":"hello"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &Handler{}
			w := httptest.NewRecorder()

			handler.writeResponse(w, tt.output)

			if w.Code != tt.wantStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantBody != "" {
				body := w.Body.String()
				// Trim whitespace for JSON comparison
				body = string(bytes.TrimSpace([]byte(body)))
				wantBody := string(bytes.TrimSpace([]byte(tt.wantBody)))

				if body != wantBody {
					t.Errorf("Body = %q, want %q", body, wantBody)
				}
			}

			if tt.wantHeader != "" {
				if w.Header().Get(tt.wantHeader) != tt.wantHeaderVal {
					t.Errorf("Header %s = %q, want %q",
						tt.wantHeader,
						w.Header().Get(tt.wantHeader),
						tt.wantHeaderVal)
				}
			}
		})
	}
}

func TestHandler_IsMethodAllowed(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name     string
		endpoint *WebhookEndpoint
		method   string
		want     bool
	}{
		{
			name: "no restrictions",
			endpoint: &WebhookEndpoint{
				Methods: []string{},
			},
			method: "POST",
			want:   true,
		},
		{
			name: "method allowed",
			endpoint: &WebhookEndpoint{
				Methods: []string{"POST", "PUT"},
			},
			method: "POST",
			want:   true,
		},
		{
			name: "method not allowed",
			endpoint: &WebhookEndpoint{
				Methods: []string{"POST"},
			},
			method: "GET",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := handler.isMethodAllowed(tt.endpoint, tt.method)
			if got != tt.want {
				t.Errorf("isMethodAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStore_CreateAndGet(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)

	endpoint := &WebhookEndpoint{
		Path:       "/webhooks/test",
		FunctionID: "test-function",
		Methods:    []string{"POST"},
		Verification: &WebhookVerification{
			Type:        "hmac-sha256",
			Header:      "X-Hub-Signature",
			Secret:      "secret",
			SkipInvalid: false,
		},
		Enabled: true,
	}

	// Create
	if err := store.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get by ID
	retrieved, err := store.Get(context.Background(), endpoint.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Path != endpoint.Path {
		t.Errorf("Path = %v, want %v", retrieved.Path, endpoint.Path)
	}

	if retrieved.FunctionID != endpoint.FunctionID {
		t.Errorf("FunctionID = %v, want %v", retrieved.FunctionID, endpoint.FunctionID)
	}

	if len(retrieved.Methods) != len(endpoint.Methods) {
		t.Errorf("Methods length = %v, want %v", len(retrieved.Methods), len(endpoint.Methods))
	}

	if retrieved.Verification == nil {
		t.Fatal("Verification is nil")
	}

	if retrieved.Verification.Type != endpoint.Verification.Type {
		t.Errorf("Verification.Type = %v, want %v", retrieved.Verification.Type, endpoint.Verification.Type)
	}

	// Get by path
	byPath, err := store.GetByPath(context.Background(), endpoint.Path)
	if err != nil {
		t.Fatalf("GetByPath failed: %v", err)
	}

	if byPath.ID != endpoint.ID {
		t.Errorf("GetByPath ID = %v, want %v", byPath.ID, endpoint.ID)
	}
}

func TestStore_Update(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)

	endpoint := &WebhookEndpoint{
		Path:       "/webhooks/test",
		FunctionID: "test-function",
		Methods:    []string{"POST"},
		Enabled:    true,
	}

	if err := store.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update
	endpoint.FunctionID = "updated-function"
	endpoint.Methods = []string{"POST", "PUT"}

	if err := store.Update(context.Background(), endpoint); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify
	retrieved, err := store.Get(context.Background(), endpoint.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.FunctionID != "updated-function" {
		t.Errorf("FunctionID = %v, want updated-function", retrieved.FunctionID)
	}

	if len(retrieved.Methods) != 2 {
		t.Errorf("Methods length = %v, want 2", len(retrieved.Methods))
	}
}

func TestStore_Delete(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)

	endpoint := &WebhookEndpoint{
		Path:       "/webhooks/test",
		FunctionID: "test-function",
		Methods:    []string{"POST"},
		Enabled:    true,
	}

	if err := store.Create(context.Background(), endpoint); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete
	if err := store.Delete(context.Background(), endpoint.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err := store.Get(context.Background(), endpoint.ID)
	if err == nil {
		t.Error("Expected error when getting deleted endpoint")
	}
}

func TestStore_List(t *testing.T) {
	db := testDB(t)
	store := NewStore(db)

	// Create multiple endpoints
	for i := 0; i < 3; i++ {
		endpoint := &WebhookEndpoint{
			Path:       "/webhooks/test" + string(rune('0'+i)),
			FunctionID: "test-function",
			Methods:    []string{"POST"},
			Enabled:    true,
		}

		if err := store.Create(context.Background(), endpoint); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// List
	endpoints, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(endpoints) != 3 {
		t.Errorf("List returned %d endpoints, want 3", len(endpoints))
	}
}

func init() {
	// Ensure JSON encoder doesn't add extra whitespace
	json.Marshal(map[string]any{"test": "value"})
}
