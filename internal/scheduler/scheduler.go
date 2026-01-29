package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/events"
)

// Scheduler manages scheduled function executions.
type Scheduler struct {
	db         *database.DB
	store      *Store
	stateStore *StateStore
	eventBus   *events.EventBus
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	running    map[string]int // scheduleID -> count of running executions
	runningMu  sync.RWMutex
}

// Config holds configuration for Scheduler.
type Config struct {
	// PollInterval is how often to poll for due schedules (default: 1 second).
	PollInterval time.Duration
}

// NewScheduler creates a new scheduler.
func NewScheduler(db *database.DB, eventBus *events.EventBus) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		db:         db,
		store:      NewStore(db),
		stateStore: NewStateStore(db),
		eventBus:   eventBus,
		ctx:        ctx,
		cancel:     cancel,
		running:    make(map[string]int),
	}
}

// Start begins background processing.
func (s *Scheduler) Start(ctx context.Context, config *Config) {
	if config == nil {
		config = &Config{}
	}
	if config.PollInterval == 0 {
		config.PollInterval = 1 * time.Second
	}

	s.wg.Add(1)
	go s.pollLoop(s.ctx, config.PollInterval)

	log.Info().
		Dur("poll_interval", config.PollInterval).
		Msg("Scheduler started")
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop() {
	s.cancel()
	s.wg.Wait()
	log.Info().Msg("Scheduler stopped")
}

// pollLoop periodically polls for due schedules.
func (s *Scheduler) pollLoop(ctx context.Context, interval time.Duration) {
	defer s.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.ProcessDue(ctx); err != nil {
				log.Error().Err(err).Msg("Failed to process due schedules")
			}
		}
	}
}

// ProcessDue processes schedules that are due to run.
func (s *Scheduler) ProcessDue(ctx context.Context) error {
	schedules, err := s.store.GetDue(ctx, 100)
	if err != nil {
		return fmt.Errorf("getting due schedules: %w", err)
	}

	for _, schedule := range schedules {
		if err := s.processSchedule(ctx, schedule); err != nil {
			log.Error().
				Err(err).
				Str("schedule_id", schedule.ID).
				Str("schedule_name", schedule.Name).
				Msg("Failed to process schedule")
		}
	}

	return nil
}

// processSchedule processes a single schedule.
func (s *Scheduler) processSchedule(ctx context.Context, schedule *Schedule) error {
	// Check concurrency limits
	if !s.canRun(schedule) {
		log.Debug().
			Str("schedule_id", schedule.ID).
			Str("schedule_name", schedule.Name).
			Msg("Skipping schedule due to concurrency limits")
		return nil
	}

	// Increment running count
	s.incrementRunning(schedule.ID)
	defer s.decrementRunning(schedule.ID)

	// Publish schedule event to event bus
	event := &events.Event{
		Type:   events.EventTypeSchedule,
		Source: "scheduler",
		Action: "execute",
		Payload: map[string]any{
			"schedule_id":   schedule.ID,
			"schedule_name": schedule.Name,
			"function_id":   schedule.FunctionID,
			"input":         schedule.Config.Input,
		},
		Metadata: events.EventMetadata{
			Extra: map[string]any{
				"schedule_id":   schedule.ID,
				"function_id":   schedule.FunctionID,
				"schedule_type": string(schedule.Type),
			},
		},
	}

	if err := s.eventBus.Publish(ctx, event); err != nil {
		return fmt.Errorf("publishing schedule event: %w", err)
	}

	log.Debug().
		Str("schedule_id", schedule.ID).
		Str("schedule_name", schedule.Name).
		Str("function_id", schedule.FunctionID).
		Msg("Schedule event published")

	// Update last_run
	now := time.Now().UTC()
	schedule.LastRun = &now

	// Calculate next_run
	nextRun, err := CalculateNextRun(schedule, now)
	if err != nil {
		// For one-time schedules, this is expected after first run
		if schedule.Type == ScheduleTypeOneTime {
			log.Debug().
				Str("schedule_id", schedule.ID).
				Str("schedule_name", schedule.Name).
				Msg("One-time schedule completed, disabling")

			// Disable the schedule
			schedule.Enabled = false
			schedule.LastRun = &now
			schedule.NextRun = nil
			if updateErr := s.store.Update(ctx, schedule); updateErr != nil {
				return fmt.Errorf("disabling one-time schedule: %w", updateErr)
			}
			return nil
		}

		return fmt.Errorf("calculating next run: %w", err)
	}

	// Update schedule
	if err := s.store.UpdateNextRun(ctx, schedule.ID, nextRun, now); err != nil {
		return fmt.Errorf("updating next_run: %w", err)
	}

	if err := s.stateStore.UpdateAfterExecution(ctx, schedule.ID, nextRun); err != nil {
		log.Error().
			Err(err).
			Str("schedule_id", schedule.ID).
			Msg("Failed to persist scheduler state")
	}

	log.Debug().
		Str("schedule_id", schedule.ID).
		Str("schedule_name", schedule.Name).
		Time("next_run", nextRun).
		Msg("Schedule next_run updated")

	return nil
}

