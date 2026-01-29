package scheduler

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/watzon/alyx/internal/events"
)

func TestStateStore_SaveAndGet(t *testing.T) {
	db := testDB(t)
	schedStore := NewStore(db)
	stateStore := NewStateStore(db)
	ctx := context.Background()

	schedule := &Schedule{
		ID:         "test-schedule-1",
		Name:       "Test Schedule",
		FunctionID: "test-function",
		Type:       ScheduleTypeCron,
		Expression: "0 * * * *",
		Timezone:   "UTC",
		Enabled:    true,
	}
	if err := schedStore.Create(ctx, schedule); err != nil {
		t.Fatalf("Failed to create schedule: %v", err)
	}

	now := time.Now().UTC()
	nextRun := now.Add(1 * time.Hour)

	state := &ScheduleState{
		ScheduleID:      "test-schedule-1",
		LastExecutionAt: &now,
		NextExecutionAt: &nextRun,
		ExecutionCount:  5,
	}

	if err := stateStore.Save(ctx, state); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	retrieved, err := stateStore.Get(ctx, "test-schedule-1")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected state, got nil")
	}

	if retrieved.ScheduleID != state.ScheduleID {
		t.Errorf("Expected ScheduleID %s, got %s", state.ScheduleID, retrieved.ScheduleID)
	}

	if retrieved.ExecutionCount != state.ExecutionCount {
		t.Errorf("Expected ExecutionCount %d, got %d", state.ExecutionCount, retrieved.ExecutionCount)
	}

	if retrieved.LastExecutionAt == nil || !retrieved.LastExecutionAt.Truncate(time.Second).Equal(state.LastExecutionAt.Truncate(time.Second)) {
		t.Errorf("Expected LastExecutionAt %v, got %v", state.LastExecutionAt, retrieved.LastExecutionAt)
	}

	if retrieved.NextExecutionAt == nil || !retrieved.NextExecutionAt.Truncate(time.Second).Equal(state.NextExecutionAt.Truncate(time.Second)) {
		t.Errorf("Expected NextExecutionAt %v, got %v", state.NextExecutionAt, retrieved.NextExecutionAt)
	}
}

func TestStateStore_UpdateAfterExecution(t *testing.T) {
	db := testDB(t)
	schedStore := NewStore(db)
	stateStore := NewStateStore(db)
	ctx := context.Background()

	schedule := &Schedule{
		ID:         "test-schedule-2",
		Name:       "Test Schedule",
		FunctionID: "test-function",
		Type:       ScheduleTypeCron,
		Expression: "0 * * * *",
		Timezone:   "UTC",
		Enabled:    true,
	}
	if err := schedStore.Create(ctx, schedule); err != nil {
		t.Fatalf("Failed to create schedule: %v", err)
	}

	now := time.Now().UTC()
	nextRun := now.Add(1 * time.Hour)

	state := &ScheduleState{
		ScheduleID:      "test-schedule-2",
		ExecutionCount:  0,
		NextExecutionAt: &nextRun,
	}

	if err := stateStore.Save(ctx, state); err != nil {
		t.Fatalf("Failed to save initial state: %v", err)
	}

	newNextRun := now.Add(2 * time.Hour)
	if err := stateStore.UpdateAfterExecution(ctx, "test-schedule-2", newNextRun); err != nil {
		t.Fatalf("Failed to update after execution: %v", err)
	}

	retrieved, err := stateStore.Get(ctx, "test-schedule-2")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if retrieved.ExecutionCount != 1 {
		t.Errorf("Expected ExecutionCount 1, got %d", retrieved.ExecutionCount)
	}

	if retrieved.LastExecutionAt == nil {
		t.Error("Expected LastExecutionAt to be set")
	}

	if retrieved.NextExecutionAt == nil || !retrieved.NextExecutionAt.Truncate(time.Second).Equal(newNextRun.Truncate(time.Second)) {
		t.Errorf("Expected NextExecutionAt %v, got %v", newNextRun, retrieved.NextExecutionAt)
	}
}

