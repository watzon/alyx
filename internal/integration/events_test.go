package integration

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/events"
	"github.com/watzon/alyx/internal/executions"
	"github.com/watzon/alyx/internal/functions"
	"github.com/watzon/alyx/internal/hooks"
	"github.com/watzon/alyx/internal/scheduler"
	"github.com/watzon/alyx/internal/schema"
	"github.com/watzon/alyx/internal/webhooks"
)

// testDB creates a test database with all migrations applied.
func testDB(t *testing.T) *database.DB {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.DatabaseConfig{
		Path: dbPath,
	}

	db, err := database.Open(cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

// testSchema creates a test schema with users collection.
func testSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.Parse([]byte(`
version: 1
collections:
  users:
    fields:
      id:
        type: uuid
        primary: true
        default: auto
      email:
        type: email
        unique: true
      name:
        type: string
      created_at:
        type: timestamp
        default: now
`))
	require.NoError(t, err)
	return s
}

// TestIntegration_DatabaseHookFlow tests the complete database hook flow:
// Create document → hook triggered → execution logged.
func TestIntegration_DatabaseHookFlow(t *testing.T) {
	ctx := context.Background()

	// Setup database and schema
	db := testDB(t)
	s := testSchema(t)

	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		_, err := db.Exec(stmt)
		require.NoError(t, err)
	}

	// Setup event system components
	bus := events.NewEventBus(db, nil)
	hookRegistry, err := hooks.NewRegistry(db)
	require.NoError(t, err)

	execStore := executions.NewStore(db)

	// Start event bus background processing
	bus.Start(ctx, nil)
	defer bus.Stop()

	// Register a database hook for user creation
	hook := &hooks.Hook{
		EventType:   string(events.EventTypeDatabase),
		EventSource: "users",
		EventAction: "insert",
		FunctionID:  "on-user-created",
		Mode:        hooks.HookModeAsync,
		Priority:    10,
		Enabled:     true,
		Config:      hooks.HookConfig{},
	}

	err = hookRegistry.Register(ctx, hook)
	require.NoError(t, err)

	// Setup database hook trigger
	dbHookTrigger := hooks.NewDatabaseHookTrigger(hookRegistry, bus)

	// Create a collection with hook trigger
	coll := database.NewCollection(db, s.Collections["users"])
	coll.SetHookTrigger(dbHookTrigger)

	// Track events published
	var eventPublished atomic.Bool
	bus.Subscribe(events.EventTypeDatabase, "users", "insert", func(ctx context.Context, event *events.Event) error {
		eventPublished.Store(true)

		// Verify event payload
		payload, ok := event.Payload.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "insert", payload["action"])
		require.Equal(t, "users", payload["collection"])
		require.NotNil(t, payload["document"])

		doc := payload["document"].(map[string]any)
		require.Equal(t, "test@example.com", doc["email"])
		require.Equal(t, "Test User", doc["name"])

		// Log execution (simulating function invocation)
		execLog := &executions.ExecutionLog{
			ID:          "exec-1",
			FunctionID:  "on-user-created",
			RequestID:   "req-1",
			TriggerType: "database",
			TriggerID:   event.ID,
			Status:      executions.ExecutionStatusRunning,
			StartedAt:   time.Now().UTC(),
		}

		inputJSON, _ := json.Marshal(payload)
		execLog.Input = string(inputJSON)

		createErr := execStore.Create(ctx, execLog)
		require.NoError(t, createErr)

		// Simulate successful execution
		execLog.Status = executions.ExecutionStatusSuccess
		execLog.Output = `{"message":"User created"}`
		execLog.DurationMs = 100
		now := time.Now().UTC()
		execLog.CompletedAt = &now

		err = execStore.Update(ctx, execLog)
		require.NoError(t, err)

		return nil
	})

	// Create a document (this should trigger the hook)
	doc := map[string]any{
		"email": "test@example.com",
		"name":  "Test User",
	}

	_, err = coll.Create(ctx, doc)
	require.NoError(t, err)

	// Wait for async event processing
	time.Sleep(2 * time.Second)

	// Verify event was published
	require.True(t, eventPublished.Load(), "Event should have been published")

	// Verify execution was logged
	execs, err := execStore.List(ctx, map[string]any{
		"function_id":  "on-user-created",
		"trigger_type": "database",
	}, 10, 0)
	require.NoError(t, err)
	require.Len(t, execs, 1)
	require.Equal(t, executions.ExecutionStatusSuccess, execs[0].Status)
}

