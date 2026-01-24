package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/watzon/alyx/internal/server/requestlog"
)

// LogsHandlers handles request log API endpoints.
type LogsHandlers struct {
	store *requestlog.Store
}

// NewLogsHandlers creates new logs handlers.
func NewLogsHandlers(store *requestlog.Store) *LogsHandlers {
	return &LogsHandlers{store: store}
}

// List handles GET /api/admin/logs.
func (h *LogsHandlers) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	opts := requestlog.FilterOptions{
		Limit:  100,
		Offset: 0,
	}

	h.parsePaginationParams(query, &opts)
	h.parseStringFilters(query, &opts)
	h.parseStatusFilters(query, &opts)
	h.parseTimeFilters(query, &opts)

	result := h.store.List(opts)
	JSON(w, http.StatusOK, result)
}

func (h *LogsHandlers) parsePaginationParams(query map[string][]string, opts *requestlog.FilterOptions) {
	if v := getQueryParam(query, "limit"); v != "" {
		if limit, err := strconv.Atoi(v); err == nil && limit > 0 {
			opts.Limit = limit
		}
	}
	if v := getQueryParam(query, "offset"); v != "" {
		if offset, err := strconv.Atoi(v); err == nil && offset >= 0 {
			opts.Offset = offset
		}
	}
}

func (h *LogsHandlers) parseStringFilters(query map[string][]string, opts *requestlog.FilterOptions) {
	opts.Method = getQueryParam(query, "method")
	opts.Path = getQueryParam(query, "path")
	opts.ExcludePathPrefix = getQueryParam(query, "exclude_path_prefix")
	opts.UserID = getQueryParam(query, "user_id")
}

func (h *LogsHandlers) parseStatusFilters(query map[string][]string, opts *requestlog.FilterOptions) {
	if v := getQueryParam(query, "status"); v != "" {
		if status, err := strconv.Atoi(v); err == nil {
			opts.Status = status
		}
	}
	if v := getQueryParam(query, "min_status"); v != "" {
		if minStatus, err := strconv.Atoi(v); err == nil {
			opts.MinStatus = minStatus
		}
	}
	if v := getQueryParam(query, "max_status"); v != "" {
		if maxStatus, err := strconv.Atoi(v); err == nil {
			opts.MaxStatus = maxStatus
		}
	}
}

func (h *LogsHandlers) parseTimeFilters(query map[string][]string, opts *requestlog.FilterOptions) {
	if v := getQueryParam(query, "since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			opts.Since = t
		}
	}
	if v := getQueryParam(query, "until"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			opts.Until = t
		}
	}
}

func getQueryParam(query map[string][]string, key string) string {
	if vals, ok := query[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// Stats handles GET /api/admin/logs/stats.
func (h *LogsHandlers) Stats(w http.ResponseWriter, r *http.Request) {
	stats := h.store.Stats()
	JSON(w, http.StatusOK, stats)
}

// Clear handles POST /api/admin/logs/clear.
func (h *LogsHandlers) Clear(w http.ResponseWriter, r *http.Request) {
	h.store.Clear()
	JSON(w, http.StatusOK, map[string]string{"message": "logs cleared"})
}
