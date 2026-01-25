package hooks

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/watzon/alyx/internal/auth"
	"github.com/watzon/alyx/internal/config"
	"github.com/watzon/alyx/internal/database"
	"github.com/watzon/alyx/internal/events"
)

func testAuthSetup(t *testing.T) (*auth.Service, *AuthHookTrigger, *events.EventBus, *database.DB) {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &config.DatabaseConfig{
		Path:            dbPath,
		WALMode:         true,
		ForeignKeys:     true,
		BusyTimeout:     5 * time.Second,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 1 * time.Hour,
		CacheSize:       -2000,
	}

	db, err := database.Open(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	authCfg := &config.AuthConfig{
		AllowRegistration: true,
		Password: config.PasswordConfig{
			MinLength: 8,
		},
		JWT: config.JWTConfig{
			Secret:     "test-secret-key-that-is-long-enough",
			AccessTTL:  15 * time.Minute,
			RefreshTTL: 7 * 24 * time.Hour,
		},
	}

	authService := auth.NewService(db, authCfg)

	registry, err := NewRegistry(db)
	require.NoError(t, err)

	bus := events.NewEventBus(db, nil)

	trigger := NewAuthHookTrigger(registry, bus)
	authService.SetHookTrigger(trigger)

	return authService, trigger, bus, db
}

func TestAuthHookTrigger_OnSignup(t *testing.T) {
	authService, _, bus, db := testAuthSetup(t)
	ctx := context.Background()

	hook := &Hook{
		ID:          "hook1",
		Name:        "Test Signup Hook",
		FunctionID:  "func1",
		EventType:   "auth",
		EventSource: "user",
		EventAction: "signup",
		Mode:        HookModeAsync,
		Priority:    100,
		Enabled:     true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	registry, err := NewRegistry(db)
	require.NoError(t, err)
	require.NoError(t, registry.Register(ctx, hook))

	trigger := NewAuthHookTrigger(registry, bus)
	authService.SetHookTrigger(trigger)

	input := auth.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	}

	user, tokens, err := authService.Register(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, user)
	require.NotNil(t, tokens)

	time.Sleep(100 * time.Millisecond)

	store := events.NewStore(db)
	eventList, err := store.GetPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, eventList, 1)

	event := eventList[0]
	require.Equal(t, events.EventTypeAuth, event.Type)
	require.Equal(t, "user", event.Source)
	require.Equal(t, "signup", event.Action)
	require.Equal(t, "pending", event.Status)

	payload, ok := event.Payload.(map[string]any)
	require.True(t, ok)
	require.Equal(t, hook.ID, payload["hook_id"])
	require.Equal(t, "signup", payload["action"])

	userPayload, ok := payload["user"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, user.ID, userPayload["id"])
	require.Equal(t, user.Email, userPayload["email"])
}

func TestAuthHookTrigger_OnSignup_SyncReject(t *testing.T) {
	authService, _, bus, db := testAuthSetup(t)
	ctx := context.Background()

	hook := &Hook{
		ID:          "hook1",
		Name:        "Test Sync Signup Hook",
		FunctionID:  "func1",
		EventType:   "auth",
		EventSource: "user",
		EventAction: "signup",
		Mode:        HookModeSync,
		Priority:    100,
		Enabled:     true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	registry, err := NewRegistry(db)
	require.NoError(t, err)
	require.NoError(t, registry.Register(ctx, hook))

	trigger := NewAuthHookTrigger(registry, bus)
	authService.SetHookTrigger(trigger)

	input := auth.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	}

	user, tokens, err := authService.Register(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, user)
	require.NotNil(t, tokens)

	time.Sleep(100 * time.Millisecond)

	store := events.NewStore(db)
	eventList, err := store.GetPending(ctx, 10)
	require.NoError(t, err)
	require.Len(t, eventList, 0)
}

