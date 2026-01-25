package handlers

import (
	"net/http"
	"strconv"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/executions"
)

// ExecutionHandlers handles execution log endpoints.
type ExecutionHandlers struct {
	store *executions.Store
}

// NewExecutionHandlers creates new execution handlers.
func NewExecutionHandlers(store *executions.Store) *ExecutionHandlers {
	return &ExecutionHandlers{store: store}
}

// List handles GET /api/executions.
func (h *ExecutionHandlers) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filters := make(map[string]any)

	if functionID := r.URL.Query().Get("function_id"); functionID != "" {
		filters["function_id"] = functionID
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = status
	}
	if triggerType := r.URL.Query().Get("trigger_type"); triggerType != "" {
		filters["trigger_type"] = triggerType
	}
	if triggerID := r.URL.Query().Get("trigger_id"); triggerID != "" {
		filters["trigger_id"] = triggerID
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	offset := 0

	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	logs, err := h.store.List(ctx, filters, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list execution logs")
		InternalError(w, "Failed to list execution logs")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"executions": logs,
		"count":      len(logs),
		"limit":      limit,
		"offset":     offset,
	})
}

// Get handles GET /api/executions/{id}.
func (h *ExecutionHandlers) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Execution ID is required")
		return
	}

	executionLog, err := h.store.Get(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get execution log")
		NotFound(w, "Execution log not found")
		return
	}

	JSON(w, http.StatusOK, executionLog)
}

// ListForFunction handles GET /api/functions/{name}/executions.
func (h *ExecutionHandlers) ListForFunction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	functionName := r.PathValue("name")

	if functionName == "" {
		BadRequest(w, "Function name is required")
		return
	}

	filters := map[string]any{
		"function_id": functionName,
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = status
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	offset := 0

	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	if offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	logs, err := h.store.List(ctx, filters, limit, offset)
	if err != nil {
		log.Error().Err(err).Str("function", functionName).Msg("Failed to list execution logs for function")
		InternalError(w, "Failed to list execution logs for function")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"executions": logs,
		"count":      len(logs),
		"limit":      limit,
		"offset":     offset,
	})
}
