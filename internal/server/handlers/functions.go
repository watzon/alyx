package handlers

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/auth"
	"github.com/watzon/alyx/internal/functions"
)

// FunctionHandlers handles function-related endpoints.
type FunctionHandlers struct {
	service *functions.Service
}

// NewFunctionHandlers creates new function handlers.
func NewFunctionHandlers(service *functions.Service) *FunctionHandlers {
	return &FunctionHandlers{service: service}
}

// InvokeResponse is the response for function invocation.
type InvokeResponse struct {
	Success    bool                     `json:"success"`
	Output     any                      `json:"output,omitempty"`
	Error      *functions.FunctionError `json:"error,omitempty"`
	Logs       []functions.LogEntry     `json:"logs,omitempty"`
	DurationMs int64                    `json:"duration_ms"`
}

// Invoke handles POST /api/functions/:name.
func (h *FunctionHandlers) Invoke(w http.ResponseWriter, r *http.Request) {
	functionName := r.PathValue("name")
	if functionName == "" {
		Error(w, http.StatusBadRequest, "MISSING_FUNCTION_NAME", "Function name is required")
		return
	}

	// Check if function exists
	funcDef, ok := h.service.GetFunction(functionName)
	if !ok {
		Error(w, http.StatusNotFound, "FUNCTION_NOT_FOUND", "Function not found: "+functionName)
		return
	}

	var input map[string]any
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Parse multipart form (32MB max memory)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			Error(w, http.StatusBadRequest, "INVALID_MULTIPART", "Failed to parse multipart form: "+err.Error())
			return
		}

		input = make(map[string]any)

		// Extract form fields
		for key, values := range r.MultipartForm.Value {
			if len(values) == 1 {
				input[key] = values[0]
			} else {
				input[key] = values
			}
		}

		// Extract files and encode as base64
		var files []functions.FileUpload
		for fieldName, fileHeaders := range r.MultipartForm.File {
			for _, fh := range fileHeaders {
				file, err := fh.Open()
				if err != nil {
					Error(w, http.StatusBadRequest, "FILE_READ_ERROR", "Failed to read file: "+err.Error())
					return
				}
				data, err := io.ReadAll(file)
				file.Close()
				if err != nil {
					Error(w, http.StatusBadRequest, "FILE_READ_ERROR", "Failed to read file data: "+err.Error())
					return
				}

				files = append(files, functions.FileUpload{
					Name:        fieldName,
					Filename:    fh.Filename,
					ContentType: fh.Header.Get("Content-Type"),
					Size:        fh.Size,
					Data:        base64.StdEncoding.EncodeToString(data),
				})
			}
		}

		if len(files) > 0 {
			input["_files"] = files
		}
	} else if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body: "+err.Error())
			return
		}
	}

	// Build auth context from request
	var authCtx *functions.AuthContext
	if user := auth.UserFromContext(r.Context()); user != nil {
		authCtx = &functions.AuthContext{
			ID:       user.ID,
			Email:    user.Email,
			Verified: user.Verified,
		}
		if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
			authCtx.Role = claims.Role
		}
		if user.Metadata != nil {
			authCtx.Metadata = user.Metadata
		}
	}

	log.Debug().
		Str("function", functionName).
		Str("runtime", string(funcDef.Runtime)).
		Bool("authenticated", authCtx != nil).
		Msg("Invoking function")

	// Invoke function
	resp, err := h.service.Invoke(r.Context(), functionName, input, authCtx)
	if err != nil {
		log.Error().Err(err).Str("function", functionName).Msg("Function invocation failed")
		Error(w, http.StatusInternalServerError, "INVOCATION_ERROR", "Failed to invoke function: "+err.Error())
		return
	}

	// Return response
	JSON(w, http.StatusOK, InvokeResponse{
		Success:    resp.Success,
		Output:     resp.Output,
		Error:      resp.Error,
		Logs:       resp.Logs,
		DurationMs: resp.DurationMs,
	})
}

// Get handles GET /api/functions/:name.
func (h *FunctionHandlers) Get(w http.ResponseWriter, r *http.Request) {
	functionName := r.PathValue("name")
	if functionName == "" {
		Error(w, http.StatusBadRequest, "MISSING_FUNCTION_NAME", "Function name is required")
		return
	}

	funcDef, ok := h.service.GetFunction(functionName)
	if !ok {
		Error(w, http.StatusNotFound, "FUNCTION_NOT_FOUND", "Function not found: "+functionName)
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"name":         funcDef.Name,
		"runtime":      funcDef.Runtime,
		"path":         funcDef.Path,
		"entrypoint":   funcDef.Path,
		"description":  funcDef.Description,
		"sample_input": funcDef.SampleInput,
		"enabled":      true,
		"timeout":      funcDef.Timeout,
		"memory":       funcDef.Memory,
		"env":          funcDef.Env,
	})
}

// List handles GET /api/functions.
func (h *FunctionHandlers) List(w http.ResponseWriter, r *http.Request) {
	funcs := h.service.ListFunctions()

	// Build response
	result := make([]map[string]any, 0, len(funcs))
	for _, fn := range funcs {
		result = append(result, map[string]any{
			"name":    fn.Name,
			"runtime": fn.Runtime,
		})
	}

	JSON(w, http.StatusOK, map[string]any{
		"functions": result,
		"count":     len(result),
	})
}

// Stats handles GET /api/functions/stats.
func (h *FunctionHandlers) Stats(w http.ResponseWriter, r *http.Request) {
	stats := h.service.Stats()

	// Convert to JSON-friendly format
	result := make(map[string]any)
	for runtime, poolStats := range stats {
		result[string(runtime)] = map[string]any{
			"ready": poolStats.Ready,
			"busy":  poolStats.Busy,
			"total": poolStats.Total,
		}
	}

	JSON(w, http.StatusOK, map[string]any{
		"pools": result,
	})
}

// Reload handles POST /api/functions/reload (admin only).
func (h *FunctionHandlers) Reload(w http.ResponseWriter, r *http.Request) {
	if err := h.service.ReloadFunctions(); err != nil {
		log.Error().Err(err).Msg("Failed to reload functions")
		Error(w, http.StatusInternalServerError, "RELOAD_ERROR", "Failed to reload functions: "+err.Error())
		return
	}

	funcs := h.service.ListFunctions()
	JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"count":   len(funcs),
		"message": "Functions reloaded successfully",
	})
}

// Service returns the underlying function service.
func (h *FunctionHandlers) Service() *functions.Service {
	return h.service
}
