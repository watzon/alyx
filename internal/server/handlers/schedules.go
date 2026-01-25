package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/scheduler"
)

// ScheduleHandlers handles schedule-related endpoints.
type ScheduleHandlers struct {
	store     *scheduler.Store
	scheduler *scheduler.Scheduler
}

// NewScheduleHandlers creates new schedule handlers.
func NewScheduleHandlers(store *scheduler.Store, sched *scheduler.Scheduler) *ScheduleHandlers {
	return &ScheduleHandlers{
		store:     store,
		scheduler: sched,
	}
}

// CreateScheduleRequest is the request body for creating a schedule.
type CreateScheduleRequest struct {
	Name       string                   `json:"name"`
	FunctionID string                   `json:"function_id"`
	Type       scheduler.ScheduleType   `json:"type"`
	Expression string                   `json:"expression"`
	Timezone   string                   `json:"timezone"`
	Enabled    bool                     `json:"enabled"`
	Config     scheduler.ScheduleConfig `json:"config"`
}

// UpdateScheduleRequest is the request body for updating a schedule.
type UpdateScheduleRequest struct {
	Name       *string                   `json:"name,omitempty"`
	Expression *string                   `json:"expression,omitempty"`
	Timezone   *string                   `json:"timezone,omitempty"`
	Enabled    *bool                     `json:"enabled,omitempty"`
	Config     *scheduler.ScheduleConfig `json:"config,omitempty"`
}

// List handles GET /api/schedules.
func (h *ScheduleHandlers) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	schedules, err := h.store.List(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list schedules")
		InternalError(w, "Failed to list schedules")
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"schedules": schedules,
		"count":     len(schedules),
	})
}

// Get handles GET /api/schedules/{id}.
func (h *ScheduleHandlers) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Schedule ID is required")
		return
	}

	schedule, err := h.store.Get(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get schedule")
		NotFound(w, "Schedule not found")
		return
	}

	JSON(w, http.StatusOK, schedule)
}

// Create handles POST /api/schedules.
func (h *ScheduleHandlers) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "Invalid JSON body: "+err.Error())
		return
	}

	if req.Name == "" {
		BadRequest(w, "Name is required")
		return
	}
	if req.FunctionID == "" {
		BadRequest(w, "Function ID is required")
		return
	}
	if req.Type == "" {
		BadRequest(w, "Type is required")
		return
	}
	if req.Expression == "" {
		BadRequest(w, "Expression is required")
		return
	}
	if req.Timezone == "" {
		req.Timezone = "UTC"
	}

	now := time.Now().UTC()
	nextRun, err := scheduler.CalculateNextRun(&scheduler.Schedule{
		Type:       req.Type,
		Expression: req.Expression,
		Timezone:   req.Timezone,
	}, now)
	if err != nil {
		BadRequest(w, "Invalid schedule expression: "+err.Error())
		return
	}

	schedule := &scheduler.Schedule{
		ID:         uuid.New().String(),
		Name:       req.Name,
		FunctionID: req.FunctionID,
		Type:       req.Type,
		Expression: req.Expression,
		Timezone:   req.Timezone,
		NextRun:    &nextRun,
		Enabled:    req.Enabled,
		Config:     req.Config,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := h.store.Create(ctx, schedule); err != nil {
		log.Error().Err(err).Msg("Failed to create schedule")
		InternalError(w, "Failed to create schedule: "+err.Error())
		return
	}

	JSON(w, http.StatusCreated, schedule)
}

// Update handles PATCH /api/schedules/{id}.
func (h *ScheduleHandlers) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Schedule ID is required")
		return
	}

	schedule, err := h.store.Get(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get schedule")
		NotFound(w, "Schedule not found")
		return
	}

	var req UpdateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "Invalid JSON body: "+err.Error())
		return
	}

	if req.Name != nil {
		schedule.Name = *req.Name
	}
	if req.Expression != nil {
		schedule.Expression = *req.Expression
	}
	if req.Timezone != nil {
		schedule.Timezone = *req.Timezone
	}
	if req.Enabled != nil {
		schedule.Enabled = *req.Enabled
	}
	if req.Config != nil {
		schedule.Config = *req.Config
	}

	if req.Expression != nil || req.Timezone != nil {
		now := time.Now().UTC()
		nextRun, calcErr := scheduler.CalculateNextRun(schedule, now)
		if calcErr != nil {
			BadRequest(w, "Invalid schedule expression: "+calcErr.Error())
			return
		}
		schedule.NextRun = &nextRun
	}

	schedule.UpdatedAt = time.Now().UTC()

	if err := h.store.Update(ctx, schedule); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to update schedule")
		InternalError(w, "Failed to update schedule: "+err.Error())
		return
	}

	JSON(w, http.StatusOK, schedule)
}

// Delete handles DELETE /api/schedules/{id}.
func (h *ScheduleHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Schedule ID is required")
		return
	}

	if err := h.store.Delete(ctx, id); err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to delete schedule")
		InternalError(w, "Failed to delete schedule: "+err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Schedule deleted successfully",
	})
}

// Trigger handles POST /api/schedules/{id}/trigger.
func (h *ScheduleHandlers) Trigger(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		BadRequest(w, "Schedule ID is required")
		return
	}

	schedule, err := h.store.Get(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Failed to get schedule")
		NotFound(w, "Schedule not found")
		return
	}

	if h.scheduler != nil {
		if err := h.scheduler.ProcessDue(ctx); err != nil {
			log.Error().Err(err).Str("id", id).Msg("Failed to trigger schedule")
			InternalError(w, "Failed to trigger schedule: "+err.Error())
			return
		}
	}

	JSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"message":     "Schedule processing triggered",
		"schedule_id": schedule.ID,
	})
}
