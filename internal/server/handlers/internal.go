package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/functions"
	"github.com/watzon/alyx/internal/schema"
)

// InternalHandlers handles internal API endpoints for function callbacks.
type InternalHandlers struct {
	db          *database.DB
	schema      *schema.Schema
	tokenStore  *functions.InternalTokenStore
	funcService *functions.Service
}

// NewInternalHandlers creates new internal API handlers.
func NewInternalHandlers(db *database.DB, s *schema.Schema, tokenStore *functions.InternalTokenStore, funcService *functions.Service) *InternalHandlers {
	return &InternalHandlers{
		db:          db,
		schema:      s,
		tokenStore:  tokenStore,
		funcService: funcService,
	}
}

// QueryRequest is the request body for internal query endpoint.
type QueryRequest struct {
	Collection string         `json:"collection"`
	Filter     map[string]any `json:"filter,omitempty"`
	Sort       string         `json:"sort,omitempty"`
	Limit      int            `json:"limit,omitempty"`
	Offset     int            `json:"offset,omitempty"`
}

// ExecRequest is the request body for internal exec endpoint.
type ExecRequest struct {
	Operation  string         `json:"operation"` // insert, update, delete
	Collection string         `json:"collection"`
	Data       map[string]any `json:"data,omitempty"`
	ID         string         `json:"id,omitempty"`
}

// TransactionRequest is the request body for internal transaction endpoint.
type TransactionRequest struct {
	Action string `json:"action"` // begin, commit, rollback
	TxID   string `json:"tx_id,omitempty"`
}

// Query handles POST /internal/v1/db/query.
func (h *InternalHandlers) Query(w http.ResponseWriter, r *http.Request) {
	if err := h.validateToken(r); err != nil {
		Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid internal token")
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	if req.Collection == "" {
		Error(w, http.StatusBadRequest, "MISSING_COLLECTION", "Collection name is required")
		return
	}

	h.executeQuery(w, r, req)
}

// QueryGET handles GET /internal/v1/db/query (legacy SDK compatibility).
func (h *InternalHandlers) QueryGET(w http.ResponseWriter, r *http.Request) {
	if err := h.validateToken(r); err != nil {
		Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid internal token")
		return
	}

	q := r.URL.Query()
	req := QueryRequest{
		Collection: q.Get("collection"),
		Sort:       q.Get("sort"),
	}

	if limitStr := q.Get("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil {
			req.Limit = limit
		}
	}
	if offsetStr := q.Get("offset"); offsetStr != "" {
		var offset int
		if _, err := fmt.Sscanf(offsetStr, "%d", &offset); err == nil {
			req.Offset = offset
		}
	}

	if req.Collection == "" {
		Error(w, http.StatusBadRequest, "MISSING_COLLECTION", "Collection name is required")
		return
	}

	h.executeQuery(w, r, req)
}

func (h *InternalHandlers) executeQuery(w http.ResponseWriter, r *http.Request, req QueryRequest) {
	col, ok := h.schema.Collections[req.Collection]
	if !ok {
		Error(w, http.StatusNotFound, "COLLECTION_NOT_FOUND", "Collection not found")
		return
	}

	collection := database.NewCollection(h.db, col)

	opts := &database.QueryOptions{
		Limit:  req.Limit,
		Offset: req.Offset,
	}
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	if opts.Limit > 1000 {
		opts.Limit = 1000
	}

	if req.Filter != nil {
		for field, value := range req.Filter {
			opts.Filters = append(opts.Filters, &database.Filter{
				Field: field,
				Op:    database.OpEq,
				Value: value,
			})
		}
	}

	if req.Sort != "" {
		field, order := database.ParseSortString(req.Sort)
		opts.Sorts = append(opts.Sorts, &database.Sort{Field: field, Order: order})
	}

	result, err := collection.Find(r.Context(), opts)
	if err != nil {
		log.Error().Err(err).Str("collection", req.Collection).Msg("Internal query failed")
		Error(w, http.StatusInternalServerError, "QUERY_ERROR", "Query failed")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"data":  result.Docs,
		"total": result.Total,
	})
}

