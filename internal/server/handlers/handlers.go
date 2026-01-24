package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/schema"
)

type HandlerFunc func(http.ResponseWriter, *http.Request)

type Handlers struct {
	db     *database.DB
	schema *schema.Schema
	cfg    *config.Config
}

func New(db *database.DB, s *schema.Schema, cfg *config.Config) *Handlers {
	return &Handlers{
		db:     db,
		schema: s,
		cfg:    cfg,
	}
}

func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": "0.1.0",
	})
}

func (h *Handlers) getCollection(name string) (*database.Collection, error) {
	col, ok := h.schema.Collections[name]
	if !ok {
		return nil, errors.New("collection not found")
	}
	return database.NewCollection(h.db, col), nil
}

func (h *Handlers) ListDocuments(w http.ResponseWriter, r *http.Request) {
	collectionName := r.PathValue("collection")

	col, err := h.getCollection(collectionName)
	if err != nil {
		Error(w, http.StatusNotFound, "COLLECTION_NOT_FOUND", "Collection not found")
		return
	}

	opts, err := parseQueryOptions(r)
	if err != nil {
		Error(w, http.StatusBadRequest, "INVALID_QUERY", err.Error())
		return
	}

	result, err := col.Find(r.Context(), opts)
	if err != nil {
		log.Error().Err(err).Str("collection", collectionName).Msg("Failed to list documents")
		Error(w, http.StatusInternalServerError, "QUERY_ERROR", "Failed to query documents")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"docs":   result.Docs,
		"total":  result.Total,
		"limit":  opts.Limit,
		"offset": opts.Offset,
	})
}

func (h *Handlers) GetDocument(w http.ResponseWriter, r *http.Request) {
	collectionName := r.PathValue("collection")
	id := r.PathValue("id")

	col, err := h.getCollection(collectionName)
	if err != nil {
		Error(w, http.StatusNotFound, "COLLECTION_NOT_FOUND", "Collection not found")
		return
	}

	doc, err := col.FindOne(r.Context(), id)
	if errors.Is(err, database.ErrNotFound) {
		Error(w, http.StatusNotFound, "DOCUMENT_NOT_FOUND", "Document not found")
		return
	}
	if err != nil {
		log.Error().Err(err).Str("collection", collectionName).Str("id", id).Msg("Failed to get document")
		Error(w, http.StatusInternalServerError, "QUERY_ERROR", "Failed to get document")
		return
	}

	JSON(w, http.StatusOK, doc)
}

func (h *Handlers) CreateDocument(w http.ResponseWriter, r *http.Request) {
	collectionName := r.PathValue("collection")

	col, err := h.getCollection(collectionName)
	if err != nil {
		Error(w, http.StatusNotFound, "COLLECTION_NOT_FOUND", "Collection not found")
		return
	}

	var data database.Row
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	if verrs := database.ValidateInput(col.Schema(), data, true); verrs.HasErrors() {
		ErrorWithDetails(w, http.StatusBadRequest, "VALIDATION_ERROR", verrs.Errors[0].Message, verrs.Errors)
		return
	}

	doc, err := col.Create(r.Context(), data)
	if err != nil {
		if ce := database.AsConstraintError(err); ce != nil {
			Error(w, http.StatusBadRequest, constraintErrorCode(ce), ce.Message)
			return
		}
		log.Error().Err(err).Str("collection", collectionName).Msg("Failed to create document")
		Error(w, http.StatusInternalServerError, "CREATE_ERROR", "Failed to create document")
		return
	}

	JSON(w, http.StatusCreated, doc)
}

func (h *Handlers) UpdateDocument(w http.ResponseWriter, r *http.Request) {
	collectionName := r.PathValue("collection")
	id := r.PathValue("id")

	col, err := h.getCollection(collectionName)
	if err != nil {
		Error(w, http.StatusNotFound, "COLLECTION_NOT_FOUND", "Collection not found")
		return
	}

	var data database.Row
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	if verrs := database.ValidateInput(col.Schema(), data, false); verrs.HasErrors() {
		ErrorWithDetails(w, http.StatusBadRequest, "VALIDATION_ERROR", verrs.Errors[0].Message, verrs.Errors)
		return
	}

	doc, err := col.Update(r.Context(), id, data)
	if errors.Is(err, database.ErrNotFound) {
		Error(w, http.StatusNotFound, "DOCUMENT_NOT_FOUND", "Document not found")
		return
	}
	if err != nil {
		if ce := database.AsConstraintError(err); ce != nil {
			Error(w, http.StatusBadRequest, constraintErrorCode(ce), ce.Message)
			return
		}
		log.Error().Err(err).Str("collection", collectionName).Str("id", id).Msg("Failed to update document")
		Error(w, http.StatusInternalServerError, "UPDATE_ERROR", "Failed to update document")
		return
	}

	JSON(w, http.StatusOK, doc)
}

func (h *Handlers) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	collectionName := r.PathValue("collection")
	id := r.PathValue("id")

	col, err := h.getCollection(collectionName)
	if err != nil {
		Error(w, http.StatusNotFound, "COLLECTION_NOT_FOUND", "Collection not found")
		return
	}

	err = col.Delete(r.Context(), id)
	if errors.Is(err, database.ErrNotFound) {
		Error(w, http.StatusNotFound, "DOCUMENT_NOT_FOUND", "Document not found")
		return
	}
	if err != nil {
		log.Error().Err(err).Str("collection", collectionName).Str("id", id).Msg("Failed to delete document")
		Error(w, http.StatusInternalServerError, "DELETE_ERROR", "Failed to delete document")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseQueryOptions(r *http.Request) (*database.QueryOptions, error) {
	opts := &database.QueryOptions{
		Limit:  100,
		Offset: 0,
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			return nil, errors.New("invalid limit parameter")
		}
		if limit > 1000 {
			limit = 1000
		}
		opts.Limit = limit
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			return nil, errors.New("invalid offset parameter")
		}
		opts.Offset = offset
	}

	for _, filterStr := range r.URL.Query()["filter"] {
		filter, err := database.ParseFilterString(filterStr)
		if err != nil {
			return nil, err
		}
		opts.Filters = append(opts.Filters, filter)
	}

	if sortStr := r.URL.Query().Get("sort"); sortStr != "" {
		for _, s := range strings.Split(sortStr, ",") {
			field, order := database.ParseSortString(strings.TrimSpace(s))
			opts.Sorts = append(opts.Sorts, &database.Sort{Field: field, Order: order})
		}
	}

	if expandStr := r.URL.Query().Get("expand"); expandStr != "" {
		opts.Expand = strings.Split(expandStr, ",")
	}

	return opts, nil
}

func constraintErrorCode(ce *database.ConstraintError) string {
	switch ce.Type {
	case "foreign_key":
		return "FOREIGN_KEY_VIOLATION"
	case "unique":
		return "UNIQUE_VIOLATION"
	case "not_null":
		return "REQUIRED_FIELD_MISSING"
	case "check":
		return "CHECK_CONSTRAINT_FAILED"
	default:
		return "CONSTRAINT_VIOLATION"
	}
}
