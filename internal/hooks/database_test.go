package hooks

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/events"
	"github.com/watzon/alyx/internal/schema"
)

func testDBHooks(t *testing.T) *database.DB {
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

func testSchemaHooks(t *testing.T) *schema.Schema {
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

func setupTestEnvironment(t *testing.T) (*database.DB, *Registry, *events.EventBus, *DatabaseHookTrigger) {
	t.Helper()

	db := testDBHooks(t)
	s := testSchemaHooks(t)

	gen := schema.NewSQLGenerator(s)
	for _, stmt := range gen.GenerateAll() {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("Failed to execute SQL: %v\nSQL: %s", err, stmt)
		}
	}

	registry, err := NewRegistry(db)
	require.NoError(t, err)

	bus := events.NewEventBus(db, nil)

	trigger := NewDatabaseHookTrigger(registry, bus)

	return db, registry, bus, trigger
}

func TestDatabaseHookTrigger_OnInsert(t *testing.T) {
	_, registry, bus, trigger := setupTestEnvironment(t)
	ctx := context.Background()

	eventReceived := false
	bus.Subscribe(events.EventTypeDatabase, "users", "insert", func(ctx context.Context, event *events.Event) error {
		eventReceived = true

		payload, ok := event.Payload.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "insert", payload["action"])
		require.Equal(t, "users", payload["collection"])
		require.NotNil(t, payload["document"])

		doc := payload["document"].(map[string]any)
		require.Equal(t, "test@example.com", doc["email"])
		require.Equal(t, "Test User", doc["name"])

		return nil
	})

	hook := &Hook{
		ID:          "hook1",
		Name:        "Test Insert Hook",
		FunctionID:  "func1",
		EventType:   string(events.EventTypeDatabase),
		EventSource: "users",
		EventAction: "insert",
		Mode:        HookModeAsync,
		Priority:    100,
		Config: HookConfig{
			OnFailure: "continue",
			Timeout:   5 * time.Second,
		},
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := registry.Register(ctx, hook)
	require.NoError(t, err)

	document := map[string]any{
		"email": "test@example.com",
		"name":  "Test User",
	}

	err = trigger.OnInsert(ctx, "users", document)
	require.NoError(t, err)

	err = bus.ProcessPending(ctx)
	require.NoError(t, err)

	require.True(t, eventReceived, "Event should have been received by subscriber")
}

func TestDatabaseHookTrigger_OnUpdate(t *testing.T) {
	_, registry, bus, trigger := setupTestEnvironment(t)
	ctx := context.Background()

	eventReceived := false
	bus.Subscribe(events.EventTypeDatabase, "users", "update", func(ctx context.Context, event *events.Event) error {
		eventReceived = true

		payload, ok := event.Payload.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "update", payload["action"])
		require.Equal(t, "users", payload["collection"])
		require.NotNil(t, payload["document"])
		require.NotNil(t, payload["previous_document"])
		require.NotNil(t, payload["changed_fields"])

		changedFields, ok := payload["changed_fields"].([]string)
		if !ok {
			changedFieldsIface := payload["changed_fields"].([]any)
			changedFields = make([]string, len(changedFieldsIface))
			for i, v := range changedFieldsIface {
				changedFields[i] = v.(string)
			}
		}
		require.Contains(t, changedFields, "name")

		return nil
	})

	hook := &Hook{
		ID:          "hook2",
		Name:        "Test Update Hook",
		FunctionID:  "func2",
		EventType:   string(events.EventTypeDatabase),
		EventSource: "users",
		EventAction: "update",
		Mode:        HookModeAsync,
		Priority:    100,
		Config: HookConfig{
			OnFailure: "continue",
			Timeout:   5 * time.Second,
		},
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := registry.Register(ctx, hook)
	require.NoError(t, err)

	previousDocument := map[string]any{
		"id":    "123",
		"email": "test@example.com",
		"name":  "Old Name",
	}

	currentDocument := map[string]any{
		"id":    "123",
		"email": "test@example.com",
		"name":  "New Name",
	}

	err = trigger.OnUpdate(ctx, "users", currentDocument, previousDocument)
	require.NoError(t, err)

	err = bus.ProcessPending(ctx)
	require.NoError(t, err)

	require.True(t, eventReceived, "Event should have been received by subscriber")
}

func TestDatabaseHookTrigger_OnDelete(t *testing.T) {
	_, registry, bus, trigger := setupTestEnvironment(t)
	ctx := context.Background()

	eventReceived := false
	bus.Subscribe(events.EventTypeDatabase, "users", "delete", func(ctx context.Context, event *events.Event) error {
		eventReceived = true

		payload, ok := event.Payload.(map[string]any)
		require.True(t, ok)
		require.Equal(t, "delete", payload["action"])
		require.Equal(t, "users", payload["collection"])
		require.NotNil(t, payload["document"])

		doc := payload["document"].(map[string]any)
		require.Equal(t, "test@example.com", doc["email"])

		return nil
	})

	hook := &Hook{
		ID:          "hook3",
		Name:        "Test Delete Hook",
		FunctionID:  "func3",
		EventType:   string(events.EventTypeDatabase),
		EventSource: "users",
		EventAction: "delete",
		Mode:        HookModeAsync,
		Priority:    100,
		Config: HookConfig{
			OnFailure: "continue",
			Timeout:   5 * time.Second,
		},
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := registry.Register(ctx, hook)
	require.NoError(t, err)

	document := map[string]any{
		"id":    "123",
		"email": "test@example.com",
		"name":  "Test User",
	}

	err = trigger.OnDelete(ctx, "users", document)
	require.NoError(t, err)

	err = bus.ProcessPending(ctx)
	require.NoError(t, err)

	require.True(t, eventReceived, "Event should have been received by subscriber")
}

func TestDatabaseHookTrigger_SyncReject(t *testing.T) {
	_, registry, _, trigger := setupTestEnvironment(t)
	ctx := context.Background()

	hook := &Hook{
		ID:          "hook4",
		Name:        "Test Sync Reject Hook",
		FunctionID:  "func4",
		EventType:   string(events.EventTypeDatabase),
		EventSource: "users",
		EventAction: "insert",
		Mode:        HookModeSync,
		Priority:    100,
		Config: HookConfig{
			OnFailure: "reject",
			Timeout:   5 * time.Second,
		},
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := registry.Register(ctx, hook)
	require.NoError(t, err)

	document := map[string]any{
		"email": "test@example.com",
		"name":  "Test User",
	}

	err = trigger.OnInsert(ctx, "users", document)
	require.NoError(t, err)
}

func TestDatabaseHookTrigger_CycleDetection(t *testing.T) {
	_, registry, _, trigger := setupTestEnvironment(t)
	ctx := context.Background()

	hook := &Hook{
		ID:          "hook5",
		Name:        "Test Cycle Detection Hook",
		FunctionID:  "func5",
		EventType:   string(events.EventTypeDatabase),
		EventSource: "users",
		EventAction: "insert",
		Mode:        HookModeSync,
		Priority:    100,
		Config: HookConfig{
			OnFailure: "continue",
			Timeout:   5 * time.Second,
		},
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := registry.Register(ctx, hook)
	require.NoError(t, err)

	trigger.mu.Lock()
	trigger.executing["func5"] = true
	trigger.mu.Unlock()

	document := map[string]any{
		"email": "test@example.com",
		"name":  "Test User",
	}

	err = trigger.OnInsert(ctx, "users", document)
	require.NoError(t, err)

	trigger.mu.Lock()
	delete(trigger.executing, "func5")
	trigger.mu.Unlock()
}

func TestDatabaseHookTrigger_WildcardMatching(t *testing.T) {
	_, registry, bus, trigger := setupTestEnvironment(t)
	ctx := context.Background()

	eventReceived := false
	bus.Subscribe(events.EventTypeDatabase, "*", "*", func(ctx context.Context, event *events.Event) error {
		eventReceived = true
		return nil
	})

	hook := &Hook{
		ID:          "hook6",
		Name:        "Test Wildcard Hook",
		FunctionID:  "func6",
		EventType:   string(events.EventTypeDatabase),
		EventSource: "*",
		EventAction: "*",
		Mode:        HookModeAsync,
		Priority:    100,
		Config: HookConfig{
			OnFailure: "continue",
			Timeout:   5 * time.Second,
		},
		Enabled:   true,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	err := registry.Register(ctx, hook)
	require.NoError(t, err)

	document := map[string]any{
		"email": "test@example.com",
		"name":  "Test User",
	}

	err = trigger.OnInsert(ctx, "users", document)
	require.NoError(t, err)

	err = bus.ProcessPending(ctx)
	require.NoError(t, err)

	require.True(t, eventReceived, "Wildcard hook should match any event")
}

func TestCalculateChangedFields(t *testing.T) {
	tests := []struct {
		name     string
		current  map[string]any
		previous map[string]any
		expected []string
	}{
		{
			name: "single field changed",
			current: map[string]any{
				"id":    "123",
				"email": "test@example.com",
				"name":  "New Name",
			},
			previous: map[string]any{
				"id":    "123",
				"email": "test@example.com",
				"name":  "Old Name",
			},
			expected: []string{"name"},
		},
		{
			name: "multiple fields changed",
			current: map[string]any{
				"id":    "123",
				"email": "new@example.com",
				"name":  "New Name",
			},
			previous: map[string]any{
				"id":    "123",
				"email": "old@example.com",
				"name":  "Old Name",
			},
			expected: []string{"email", "name"},
		},
		{
			name: "field added",
			current: map[string]any{
				"id":    "123",
				"email": "test@example.com",
				"name":  "Test User",
				"age":   30,
			},
			previous: map[string]any{
				"id":    "123",
				"email": "test@example.com",
				"name":  "Test User",
			},
			expected: []string{"age"},
		},
		{
			name: "field removed",
			current: map[string]any{
				"id":    "123",
				"email": "test@example.com",
			},
			previous: map[string]any{
				"id":    "123",
				"email": "test@example.com",
				"name":  "Test User",
			},
			expected: []string{"name"},
		},
		{
			name: "no changes",
			current: map[string]any{
				"id":    "123",
				"email": "test@example.com",
				"name":  "Test User",
			},
			previous: map[string]any{
				"id":    "123",
				"email": "test@example.com",
				"name":  "Test User",
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateChangedFields(tt.current, tt.previous)

			require.ElementsMatch(t, tt.expected, result)
		})
	}
}