func (h *InternalHandlers) Exec(w http.ResponseWriter, r *http.Request) {
	if err := h.validateToken(r); err != nil {
		Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid internal token")
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	if req.Collection == "" {
		Error(w, http.StatusBadRequest, "MISSING_COLLECTION", "Collection name is required")
		return
	}

	col, ok := h.schema.Collections[req.Collection]
	if !ok {
		Error(w, http.StatusNotFound, "COLLECTION_NOT_FOUND", "Collection not found")
		return
	}

	collection := database.NewCollection(h.db, col)

	switch req.Operation {
	case "insert":
		h.execInsert(w, r, collection, req)
	case "update":
		h.execUpdate(w, r, collection, req)
	case "delete":
		h.execDelete(w, r, collection, req)
	default:
		Error(w, http.StatusBadRequest, "INVALID_OPERATION", "Operation must be insert, update, or delete")
	}
}

func (h *InternalHandlers) execInsert(w http.ResponseWriter, r *http.Request, collection *database.Collection, req ExecRequest) {
	if req.Data == nil {
		Error(w, http.StatusBadRequest, "MISSING_DATA", "Data is required for insert")
		return
	}
	doc, err := collection.Create(r.Context(), req.Data)
	if err != nil {
		h.handleExecError(w, req.Collection, "insert", err)
		return
	}
	JSON(w, http.StatusCreated, doc)
}

func (h *InternalHandlers) execUpdate(w http.ResponseWriter, r *http.Request, collection *database.Collection, req ExecRequest) {
	if req.ID == "" {
		Error(w, http.StatusBadRequest, "MISSING_ID", "ID is required for update")
		return
	}
	if req.Data == nil {
		Error(w, http.StatusBadRequest, "MISSING_DATA", "Data is required for update")
		return
	}
	doc, err := collection.Update(r.Context(), req.ID, req.Data)
	if errors.Is(err, database.ErrNotFound) {
		Error(w, http.StatusNotFound, "DOCUMENT_NOT_FOUND", "Document not found")
		return
	}
	if err != nil {
		h.handleExecError(w, req.Collection, "update", err)
		return
	}
	JSON(w, http.StatusOK, doc)
}

func (h *InternalHandlers) execDelete(w http.ResponseWriter, r *http.Request, collection *database.Collection, req ExecRequest) {
	if req.ID == "" {
		Error(w, http.StatusBadRequest, "MISSING_ID", "ID is required for delete")
		return
	}
	err := collection.Delete(r.Context(), req.ID)
	if errors.Is(err, database.ErrNotFound) {
		Error(w, http.StatusNotFound, "DOCUMENT_NOT_FOUND", "Document not found")
		return
	}
	if err != nil {
		h.handleExecError(w, req.Collection, "delete", err)
		return
	}
	JSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// Transaction handles POST /internal/v1/db/tx.
// Note: This is a placeholder - full transaction support would require session-based TX tracking.
func (h *InternalHandlers) Transaction(w http.ResponseWriter, r *http.Request) {
	// Validate internal token
	if err := h.validateToken(r); err != nil {
		Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid internal token")
		return
	}

	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	// TODO: Implement transaction support with session-based TX tracking
	// For now, return not implemented
	Error(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Transaction support is not yet implemented")
}

// validateToken validates the internal token from the Authorization header.
func (h *InternalHandlers) validateToken(r *http.Request) error {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return errors.New("missing authorization header")
	}

	// Extract Bearer token
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return errors.New("invalid authorization header format")
	}

	token := parts[1]
	if h.tokenStore == nil {
		// If no token store, skip validation (dev mode)
		return nil
	}

	if !h.tokenStore.Validate(token) {
		return errors.New("invalid token")
	}

	return nil
}

// handleExecError handles errors from database exec operations.
func (h *InternalHandlers) handleExecError(w http.ResponseWriter, collection, operation string, err error) {
	if ce := database.AsConstraintError(err); ce != nil {
		code := "CONSTRAINT_ERROR"
		switch ce.Type {
		case "foreign_key":
			code = "FOREIGN_KEY_VIOLATION"
		case "unique":
			code = "UNIQUE_VIOLATION"
		case "not_null":
			code = "REQUIRED_FIELD_MISSING"
		}
		Error(w, http.StatusBadRequest, code, ce.Message)
		return
	}

	log.Error().Err(err).
		Str("collection", collection).
		Str("operation", operation).
		Msg("Internal exec failed")
	Error(w, http.StatusInternalServerError, "EXEC_ERROR", "Operation failed")
}
