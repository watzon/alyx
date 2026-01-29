package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/auth"
	"github.com/watzon/alyx/internal/storage"
)

type FileHandlers struct {
	service       *storage.Service
	tusService    *storage.TUSService
	signedService *storage.SignedURLService
}

func NewFileHandlers(service *storage.Service, tusService *storage.TUSService, signedService *storage.SignedURLService) *FileHandlers {
	return &FileHandlers{
		service:       service,
		tusService:    tusService,
		signedService: signedService,
	}
}

func (h *FileHandlers) Upload(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if bucket == "" {
		Error(w, http.StatusBadRequest, "BUCKET_REQUIRED", "Bucket name is required")
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		Error(w, http.StatusBadRequest, "INVALID_FORM", "Invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusBadRequest, "FILE_REQUIRED", "File is required")
		return
	}
	defer file.Close()

	// Validate file type
	if err := ValidateFileUpload(header); err != nil {
		Error(w, http.StatusBadRequest, "INVALID_FILE_TYPE", err.Error())
		return
	}

	uploaded, err := h.service.Upload(r.Context(), bucket, header.Filename, file, header.Size)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			Error(w, http.StatusNotFound, "BUCKET_NOT_FOUND", "Bucket not found")
			return
		}
		log.Error().Err(err).Str("bucket", bucket).Str("filename", header.Filename).Msg("Failed to upload file")
		Error(w, http.StatusInternalServerError, "UPLOAD_ERROR", "Failed to upload file")
		return
	}

	JSON(w, http.StatusCreated, uploaded)
}

// AllowedMIMETypes for file uploads
var AllowedMIMETypes = map[string]bool{
	"image/jpeg":       true,
	"image/png":        true,
	"image/gif":        true,
	"image/webp":       true,
	"application/pdf":  true,
	"text/plain":       true,
	"application/json": true,
}

// ValidateFileUpload checks MIME type and magic bytes of uploaded file.
func ValidateFileUpload(fh *multipart.FileHeader) error {
	// Check MIME type from header
	contentType := fh.Header.Get("Content-Type")
	if !AllowedMIMETypes[contentType] {
		return fmt.Errorf("unsupported file type: %s", contentType)
	}

	// Open file to check magic bytes
	file, err := fh.Open()
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read first 512 bytes for magic byte detection
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Detect actual content type from magic bytes
	detectedType := http.DetectContentType(buffer[:n])

	// Verify detected type matches declared type (allowing for charset variations in text types)
	if detectedType != contentType {
		// Special case: text/plain may be detected as "text/plain; charset=utf-8"
		// We consider this a match if the base type is the same
		if strings.HasPrefix(detectedType, contentType) {
			return nil
		}
		return fmt.Errorf("file type mismatch: declared %s, detected %s", contentType, detectedType)
	}

	return nil
}

func (h *FileHandlers) List(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if bucket == "" {
		Error(w, http.StatusBadRequest, "BUCKET_REQUIRED", "Bucket name is required")
		return
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		var err error
		offset, err = strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			Error(w, http.StatusBadRequest, "INVALID_OFFSET", "Invalid offset parameter")
			return
		}
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			Error(w, http.StatusBadRequest, "INVALID_LIMIT", "Invalid limit parameter")
			return
		}
		if limit > 1000 {
			limit = 1000
		}
	}

	search := r.URL.Query().Get("search")
	mimeType := r.URL.Query().Get("mime_type")

	files, total, err := h.service.List(r.Context(), bucket, search, mimeType, offset, limit)
	if err != nil {
		log.Error().Err(err).Str("bucket", bucket).Msg("Failed to list files")
		Error(w, http.StatusInternalServerError, "LIST_ERROR", "Failed to list files")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"files":  files,
		"total":  total,
		"offset": offset,
		"limit":  limit,
	})
}

func (h *FileHandlers) GetMetadata(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	fileID := r.PathValue("id")

	if bucket == "" {
		Error(w, http.StatusBadRequest, "BUCKET_REQUIRED", "Bucket name is required")
		return
	}
	if fileID == "" {
		Error(w, http.StatusBadRequest, "FILE_ID_REQUIRED", "File ID is required")
		return
	}

	file, err := h.service.GetMetadata(r.Context(), bucket, fileID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			Error(w, http.StatusNotFound, "FILE_NOT_FOUND", "File not found")
			return
		}
		log.Error().Err(err).Str("bucket", bucket).Str("file_id", fileID).Msg("Failed to get file metadata")
		Error(w, http.StatusInternalServerError, "METADATA_ERROR", "Failed to get file metadata")
		return
	}

	JSON(w, http.StatusOK, file)
}

