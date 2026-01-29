package auth

import (
	"testing"
	"time"
)

func TestTokenBlacklist_RevokeAndCheck(t *testing.T) {
	bl := NewTokenBlacklist()
	defer bl.Stop()

	token := "test-token-123"
	expiresAt := time.Now().Add(15 * time.Minute)

	if bl.IsRevoked(token) {
		t.Error("Token should not be revoked initially")
	}

	bl.Revoke(token, expiresAt)

	if !bl.IsRevoked(token) {
		t.Error("Token should be revoked after Revoke()")
	}
}

func TestTokenBlacklist_ExpiredTokens(t *testing.T) {
	bl := NewTokenBlacklist()
	defer bl.Stop()

	token := "expired-token"
	expiresAt := time.Now().Add(-1 * time.Minute)

	bl.Revoke(token, expiresAt)

	if bl.IsRevoked(token) {
		t.Error("Expired token should not be considered revoked")
	}
}

func TestTokenBlacklist_Cleanup(t *testing.T) {
	bl := NewTokenBlacklist()
	defer bl.Stop()

	expiredToken := "expired"
	bl.Revoke(expiredToken, time.Now().Add(-1*time.Minute))

	validToken := "valid"
	bl.Revoke(validToken, time.Now().Add(15*time.Minute))

	if bl.Count() != 2 {
		t.Errorf("Expected 2 tokens, got %d", bl.Count())
	}

	bl.mu.Lock()
	now := time.Now()
	for token, expiresAt := range bl.tokens {
		if now.After(expiresAt) {
			delete(bl.tokens, token)
		}
	}
	bl.mu.Unlock()

	if bl.Count() != 1 {
		t.Errorf("Expected 1 token after cleanup, got %d", bl.Count())
	}
}

func TestTokenBlacklist_MultipleTokens(t *testing.T) {
	bl := NewTokenBlacklist()
	defer bl.Stop()

	tokens := []string{"token1", "token2", "token3"}
	expiresAt := time.Now().Add(15 * time.Minute)

	for _, token := range tokens {
		bl.Revoke(token, expiresAt)
	}

	if bl.Count() != 3 {
		t.Errorf("Expected 3 tokens, got %d", bl.Count())
	}

	for _, token := range tokens {
		if !bl.IsRevoked(token) {
			t.Errorf("Token %s should be revoked", token)
		}
	}

	nonRevokedToken := "token4"
	if bl.IsRevoked(nonRevokedToken) {
		t.Error("Non-revoked token should not be marked as revoked")
	}
}

func TestTokenBlacklist_StopCleanup(t *testing.T) {
	bl := NewTokenBlacklist()

	bl.Revoke("token", time.Now().Add(15*time.Minute))

	if bl.Count() != 1 {
		t.Errorf("Expected 1 token, got %d", bl.Count())
	}

	bl.Stop()

	if bl.Count() != 1 {
		t.Error("Stop() should not clear tokens, only stop cleanup goroutine")
	}
}

func TestTokenBlacklist_ConcurrentAccess(t *testing.T) {
	bl := NewTokenBlacklist()
	defer bl.Stop()

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			token := string(rune('a' + id))
			expiresAt := time.Now().Add(15 * time.Minute)
			bl.Revoke(token, expiresAt)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if bl.Count() != 10 {
		t.Errorf("Expected 10 tokens after concurrent revocations, got %d", bl.Count())
	}
}
