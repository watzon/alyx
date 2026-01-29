package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/hooks"
)

type HookHandlers struct {
	registry *hooks.Registry
}

func NewHookHandlers(registry *hooks.Registry) *HookHandlers {
	return &HookHandlers{registry: registry}
}

type CreateHookRequest struct {
	Type         hooks.HookType `json:"type"`
	Source       string         `json:"source"`
	Action       string         `json:"action,omitempty"`
	FunctionName string         `json:"function_name"`
	Mode         hooks.HookMode `json:"mode"`
	Config       map[string]any `json:"config,omitempty"`
}

type UpdateHookRequest struct {
	Type         *hooks.HookType `json:"type,omitempty"`
	Source       *string         `json:"source,omitempty"`
	Action       *string         `json:"action,omitempty"`
	FunctionName *string         `json:"function_name,omitempty"`
	Mode         *hooks.HookMode `json:"mode,omitempty"`
	Config       map[string]any  `json:"config,omitempty"`
	Enabled      *bool           `json:"enabled,omitempty"`
}

func (h *HookHandlers) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	hooksList, err := h.registry.List(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list hooks")
		InternalError(w, "Failed to list hooks")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"hooks": hooksList,
		"count": len(hooksList),
	})
}

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

func (h *HookHandlers) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateHookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "Invalid JSON body: "+err.Error())
		return
	}

	if req.Type == "" {
		BadRequest(w, "Type is required")
		return
	}
	if req.Source == "" {
		BadRequest(w, "Source is required")
		return
	}
	if req.FunctionName == "" {
		BadRequest(w, "Function name is required")
		return
	}
	if req.Mode == "" {
		req.Mode = hooks.HookModeAsync
	}

	hook := &hooks.Hook{
		Type:         req.Type,
		Source:       req.Source,
		Action:       req.Action,
		FunctionName: req.FunctionName,
		Mode:         req.Mode,
		Enabled:      true,
		Config:       req.Config,
	}

	if err := h.registry.Create(ctx, hook); err != nil {
		log.Error().Err(err).Msg("Failed to create hook")
		InternalError(w, "Failed to create hook: "+err.Error())
		return
	}

	JSON(w, http.StatusCreated, hook)
}

func (h *HookHandlers) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Hook ID is required")
		return
	}

	existing, err := h.registry.Get(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get hook")
		NotFound(w, "Hook not found")
		return
	}

	var req UpdateHookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "Invalid JSON body: "+err.Error())
		return
	}

	if req.Type != nil {
		existing.Type = *req.Type
	}
	if req.Source != nil {
		existing.Source = *req.Source
	}
	if req.Action != nil {
		existing.Action = *req.Action
	}
	if req.FunctionName != nil {
		existing.FunctionName = *req.FunctionName
	}
	if req.Mode != nil {
		existing.Mode = *req.Mode
	}
	if req.Config != nil {
		existing.Config = req.Config
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := h.registry.Update(ctx, id, existing); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to update hook")
		InternalError(w, "Failed to update hook: "+err.Error())
		return
	}

	JSON(w, http.StatusOK, existing)
}

func (h *HookHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Hook ID is required")
		return
	}

	if err := h.registry.Delete(ctx, id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to delete hook")
		InternalError(w, "Failed to delete hook: "+err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Hook deleted successfully",
	})
}

func (h *HookHandlers) Enable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Hook ID is required")
		return
	}

	if err := h.registry.Enable(ctx, id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to enable hook")
		InternalError(w, "Failed to enable hook: "+err.Error())
		return
	}

	hook, err := h.registry.Get(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get hook after enable")
		InternalError(w, "Failed to get hook")
		return
	}

	JSON(w, http.StatusOK, hook)
}

func (h *HookHandlers) Disable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Hook ID is required")
		return
	}

	if err := h.registry.Disable(ctx, id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to disable hook")
		InternalError(w, "Failed to disable hook: "+err.Error())
		return
	}

	hook, err := h.registry.Get(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get hook after disable")
		InternalError(w, "Failed to get hook")
		return
	}

	JSON(w, http.StatusOK, hook)
}
