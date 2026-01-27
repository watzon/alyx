package rules

import (
	"errors"
	"testing"

	"github.com/watzon/alyx/internal/schema"
)

func TestEngine_LoadBucketRules(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{},
		Buckets: map[string]*schema.Bucket{
			"avatars": {
				Name:    "avatars",
				Backend: "local",
				Rules: &schema.Rules{
					Create:   "auth.id != null",
					Read:     "true",
					Download: "auth.verified == true",
					Delete:   "auth.role == 'admin'",
				},
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr != nil {
		t.Fatalf("LoadSchema failed: %v", loadErr)
	}

	if !engine.HasRule("avatars", OpCreate) {
		t.Error("Expected avatars create rule to exist")
	}
	if !engine.HasRule("avatars", OpRead) {
		t.Error("Expected avatars read rule to exist")
	}
	if !engine.HasRule("avatars", OpDownload) {
		t.Error("Expected avatars download rule to exist")
	}
	if !engine.HasRule("avatars", OpDelete) {
		t.Error("Expected avatars delete rule to exist")
	}
}

func TestEngine_BucketCreateRule(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{},
		Buckets: map[string]*schema.Bucket{
			"uploads": {
				Name:    "uploads",
				Backend: "local",
				Rules: &schema.Rules{
					Create: "has(auth.id)",
				},
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr != nil {
		t.Fatalf("LoadSchema failed: %v", loadErr)
	}

	// Test without auth
	allowed, err := engine.Evaluate("uploads", OpCreate, &EvalContext{
		Auth: nil,
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if allowed {
		t.Error("Expected upload without auth to be denied")
	}

	// Test with auth
	allowed, err = engine.Evaluate("uploads", OpCreate, &EvalContext{
		Auth: map[string]any{"id": "user123"},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !allowed {
		t.Error("Expected upload with auth to be allowed")
	}
}

func TestEngine_BucketReadRule(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{},
		Buckets: map[string]*schema.Bucket{
			"private": {
				Name:    "private",
				Backend: "local",
				Rules: &schema.Rules{
					Read: "auth.id == file.owner_id",
				},
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr != nil {
		t.Fatalf("LoadSchema failed: %v", loadErr)
	}

	// Test owner access
	allowed, err := engine.Evaluate("private", OpRead, &EvalContext{
		Auth: map[string]any{"id": "user123"},
		File: map[string]any{"owner_id": "user123"},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !allowed {
		t.Error("Expected owner to read file metadata")
	}

	// Test non-owner access
	allowed, err = engine.Evaluate("private", OpRead, &EvalContext{
		Auth: map[string]any{"id": "user456"},
		File: map[string]any{"owner_id": "user123"},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if allowed {
		t.Error("Expected non-owner to be denied file metadata access")
	}
}

func TestEngine_BucketDownloadRule(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{},
		Buckets: map[string]*schema.Bucket{
			"documents": {
				Name:    "documents",
				Backend: "local",
				Rules: &schema.Rules{
					Read:     "true",
					Download: "auth.verified == true",
				},
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr != nil {
		t.Fatalf("LoadSchema failed: %v", loadErr)
	}

	// Test verified user can download
	allowed, err := engine.Evaluate("documents", OpDownload, &EvalContext{
		Auth: map[string]any{"id": "user123", "verified": true},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !allowed {
		t.Error("Expected verified user to download")
	}

	// Test unverified user cannot download
	allowed, err = engine.Evaluate("documents", OpDownload, &EvalContext{
		Auth: map[string]any{"id": "user456", "verified": false},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if allowed {
		t.Error("Expected unverified user to be denied download")
	}

	// Test read is still allowed for unverified
	allowed, err = engine.Evaluate("documents", OpRead, &EvalContext{
		Auth: map[string]any{"id": "user456", "verified": false},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !allowed {
		t.Error("Expected unverified user to read metadata")
	}
}

func TestEngine_BucketFileContext(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{},
		Buckets: map[string]*schema.Bucket{
			"media": {
				Name:    "media",
				Backend: "local",
				Rules: &schema.Rules{
					Download: "file.mime_type.startsWith('image/') || auth.role == 'admin'",
				},
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr != nil {
		t.Fatalf("LoadSchema failed: %v", loadErr)
	}

	// Test image download for regular user
	allowed, err := engine.Evaluate("media", OpDownload, &EvalContext{
		Auth: map[string]any{"id": "user123", "role": "user"},
		File: map[string]any{"mime_type": "image/jpeg"},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !allowed {
		t.Error("Expected user to download image")
	}

	// Test video download for regular user (should fail)
	allowed, err = engine.Evaluate("media", OpDownload, &EvalContext{
		Auth: map[string]any{"id": "user123", "role": "user"},
		File: map[string]any{"mime_type": "video/mp4"},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if allowed {
		t.Error("Expected user to be denied video download")
	}

	// Test video download for admin
	allowed, err = engine.Evaluate("media", OpDownload, &EvalContext{
		Auth: map[string]any{"id": "admin1", "role": "admin"},
		File: map[string]any{"mime_type": "video/mp4"},
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !allowed {
		t.Error("Expected admin to download video")
	}
}

func TestEngine_BucketCheckAccess(t *testing.T) {
	engine, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	s := &schema.Schema{
		Collections: map[string]*schema.Collection{},
		Buckets: map[string]*schema.Bucket{
			"restricted": {
				Name:    "restricted",
				Backend: "local",
				Rules: &schema.Rules{
					Download: "false",
				},
			},
		},
	}

	if loadErr := engine.LoadSchema(s); loadErr != nil {
		t.Fatalf("LoadSchema failed: %v", loadErr)
	}

	err = engine.CheckAccess("restricted", OpDownload, &EvalContext{})
	if err == nil {
		t.Error("Expected CheckAccess to return error for denied download")
	}

	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("Expected ErrAccessDenied, got %v", err)
	}
}