func (h *FileHandlers) Sign(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	fileID := r.PathValue("id")

	if bucket == "" {
		Error(w, http.StatusBadRequest, "BUCKET_REQUIRED", "Bucket name is required")
		return
	}
	if fileID == "" {
		Error(w, http.StatusBadRequest, "FILE_ID_REQUIRED", "File ID is required")
		return
	}

	expiryStr := r.URL.Query().Get("expiry")
	if expiryStr == "" {
		expiryStr = "15m"
	}

	expiry, err := time.ParseDuration(expiryStr)
	if err != nil {
		Error(w, http.StatusBadRequest, "INVALID_EXPIRY", "Invalid expiry duration")
		return
	}

	operation := r.URL.Query().Get("operation")
	if operation == "" {
		operation = "download"
	}

	if operation != "download" && operation != "view" {
		Error(w, http.StatusBadRequest, "INVALID_OPERATION", "Operation must be 'download' or 'view'")
		return
	}

	_, err = h.service.GetMetadata(r.Context(), bucket, fileID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			Error(w, http.StatusNotFound, "FILE_NOT_FOUND", "File not found")
			return
		}
		log.Error().Err(err).Str("bucket", bucket).Str("file_id", fileID).Msg("Failed to get file metadata")
		Error(w, http.StatusInternalServerError, "METADATA_ERROR", "Failed to get file metadata")
		return
	}

	userID := ""
	if claims, ok := r.Context().Value("claims").(*auth.Claims); ok {
		userID = claims.UserID
	}

	token, expiresAt, err := h.signedService.GenerateSignedURL(fileID, bucket, operation, expiry, userID)
	if err != nil {
		log.Error().Err(err).Str("bucket", bucket).Str("file_id", fileID).Msg("Failed to generate signed URL")
		Error(w, http.StatusInternalServerError, "SIGN_ERROR", "Failed to generate signed URL")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"url":        r.URL.Scheme + "://" + r.Host + "/api/files/" + bucket + "/" + fileID + "/" + operation + "?token=" + token,
		"token":      token,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

func (h *FileHandlers) Download(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	fileID := r.PathValue("id")

	if bucket == "" {
		Error(w, http.StatusBadRequest, "BUCKET_REQUIRED", "Bucket name is required")
		return
	}
	if fileID == "" {
		Error(w, http.StatusBadRequest, "FILE_ID_REQUIRED", "File ID is required")
		return
	}

	token := r.URL.Query().Get("token")
	if token != "" {
		if err := h.validateToken(token, fileID, bucket, "download"); err != nil {
			if errors.Is(err, storage.ErrExpiredToken) {
				Error(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "Signed URL has expired")
				return
			}
			Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid or tampered token")
			return
		}
	}

	rc, file, err := h.service.Download(r.Context(), bucket, fileID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			Error(w, http.StatusNotFound, "FILE_NOT_FOUND", "File not found")
			return
		}
		log.Error().Err(err).Str("bucket", bucket).Str("file_id", fileID).Msg("Failed to download file")
		Error(w, http.StatusInternalServerError, "DOWNLOAD_ERROR", "Failed to download file")
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))
	w.Header().Set("Content-Disposition", "attachment; filename=\""+file.Name+"\"")

	if _, err := io.Copy(w, rc); err != nil {
		log.Error().Err(err).Str("bucket", bucket).Str("file_id", fileID).Msg("Failed to stream file")
	}
}

