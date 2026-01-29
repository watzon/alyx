package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/events"
)

// testDB creates a test database with migrations.
func testDB(t *testing.T) *database.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.DatabaseConfig{
		Path: dbPath,
	}

	db, err := database.Open(cfg)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestScheduler_CreateAndGet(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)

	ctx := context.Background()

	schedule := &Schedule{
		Name:       "test-schedule",
		FunctionID: "test-function",
		Type:       ScheduleTypeCron,
		Expression: "0 * * * *",
		Timezone:   "UTC",
		Enabled:    true,
		Config: ScheduleConfig{
			SkipIfRunning: true,
			MaxOverlap:    1,
		},
	}

	// Create schedule
	if err := scheduler.Create(ctx, schedule); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Get schedule
	got, err := scheduler.Get(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.Name != schedule.Name {
		t.Errorf("Get() name = %v, want %v", got.Name, schedule.Name)
	}
	if got.FunctionID != schedule.FunctionID {
		t.Errorf("Get() function_id = %v, want %v", got.FunctionID, schedule.FunctionID)
	}
	if got.Type != schedule.Type {
		t.Errorf("Get() type = %v, want %v", got.Type, schedule.Type)
	}
	if got.Expression != schedule.Expression {
		t.Errorf("Get() expression = %v, want %v", got.Expression, schedule.Expression)
	}
	if got.NextRun == nil {
		t.Error("Get() next_run should be set")
	}
}

func TestScheduler_Update(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)

	ctx := context.Background()

	schedule := &Schedule{
		Name:       "test-schedule",
		FunctionID: "test-function",
		Type:       ScheduleTypeCron,
		Expression: "0 * * * *",
		Timezone:   "UTC",
		Enabled:    true,
	}

	if err := scheduler.Create(ctx, schedule); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update schedule
	schedule.Expression = "*/5 * * * *"
	schedule.Enabled = false

	if err := scheduler.Update(ctx, schedule); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Get updated schedule
	got, err := scheduler.Get(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.Expression != "*/5 * * * *" {
		t.Errorf("Update() expression = %v, want */5 * * * *", got.Expression)
	}
	if got.Enabled {
		t.Error("Update() enabled should be false")
	}
}

func TestScheduler_Delete(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)

	ctx := context.Background()

	schedule := &Schedule{
		Name:       "test-schedule",
		FunctionID: "test-function",
		Type:       ScheduleTypeCron,
		Expression: "0 * * * *",
		Timezone:   "UTC",
		Enabled:    true,
	}

	if err := scheduler.Create(ctx, schedule); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete schedule
	if err := scheduler.Delete(ctx, schedule.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deletion
	_, err := scheduler.Get(ctx, schedule.ID)
	if err == nil {
		t.Error("Get() should return error for deleted schedule")
	}
}

func TestScheduler_List(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)

	ctx := context.Background()

	// Create multiple schedules
	for i := 0; i < 3; i++ {
		schedule := &Schedule{
			Name:       "test-schedule",
			FunctionID: "test-function",
			Type:       ScheduleTypeCron,
			Expression: "0 * * * *",
			Timezone:   "UTC",
			Enabled:    true,
		}
		if err := scheduler.Create(ctx, schedule); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	// List schedules
	schedules, err := scheduler.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(schedules) != 3 {
		t.Errorf("List() count = %v, want 3", len(schedules))
	}
}

func TestScheduler_FindByFunction(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)

	ctx := context.Background()

	// Create schedules for different functions
	for i := 0; i < 2; i++ {
		schedule := &Schedule{
			Name:       "test-schedule",
			FunctionID: "function-1",
			Type:       ScheduleTypeCron,
			Expression: "0 * * * *",
			Timezone:   "UTC",
			Enabled:    true,
		}
		if err := scheduler.Create(ctx, schedule); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	schedule := &Schedule{
		Name:       "test-schedule",
		FunctionID: "function-2",
		Type:       ScheduleTypeCron,
		Expression: "0 * * * *",
		Timezone:   "UTC",
		Enabled:    true,
	}
	if err := scheduler.Create(ctx, schedule); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Find by function
	schedules, err := scheduler.FindByFunction(ctx, "function-1")
	if err != nil {
		t.Fatalf("FindByFunction() error = %v", err)
	}

	if len(schedules) != 2 {
		t.Errorf("FindByFunction() count = %v, want 2", len(schedules))
	}
}

