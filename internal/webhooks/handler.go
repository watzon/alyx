package webhooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/functions"
)

// Handler handles webhook HTTP requests.
type Handler struct {
	store   *Store
	service *functions.Service
}

// NewHandler creates a new webhook handler.
func NewHandler(store *Store, service *functions.Service) *Handler {
	return &Handler{
		store:   store,
		service: service,
	}
}

// ServeHTTP handles incoming webhook requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extract webhook path from request
	path := r.URL.Path

	// Look up webhook endpoint
	endpoint, err := h.store.GetByPath(r.Context(), path)
	if err != nil {
		log.Debug().Str("path", path).Msg("Webhook endpoint not found")
		http.NotFound(w, r)
		return
	}

	// Check if method is allowed
	if !h.isMethodAllowed(endpoint, r.Method) {
		log.Debug().
			Str("path", path).
			Str("method", r.Method).
			Strs("allowed", endpoint.Methods).
			Msg("Method not allowed for webhook")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to read webhook body")
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Verify signature if configured
	verified := true
	var verificationError string

	if endpoint.Verification != nil {
		// Extract signature from headers
		headers := h.extractHeaders(r)
		signature := ExtractSignature(headers, endpoint.Verification.Header)

		// Verify signature
		result := VerifySignature(endpoint.Verification, body, signature)
		verified = result.Valid

		if !result.Valid {
			verificationError = result.Error

			log.Warn().
				Str("path", path).
				Str("method", result.Method).
				Str("error", result.Error).
				Msg("Webhook signature verification failed")

			// If skip_invalid is false, reject the request
			if !endpoint.Verification.SkipInvalid {
				http.Error(w, "Invalid signature", http.StatusUnauthorized)
				return
			}
		}
	}

	// Build event payload
	payload := map[string]any{
		"method":     r.Method,
		"path":       path,
		"headers":    h.extractHeaders(r),
		"body":       string(body),
		"query":      h.extractQuery(r),
		"verified":   verified,
		"webhook_id": endpoint.ID,
	}

	if verificationError != "" {
		payload["verification_error"] = verificationError
	}

	log.Debug().
		Str("path", path).
		Str("function", endpoint.FunctionID).
		Bool("verified", verified).
		Msg("Invoking webhook function")

	// Invoke function
	resp, err := h.service.Invoke(r.Context(), endpoint.FunctionID, payload, nil)
	if err != nil {
		log.Error().
			Err(err).
			Str("path", path).
			Str("function", endpoint.FunctionID).
			Msg("Webhook function invocation failed")
		http.Error(w, "Function invocation failed", http.StatusInternalServerError)
		return
	}

	// Handle function error
	if !resp.Success && resp.Error != nil {
		log.Error().
			Str("path", path).
			Str("function", endpoint.FunctionID).
			Str("error", resp.Error.Message).
			Msg("Webhook function returned error")

		// Return function error to caller
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]any{
			"error": resp.Error.Message,
		}); err != nil {
			log.Error().Err(err).Msg("Failed to encode error response")
		}
		return
	}

	// Return function output directly
	// The function is responsible for formatting the response
	h.writeResponse(w, resp.Output)
}

// isMethodAllowed checks if the HTTP method is allowed for the endpoint.
func (h *Handler) isMethodAllowed(endpoint *WebhookEndpoint, method string) bool {
	if len(endpoint.Methods) == 0 {
		return true // No restrictions
	}

	for _, allowed := range endpoint.Methods {
		if allowed == method {
			return true
		}
	}

	return false
}

// extractHeaders extracts all headers from the request.
func (h *Handler) extractHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string)
	for name, values := range r.Header {
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}
	return headers
}

// extractQuery extracts query parameters from the request.
func (h *Handler) extractQuery(r *http.Request) map[string]string {
	query := make(map[string]string)
	for name, values := range r.URL.Query() {
		if len(values) > 0 {
			query[name] = values[0]
		}
	}
	return query
}

// writeResponse writes the function output to the response.
func (h *Handler) writeResponse(w http.ResponseWriter, output any) {
	// If output is nil, return 200 OK with empty body
	if output == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	// If output is a map with "status" and "body" keys, use them
	if m, ok := output.(map[string]any); ok {
		if status, ok := m["status"].(float64); ok {
			w.WriteHeader(int(status))
		}

		if headers, ok := m["headers"].(map[string]any); ok {
			for k, v := range headers {
				if s, ok := v.(string); ok {
					w.Header().Set(k, s)
				}
			}
		}

		if body, ok := m["body"]; ok {
			// If body is a string, write it directly
			if s, ok := body.(string); ok {
				if _, err := w.Write([]byte(s)); err != nil {
					log.Error().Err(err).Msg("Failed to write response body")
				}
				return
			}

			// Otherwise, encode as JSON
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(body); err != nil {
				log.Error().Err(err).Msg("Failed to encode response body")
			}
			return
		}
	}

	// Default: encode output as JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(output); err != nil {
		log.Error().Err(err).Msg("Failed to encode response output")
	}
}

// RegisterRoutes registers webhook routes with the router.
func (h *Handler) RegisterRoutes(ctx context.Context, mux *http.ServeMux) error {
	// Get all webhook endpoints
	endpoints, err := h.store.List(ctx)
	if err != nil {
		return fmt.Errorf("listing webhook endpoints: %w", err)
	}

	// Register each endpoint
	for _, endpoint := range endpoints {
		if !endpoint.Enabled {
			continue
		}

		// Register for all allowed methods
		for _, method := range endpoint.Methods {
			pattern := fmt.Sprintf("%s %s", method, endpoint.Path)
			log.Debug().
				Str("pattern", pattern).
				Str("function", endpoint.FunctionID).
				Msg("Registering webhook route")
			mux.Handle(pattern, h)
		}
	}

	return nil
}
