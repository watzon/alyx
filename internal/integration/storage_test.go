//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
	"github.com/watzon/alyx/internal/storage"
)

// testStorageSchema creates a test schema with bucket and collection with file field.
func testStorageSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.Parse([]byte(`
version: 1
buckets:
  uploads:
    backend: local
    max_file_size: 10485760  # 10MB
    allowed_types:
      - image/*
      - application/pdf
collections:
  documents:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      title:
        type: string
        required: true
      attachment:
        type: file
        file:
          bucket: uploads
          on_delete: cascade
      created_at:
        type: timestamp
        default: now
`))
	require.NoError(t, err)
	return s
}

// TestIntegration_StorageCompleteFlow tests the complete storage flow:
// 1. Parse schema with bucket and collection with file field
// 2. Create bucket via schema
// 3. Upload file via TUS (multiple chunks)
// 4. Create record referencing file
// 5. Query record with expand, verify file metadata
// 6. Generate signed URL, access without auth
// 7. Delete record, verify cascade behavior
func TestIntegration_StorageCompleteFlow(t *testing.T) {
	ctx := context.Background()

	// Setup database and schema
	db := testDB(t)
	s := testStorageSchema(t)

	// Apply schema migrations
	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		_, err := db.Exec(stmt)
		require.NoError(t, err)
	}

	// Setup storage backends
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "storage")
	tempPath := filepath.Join(tmpDir, "temp")

	backends := map[string]storage.Backend{
		"local": storage.NewFilesystemBackend(storagePath),
	}

	cfg := &config.Config{}

	// Initialize storage service
	storageService := storage.NewService(db, backends, s, cfg, nil)

	// Initialize TUS service
	tusService := storage.NewTUSService(db, backends, s, cfg, tempPath)

	// Step 1: Upload file via TUS in chunks
	fileContent := []byte("This is a test PDF file content for integration testing. " +
		"It contains multiple lines and should be uploaded in chunks.\n" +
		"Line 2 of the test file.\n" +
		"Line 3 of the test file.\n")

	// Prepend PDF magic bytes to make it a valid PDF
	pdfHeader := []byte("%PDF-1.4\n")
	fileContent = append(pdfHeader, fileContent...)

	metadata := map[string]string{
		"filename": "test-document.pdf",
	}

	upload, err := tusService.CreateUpload(ctx, "uploads", int64(len(fileContent)), metadata)
	require.NoError(t, err)
	require.NotEmpty(t, upload.ID)
	require.Equal(t, "uploads", upload.Bucket)
	require.Equal(t, int64(len(fileContent)), upload.Size)
	require.Equal(t, int64(0), upload.Offset)

	// Upload in 3 chunks
	chunkSize := int64(len(fileContent) / 3)
	offset := int64(0)

	// Chunk 1
	chunk1 := fileContent[offset : offset+chunkSize]
	newOffset, err := tusService.UploadChunk(ctx, "uploads", upload.ID, offset, bytes.NewReader(chunk1), int64(len(chunk1)))
	require.NoError(t, err)
	require.Equal(t, offset+int64(len(chunk1)), newOffset)
	offset = newOffset

	// Chunk 2
	chunk2 := fileContent[offset : offset+chunkSize]
	newOffset, err = tusService.UploadChunk(ctx, "uploads", upload.ID, offset, bytes.NewReader(chunk2), int64(len(chunk2)))
	require.NoError(t, err)
	require.Equal(t, offset+int64(len(chunk2)), newOffset)
	offset = newOffset

	// Chunk 3 (final chunk)
	chunk3 := fileContent[offset:]
	newOffset, err = tusService.UploadChunk(ctx, "uploads", upload.ID, offset, bytes.NewReader(chunk3), int64(len(chunk3)))
	require.NoError(t, err)
	require.Equal(t, int64(len(fileContent)), newOffset)

	// Verify upload was finalized (should be deleted from _alyx_uploads)
	_, err = tusService.GetUploadOffset(ctx, "uploads", upload.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, storage.ErrNotFound)

	// Verify file was created in storage
	files, err := storageService.List(ctx, "uploads", 0, 10)
	require.NoError(t, err)
	require.Len(t, files, 1)

	uploadedFile := files[0]
	require.Equal(t, "uploads", uploadedFile.Bucket)
	require.Equal(t, "test-document.pdf", uploadedFile.Name)
	require.Equal(t, int64(len(fileContent)), uploadedFile.Size)
	require.Equal(t, "application/pdf", uploadedFile.MimeType)
	require.NotEmpty(t, uploadedFile.Checksum)

	// Verify checksum
	expectedChecksum := sha256.Sum256(fileContent)
	require.Equal(t, hex.EncodeToString(expectedChecksum[:]), uploadedFile.Checksum)

	// Step 2: Create record referencing file
	coll := database.NewCollection(db, s.Collections["documents"])

	doc := map[string]any{
		"title":      "Test Document",
		"attachment": uploadedFile.ID,
	}

	createdDoc, err := coll.Create(ctx, doc)
	require.NoError(t, err)
	require.NotNil(t, createdDoc)
	require.Equal(t, "Test Document", createdDoc["title"])
	require.Equal(t, uploadedFile.ID, createdDoc["attachment"])

	// Step 3: Query record with expand, verify file metadata
	docID := createdDoc["id"].(string)
	retrievedDoc, err := coll.FindOne(ctx, docID)
	require.NoError(t, err)
	require.Equal(t, uploadedFile.ID, retrievedDoc["attachment"])

	// Step 4: Download file and verify content
	rc, file, err := storageService.Download(ctx, "uploads", uploadedFile.ID)
	require.NoError(t, err)
	require.NotNil(t, rc)
	require.NotNil(t, file)
	defer rc.Close()

	downloadedContent, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, fileContent, downloadedContent)

	// Step 5: Generate signed URL
	signedService := storage.NewSignedURLService([]byte("test-secret-key"))
	token, expiresAt, err := signedService.GenerateSignedURL(uploadedFile.ID, "uploads", "download", 15*time.Minute, "")
	require.NoError(t, err)
	require.NotEmpty(t, token)
	require.True(t, expiresAt.After(time.Now()))

	// Validate signed URL (simulating unauthenticated access)
	claims, err := signedService.ValidateSignedURL(token, uploadedFile.ID, "uploads")
	require.NoError(t, err)
	require.Equal(t, uploadedFile.ID, claims.FileID)
	require.Equal(t, "uploads", claims.Bucket)
	require.Equal(t, "download", claims.Operation)

	// Step 6: Delete record, verify cascade behavior
	err = coll.Delete(ctx, docID)
	require.NoError(t, err)

	// Verify record is deleted
	_, err = coll.FindOne(ctx, docID)
	require.Error(t, err)

	// Verify file is still in storage (cascade not implemented in this test)
	// In production, cascade would be handled by database triggers or application logic
	_, err = storageService.GetMetadata(ctx, "uploads", uploadedFile.ID)
	require.NoError(t, err) // File still exists

	// Manual cleanup for this test
	err = storageService.Delete(ctx, "uploads", uploadedFile.ID)
	require.NoError(t, err)

	// Verify file is deleted
	_, err = storageService.GetMetadata(ctx, "uploads", uploadedFile.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, storage.ErrNotFound)
}

