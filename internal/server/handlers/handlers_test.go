package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

func setupTestHandlers(t *testing.T) (*Handlers, *database.DB) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.DatabaseConfig{
		Path:         dbPath,
		WALMode:      true,
		ForeignKeys:  true,
		CacheSize:    -2000,
		BusyTimeout:  5 * time.Second,
		MaxOpenConns: 1,
		MaxIdleConns: 1,
	}

	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	schemaYAML := `
version: 1
collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      name:
        type: string
      email:
        type: string
        unique: true
      active:
        type: bool
        default: true
      created_at:
        type: timestamp
        default: now
`
	s, err := schema.Parse([]byte(schemaYAML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		if _, err := db.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("execute DDL: %v", err)
		}
	}

	h := New(db, s, config.Default(), nil)

	t.Cleanup(func() {
		db.Close()
	})

	return h, db
}

func TestHealthCheck(t *testing.T) {
	_, db := setupTestHandlers(t)
	h := NewHealthHandlers(db, nil, nil, "test")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != HealthStatusHealthy {
		t.Errorf("expected status 'healthy', got %v", resp.Status)
	}

	if resp.Version != "test" {
		t.Errorf("expected version 'test', got %v", resp.Version)
	}

	if resp.Components["database"].Status != HealthStatusHealthy {
		t.Errorf("expected database status 'healthy', got %v", resp.Components["database"].Status)
	}
}

func TestLiveness(t *testing.T) {
	_, db := setupTestHandlers(t)
	h := NewHealthHandlers(db, nil, nil, "test")

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	w := httptest.NewRecorder()

	h.Liveness(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestReadiness(t *testing.T) {
	_, db := setupTestHandlers(t)
	h := NewHealthHandlers(db, nil, nil, "test")

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	w := httptest.NewRecorder()

	h.Readiness(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestCreateAndGetDocument(t *testing.T) {
	h, _ := setupTestHandlers(t)

	body := bytes.NewBufferString(`{"name":"Alice","email":"alice@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/users", body)
	req.SetPathValue("collection", "users")
	w := httptest.NewRecorder()

	h.CreateDocument(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	id, ok := created["id"].(string)
	if !ok || id == "" {
		t.Error("expected id in response")
	}

	if created["name"] != "Alice" {
		t.Errorf("expected name 'Alice', got %v", created["name"])
	}

	req = httptest.NewRequest(http.MethodGet, "/api/collections/users/"+id, nil)
	req.SetPathValue("collection", "users")
	req.SetPathValue("id", id)
	w = httptest.NewRecorder()

	h.GetDocument(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if got["email"] != "alice@example.com" {
		t.Errorf("expected email 'alice@example.com', got %v", got["email"])
	}
}

func TestListDocuments(t *testing.T) {
	h, db := setupTestHandlers(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		db.ExecContext(ctx, "INSERT INTO users (id, name, email, active, created_at) VALUES (?, ?, ?, ?, datetime('now'))",
			"user-"+string(rune('a'+i)),
			"User "+string(rune('A'+i)),
			"user"+string(rune('a'+i))+"@example.com",
			1)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/collections/users?limit=3", nil)
	req.SetPathValue("collection", "users")
	w := httptest.NewRecorder()

	h.ListDocuments(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	docs, ok := resp["docs"].([]any)
	if !ok {
		t.Fatal("expected docs array in response")
	}

	if len(docs) != 3 {
		t.Errorf("expected 3 docs, got %d", len(docs))
	}

	total, ok := resp["total"].(float64)
	if !ok || int(total) != 5 {
		t.Errorf("expected total 5, got %v", resp["total"])
	}
}

func TestUpdateDocument(t *testing.T) {
	h, _ := setupTestHandlers(t)

	body := bytes.NewBufferString(`{"name":"Bob","email":"bob@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/users", body)
	req.SetPathValue("collection", "users")
	w := httptest.NewRecorder()

	h.CreateDocument(w, req)

	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	id := created["id"].(string)

	body = bytes.NewBufferString(`{"name":"Robert"}`)
	req = httptest.NewRequest(http.MethodPatch, "/api/collections/users/"+id, body)
	req.SetPathValue("collection", "users")
	req.SetPathValue("id", id)
	w = httptest.NewRecorder()

	h.UpdateDocument(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var updated map[string]any
	json.Unmarshal(w.Body.Bytes(), &updated)

	if updated["name"] != "Robert" {
		t.Errorf("expected name 'Robert', got %v", updated["name"])
	}

	if updated["email"] != "bob@example.com" {
		t.Errorf("expected email unchanged, got %v", updated["email"])
	}
}

func TestDeleteDocument(t *testing.T) {
	h, _ := setupTestHandlers(t)

	body := bytes.NewBufferString(`{"name":"Charlie","email":"charlie@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/users", body)
	req.SetPathValue("collection", "users")
	w := httptest.NewRecorder()

	h.CreateDocument(w, req)

	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	id := created["id"].(string)

	req = httptest.NewRequest(http.MethodDelete, "/api/collections/users/"+id, nil)
	req.SetPathValue("collection", "users")
	req.SetPathValue("id", id)
	w = httptest.NewRecorder()

	h.DeleteDocument(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/collections/users/"+id, nil)
	req.SetPathValue("collection", "users")
	req.SetPathValue("id", id)
	w = httptest.NewRecorder()

	h.GetDocument(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestGetDocumentNotFound(t *testing.T) {
	h, _ := setupTestHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/collections/users/nonexistent", nil)
	req.SetPathValue("collection", "users")
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.GetDocument(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestCollectionNotFound(t *testing.T) {
	h, _ := setupTestHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/collections/nonexistent", nil)
	req.SetPathValue("collection", "nonexistent")
	w := httptest.NewRecorder()

	h.ListDocuments(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestInvalidJSON(t *testing.T) {
	h, _ := setupTestHandlers(t)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/users", body)
	req.SetPathValue("collection", "users")
	w := httptest.NewRecorder()

	h.CreateDocument(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func init() {
	os.Setenv("TZ", "UTC")
}