func TestStateStore_Delete(t *testing.T) {
	db := testDB(t)
	schedStore := NewStore(db)
	stateStore := NewStateStore(db)
	ctx := context.Background()

	schedule := &Schedule{
		ID:         "test-schedule-3",
		Name:       "Test Schedule",
		FunctionID: "test-function",
		Type:       ScheduleTypeCron,
		Expression: "0 * * * *",
		Timezone:   "UTC",
		Enabled:    true,
	}
	if err := schedStore.Create(ctx, schedule); err != nil {
		t.Fatalf("Failed to create schedule: %v", err)
	}

	state := &ScheduleState{
		ScheduleID:     "test-schedule-3",
		ExecutionCount: 0,
	}

	if err := stateStore.Save(ctx, state); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	if err := stateStore.Delete(ctx, "test-schedule-3"); err != nil {
		t.Fatalf("Failed to delete state: %v", err)
	}

	retrieved, err := stateStore.Get(ctx, "test-schedule-3")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if retrieved != nil {
		t.Error("Expected state to be deleted, but it still exists")
	}
}

func TestStateStore_List(t *testing.T) {
	db := testDB(t)
	schedStore := NewStore(db)
	stateStore := NewStateStore(db)
	ctx := context.Background()

	scheduleIDs := []string{"test-schedule-4", "test-schedule-5", "test-schedule-6"}
	for _, id := range scheduleIDs {
		schedule := &Schedule{
			ID:         id,
			Name:       "Test Schedule",
			FunctionID: "test-function",
			Type:       ScheduleTypeCron,
			Expression: "0 * * * *",
			Timezone:   "UTC",
			Enabled:    true,
		}
		if err := schedStore.Create(ctx, schedule); err != nil {
			t.Fatalf("Failed to create schedule: %v", err)
		}
	}

	states := []*ScheduleState{
		{ScheduleID: "test-schedule-4", ExecutionCount: 1},
		{ScheduleID: "test-schedule-5", ExecutionCount: 2},
		{ScheduleID: "test-schedule-6", ExecutionCount: 3},
	}

	for _, state := range states {
		if err := stateStore.Save(ctx, state); err != nil {
			t.Fatalf("Failed to save state: %v", err)
		}
	}

	retrieved, err := stateStore.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list states: %v", err)
	}

	if len(retrieved) < len(states) {
		t.Errorf("Expected at least %d states, got %d", len(states), len(retrieved))
	}
}

func TestScheduler_RecoverSchedules_CronWithCatchup(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)
	ctx := context.Background()

	schedule := &Schedule{
		ID:         "cron-schedule-1",
		Name:       "Test Cron Schedule",
		FunctionID: "test-function",
		Type:       ScheduleTypeCron,
		Expression: "*/1 * * * *",
		Timezone:   "UTC",
		Enabled:    true,
	}

	pastTime := time.Now().UTC().Add(-5 * time.Minute)
	schedule.NextRun = &pastTime

	if err := scheduler.Create(ctx, schedule); err != nil {
		t.Fatalf("Failed to create schedule: %v", err)
	}

	config := &RecoveryConfig{
		EnableCatchup: true,
	}

	if err := scheduler.RecoverSchedules(ctx, config); err != nil {
		t.Fatalf("Failed to recover schedules: %v", err)
	}

	retrieved, err := scheduler.Get(ctx, "cron-schedule-1")
	if err != nil {
		t.Fatalf("Failed to get schedule: %v", err)
	}

	if retrieved.NextRun == nil {
		t.Error("Expected NextRun to be set after recovery")
	} else if retrieved.NextRun.Before(time.Now().UTC()) {
		t.Error("Expected NextRun to be in the future after recovery")
	}
}