func TestAuthHookTrigger_OnLogin(t *testing.T) {
	authService, _, bus, db := testAuthSetup(t)
	ctx := context.Background()

	hook := &Hook{
		ID:          "hook1",
		Name:        "Test Login Hook",
		FunctionID:  "func1",
		EventType:   "auth",
		EventSource: "user",
		EventAction: "login",
		Mode:        HookModeAsync,
		Priority:    100,
		Enabled:     true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	registry, err := NewRegistry(db)
	require.NoError(t, err)
	require.NoError(t, registry.Register(ctx, hook))

	trigger := NewAuthHookTrigger(registry, bus)
	authService.SetHookTrigger(trigger)

	registerInput := auth.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	}
	_, _, err = authService.Register(ctx, registerInput)
	require.NoError(t, err)

	loginInput := auth.LoginInput{
		Email:    "test@example.com",
		Password: "password123",
	}

	user, tokens, err := authService.Login(ctx, loginInput, "TestAgent/1.0", "127.0.0.1")
	require.NoError(t, err)
	require.NotNil(t, user)
	require.NotNil(t, tokens)

	time.Sleep(100 * time.Millisecond)

	store := events.NewStore(db)
	eventList, err := store.GetPending(ctx, 10)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(eventList), 1)

	var loginEvent *events.Event
	for _, e := range eventList {
		if e.Action == "login" {
			loginEvent = e
			break
		}
	}
	require.NotNil(t, loginEvent)

	require.Equal(t, events.EventTypeAuth, loginEvent.Type)
	require.Equal(t, "user", loginEvent.Source)
	require.Equal(t, "login", loginEvent.Action)

	payload, ok := loginEvent.Payload.(map[string]any)
	require.True(t, ok)

	metadata, ok := payload["metadata"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "127.0.0.1", metadata["ip"])
	require.Equal(t, "TestAgent/1.0", metadata["user_agent"])

	userPayload, ok := payload["user"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, user.ID, userPayload["id"])
	require.Equal(t, user.Email, userPayload["email"])
}

func TestAuthHookTrigger_OnLogout(t *testing.T) {
	authService, _, bus, db := testAuthSetup(t)
	ctx := context.Background()

	hook := &Hook{
		ID:          "hook1",
		Name:        "Test Logout Hook",
		FunctionID:  "func1",
		EventType:   "auth",
		EventSource: "user",
		EventAction: "logout",
		Mode:        HookModeAsync,
		Priority:    100,
		Enabled:     true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	registry, err := NewRegistry(db)
	require.NoError(t, err)
	require.NoError(t, registry.Register(ctx, hook))

	trigger := NewAuthHookTrigger(registry, bus)
	authService.SetHookTrigger(trigger)

	registerInput := auth.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	}
	_, _, err = authService.Register(ctx, registerInput)
	require.NoError(t, err)

	loginInput := auth.LoginInput{
		Email:    "test@example.com",
		Password: "password123",
	}
	_, tokens, err := authService.Login(ctx, loginInput, "TestAgent/1.0", "127.0.0.1")
	require.NoError(t, err)

	err = authService.Logout(ctx, tokens.RefreshToken)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	store := events.NewStore(db)
	eventList, err := store.GetPending(ctx, 10)
	require.NoError(t, err)

	var logoutEvent *events.Event
	for _, e := range eventList {
		if e.Action == "logout" {
			logoutEvent = e
			break
		}
	}
	require.NotNil(t, logoutEvent)

	require.Equal(t, events.EventTypeAuth, logoutEvent.Type)
	require.Equal(t, "user", logoutEvent.Source)
	require.Equal(t, "logout", logoutEvent.Action)
}

func TestAuthHookTrigger_OnPasswordReset(t *testing.T) {
	authService, _, bus, db := testAuthSetup(t)
	ctx := context.Background()

	hook := &Hook{
		ID:          "hook1",
		Name:        "Test Password Reset Hook",
		FunctionID:  "func1",
		EventType:   "auth",
		EventSource: "user",
		EventAction: "password_reset",
		Mode:        HookModeAsync,
		Priority:    100,
		Enabled:     true,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	registry, err := NewRegistry(db)
	require.NoError(t, err)
	require.NoError(t, registry.Register(ctx, hook))

	trigger := NewAuthHookTrigger(registry, bus)
	authService.SetHookTrigger(trigger)

	registerInput := auth.RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	}
	user, _, err := authService.Register(ctx, registerInput)
	require.NoError(t, err)

	err = authService.SetPassword(ctx, user.ID, "newpassword123")
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	store := events.NewStore(db)
	eventList, err := store.GetPending(ctx, 10)
	require.NoError(t, err)

	var resetEvent *events.Event
	for _, e := range eventList {
		if e.Action == "password_reset" {
			resetEvent = e
			break
		}
	}
	require.NotNil(t, resetEvent)

	require.Equal(t, events.EventTypeAuth, resetEvent.Type)
	require.Equal(t, "user", resetEvent.Source)
	require.Equal(t, "password_reset", resetEvent.Action)
}
