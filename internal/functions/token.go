package functions

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// InternalTokenStore manages short-lived tokens for container->host communication.
type InternalTokenStore struct {
	tokens map[string]tokenEntry
	mu     sync.RWMutex
	ttl    time.Duration
	wg     sync.WaitGroup
	stopCh chan struct{}
}

type tokenEntry struct {
	createdAt time.Time
}

// NewInternalTokenStore creates a new token store.
func NewInternalTokenStore(ttl time.Duration) *InternalTokenStore {
	store := &InternalTokenStore{
		tokens: make(map[string]tokenEntry),
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}

	// Start cleanup goroutine
	store.wg.Add(1)
	go func() {
		defer store.wg.Done()
		store.cleanup()
	}()

	return store
}

// Generate creates a new internal token.
func (s *InternalTokenStore) Generate() string {
	// Generate random token
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback to less secure but still functional token
		b = []byte(time.Now().String())
	}
	token := hex.EncodeToString(b)

	s.mu.Lock()
	s.tokens[token] = tokenEntry{
		createdAt: time.Now(),
	}
	s.mu.Unlock()

	return token
}

// Validate checks if a token is valid.
func (s *InternalTokenStore) Validate(token string) bool {
	s.mu.RLock()
	entry, exists := s.tokens[token]
	s.mu.RUnlock()

	if !exists {
		return false
	}

	// Check if token has expired
	if time.Since(entry.createdAt) > s.ttl {
		s.mu.Lock()
		delete(s.tokens, token)
		s.mu.Unlock()
		return false
	}

	return true
}

// Revoke removes a token.
func (s *InternalTokenStore) Revoke(token string) {
	s.mu.Lock()
	delete(s.tokens, token)
	s.mu.Unlock()
}

// cleanup periodically removes expired tokens.
func (s *InternalTokenStore) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for token, entry := range s.tokens {
				if now.Sub(entry.createdAt) > s.ttl {
					delete(s.tokens, token)
				}
			}
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

// Count returns the number of active tokens.
func (s *InternalTokenStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tokens)
}

// Stop gracefully shuts down the token store.
func (s *InternalTokenStore) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}
