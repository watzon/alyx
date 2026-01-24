package functions

import (
	"testing"
	"time"
)

func TestInternalTokenStore_Generate(t *testing.T) {
	store := NewInternalTokenStore(5 * time.Minute)

	token := store.Generate()
	if token == "" {
		t.Error("expected non-empty token")
	}

	if len(token) != 64 { // 32 bytes hex encoded
		t.Errorf("expected token length 64, got %d", len(token))
	}
}

func TestInternalTokenStore_Validate(t *testing.T) {
	store := NewInternalTokenStore(5 * time.Minute)

	token := store.Generate()

	if !store.Validate(token) {
		t.Error("expected token to be valid")
	}

	if store.Validate("invalid-token") {
		t.Error("expected invalid token to fail validation")
	}
}

func TestInternalTokenStore_Revoke(t *testing.T) {
	store := NewInternalTokenStore(5 * time.Minute)

	token := store.Generate()

	if !store.Validate(token) {
		t.Error("expected token to be valid before revocation")
	}

	store.Revoke(token)

	if store.Validate(token) {
		t.Error("expected token to be invalid after revocation")
	}
}

func TestInternalTokenStore_Expiration(t *testing.T) {
	store := NewInternalTokenStore(50 * time.Millisecond)

	token := store.Generate()

	if !store.Validate(token) {
		t.Error("expected token to be valid immediately")
	}

	time.Sleep(100 * time.Millisecond)

	if store.Validate(token) {
		t.Error("expected token to be expired after TTL")
	}
}

func TestInternalTokenStore_Count(t *testing.T) {
	store := NewInternalTokenStore(5 * time.Minute)

	if store.Count() != 0 {
		t.Errorf("expected 0 tokens, got %d", store.Count())
	}

	store.Generate()
	store.Generate()
	store.Generate()

	if store.Count() != 3 {
		t.Errorf("expected 3 tokens, got %d", store.Count())
	}
}

func TestInternalTokenStore_UniqueTokens(t *testing.T) {
	store := NewInternalTokenStore(5 * time.Minute)

	tokens := make(map[string]bool)
	for range 100 {
		token := store.Generate()
		if tokens[token] {
			t.Error("generated duplicate token")
		}
		tokens[token] = true
	}
}
