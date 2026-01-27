package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
	"github.com/watzon/alyx/internal/storage"
)

// Test constants for repeated strings.
const (
	testPNGHeader = "\x89PNG\r\n\x1a\n"
)

func setupFileFieldHandlers(t *testing.T) (*Handlers, *storage.Service) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	storagePath := filepath.Join(tmpDir, "storage")

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
buckets:
  avatars:
    backend: filesystem
    max_file_size: 5242880  # 5MB
    allowed_types:
      - image/jpeg
      - image/png
      - image/gif
  documents:
    backend: filesystem
    max_file_size: 10485760  # 10MB
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
      avatar:
        type: file
        nullable: true
        file:
          bucket: avatars
          on_delete: cascade
      created_at:
        type: timestamp
        default: now
  posts:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      title:
        type: string
      attachment:
        type: file
        nullable: true
        file:
          bucket: documents
          on_delete: restrict
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

	backend := storage.NewFilesystemBackend(storagePath)

	backends := map[string]storage.Backend{
		"filesystem": backend,
	}

	storageService := storage.NewService(db, backends, s, config.Default(), nil)

	h := New(db, s, config.Default(), nil)
	h.SetStorageService(storageService)

	t.Cleanup(func() {
		db.Close()
	})

	return h, storageService
}

func createTestFile(t *testing.T, service *storage.Service, bucket, filename, content string) string {
	t.Helper()

	file, err := service.Upload(context.Background(), bucket, filename, strings.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("upload test file: %v", err)
	}

	return file.ID
}

