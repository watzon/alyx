package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/auth"
	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/rules"
	"github.com/watzon/alyx/internal/schema"
)

type HandlerFunc func(http.ResponseWriter, *http.Request)

type Handlers struct {
	db          *database.DB
	schema      *schema.Schema
	cfg         *config.Config
	rules       *rules.Engine
	hookTrigger database.HookTrigger
}

func New(db *database.DB, s *schema.Schema, cfg *config.Config, rulesEngine *rules.Engine) *Handlers {
	return &Handlers{
		db:     db,
		schema: s,
		cfg:    cfg,
		rules:  rulesEngine,
	}
}

func (h *Handlers) SetHookTrigger(trigger database.HookTrigger) {
	h.hookTrigger = trigger
}

func (h *Handlers) Rules() *rules.Engine {
	return h.rules
}

func (h *Handlers) checkAccess(r *http.Request, collection string, op rules.Operation, doc map[string]any) error {
	if h.rules == nil {
		return nil
	}

	user := auth.UserFromContext(r.Context())
	claims := auth.ClaimsFromContext(r.Context())

	evalCtx := &rules.EvalContext{
		Auth:    rules.BuildAuthContext(user, claims),
		Doc:     doc,
		Request: rules.BuildRequestContext(r.Method, extractClientIP(r)),
	}

	return h.rules.CheckAccess(collection, op, evalCtx)
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip := r.RemoteAddr
	if colonIdx := strings.LastIndex(ip, ":"); colonIdx != -1 {
		ip = ip[:colonIdx]
	}
	return ip
}

func (h *Handlers) getCollection(name string) (*database.Collection, error) {
	col, ok := h.schema.Collections[name]
	if !ok {
		return nil, errors.New("collection not found")
	}
	coll := database.NewCollection(h.db, col)
	if h.hookTrigger != nil {
		coll.SetHookTrigger(h.hookTrigger)
	}
	return coll, nil
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

	if err := h.checkAccess(r, collectionName, rules.OpRead, doc); err != nil {
		if errors.Is(err, rules.ErrAccessDenied) {
			Forbidden(w, "Access denied")
			return
		}
		log.Error().Err(err).Str("collection", collectionName).Msg("Rule evaluation failed")
		InternalError(w, "Failed to check access")
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
	if decodeErr := json.NewDecoder(r.Body).Decode(&data); decodeErr != nil {
		Error(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON body")
		return
	}

	if accessErr := h.checkAccess(r, collectionName, rules.OpCreate, data); accessErr != nil {
		if errors.Is(accessErr, rules.ErrAccessDenied) {
			Forbidden(w, "Access denied")
			return
		}
		log.Error().Err(accessErr).Str("collection", collectionName).Msg("Rule evaluation failed")
		InternalError(w, "Failed to check access")
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

	existingDoc, err := col.FindOne(r.Context(), id)
	if errors.Is(err, database.ErrNotFound) {
		Error(w, http.StatusNotFound, "DOCUMENT_NOT_FOUND", "Document not found")
		return
	}
	if err != nil {
		log.Error().Err(err).Str("collection", collectionName).Str("id", id).Msg("Failed to get document for update")
		Error(w, http.StatusInternalServerError, "QUERY_ERROR", "Failed to get document")
		return
	}

	if accessErr := h.checkAccess(r, collectionName, rules.OpUpdate, existingDoc); accessErr != nil {
		if errors.Is(accessErr, rules.ErrAccessDenied) {
			Forbidden(w, "Access denied")
			return
		}
		log.Error().Err(accessErr).Str("collection", collectionName).Msg("Rule evaluation failed")
		InternalError(w, "Failed to check access")
		return
	}

	var data database.Row
	if decodeErr := json.NewDecoder(r.Body).Decode(&data); decodeErr != nil {
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

	existingDoc, err := col.FindOne(r.Context(), id)
	if errors.Is(err, database.ErrNotFound) {
		Error(w, http.StatusNotFound, "DOCUMENT_NOT_FOUND", "Document not found")
		return
	}
	if err != nil {
		log.Error().Err(err).Str("collection", collectionName).Str("id", id).Msg("Failed to get document for delete")
		Error(w, http.StatusInternalServerError, "QUERY_ERROR", "Failed to get document")
		return
	}

	if accessErr := h.checkAccess(r, collectionName, rules.OpDelete, existingDoc); accessErr != nil {
		if errors.Is(accessErr, rules.ErrAccessDenied) {
			Forbidden(w, "Access denied")
			return
		}
		log.Error().Err(accessErr).Str("collection", collectionName).Msg("Rule evaluation failed")
		InternalError(w, "Failed to check access")
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
	query := r.URL.Query()

	if err := parsePaginationOptions(query, opts); err != nil {
		return nil, err
	}

	if err := parseFilterOptions(query, opts); err != nil {
		return nil, err
	}

	parseSortAndExpandOptions(query, opts)

	opts.Search = query.Get("search")

	return opts, nil
}

func parsePaginationOptions(query map[string][]string, opts *database.QueryOptions) error {
	if limitStr := getQueryParam(query, "limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			return errors.New("invalid limit parameter")
		}
		opts.Limit = min(limit, 1000)
	}

	if perPageStr := getQueryParam(query, "perPage"); perPageStr != "" {
		perPage, err := strconv.Atoi(perPageStr)
		if err != nil || perPage < 0 {
			return errors.New("invalid perPage parameter")
		}
		opts.Limit = min(perPage, 1000)
	}

	if offsetStr := getQueryParam(query, "offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			return errors.New("invalid offset parameter")
		}
		opts.Offset = offset
	}

	if pageStr := getQueryParam(query, "page"); pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			return errors.New("invalid page parameter")
		}
		opts.Offset = (page - 1) * opts.Limit
	}

	return nil
}

func parseFilterOptions(query map[string][]string, opts *database.QueryOptions) error {
	for _, filterStr := range query["filter"] {
		filter, err := database.ParseFilterString(filterStr)
		if err != nil {
			return err
		}
		opts.Filters = append(opts.Filters, filter)
	}
	return nil
}

func parseSortAndExpandOptions(query map[string][]string, opts *database.QueryOptions) {
	if sortStr := getQueryParam(query, "sort"); sortStr != "" {
		for _, s := range strings.Split(sortStr, ",") {
			field, order := database.ParseSortString(strings.TrimSpace(s))
			opts.Sorts = append(opts.Sorts, &database.Sort{Field: field, Order: order})
		}
	}

	if expandStr := getQueryParam(query, "expand"); expandStr != "" {
		opts.Expand = strings.Split(expandStr, ",")
	}
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

// Config returns the server configuration.
func (h *Handlers) Config(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, h.cfg)
}
