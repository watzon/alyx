package auth

import (
	"sync"
	"time"
)

// TokenBlacklist manages revoked JWT tokens.
type TokenBlacklist struct {
	mu     sync.RWMutex
	tokens map[string]time.Time // token -> expiration time
	wg     sync.WaitGroup
	stopCh chan struct{}
}

// NewTokenBlacklist creates a new token blacklist.
func NewTokenBlacklist() *TokenBlacklist {
	bl := &TokenBlacklist{
		tokens: make(map[string]time.Time),
		stopCh: make(chan struct{}),
	}

	// Start cleanup goroutine
	bl.wg.Add(1)
	go func() {
		defer bl.wg.Done()
		bl.cleanup()
	}()

	return bl
}

// Revoke adds a token to the blacklist.
// expiresAt is when the token naturally expires (no need to track after that).
func (bl *TokenBlacklist) Revoke(token string, expiresAt time.Time) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.tokens[token] = expiresAt
}

// IsRevoked checks if a token has been revoked.
func (bl *TokenBlacklist) IsRevoked(token string) bool {
	bl.mu.RLock()
	defer bl.mu.RUnlock()

	expiresAt, exists := bl.tokens[token]
	if !exists {
		return false
	}

	// If token has expired, it's no longer in use anyway
	if time.Now().After(expiresAt) {
		return false
	}

	return true
}

// cleanup removes expired tokens from the blacklist.
func (bl *TokenBlacklist) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bl.mu.Lock()
			now := time.Now()
			for token, expiresAt := range bl.tokens {
				if now.After(expiresAt) {
					delete(bl.tokens, token)
				}
			}
			bl.mu.Unlock()
		case <-bl.stopCh:
			return
		}
	}
}

// Stop stops the cleanup goroutine.
func (bl *TokenBlacklist) Stop() {
	close(bl.stopCh)
	bl.wg.Wait()
}

// Count returns the number of revoked tokens (for testing/monitoring).
func (bl *TokenBlacklist) Count() int {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	return len(bl.tokens)
}