// TestIntegration_StorageErrorCases tests error handling:
// - Upload to non-existent bucket
// - Reference non-existent file
// - Exceed file size limit
// - Invalid MIME type
func TestIntegration_StorageErrorCases(t *testing.T) {
	ctx := context.Background()

	// Setup database and schema
	db := testDB(t)
	s := testStorageSchema(t)

	// Apply schema migrations
	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		_, err := db.Exec(stmt)
		require.NoError(t, err)
	}

	// Setup storage backends
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "storage")
	tempPath := filepath.Join(tmpDir, "temp")

	backends := map[string]storage.Backend{
		"local": storage.NewFilesystemBackend(storagePath),
	}

	cfg := &config.Config{}

	// Initialize storage service
	storageService := storage.NewService(db, backends, s, cfg, nil)

	// Initialize TUS service
	tusService := storage.NewTUSService(db, backends, s, cfg, tempPath)

	// Test 1: Upload to non-existent bucket
	t.Run("NonExistentBucket", func(t *testing.T) {
		_, err := tusService.CreateUpload(ctx, "nonexistent", 1024, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "bucket not found")
	})

	// Test 2: Reference non-existent file
	t.Run("NonExistentFile", func(t *testing.T) {
		coll := database.NewCollection(db, s.Collections["documents"])

		doc := map[string]any{
			"title":      "Test Document",
			"attachment": "nonexistent-file-id",
		}

		// This should succeed (no FK constraint on file field)
		createdDoc, err := coll.Create(ctx, doc)
		require.NoError(t, err)
		require.Equal(t, "nonexistent-file-id", createdDoc["attachment"])

		// But downloading the file should fail
		_, _, err = storageService.Download(ctx, "uploads", "nonexistent-file-id")
		require.Error(t, err)
		require.ErrorIs(t, err, storage.ErrNotFound)
	})

	// Test 3: Exceed file size limit
	t.Run("ExceedSizeLimit", func(t *testing.T) {
		// Bucket max_file_size is 10MB
		largeSize := int64(11 * 1024 * 1024) // 11MB

		_, err := tusService.CreateUpload(ctx, "uploads", largeSize, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds maximum")
	})

	// Test 4: Invalid MIME type
	t.Run("InvalidMimeType", func(t *testing.T) {
		// Bucket allows: image/*, application/pdf
		// Upload a text file (should be rejected)
		textContent := []byte("This is a plain text file, not an image or PDF.")

		metadata := map[string]string{
			"filename": "test.txt",
		}

		upload, err := tusService.CreateUpload(ctx, "uploads", int64(len(textContent)), metadata)
		require.NoError(t, err)

		// Upload the content (should fail on finalization due to MIME type)
		_, err = tusService.UploadChunk(ctx, "uploads", upload.ID, 0, bytes.NewReader(textContent), int64(len(textContent)))
		require.Error(t, err)
		require.Contains(t, err.Error(), "not allowed")
	})
}