// canRun checks if a schedule can run based on concurrency limits.
func (s *Scheduler) canRun(schedule *Schedule) bool {
	s.runningMu.RLock()
	defer s.runningMu.RUnlock()

	runningCount := s.running[schedule.ID]

	// Check skip_if_running
	if schedule.Config.SkipIfRunning && runningCount > 0 {
		return false
	}

	// Check max_overlap (0 = unlimited)
	if schedule.Config.MaxOverlap > 0 && runningCount >= schedule.Config.MaxOverlap {
		return false
	}

	return true
}

// incrementRunning increments the running count for a schedule.
func (s *Scheduler) incrementRunning(scheduleID string) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	s.running[scheduleID]++
}

// decrementRunning decrements the running count for a schedule.
func (s *Scheduler) decrementRunning(scheduleID string) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	s.running[scheduleID]--
	if s.running[scheduleID] <= 0 {
		delete(s.running, scheduleID)
	}
}

// Create creates a new schedule.
func (s *Scheduler) Create(ctx context.Context, schedule *Schedule) error {
	if err := s.store.Create(ctx, schedule); err != nil {
		return err
	}

	state := &ScheduleState{
		ScheduleID:      schedule.ID,
		NextExecutionAt: schedule.NextRun,
		ExecutionCount:  0,
	}
	if err := s.stateStore.Save(ctx, state); err != nil {
		log.Error().
			Err(err).
			Str("schedule_id", schedule.ID).
			Msg("Failed to initialize scheduler state")
	}

	return nil
}

// Update updates an existing schedule.
func (s *Scheduler) Update(ctx context.Context, schedule *Schedule) error {
	return s.store.Update(ctx, schedule)
}

// Delete removes a schedule.
func (s *Scheduler) Delete(ctx context.Context, scheduleID string) error {
	if err := s.store.Delete(ctx, scheduleID); err != nil {
		return err
	}

	if err := s.stateStore.Delete(ctx, scheduleID); err != nil {
		log.Error().
			Err(err).
			Str("schedule_id", scheduleID).
			Msg("Failed to delete scheduler state")
	}

	return nil
}

// Get retrieves a schedule by ID.
func (s *Scheduler) Get(ctx context.Context, scheduleID string) (*Schedule, error) {
	return s.store.Get(ctx, scheduleID)
}

// List retrieves all schedules.
func (s *Scheduler) List(ctx context.Context) ([]*Schedule, error) {
	return s.store.List(ctx)
}

// FindByFunction finds schedules for a function.
func (s *Scheduler) FindByFunction(ctx context.Context, functionID string) ([]*Schedule, error) {
	return s.store.FindByFunction(ctx, functionID)
}