func (h *FileHandlers) View(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	fileID := r.PathValue("id")

	if bucket == "" {
		Error(w, http.StatusBadRequest, "BUCKET_REQUIRED", "Bucket name is required")
		return
	}
	if fileID == "" {
		Error(w, http.StatusBadRequest, "FILE_ID_REQUIRED", "File ID is required")
		return
	}

	token := r.URL.Query().Get("token")
	if token != "" {
		if err := h.validateToken(token, fileID, bucket, "view"); err != nil {
			if errors.Is(err, storage.ErrExpiredToken) {
				Error(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "Signed URL has expired")
				return
			}
			Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid or tampered token")
			return
		}
	}

	rc, file, err := h.service.Download(r.Context(), bucket, fileID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			Error(w, http.StatusNotFound, "FILE_NOT_FOUND", "File not found")
			return
		}
		log.Error().Err(err).Str("bucket", bucket).Str("file_id", fileID).Msg("Failed to view file")
		Error(w, http.StatusInternalServerError, "VIEW_ERROR", "Failed to view file")
		return
	}
	defer rc.Close()

	w.Header().Set("Content-Type", file.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))

	if _, err := io.Copy(w, rc); err != nil {
		log.Error().Err(err).Str("bucket", bucket).Str("file_id", fileID).Msg("Failed to stream file")
	}
}

func (h *FileHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	fileID := r.PathValue("id")

	if bucket == "" {
		Error(w, http.StatusBadRequest, "BUCKET_REQUIRED", "Bucket name is required")
		return
	}
	if fileID == "" {
		Error(w, http.StatusBadRequest, "FILE_ID_REQUIRED", "File ID is required")
		return
	}

	err := h.service.Delete(r.Context(), bucket, fileID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			Error(w, http.StatusNotFound, "FILE_NOT_FOUND", "File not found")
			return
		}
		log.Error().Err(err).Str("bucket", bucket).Str("file_id", fileID).Msg("Failed to delete file")
		Error(w, http.StatusInternalServerError, "DELETE_ERROR", "Failed to delete file")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *FileHandlers) BatchDelete(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")

	if bucket == "" {
		Error(w, http.StatusBadRequest, "BUCKET_REQUIRED", "Bucket name is required")
		return
	}

	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid request body")
		return
	}

	if len(req.IDs) == 0 {
		Error(w, http.StatusBadRequest, "IDS_REQUIRED", "At least one file ID is required")
		return
	}

	deleted := 0
	var failed []map[string]string

	for _, id := range req.IDs {
		if err := h.service.Delete(r.Context(), bucket, id); err != nil {
			failed = append(failed, map[string]string{"id": id, "error": err.Error()})
		} else {
			deleted++
		}
	}

	JSON(w, http.StatusOK, map[string]any{
		"deleted": deleted,
		"failed":  failed,
	})
}

func (h *FileHandlers) validateToken(token, fileID, bucket, operation string) error {
	claims, err := h.signedService.ValidateSignedURL(token, fileID, bucket)
	if err != nil {
		return err
	}

	if claims.Operation != operation {
		return storage.ErrInvalidSignature
	}

	return nil
}

func (h *FileHandlers) TUSCreate(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if bucket == "" {
		Error(w, http.StatusBadRequest, "BUCKET_REQUIRED", "Bucket name is required")
		return
	}

	uploadLengthStr := r.Header.Get("Upload-Length")
	if uploadLengthStr == "" {
		Error(w, http.StatusBadRequest, "UPLOAD_LENGTH_REQUIRED", "Upload-Length header is required")
		return
	}

	uploadLength, err := strconv.ParseInt(uploadLengthStr, 10, 64)
	if err != nil || uploadLength < 0 {
		Error(w, http.StatusBadRequest, "INVALID_UPLOAD_LENGTH", "Invalid Upload-Length header")
		return
	}

	metadata := storage.ParseTUSMetadata(r.Header.Get("Upload-Metadata"))

	upload, err := h.tusService.CreateUpload(r.Context(), bucket, uploadLength, metadata)
	if err != nil {
		log.Error().Err(err).Str("bucket", bucket).Int64("size", uploadLength).Msg("Failed to create TUS upload")
		Error(w, http.StatusInternalServerError, "CREATE_UPLOAD_ERROR", "Failed to create upload")
		return
	}

	uploadURL := r.URL.Scheme + "://" + r.Host + "/api/files/" + bucket + "/tus/" + upload.ID
	if r.URL.Scheme == "" {
		uploadURL = "http://" + r.Host + "/api/files/" + bucket + "/tus/" + upload.ID
	}

	w.Header().Set("Location", uploadURL)
	w.Header().Set("Tus-Resumable", storage.TUSResumableSupported)
	w.WriteHeader(http.StatusCreated)
}

