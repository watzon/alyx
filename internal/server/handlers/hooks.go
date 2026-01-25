package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/hooks"
)

// HookHandlers handles hook-related endpoints.
type HookHandlers struct {
	registry *hooks.Registry
}

// NewHookHandlers creates new hook handlers.
func NewHookHandlers(registry *hooks.Registry) *HookHandlers {
	return &HookHandlers{registry: registry}
}

// CreateHookRequest is the request body for creating a hook.
type CreateHookRequest struct {
	Name        string           `json:"name"`
	FunctionID  string           `json:"function_id"`
	EventType   string           `json:"event_type"`
	EventSource string           `json:"event_source"`
	EventAction string           `json:"event_action"`
	Mode        hooks.HookMode   `json:"mode"`
	Priority    int              `json:"priority"`
	Config      hooks.HookConfig `json:"config"`
	Enabled     bool             `json:"enabled"`
}

// UpdateHookRequest is the request body for updating a hook.
type UpdateHookRequest struct {
	Name        *string           `json:"name,omitempty"`
	EventSource *string           `json:"event_source,omitempty"`
	EventAction *string           `json:"event_action,omitempty"`
	Mode        *hooks.HookMode   `json:"mode,omitempty"`
	Priority    *int              `json:"priority,omitempty"`
	Config      *hooks.HookConfig `json:"config,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
}

// List handles GET /api/hooks.
func (h *HookHandlers) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get optional function_id filter
	functionID := r.URL.Query().Get("function_id")

	var hookList []*hooks.Hook
	var err error

	if functionID != "" {
		hookList, err = h.registry.FindByFunction(ctx, functionID)
	} else {
		hookList, err = h.registry.List(ctx)
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to list hooks")
		InternalError(w, "Failed to list hooks")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"hooks": hookList,
		"count": len(hookList),
	})
}

// Get handles GET /api/hooks/{id}.
func (h *HookHandlers) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Hook ID is required")
		return
	}

	hook, err := h.registry.Get(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get hook")
		NotFound(w, "Hook not found")
		return
	}

	JSON(w, http.StatusOK, hook)
}

// Create handles POST /api/hooks.
func (h *HookHandlers) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateHookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "Invalid JSON body: "+err.Error())
		return
	}

	// Validate required fields
	if req.Name == "" {
		BadRequest(w, "Name is required")
		return
	}
	if req.FunctionID == "" {
		BadRequest(w, "Function ID is required")
		return
	}
	if req.EventType == "" {
		BadRequest(w, "Event type is required")
		return
	}
	if req.EventSource == "" {
		BadRequest(w, "Event source is required")
		return
	}
	if req.EventAction == "" {
		BadRequest(w, "Event action is required")
		return
	}
	if req.Mode == "" {
		req.Mode = hooks.HookModeAsync // Default to async
	}

	// Set default timeout for sync hooks
	if req.Mode == hooks.HookModeSync && req.Config.Timeout == 0 {
		req.Config.Timeout = 5 * time.Second
	}

	// Create hook
	hook := &hooks.Hook{
		ID:          uuid.New().String(),
		Name:        req.Name,
		FunctionID:  req.FunctionID,
		EventType:   req.EventType,
		EventSource: req.EventSource,
		EventAction: req.EventAction,
		Mode:        req.Mode,
		Priority:    req.Priority,
		Config:      req.Config,
		Enabled:     req.Enabled,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := h.registry.Register(ctx, hook); err != nil {
		log.Error().Err(err).Msg("Failed to create hook")
		InternalError(w, "Failed to create hook: "+err.Error())
		return
	}

	JSON(w, http.StatusCreated, hook)
}

// Update handles PATCH /api/hooks/{id}.
func (h *HookHandlers) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Hook ID is required")
		return
	}

	// Get existing hook
	hook, err := h.registry.Get(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get hook")
		NotFound(w, "Hook not found")
		return
	}

	// Parse update request
	var req UpdateHookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "Invalid JSON body: "+err.Error())
		return
	}

	// Apply updates
	if req.Name != nil {
		hook.Name = *req.Name
	}
	if req.EventSource != nil {
		hook.EventSource = *req.EventSource
	}
	if req.EventAction != nil {
		hook.EventAction = *req.EventAction
	}
	if req.Mode != nil {
		hook.Mode = *req.Mode
	}
	if req.Priority != nil {
		hook.Priority = *req.Priority
	}
	if req.Config != nil {
		hook.Config = *req.Config
	}
	if req.Enabled != nil {
		hook.Enabled = *req.Enabled
	}

	hook.UpdatedAt = time.Now().UTC()

	if err := h.registry.Unregister(ctx, id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to unregister hook")
		InternalError(w, "Failed to update hook: "+err.Error())
		return
	}

	if err := h.registry.Register(ctx, hook); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to register updated hook")
		InternalError(w, "Failed to update hook: "+err.Error())
		return
	}

	JSON(w, http.StatusOK, hook)
}

// Delete handles DELETE /api/hooks/{id}.
func (h *HookHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Hook ID is required")
		return
	}

	if err := h.registry.Unregister(ctx, id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to delete hook")
		InternalError(w, "Failed to delete hook: "+err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Hook deleted successfully",
	})
}

// ListForFunction handles GET /api/functions/{name}/hooks.
func (h *HookHandlers) ListForFunction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	functionName := r.PathValue("name")

	if functionName == "" {
		BadRequest(w, "Function name is required")
		return
	}

	hookList, err := h.registry.FindByFunction(ctx, functionName)
	if err != nil {
		log.Error().Err(err).Str("function", functionName).Msg("Failed to list hooks for function")
		InternalError(w, "Failed to list hooks for function")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"hooks": hookList,
		"count": len(hookList),
	})
}
