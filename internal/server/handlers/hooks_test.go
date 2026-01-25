package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/hooks"
)

func testHookDB(t *testing.T) (*database.DB, *hooks.Registry) {
	t.Helper()

	tmpDir := t.TempDir()
	cfg := &config.DatabaseConfig{
		Path:            tmpDir + "/test.db",
		WALMode:         true,
		ForeignKeys:     true,
		BusyTimeout:     5000,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 3600,
		CacheSize:       -2000,
	}

	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	registry, err := hooks.NewRegistry(db)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	return db, registry
}

func TestHookHandlers_Create(t *testing.T) {
	_, registry := testHookDB(t)
	handlers := NewHookHandlers(registry)

	reqBody := CreateHookRequest{
		Name:        "test-hook",
		FunctionID:  "test-function",
		EventType:   "http",
		EventSource: "api",
		EventAction: "request",
		Mode:        hooks.HookModeAsync,
		Priority:    10,
		Enabled:     true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/hooks", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handlers.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var hook hooks.Hook
	if err := json.NewDecoder(w.Body).Decode(&hook); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if hook.Name != "test-hook" {
		t.Errorf("Expected name 'test-hook', got %s", hook.Name)
	}
}

func TestHookHandlers_List(t *testing.T) {
	_, registry := testHookDB(t)
	handlers := NewHookHandlers(registry)

	hook := &hooks.Hook{
		ID:          "test-id",
		Name:        "test-hook",
		FunctionID:  "test-function",
		EventType:   "http",
		EventSource: "api",
		EventAction: "request",
		Mode:        hooks.HookModeAsync,
		Priority:    10,
		Enabled:     true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := registry.Register(context.Background(), hook); err != nil {
		t.Fatalf("Failed to register hook: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/hooks", nil)
	w := httptest.NewRecorder()

	handlers.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]any
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	count, ok := response["count"].(float64)
	if !ok || count != 1 {
		t.Errorf("Expected count 1, got %v", response["count"])
	}
}

func TestHookHandlers_Get(t *testing.T) {
	_, registry := testHookDB(t)
	handlers := NewHookHandlers(registry)

	hook := &hooks.Hook{
		ID:          "test-id",
		Name:        "test-hook",
		FunctionID:  "test-function",
		EventType:   "http",
		EventSource: "api",
		EventAction: "request",
		Mode:        hooks.HookModeAsync,
		Priority:    10,
		Enabled:     true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := registry.Register(context.Background(), hook); err != nil {
		t.Fatalf("Failed to register hook: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/hooks/test-id", nil)
	req.SetPathValue("id", "test-id")
	w := httptest.NewRecorder()

	handlers.Get(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result hooks.Hook
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got %s", result.ID)
	}
}

func TestHookHandlers_Update(t *testing.T) {
	_, registry := testHookDB(t)
	handlers := NewHookHandlers(registry)

	hook := &hooks.Hook{
		ID:          "test-id",
		Name:        "test-hook",
		FunctionID:  "test-function",
		EventType:   "http",
		EventSource: "api",
		EventAction: "request",
		Mode:        hooks.HookModeAsync,
		Priority:    10,
		Enabled:     true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := registry.Register(context.Background(), hook); err != nil {
		t.Fatalf("Failed to register hook: %v", err)
	}

	newName := "updated-hook"
	reqBody := UpdateHookRequest{
		Name: &newName,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPatch, "/api/hooks/test-id", bytes.NewReader(body))
	req.SetPathValue("id", "test-id")
	w := httptest.NewRecorder()

	handlers.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result hooks.Hook
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Name != "updated-hook" {
		t.Errorf("Expected name 'updated-hook', got %s", result.Name)
	}
}

func TestHookHandlers_Delete(t *testing.T) {
	_, registry := testHookDB(t)
	handlers := NewHookHandlers(registry)

	hook := &hooks.Hook{
		ID:          "test-id",
		Name:        "test-hook",
		FunctionID:  "test-function",
		EventType:   "http",
		EventSource: "api",
		EventAction: "request",
		Mode:        hooks.HookModeAsync,
		Priority:    10,
		Enabled:     true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := registry.Register(context.Background(), hook); err != nil {
		t.Fatalf("Failed to register hook: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/hooks/test-id", nil)
	req.SetPathValue("id", "test-id")
	w := httptest.NewRecorder()

	handlers.Delete(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	_, err := registry.Get(context.Background(), "test-id")
	if err == nil {
		t.Error("Expected hook to be deleted")
	}
}
