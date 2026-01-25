package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/webhooks"
)

// WebhookHandlers handles webhook endpoint CRUD operations.
type WebhookHandlers struct {
	store *webhooks.Store
}

// NewWebhookHandlers creates new webhook handlers.
func NewWebhookHandlers(store *webhooks.Store) *WebhookHandlers {
	return &WebhookHandlers{store: store}
}

// CreateWebhookRequest is the request body for creating a webhook endpoint.
type CreateWebhookRequest struct {
	Path         string                        `json:"path"`
	FunctionID   string                        `json:"function_id"`
	Methods      []string                      `json:"methods"`
	Verification *webhooks.WebhookVerification `json:"verification,omitempty"`
	Enabled      bool                          `json:"enabled"`
}

// UpdateWebhookRequest is the request body for updating a webhook endpoint.
type UpdateWebhookRequest struct {
	Path         *string                       `json:"path,omitempty"`
	FunctionID   *string                       `json:"function_id,omitempty"`
	Methods      *[]string                     `json:"methods,omitempty"`
	Verification *webhooks.WebhookVerification `json:"verification,omitempty"`
	Enabled      *bool                         `json:"enabled,omitempty"`
}

// List handles GET /api/webhooks.
func (h *WebhookHandlers) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	endpoints, err := h.store.List(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list webhook endpoints")
		InternalError(w, "Failed to list webhook endpoints")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"webhooks": endpoints,
		"count":    len(endpoints),
	})
}

// Get handles GET /api/webhooks/{id}.
func (h *WebhookHandlers) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Webhook ID is required")
		return
	}

	endpoint, err := h.store.Get(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get webhook endpoint")
		NotFound(w, "Webhook endpoint not found")
		return
	}

	JSON(w, http.StatusOK, endpoint)
}

// Create handles POST /api/webhooks.
func (h *WebhookHandlers) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "Invalid JSON body: "+err.Error())
		return
	}

	if req.Path == "" {
		BadRequest(w, "Path is required")
		return
	}
	if req.FunctionID == "" {
		BadRequest(w, "Function ID is required")
		return
	}
	if len(req.Methods) == 0 {
		req.Methods = []string{"POST"}
	}

	endpoint := &webhooks.WebhookEndpoint{
		ID:           uuid.New().String(),
		Path:         req.Path,
		FunctionID:   req.FunctionID,
		Methods:      req.Methods,
		Verification: req.Verification,
		Enabled:      req.Enabled,
		CreatedAt:    time.Now().UTC(),
	}

	if err := h.store.Create(ctx, endpoint); err != nil {
		log.Error().Err(err).Msg("Failed to create webhook endpoint")
		InternalError(w, "Failed to create webhook endpoint: "+err.Error())
		return
	}

	JSON(w, http.StatusCreated, endpoint)
}

// Update handles PATCH /api/webhooks/{id}.
func (h *WebhookHandlers) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Webhook ID is required")
		return
	}

	endpoint, err := h.store.Get(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get webhook endpoint")
		NotFound(w, "Webhook endpoint not found")
		return
	}

	var req UpdateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "Invalid JSON body: "+err.Error())
		return
	}

	if req.Path != nil {
		endpoint.Path = *req.Path
	}
	if req.FunctionID != nil {
		endpoint.FunctionID = *req.FunctionID
	}
	if req.Methods != nil {
		endpoint.Methods = *req.Methods
	}
	if req.Verification != nil {
		endpoint.Verification = req.Verification
	}
	if req.Enabled != nil {
		endpoint.Enabled = *req.Enabled
	}

	if err := h.store.Update(ctx, endpoint); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to update webhook endpoint")
		InternalError(w, "Failed to update webhook endpoint: "+err.Error())
		return
	}

	JSON(w, http.StatusOK, endpoint)
}

// Delete handles DELETE /api/webhooks/{id}.
func (h *WebhookHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Webhook ID is required")
		return
	}

	if err := h.store.Delete(ctx, id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to delete webhook endpoint")
		InternalError(w, "Failed to delete webhook endpoint: "+err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Webhook endpoint deleted successfully",
	})
}
