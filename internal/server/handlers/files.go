package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/storage"
)

type FileHandlers struct {
	service *storage.Service
}

func NewFileHandlers(service *storage.Service) *FileHandlers {
	return &FileHandlers{
		service: service,
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

	files, err := h.service.List(r.Context(), bucket, offset, limit)
	if err != nil {
		log.Error().Err(err).Str("bucket", bucket).Msg("Failed to list files")
		Error(w, http.StatusInternalServerError, "LIST_ERROR", "Failed to list files")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"files":  files,
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