func TestScheduler_ProcessDue(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)

	ctx := context.Background()

	// Create a schedule that's due now
	now := time.Now().UTC()
	pastTime := now.Add(-1 * time.Minute)

	schedule := &Schedule{
		Name:       "test-schedule",
		FunctionID: "test-function",
		Type:       ScheduleTypeCron,
		Expression: "* * * * *",
		Timezone:   "UTC",
		Enabled:    true,
		NextRun:    &pastTime,
	}

	if err := scheduler.Create(ctx, schedule); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Subscribe to schedule events
	eventReceived := false
	eventBus.Subscribe(events.EventTypeSchedule, "scheduler", "execute", func(ctx context.Context, event *events.Event) error {
		eventReceived = true
		payload, ok := event.Payload.(map[string]any)
		if !ok {
			t.Error("Event payload is not a map")
			return nil
		}
		if payload["schedule_id"] != schedule.ID {
			t.Errorf("Event schedule_id = %v, want %v", payload["schedule_id"], schedule.ID)
		}
		return nil
	})

	// Process due schedules
	if err := scheduler.ProcessDue(ctx); err != nil {
		t.Fatalf("ProcessDue() error = %v", err)
	}

	// Verify event was published
	if !eventReceived {
		// Event might be in pending queue, process it
		if err := eventBus.ProcessPending(ctx); err != nil {
			t.Fatalf("ProcessPending() error = %v", err)
		}

		// Give it a moment to process
		time.Sleep(100 * time.Millisecond)

		if !eventReceived {
			t.Error("Schedule event was not received")
		}
	}

	// Verify next_run was updated
	updated, err := scheduler.Get(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if updated.NextRun == nil {
		t.Error("NextRun should be set after processing")
	} else if !updated.NextRun.After(now) {
		t.Errorf("NextRun should be in the future, got %v", updated.NextRun)
	}

	if updated.LastRun == nil {
		t.Error("LastRun should be set after processing")
	}
}

func TestScheduler_ConcurrencyControl(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)

	tests := []struct {
		name           string
		config         ScheduleConfig
		runningCount   int
		expectedCanRun bool
	}{
		{
			name: "skip_if_running=true, running=0",
			config: ScheduleConfig{
				SkipIfRunning: true,
			},
			runningCount:   0,
			expectedCanRun: true,
		},
		{
			name: "skip_if_running=true, running=1",
			config: ScheduleConfig{
				SkipIfRunning: true,
			},
			runningCount:   1,
			expectedCanRun: false,
		},
		{
			name: "max_overlap=2, running=1",
			config: ScheduleConfig{
				MaxOverlap: 2,
			},
			runningCount:   1,
			expectedCanRun: true,
		},
		{
			name: "max_overlap=2, running=2",
			config: ScheduleConfig{
				MaxOverlap: 2,
			},
			runningCount:   2,
			expectedCanRun: false,
		},
		{
			name: "max_overlap=0 (unlimited), running=10",
			config: ScheduleConfig{
				MaxOverlap: 0,
			},
			runningCount:   10,
			expectedCanRun: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule := &Schedule{
				ID:     "test-schedule-" + tt.name,
				Config: tt.config,
			}

			// Set running count
			scheduler.runningMu.Lock()
			if tt.runningCount > 0 {
				scheduler.running[schedule.ID] = tt.runningCount
			}
			scheduler.runningMu.Unlock()

			// Test canRun
			canRun := scheduler.canRun(schedule)
			if canRun != tt.expectedCanRun {
				t.Errorf("canRun() = %v, want %v", canRun, tt.expectedCanRun)
			}

			// Cleanup
			scheduler.runningMu.Lock()
			delete(scheduler.running, schedule.ID)
			scheduler.runningMu.Unlock()
		})
	}
}

func TestScheduler_OneTimeSchedule(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)

	ctx := context.Background()

	// Create a one-time schedule that's due now
	futureTime := time.Now().UTC().Add(-1 * time.Minute)

	schedule := &Schedule{
		Name:       "one-time-schedule",
		FunctionID: "test-function",
		Type:       ScheduleTypeOneTime,
		Expression: futureTime.Format(time.RFC3339),
		Timezone:   "UTC",
		Enabled:    true,
	}

	if err := scheduler.Create(ctx, schedule); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Process due schedules
	if err := scheduler.ProcessDue(ctx); err != nil {
		t.Fatalf("ProcessDue() error = %v", err)
	}

	// Verify schedule was disabled
	updated, err := scheduler.Get(ctx, schedule.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if updated.Enabled {
		t.Error("One-time schedule should be disabled after execution")
	}

	if updated.NextRun != nil {
		t.Error("One-time schedule should have nil next_run after execution")
	}

	if updated.LastRun == nil {
		t.Error("One-time schedule should have last_run set after execution")
	}
}

func TestScheduler_StartStop(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)

	// Start scheduler
	scheduler.Start(context.Background(), &Config{
		PollInterval: 100 * time.Millisecond,
	})

	// Let it run for a bit
	time.Sleep(300 * time.Millisecond)

	// Stop scheduler
	scheduler.Stop()

	// Verify it stopped gracefully
	select {
	case <-scheduler.ctx.Done():
		// Expected
	default:
		t.Error("Scheduler context should be done after Stop()")
	}
}
