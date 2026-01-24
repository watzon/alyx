package rules

import (
	"errors"
	"testing"

	"github.com/watzon/alyx/internal/auth"
	"github.com/watzon/alyx/internal/schema"
)

func TestNewEngine(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
}

func TestEngine_LoadSchema(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{
			"posts": {
				Name: "posts",
				Rules: &schema.Rules{
					Create: "auth.id != null",
					Read:   "true",
					Update: "auth.id == doc.author_id",
					Delete: "auth.role == 'admin'",
				},
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr != nil {
		t.Fatalf("LoadSchema failed: %v", loadErr)
	}

	if !engine.HasRule("posts", OpCreate) {
		t.Error("Expected posts create rule to exist")
	}
	if !engine.HasRule("posts", OpRead) {
		t.Error("Expected posts read rule to exist")
	}
	if !engine.HasRule("posts", OpUpdate) {
		t.Error("Expected posts update rule to exist")
	}
	if !engine.HasRule("posts", OpDelete) {
		t.Error("Expected posts delete rule to exist")
	}

	if engine.HasRule("users", OpCreate) {
		t.Error("Expected users create rule to not exist")
	}
}

func TestEngine_InvalidRule(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{
			"posts": {
				Name: "posts",
				Rules: &schema.Rules{
					Create: "invalid syntax !!@@##",
				},
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr == nil {
		t.Error("Expected LoadSchema to fail with invalid rule")
	}
}

func TestEngine_Evaluate_PublicRead(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{
			"posts": {
				Name: "posts",
				Rules: &schema.Rules{
					Read: "true",
				},
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr != nil {
		t.Fatalf("LoadSchema failed: %v", loadErr)
	}

	allowed, err := engine.Evaluate("posts", OpRead, &EvalContext{})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !allowed {
		t.Error("Expected public read to be allowed")
	}
}

func TestEngine_Evaluate_RequireAuth(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{
			"posts": {
				Name: "posts",
				Rules: &schema.Rules{
					Create: "has(auth.id)",
				},
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr != nil {
		t.Fatalf("LoadSchema failed: %v", loadErr)
	}

	allowed, err := engine.Evaluate("posts", OpCreate, &EvalContext{
		Auth: nil,
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if allowed {
		t.Error("Expected create without auth to be denied")
	}

	allowed, err = engine.Evaluate("posts", OpCreate, &EvalContext{
		Auth: map[string]any{"id": "user123"},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !allowed {
		t.Error("Expected create with auth to be allowed")
	}
}

func TestEngine_Evaluate_OwnerOnly(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{
			"posts": {
				Name: "posts",
				Rules: &schema.Rules{
					Update: "auth.id == doc.author_id",
				},
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr != nil {
		t.Fatalf("LoadSchema failed: %v", loadErr)
	}

	allowed, err := engine.Evaluate("posts", OpUpdate, &EvalContext{
		Auth: map[string]any{"id": "user123"},
		Doc:  map[string]any{"author_id": "user123"},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !allowed {
		t.Error("Expected owner to be allowed to update")
	}

	allowed, err = engine.Evaluate("posts", OpUpdate, &EvalContext{
		Auth: map[string]any{"id": "user456"},
		Doc:  map[string]any{"author_id": "user123"},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if allowed {
		t.Error("Expected non-owner to be denied update")
	}
}

func TestEngine_Evaluate_RoleCheck(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{
			"posts": {
				Name: "posts",
				Rules: &schema.Rules{
					Delete: "auth.role == 'admin'",
				},
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr != nil {
		t.Fatalf("LoadSchema failed: %v", loadErr)
	}

	allowed, err := engine.Evaluate("posts", OpDelete, &EvalContext{
		Auth: map[string]any{"id": "user123", "role": "user"},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if allowed {
		t.Error("Expected regular user to be denied delete")
	}

	allowed, err = engine.Evaluate("posts", OpDelete, &EvalContext{
		Auth: map[string]any{"id": "admin1", "role": "admin"},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !allowed {
		t.Error("Expected admin to be allowed to delete")
	}
}

func TestEngine_Evaluate_NoRule(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{
			"posts": {
				Name:  "posts",
				Rules: nil,
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr != nil {
		t.Fatalf("LoadSchema failed: %v", loadErr)
	}

	allowed, err := engine.Evaluate("posts", OpCreate, &EvalContext{})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !allowed {
		t.Error("Expected missing rule to allow access by default")
	}
}

func TestEngine_CheckAccess(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{
			"posts": {
				Name: "posts",
				Rules: &schema.Rules{
					Read: "false",
				},
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr != nil {
		t.Fatalf("LoadSchema failed: %v", loadErr)
	}

	err = engine.CheckAccess("posts", OpRead, &EvalContext{})
	if err == nil {
		t.Error("Expected CheckAccess to return error for denied access")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("Expected ErrAccessDenied, got %v", err)
	}
}

func TestBuildAuthContext(t *testing.T) {
	user := &auth.User{
		ID:       "user123",
		Email:    "test@example.com",
		Verified: true,
		Metadata: map[string]any{"plan": "pro"},
	}

	ctx := BuildAuthContext(user, nil)

	if ctx["id"] != user.ID {
		t.Errorf("id mismatch: got %v, want %v", ctx["id"], user.ID)
	}
	if ctx["email"] != user.Email {
		t.Errorf("email mismatch: got %v, want %v", ctx["email"], user.Email)
	}
	if ctx["verified"] != user.Verified {
		t.Errorf("verified mismatch: got %v, want %v", ctx["verified"], user.Verified)
	}

	claims := &auth.Claims{
		UserID:   "user456",
		Email:    "claims@example.com",
		Verified: false,
		Role:     "admin",
	}

	ctx = BuildAuthContext(nil, claims)

	if ctx["id"] != claims.UserID {
		t.Errorf("id mismatch from claims: got %v, want %v", ctx["id"], claims.UserID)
	}
	if ctx["role"] != claims.Role {
		t.Errorf("role mismatch: got %v, want %v", ctx["role"], claims.Role)
	}

	ctx = BuildAuthContext(nil, nil)
	if ctx != nil {
		t.Error("Expected nil context for no user or claims")
	}
}

func TestBuildRequestContext(t *testing.T) {
	ctx := BuildRequestContext("POST", "192.168.1.1")

	if ctx["method"] != "POST" {
		t.Errorf("method mismatch: got %v, want POST", ctx["method"])
	}
	if ctx["ip"] != "192.168.1.1" {
		t.Errorf("ip mismatch: got %v, want 192.168.1.1", ctx["ip"])
	}
}