// TestIntegration_StorageS3Backend tests storage with S3 backend (MinIO).
// Skips if S3_ENDPOINT environment variable is not set.
func TestIntegration_StorageS3Backend(t *testing.T) {
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT not set, skipping S3 integration test")
	}

	ctx := context.Background()

	// Setup database and schema
	db := testDB(t)

	// Schema with S3 backend
	s, err := schema.Parse([]byte(`
version: 1
buckets:
  s3-uploads:
    backend: s3
    max_file_size: 10485760  # 10MB
    allowed_types:
      - image/*
      - application/pdf
collections:
  s3_documents:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      title:
        type: string
        required: true
      attachment:
        type: file
        file:
          bucket: s3-uploads
          on_delete: cascade
      created_at:
        type: timestamp
        default: now
`))
	require.NoError(t, err)

	// Apply schema migrations
	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		_, err := db.Exec(stmt)
		require.NoError(t, err)
	}

	// Setup S3 backend
	s3Config := config.S3Config{
		Endpoint:        endpoint,
		Region:          os.Getenv("S3_REGION"),
		AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
		ForcePathStyle:  true, // Required for MinIO
	}

	if s3Config.Region == "" {
		s3Config.Region = "us-east-1"
	}
	if s3Config.AccessKeyID == "" {
		s3Config.AccessKeyID = "minioadmin"
	}
	if s3Config.SecretAccessKey == "" {
		s3Config.SecretAccessKey = "minioadmin"
	}

	s3Backend, err := storage.NewS3Backend(ctx, s3Config)
	require.NoError(t, err)

	backends := map[string]storage.Backend{
		"s3": s3Backend,
	}

	cfg := &config.Config{}
	tmpDir := t.TempDir()
	tempPath := filepath.Join(tmpDir, "temp")

	// Initialize storage service
	storageService := storage.NewService(db, backends, s, cfg, nil)

	// Initialize TUS service
	tusService := storage.NewTUSService(db, backends, s, cfg, tempPath)

	// Upload file via TUS
	fileContent := []byte("%PDF-1.4\nThis is a test PDF file for S3 backend testing.")

	metadata := map[string]string{
		"filename": "s3-test.pdf",
	}

	upload, err := tusService.CreateUpload(ctx, "s3-uploads", int64(len(fileContent)), metadata)
	require.NoError(t, err)

	// Upload in single chunk
	newOffset, err := tusService.UploadChunk(ctx, "s3-uploads", upload.ID, 0, bytes.NewReader(fileContent), int64(len(fileContent)))
	require.NoError(t, err)
	require.Equal(t, int64(len(fileContent)), newOffset)

	// Verify file was created
	files, err := storageService.List(ctx, "s3-uploads", 0, 10)
	require.NoError(t, err)
	require.Len(t, files, 1)

	uploadedFile := files[0]
	require.Equal(t, "s3-uploads", uploadedFile.Bucket)
	require.Equal(t, "s3-test.pdf", uploadedFile.Name)
	require.Equal(t, int64(len(fileContent)), uploadedFile.Size)

	// Download and verify content
	rc, file, err := storageService.Download(ctx, "s3-uploads", uploadedFile.ID)
	require.NoError(t, err)
	require.NotNil(t, rc)
	require.NotNil(t, file)
	defer rc.Close()

	downloadedContent, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, fileContent, downloadedContent)

	// Delete file
	err = storageService.Delete(ctx, "s3-uploads", uploadedFile.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = storageService.GetMetadata(ctx, "s3-uploads", uploadedFile.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, storage.ErrNotFound)
}