// TestIntegration_WebhookFlow tests webhook signature verification.
// Full webhook-to-function flow is covered by webhooks/handler_test.go.
func TestIntegration_WebhookFlow(t *testing.T) {
	ctx := context.Background()

	db := testDB(t)
	webhookStore := webhooks.NewStore(db)

	endpoint := &webhooks.WebhookEndpoint{
		Path:       "/webhooks/stripe",
		FunctionID: "stripe-webhook",
		Methods:    []string{"POST"},
		Verification: &webhooks.WebhookVerification{
			Type:        "hmac-sha256",
			Header:      "X-Stripe-Signature",
			Secret:      "whsec_test123",
			SkipInvalid: false,
		},
		Enabled: true,
	}

	err := webhookStore.Create(ctx, endpoint)
	require.NoError(t, err)

	retrieved, err := webhookStore.GetByPath(ctx, "/webhooks/stripe")
	require.NoError(t, err)
	require.Equal(t, endpoint.FunctionID, retrieved.FunctionID)
	require.Equal(t, "hmac-sha256", retrieved.Verification.Type)

	payload := []byte(`{"event":"charge.succeeded","amount":1000}`)
	mac := hmac.New(sha256.New, []byte("whsec_test123"))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	result := webhooks.VerifySignature(retrieved.Verification, payload, signature)
	require.True(t, result.Valid)
	require.Equal(t, "hmac-sha256", result.Method)
}

// TestIntegration_ScheduleFlow tests the complete schedule flow:
// Wait for schedule → function invoked → next_run updated.
func TestIntegration_ScheduleFlow(t *testing.T) {
	ctx := context.Background()

	// Setup database
	db := testDB(t)

	// Setup event bus
	bus := events.NewEventBus(db, nil)
	bus.Start(ctx, nil)
	defer bus.Stop()

	// Setup scheduler
	schedStore := scheduler.NewStore(db)
	sched := scheduler.NewScheduler(db, bus)

	sched.Start(ctx, &scheduler.Config{
		PollInterval: 1 * time.Second,
	})
	defer sched.Stop()

	// Track events published
	var eventPublished atomic.Bool
	var eventPayload atomic.Value

	bus.Subscribe(events.EventTypeSchedule, "scheduler", "execute", func(ctx context.Context, event *events.Event) error {
		eventPublished.Store(true)
		payload, ok := event.Payload.(map[string]any)
		require.True(t, ok)
		eventPayload.Store(payload)
		return nil
	})

	// Create a schedule that runs immediately (one-time)
	now := time.Now().UTC()
	nextRun := now.Add(1 * time.Second)
	schedule := &scheduler.Schedule{
		Name:       "test-schedule",
		FunctionID: "daily-cleanup",
		Type:       scheduler.ScheduleTypeOneTime,
		Expression: nextRun.Format(time.RFC3339),
		Timezone:   "UTC",
		Enabled:    true,
		NextRun:    &nextRun,
		Config: scheduler.ScheduleConfig{
			Input: map[string]any{
				"cleanup_type": "old_logs",
			},
		},
	}

	err := schedStore.Create(ctx, schedule)
	require.NoError(t, err)

	// Wait for schedule to execute
	time.Sleep(3 * time.Second)

	// Verify event was published
	require.True(t, eventPublished.Load(), "Event should have been published")
	payload := eventPayload.Load()
	require.NotNil(t, payload)
	payloadMap := payload.(map[string]any)
	require.Equal(t, "daily-cleanup", payloadMap["function_id"])
	require.Equal(t, "test-schedule", payloadMap["schedule_name"])

	// Verify schedule was updated (one-time schedules should be disabled)
	updated, err := schedStore.Get(ctx, schedule.ID)
	require.NoError(t, err)
	require.False(t, updated.Enabled, "One-time schedule should be disabled after execution")
	require.NotNil(t, updated.LastRun, "LastRun should be set")
}

//nolint:unused // Mock implementation for future function service tests
type mockFunctionService struct {
	invokeFunc func(ctx context.Context, name string, input map[string]any, auth *functions.AuthContext, triggerType, triggerID string) (*functions.FunctionResponse, error)
}

//nolint:unused // Required by FunctionService interface
func (m *mockFunctionService) Invoke(ctx context.Context, name string, input map[string]any, auth *functions.AuthContext) (*functions.FunctionResponse, error) {
	return m.InvokeWithTrigger(ctx, name, input, auth, "http", "")
}

//nolint:unused // Required by FunctionService interface
func (m *mockFunctionService) InvokeWithTrigger(ctx context.Context, name string, input map[string]any, auth *functions.AuthContext, triggerType, triggerID string) (*functions.FunctionResponse, error) {
	if m.invokeFunc != nil {
		return m.invokeFunc(ctx, name, input, auth, triggerType, triggerID)
	}
	return &functions.FunctionResponse{
		RequestID:  "req-1",
		Success:    true,
		Output:     map[string]any{},
		DurationMs: 0,
	}, nil
}

//nolint:unused // Required by FunctionService interface
func (m *mockFunctionService) List() []*functions.FunctionDef {
	return nil
}

//nolint:unused // Required by FunctionService interface
func (m *mockFunctionService) Get(functionID string) (*functions.FunctionDef, bool) {
	return nil, false
}

//nolint:unused // Required by FunctionService interface
func (m *mockFunctionService) Reload() error {
	return nil
}

//nolint:unused // Required by FunctionService interface
func (m *mockFunctionService) Close() error {
	return nil
}

//nolint:unused // Required by FunctionService interface
func (m *mockFunctionService) Start(ctx context.Context) error {
	return nil
}
