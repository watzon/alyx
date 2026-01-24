package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/watzon/alyx/internal/requestctx"
)

type ErrorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code,omitempty"`
	Details   any    `json:"details,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}
}

func errorResponse(r *http.Request, code, message string, details any) ErrorResponse {
	resp := ErrorResponse{
		Error:     message,
		Code:      code,
		Details:   details,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if r != nil {
		resp.RequestID = requestctx.RequestID(r.Context())
	}
	return resp
}

func Error(w http.ResponseWriter, status int, code string, message string) {
	JSON(w, status, ErrorResponse{
		Error:     message,
		Code:      code,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func ErrorWithRequest(w http.ResponseWriter, r *http.Request, status int, code string, message string) {
	JSON(w, status, errorResponse(r, code, message, nil))
}

func ErrorWithDetails(w http.ResponseWriter, status int, code string, message string, details any) {
	JSON(w, status, ErrorResponse{
		Error:     message,
		Code:      code,
		Details:   details,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func ErrorWithRequestAndDetails(w http.ResponseWriter, r *http.Request, status int, code string, message string, details any) {
	JSON(w, status, errorResponse(r, code, message, details))
}

func NotFound(w http.ResponseWriter, message string) {
	Error(w, http.StatusNotFound, "NOT_FOUND", message)
}

func NotFoundWithRequest(w http.ResponseWriter, r *http.Request, message string) {
	ErrorWithRequest(w, r, http.StatusNotFound, "NOT_FOUND", message)
}

func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, "BAD_REQUEST", message)
}

func BadRequestWithRequest(w http.ResponseWriter, r *http.Request, message string) {
	ErrorWithRequest(w, r, http.StatusBadRequest, "BAD_REQUEST", message)
}

func Unauthorized(w http.ResponseWriter, message string) {
	Error(w, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

func UnauthorizedWithRequest(w http.ResponseWriter, r *http.Request, message string) {
	ErrorWithRequest(w, r, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

func Forbidden(w http.ResponseWriter, message string) {
	Error(w, http.StatusForbidden, "FORBIDDEN", message)
}

func ForbiddenWithRequest(w http.ResponseWriter, r *http.Request, message string) {
	ErrorWithRequest(w, r, http.StatusForbidden, "FORBIDDEN", message)
}

func InternalError(w http.ResponseWriter, message string) {
	Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", message)
}

func InternalErrorWithRequest(w http.ResponseWriter, r *http.Request, message string) {
	ErrorWithRequest(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", message)
}
