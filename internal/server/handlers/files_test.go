package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
	"github.com/watzon/alyx/internal/storage"
)

func testFileHandlers(t *testing.T) (*FileHandlers, *storage.Service) {
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

	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("failed to close database: %v", closeErr)
		}
	})

	storagePath := filepath.Join(tmpDir, "storage")
	backend := storage.NewFilesystemBackend(storagePath)

	backends := map[string]storage.Backend{
		"local": backend,
	}

	s := &schema.Schema{
		Buckets: map[string]*schema.Bucket{
			"uploads": {
				Name:        "uploads",
				Backend:     "local",
				MaxFileSize: 10 * 1024 * 1024,
				AllowedTypes: []string{
					"text/plain",
					"image/*",
				},
			},
		},
	}

	appCfg := &config.Config{}

	service := storage.NewService(db, backends, s, appCfg, nil)
	tusService := storage.NewTUSService(db, backends, s, appCfg, tmpDir)
	signedService := storage.NewSignedURLService([]byte("test-secret-key-for-signing"))
	handlers := NewFileHandlers(service, tusService, signedService)

	return handlers, service
}

func TestFileHandlersUpload(t *testing.T) {
	handlers, _ := testFileHandlers(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := part.Write([]byte("Hello, World!")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/files/uploads", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetPathValue("bucket", "uploads")

	w := httptest.NewRecorder()
	handlers.Upload(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusCreated)
	}

	var file storage.File
	if err := json.NewDecoder(w.Body).Decode(&file); err != nil {
		t.Fatalf("Decode response failed: %v", err)
	}

	if file.ID == "" {
		t.Error("File ID not set")
	}
	if file.Name != "test.txt" {
		t.Errorf("Name = %s, want test.txt", file.Name)
	}
}

func TestFileHandlersList(t *testing.T) {
	handlers, service := testFileHandlers(t)

	content := []byte("content")
	for i := 0; i < 3; i++ {
		filename := string(rune('a'+i)) + ".txt"
		_, err := service.Upload(httptest.NewRequest(http.MethodPost, "/", nil).Context(), "uploads", filename, bytes.NewReader(content), int64(len(content)))
		if err != nil {
			t.Fatalf("Upload failed: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/uploads", nil)
	req.SetPathValue("bucket", "uploads")

	w := httptest.NewRecorder()
	handlers.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Decode response failed: %v", err)
	}

	files, ok := response["files"].([]any)
	if !ok {
		t.Fatal("files field not found or wrong type")
	}

	if len(files) != 3 {
		t.Errorf("Files count = %d, want 3", len(files))
	}
}

func TestFileHandlersGetMetadata(t *testing.T) {
	handlers, service := testFileHandlers(t)

	content := []byte("Hello, World!")
	file, err := service.Upload(httptest.NewRequest(http.MethodPost, "/", nil).Context(), "uploads", "test.txt", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID, nil)
	req.SetPathValue("bucket", "uploads")
	req.SetPathValue("id", file.ID)

	w := httptest.NewRecorder()
	handlers.GetMetadata(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var metadata storage.File
	if err := json.NewDecoder(w.Body).Decode(&metadata); err != nil {
		t.Fatalf("Decode response failed: %v", err)
	}

	if metadata.ID != file.ID {
		t.Errorf("ID = %s, want %s", metadata.ID, file.ID)
	}
}

func TestFileHandlersDownload(t *testing.T) {
	handlers, service := testFileHandlers(t)

	content := []byte("Hello, World!")
	file, err := service.Upload(httptest.NewRequest(http.MethodPost, "/", nil).Context(), "uploads", "test.txt", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID+"/download", nil)
	req.SetPathValue("bucket", "uploads")
	req.SetPathValue("id", file.ID)

	w := httptest.NewRecorder()
	handlers.Download(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	downloaded, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("Read body failed: %v", err)
	}

	if !bytes.Equal(downloaded, content) {
		t.Errorf("Downloaded content = %q, want %q", downloaded, content)
	}

	contentDisposition := w.Header().Get("Content-Disposition")
	if contentDisposition != "attachment; filename=\"test.txt\"" {
		t.Errorf("Content-Disposition = %s, want attachment", contentDisposition)
	}
}

func TestFileHandlersView(t *testing.T) {
	handlers, service := testFileHandlers(t)

	content := []byte("Hello, World!")
	file, err := service.Upload(httptest.NewRequest(http.MethodPost, "/", nil).Context(), "uploads", "test.txt", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID+"/view", nil)
	req.SetPathValue("bucket", "uploads")
	req.SetPathValue("id", file.ID)

	w := httptest.NewRecorder()
	handlers.View(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	downloaded, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("Read body failed: %v", err)
	}

	if !bytes.Equal(downloaded, content) {
		t.Errorf("Downloaded content = %q, want %q", downloaded, content)
	}

	contentDisposition := w.Header().Get("Content-Disposition")
	if contentDisposition != "" {
		t.Errorf("Content-Disposition = %s, want empty (inline)", contentDisposition)
	}
}

func TestFileHandlersDelete(t *testing.T) {
	handlers, service := testFileHandlers(t)

	content := []byte("Hello, World!")
	file, err := service.Upload(httptest.NewRequest(http.MethodPost, "/", nil).Context(), "uploads", "test.txt", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/files/uploads/"+file.ID, nil)
	req.SetPathValue("bucket", "uploads")
	req.SetPathValue("id", file.ID)

	w := httptest.NewRecorder()
	handlers.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNoContent)
	}

	_, err = service.GetMetadata(httptest.NewRequest(http.MethodGet, "/", nil).Context(), "uploads", file.ID)
	if err != storage.ErrNotFound {
		t.Errorf("GetMetadata after Delete error = %v, want ErrNotFound", err)
	}
}

func TestFileHandlersNotFound(t *testing.T) {
	handlers, _ := testFileHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/api/files/uploads/nonexistent", nil)
	req.SetPathValue("bucket", "uploads")
	req.SetPathValue("id", "nonexistent")

	w := httptest.NewRecorder()
	handlers.GetMetadata(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestFileHandlersSign(t *testing.T) {
	handlers, service := testFileHandlers(t)

	content := []byte("Hello, World!")
	file, err := service.Upload(httptest.NewRequest(http.MethodPost, "/", nil).Context(), "uploads", "test.txt", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID+"/sign?expiry=15m&operation=download", nil)
	req.SetPathValue("bucket", "uploads")
	req.SetPathValue("id", file.ID)

	w := httptest.NewRecorder()
	handlers.Sign(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var response map[string]any
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Decode response failed: %v", err)
	}

	if response["token"] == nil {
		t.Error("Expected token in response")
	}

	if response["url"] == nil {
		t.Error("Expected url in response")
	}

	if response["expires_at"] == nil {
		t.Error("Expected expires_at in response")
	}
}

func TestFileHandlersDownloadWithToken(t *testing.T) {
	handlers, service := testFileHandlers(t)

	content := []byte("Hello, World!")
	file, err := service.Upload(httptest.NewRequest(http.MethodPost, "/", nil).Context(), "uploads", "test.txt", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	signReq := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID+"/sign?expiry=15m&operation=download", nil)
	signReq.SetPathValue("bucket", "uploads")
	signReq.SetPathValue("id", file.ID)

	signW := httptest.NewRecorder()
	handlers.Sign(signW, signReq)

	var signResponse map[string]any
	if err := json.NewDecoder(signW.Body).Decode(&signResponse); err != nil {
		t.Fatalf("Decode sign response failed: %v", err)
	}

	token := signResponse["token"].(string)

	req := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID+"/download?token="+token, nil)
	req.SetPathValue("bucket", "uploads")
	req.SetPathValue("id", file.ID)

	w := httptest.NewRecorder()
	handlers.Download(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	downloaded, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("Read body failed: %v", err)
	}

	if !bytes.Equal(downloaded, content) {
		t.Errorf("Downloaded content = %q, want %q", downloaded, content)
	}
}

func TestFileHandlersViewWithToken(t *testing.T) {
	handlers, service := testFileHandlers(t)

	content := []byte("Hello, World!")
	file, err := service.Upload(httptest.NewRequest(http.MethodPost, "/", nil).Context(), "uploads", "test.txt", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	signReq := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID+"/sign?expiry=15m&operation=view", nil)
	signReq.SetPathValue("bucket", "uploads")
	signReq.SetPathValue("id", file.ID)

	signW := httptest.NewRecorder()
	handlers.Sign(signW, signReq)

	var signResponse map[string]any
	if err := json.NewDecoder(signW.Body).Decode(&signResponse); err != nil {
		t.Fatalf("Decode sign response failed: %v", err)
	}

	token := signResponse["token"].(string)

	req := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID+"/view?token="+token, nil)
	req.SetPathValue("bucket", "uploads")
	req.SetPathValue("id", file.ID)

	w := httptest.NewRecorder()
	handlers.View(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	downloaded, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("Read body failed: %v", err)
	}

	if !bytes.Equal(downloaded, content) {
		t.Errorf("Downloaded content = %q, want %q", downloaded, content)
	}
}

func TestFileHandlersExpiredToken(t *testing.T) {
	handlers, service := testFileHandlers(t)

	content := []byte("Hello, World!")
	file, err := service.Upload(httptest.NewRequest(http.MethodPost, "/", nil).Context(), "uploads", "test.txt", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	signReq := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID+"/sign?expiry=-1s&operation=download", nil)
	signReq.SetPathValue("bucket", "uploads")
	signReq.SetPathValue("id", file.ID)

	signW := httptest.NewRecorder()
	handlers.Sign(signW, signReq)

	var signResponse map[string]any
	if err := json.NewDecoder(signW.Body).Decode(&signResponse); err != nil {
		t.Fatalf("Decode sign response failed: %v", err)
	}

	token := signResponse["token"].(string)

	req := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID+"/download?token="+token, nil)
	req.SetPathValue("bucket", "uploads")
	req.SetPathValue("id", file.ID)

	w := httptest.NewRecorder()
	handlers.Download(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestFileHandlersTamperedToken(t *testing.T) {
	handlers, service := testFileHandlers(t)

	content := []byte("Hello, World!")
	file, err := service.Upload(httptest.NewRequest(http.MethodPost, "/", nil).Context(), "uploads", "test.txt", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	signReq := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID+"/sign?expiry=15m&operation=download", nil)
	signReq.SetPathValue("bucket", "uploads")
	signReq.SetPathValue("id", file.ID)

	signW := httptest.NewRecorder()
	handlers.Sign(signW, signReq)

	var signResponse map[string]any
	if err := json.NewDecoder(signW.Body).Decode(&signResponse); err != nil {
		t.Fatalf("Decode sign response failed: %v", err)
	}

	token := signResponse["token"].(string)
	tamperedToken := token[:len(token)-5] + "XXXXX"

	req := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID+"/download?token="+tamperedToken, nil)
	req.SetPathValue("bucket", "uploads")
	req.SetPathValue("id", file.ID)

	w := httptest.NewRecorder()
	handlers.Download(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestFileHandlersDeletedFileWithValidToken(t *testing.T) {
	handlers, service := testFileHandlers(t)

	content := []byte("Hello, World!")
	file, err := service.Upload(httptest.NewRequest(http.MethodPost, "/", nil).Context(), "uploads", "test.txt", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	signReq := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID+"/sign?expiry=15m&operation=download", nil)
	signReq.SetPathValue("bucket", "uploads")
	signReq.SetPathValue("id", file.ID)

	signW := httptest.NewRecorder()
	handlers.Sign(signW, signReq)

	var signResponse map[string]any
	if err := json.NewDecoder(signW.Body).Decode(&signResponse); err != nil {
		t.Fatalf("Decode sign response failed: %v", err)
	}

	token := signResponse["token"].(string)

	if err := service.Delete(httptest.NewRequest(http.MethodDelete, "/", nil).Context(), "uploads", file.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/files/uploads/"+file.ID+"/download?token="+token, nil)
	req.SetPathValue("bucket", "uploads")
	req.SetPathValue("id", file.ID)

	w := httptest.NewRecorder()
	handlers.Download(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d (file deleted should return 404, not 403)", w.Code, http.StatusNotFound)
	}
}