func TestScheduler_RecoverSchedules_IntervalWithoutCatchup(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)
	ctx := context.Background()

	schedule := &Schedule{
		ID:         "interval-schedule-1",
		Name:       "Test Interval Schedule",
		FunctionID: "test-function",
		Type:       ScheduleTypeInterval,
		Expression: "5m",
		Timezone:   "UTC",
		Enabled:    true,
	}

	pastTime := time.Now().UTC().Add(-10 * time.Minute)
	schedule.NextRun = &pastTime

	if err := scheduler.Create(ctx, schedule); err != nil {
		t.Fatalf("Failed to create schedule: %v", err)
	}

	config := &RecoveryConfig{
		EnableCatchup: false,
	}

	if err := scheduler.RecoverSchedules(ctx, config); err != nil {
		t.Fatalf("Failed to recover schedules: %v", err)
	}

	retrieved, err := scheduler.Get(ctx, "interval-schedule-1")
	if err != nil {
		t.Fatalf("Failed to get schedule: %v", err)
	}

	if retrieved.NextRun == nil {
		t.Error("Expected NextRun to be set after recovery")
	} else if retrieved.NextRun.Before(time.Now().UTC()) {
		t.Error("Expected NextRun to be in the future after recovery without catchup")
	}
}

func TestScheduler_RecoverSchedules_WithEnvVar(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)
	ctx := context.Background()

	os.Setenv("ALYX_SCHEDULER_CATCHUP", "true")
	t.Cleanup(func() {
		os.Unsetenv("ALYX_SCHEDULER_CATCHUP")
	})

	schedule := &Schedule{
		ID:         "env-schedule-1",
		Name:       "Test Env Schedule",
		FunctionID: "test-function",
		Type:       ScheduleTypeInterval,
		Expression: "1m",
		Timezone:   "UTC",
		Enabled:    true,
	}

	pastTime := time.Now().UTC().Add(-2 * time.Minute)
	schedule.NextRun = &pastTime

	if err := scheduler.Create(ctx, schedule); err != nil {
		t.Fatalf("Failed to create schedule: %v", err)
	}

	if err := scheduler.RecoverSchedules(ctx, nil); err != nil {
		t.Fatalf("Failed to recover schedules with env config: %v", err)
	}

	retrieved, err := scheduler.Get(ctx, "env-schedule-1")
	if err != nil {
		t.Fatalf("Failed to get schedule: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Expected schedule to exist")
	}
}

func TestScheduler_PersistStateOnCreate(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)
	ctx := context.Background()

	nextRun := time.Now().UTC().Add(1 * time.Hour)
	schedule := &Schedule{
		ID:         "persist-schedule-1",
		Name:       "Test Persist Schedule",
		FunctionID: "test-function",
		Type:       ScheduleTypeOneTime,
		Expression: nextRun.Format(time.RFC3339),
		Timezone:   "UTC",
		Enabled:    true,
		NextRun:    &nextRun,
	}

	if err := scheduler.Create(ctx, schedule); err != nil {
		t.Fatalf("Failed to create schedule: %v", err)
	}

	state, err := scheduler.stateStore.Get(ctx, "persist-schedule-1")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if state == nil {
		t.Fatal("Expected state to be created")
	}

	if state.ExecutionCount != 0 {
		t.Errorf("Expected ExecutionCount 0, got %d", state.ExecutionCount)
	}

	if state.NextExecutionAt == nil || !state.NextExecutionAt.Truncate(time.Second).Equal(nextRun.Truncate(time.Second)) {
		t.Errorf("Expected NextExecutionAt %v, got %v", nextRun, state.NextExecutionAt)
	}
}

func TestScheduler_DeleteStateOnDelete(t *testing.T) {
	db := testDB(t)
	eventBus := events.NewEventBus(db, nil)
	scheduler := NewScheduler(db, eventBus)
	ctx := context.Background()

	nextRun := time.Now().UTC().Add(1 * time.Hour)
	schedule := &Schedule{
		ID:         "delete-schedule-1",
		Name:       "Test Delete Schedule",
		FunctionID: "test-function",
		Type:       ScheduleTypeOneTime,
		Expression: nextRun.Format(time.RFC3339),
		Timezone:   "UTC",
		Enabled:    true,
		NextRun:    &nextRun,
	}

	if err := scheduler.Create(ctx, schedule); err != nil {
		t.Fatalf("Failed to create schedule: %v", err)
	}

	if err := scheduler.Delete(ctx, "delete-schedule-1"); err != nil {
		t.Fatalf("Failed to delete schedule: %v", err)
	}

	state, err := scheduler.stateStore.Get(ctx, "delete-schedule-1")
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	if state != nil {
		t.Error("Expected state to be deleted")
	}
}