func (h *FileHandlers) TUSHead(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	uploadID := r.PathValue("upload_id")

	if bucket == "" {
		Error(w, http.StatusBadRequest, "BUCKET_REQUIRED", "Bucket name is required")
		return
	}
	if uploadID == "" {
		Error(w, http.StatusBadRequest, "UPLOAD_ID_REQUIRED", "Upload ID is required")
		return
	}

	offset, err := h.tusService.GetUploadOffset(r.Context(), bucket, uploadID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			Error(w, http.StatusNotFound, "UPLOAD_NOT_FOUND", "Upload not found")
			return
		}
		log.Error().Err(err).Str("bucket", bucket).Str("upload_id", uploadID).Msg("Failed to get upload offset")
		Error(w, http.StatusInternalServerError, "OFFSET_ERROR", "Failed to get upload offset")
		return
	}

	w.Header().Set("Upload-Offset", strconv.FormatInt(offset, 10))
	w.Header().Set("Tus-Resumable", storage.TUSResumableSupported)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}

func (h *FileHandlers) TUSPatch(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	uploadID := r.PathValue("upload_id")

	if bucket == "" {
		Error(w, http.StatusBadRequest, "BUCKET_REQUIRED", "Bucket name is required")
		return
	}
	if uploadID == "" {
		Error(w, http.StatusBadRequest, "UPLOAD_ID_REQUIRED", "Upload ID is required")
		return
	}

	uploadOffsetStr := r.Header.Get("Upload-Offset")
	if uploadOffsetStr == "" {
		Error(w, http.StatusBadRequest, "UPLOAD_OFFSET_REQUIRED", "Upload-Offset header is required")
		return
	}

	uploadOffset, err := strconv.ParseInt(uploadOffsetStr, 10, 64)
	if err != nil || uploadOffset < 0 {
		Error(w, http.StatusBadRequest, "INVALID_UPLOAD_OFFSET", "Invalid Upload-Offset header")
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/offset+octet-stream" {
		Error(w, http.StatusBadRequest, "INVALID_CONTENT_TYPE", "Content-Type must be application/offset+octet-stream")
		return
	}

	contentLengthStr := r.Header.Get("Content-Length")
	if contentLengthStr == "" {
		Error(w, http.StatusBadRequest, "CONTENT_LENGTH_REQUIRED", "Content-Length header is required")
		return
	}

	contentLength, err := strconv.ParseInt(contentLengthStr, 10, 64)
	if err != nil || contentLength < 0 {
		Error(w, http.StatusBadRequest, "INVALID_CONTENT_LENGTH", "Invalid Content-Length header")
		return
	}

	newOffset, err := h.tusService.UploadChunk(r.Context(), bucket, uploadID, uploadOffset, r.Body, contentLength)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			Error(w, http.StatusNotFound, "UPLOAD_NOT_FOUND", "Upload not found")
			return
		}
		log.Error().Err(err).Str("bucket", bucket).Str("upload_id", uploadID).Msg("Failed to upload chunk")
		Error(w, http.StatusInternalServerError, "UPLOAD_CHUNK_ERROR", "Failed to upload chunk")
		return
	}

	w.Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))
	w.Header().Set("Tus-Resumable", storage.TUSResumableSupported)
	w.WriteHeader(http.StatusNoContent)
}

func (h *FileHandlers) TUSDelete(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	uploadID := r.PathValue("upload_id")

	if bucket == "" {
		Error(w, http.StatusBadRequest, "BUCKET_REQUIRED", "Bucket name is required")
		return
	}
	if uploadID == "" {
		Error(w, http.StatusBadRequest, "UPLOAD_ID_REQUIRED", "Upload ID is required")
		return
	}

	err := h.tusService.CancelUpload(r.Context(), bucket, uploadID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			Error(w, http.StatusNotFound, "UPLOAD_NOT_FOUND", "Upload not found")
			return
		}
		log.Error().Err(err).Str("bucket", bucket).Str("upload_id", uploadID).Msg("Failed to cancel upload")
		Error(w, http.StatusInternalServerError, "CANCEL_ERROR", "Failed to cancel upload")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