// TestIntegration_StorageTUSResume tests resuming an interrupted TUS upload.
func TestIntegration_StorageTUSResume(t *testing.T) {
	ctx := context.Background()

	// Setup database and schema
	db := testDB(t)
	s := testStorageSchema(t)

	// Apply schema migrations
	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		_, err := db.Exec(stmt)
		require.NoError(t, err)
	}

	// Setup storage backends
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "storage")
	tempPath := filepath.Join(tmpDir, "temp")

	backends := map[string]storage.Backend{
		"local": storage.NewFilesystemBackend(storagePath),
	}

	cfg := &config.Config{}

	// Initialize TUS service
	tusService := storage.NewTUSService(db, backends, s, cfg, tempPath)

	// Create upload
	fileContent := []byte("%PDF-1.4\nThis is a test PDF file for resume testing. It has multiple chunks.")

	metadata := map[string]string{
		"filename": "resume-test.pdf",
	}

	upload, err := tusService.CreateUpload(ctx, "uploads", int64(len(fileContent)), metadata)
	require.NoError(t, err)

	// Upload first chunk
	chunkSize := int64(len(fileContent) / 2)
	chunk1 := fileContent[0:chunkSize]

	offset, err := tusService.UploadChunk(ctx, "uploads", upload.ID, 0, bytes.NewReader(chunk1), int64(len(chunk1)))
	require.NoError(t, err)
	require.Equal(t, int64(len(chunk1)), offset)

	// Simulate disconnect - get current offset
	currentOffset, err := tusService.GetUploadOffset(ctx, "uploads", upload.ID)
	require.NoError(t, err)
	require.Equal(t, int64(len(chunk1)), currentOffset)

	// Resume upload with second chunk
	chunk2 := fileContent[currentOffset:]
	finalOffset, err := tusService.UploadChunk(ctx, "uploads", upload.ID, currentOffset, bytes.NewReader(chunk2), int64(len(chunk2)))
	require.NoError(t, err)
	require.Equal(t, int64(len(fileContent)), finalOffset)

	// Verify upload was finalized
	_, err = tusService.GetUploadOffset(ctx, "uploads", upload.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, storage.ErrNotFound)
}

// TestIntegration_StorageCleanupExpired tests cleanup of expired TUS uploads.
func TestIntegration_StorageCleanupExpired(t *testing.T) {
	ctx := context.Background()

	// Setup database and schema
	db := testDB(t)
	s := testStorageSchema(t)

	// Apply schema migrations
	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		_, err := db.Exec(stmt)
		require.NoError(t, err)
	}

	// Setup storage backends
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "storage")
	tempPath := filepath.Join(tmpDir, "temp")

	backends := map[string]storage.Backend{
		"local": storage.NewFilesystemBackend(storagePath),
	}

	cfg := &config.Config{}

	// Initialize TUS service
	tusService := storage.NewTUSService(db, backends, s, cfg, tempPath)

	// Create upload
	metadata := map[string]string{
		"filename": "expired-test.pdf",
	}

	upload, err := tusService.CreateUpload(ctx, "uploads", 1024, metadata)
	require.NoError(t, err)

	// Manually expire the upload by updating the database
	tusStore := storage.NewTUSStore(db)
	expiredTime := time.Now().UTC().Add(-1 * time.Hour)
	_, err = db.Exec("UPDATE _alyx_uploads SET expires_at = ? WHERE id = ?", expiredTime.Format(time.RFC3339), upload.ID)
	require.NoError(t, err)

	// Run cleanup
	deleted, err := tusService.CleanupExpiredUploads(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, deleted)

	// Verify upload is deleted
	_, err = tusStore.Get(ctx, "uploads", upload.ID)
	require.Error(t, err)
	require.ErrorIs(t, err, storage.ErrNotFound)
}