func TestCreateDocument_WithValidFileID(t *testing.T) {
	h, storageService := setupFileFieldHandlers(t)

	pngHeader := testPNGHeader
	fileID := createTestFile(t, storageService, "avatars", "test.png", pngHeader+"fake-png-data")

	body := bytes.NewBufferString(`{"name":"Alice","email":"alice@example.com","avatar":"` + fileID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/users", body)
	req.SetPathValue("collection", "users")
	w := httptest.NewRecorder()

	h.CreateDocument(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["avatar"] != fileID {
		t.Errorf("expected avatar %q, got %v", fileID, resp["avatar"])
	}
}

func TestCreateDocument_WithInvalidFileID(t *testing.T) {
	h, _ := setupFileFieldHandlers(t)

	fakeFileID := "00000000-0000-0000-0000-000000000000"
	body := bytes.NewBufferString(`{"name":"Bob","email":"bob@example.com","avatar":"` + fakeFileID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/users", body)
	req.SetPathValue("collection", "users")
	w := httptest.NewRecorder()

	h.CreateDocument(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["code"] != "FILE_NOT_FOUND" {
		t.Errorf("expected error code FILE_NOT_FOUND, got %v", resp["code"])
	}
}

func TestCreateDocument_WithWrongBucketFile(t *testing.T) {
	h, storageService := setupFileFieldHandlers(t)

	fileID := createTestFile(t, storageService, "documents", "test.pdf", "fake-pdf-data")

	body := bytes.NewBufferString(`{"name":"Charlie","email":"charlie@example.com","avatar":"` + fileID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/users", body)
	req.SetPathValue("collection", "users")
	w := httptest.NewRecorder()

	h.CreateDocument(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d: %s", http.StatusBadRequest, w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["code"] != "FILE_WRONG_BUCKET" {
		t.Errorf("expected error code FILE_WRONG_BUCKET, got %v", resp["code"])
	}
}

func TestExpandFileField(t *testing.T) {
	h, storageService := setupFileFieldHandlers(t)

	pngHeader := testPNGHeader
	fileID := createTestFile(t, storageService, "avatars", "avatar.png", pngHeader+"fake-png-data")

	body := bytes.NewBufferString(`{"name":"Dave","email":"dave@example.com","avatar":"` + fileID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/users", body)
	req.SetPathValue("collection", "users")
	w := httptest.NewRecorder()
	h.CreateDocument(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create document: %s", w.Body.String())
	}

	var createResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	userID := createResp["id"].(string)

	req = httptest.NewRequest(http.MethodGet, "/api/collections/users/"+userID+"?expand=avatar", nil)
	req.SetPathValue("collection", "users")
	req.SetPathValue("id", userID)
	w = httptest.NewRecorder()

	h.GetDocument(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	avatar, ok := resp["avatar"].(map[string]any)
	if !ok {
		t.Fatalf("expected avatar to be object, got %T: %v", resp["avatar"], resp["avatar"])
	}

	if avatar["id"] != fileID {
		t.Errorf("expected file id %q, got %v", fileID, avatar["id"])
	}

	if avatar["name"] != "avatar.png" {
		t.Errorf("expected filename 'avatar.png', got %v", avatar["name"])
	}

	if avatar["bucket"] != "avatars" {
		t.Errorf("expected bucket 'avatars', got %v", avatar["bucket"])
	}
}

func TestDeleteDocument_CascadeDeletesFile(t *testing.T) {
	h, storageService := setupFileFieldHandlers(t)

	pngHeader := testPNGHeader
	fileID := createTestFile(t, storageService, "avatars", "avatar.png", pngHeader+"fake-png-data")

	body := bytes.NewBufferString(`{"name":"Eve","email":"eve@example.com","avatar":"` + fileID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/users", body)
	req.SetPathValue("collection", "users")
	w := httptest.NewRecorder()
	h.CreateDocument(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create document: %s", w.Body.String())
	}

	var createResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	userID := createResp["id"].(string)

	_, err := storageService.GetMetadata(context.Background(), "avatars", fileID)
	if err != nil {
		t.Fatalf("file should exist before delete: %v", err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/collections/users/"+userID, nil)
	req.SetPathValue("collection", "users")
	req.SetPathValue("id", userID)
	w = httptest.NewRecorder()

	h.DeleteDocument(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d: %s", http.StatusNoContent, w.Code, w.Body.String())
	}

	_, err = storageService.GetMetadata(context.Background(), "avatars", fileID)
	if err == nil {
		t.Error("file should be deleted after cascade delete")
	}
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteDocument_KeepOrphansFile(t *testing.T) {
	h, storageService := setupFileFieldHandlers(t)

	fileID := createTestFile(t, storageService, "documents", "doc.pdf", "fake-pdf-data")

	body := bytes.NewBufferString(`{"title":"Test Post","attachment":"` + fileID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/posts", body)
	req.SetPathValue("collection", "posts")
	w := httptest.NewRecorder()
	h.CreateDocument(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create document: %s", w.Body.String())
	}

	var createResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	postID := createResp["id"].(string)

	_, err := storageService.GetMetadata(context.Background(), "documents", fileID)
	if err != nil {
		t.Fatalf("file should exist before delete: %v", err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/collections/posts/"+postID, nil)
	req.SetPathValue("collection", "posts")
	req.SetPathValue("id", postID)
	w = httptest.NewRecorder()

	h.DeleteDocument(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status %d, got %d: %s", http.StatusNoContent, w.Code, w.Body.String())
	}

	_, err = storageService.GetMetadata(context.Background(), "documents", fileID)
	if err != nil {
		t.Errorf("file should still exist after keep delete: %v", err)
	}
}

func TestUpdateDocument_ChangesFileField(t *testing.T) {
	h, storageService := setupFileFieldHandlers(t)

	pngHeader := testPNGHeader
	oldFileID := createTestFile(t, storageService, "avatars", "old.png", pngHeader+"old-data")
	newFileID := createTestFile(t, storageService, "avatars", "new.png", pngHeader+"new-data")

	body := bytes.NewBufferString(`{"name":"Frank","email":"frank@example.com","avatar":"` + oldFileID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/users", body)
	req.SetPathValue("collection", "users")
	w := httptest.NewRecorder()
	h.CreateDocument(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create document: %s", w.Body.String())
	}

	var createResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	userID := createResp["id"].(string)

	body = bytes.NewBufferString(`{"avatar":"` + newFileID + `"}`)
	req = httptest.NewRequest(http.MethodPatch, "/api/collections/users/"+userID, body)
	req.SetPathValue("collection", "users")
	req.SetPathValue("id", userID)
	w = httptest.NewRecorder()

	h.UpdateDocument(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	_, err := storageService.GetMetadata(context.Background(), "avatars", oldFileID)
	if err == nil {
		t.Error("old file should be deleted after update with cascade")
	}
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	_, err = storageService.GetMetadata(context.Background(), "avatars", newFileID)
	if err != nil {
		t.Errorf("new file should exist: %v", err)
	}
}

func TestUpdateDocument_KeepsOldFileOnKeepDelete(t *testing.T) {
	h, storageService := setupFileFieldHandlers(t)

	oldFileID := createTestFile(t, storageService, "documents", "old.pdf", "old-data")
	newFileID := createTestFile(t, storageService, "documents", "new.pdf", "new-data")

	body := bytes.NewBufferString(`{"title":"Test Post","attachment":"` + oldFileID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/posts", body)
	req.SetPathValue("collection", "posts")
	w := httptest.NewRecorder()
	h.CreateDocument(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create document: %s", w.Body.String())
	}

	var createResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	postID := createResp["id"].(string)

	body = bytes.NewBufferString(`{"attachment":"` + newFileID + `"}`)
	req = httptest.NewRequest(http.MethodPatch, "/api/collections/posts/"+postID, body)
	req.SetPathValue("collection", "posts")
	req.SetPathValue("id", postID)
	w = httptest.NewRecorder()

	h.UpdateDocument(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	_, err := storageService.GetMetadata(context.Background(), "documents", oldFileID)
	if err != nil {
		t.Errorf("old file should still exist with keep on_delete: %v", err)
	}

	_, err = storageService.GetMetadata(context.Background(), "documents", newFileID)
	if err != nil {
		t.Errorf("new file should exist: %v", err)
	}
}
