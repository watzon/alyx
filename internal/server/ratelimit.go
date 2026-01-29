package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/watzon/alyx/internal/config"
)

// RateLimiter implements token bucket algorithm for rate limiting.
type RateLimiter struct {
	mu      sync.RWMutex
	buckets map[string]*bucket
	rule    config.RateLimitRule
	cleanup *time.Ticker
	wg      sync.WaitGroup
	stopCh  chan struct{}
}

type bucket struct {
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a rate limiter with the given rule.
func NewRateLimiter(rule config.RateLimitRule) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*bucket),
		rule:    rule,
		cleanup: time.NewTicker(rule.Window * 2),
		stopCh:  make(chan struct{}),
	}

	rl.wg.Add(1)
	go func() {
		defer rl.wg.Done()
		rl.cleanupLoop()
	}()

	return rl
}

// Allow checks if a request from the given key is allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.RLock()
	b, exists := rl.buckets[key]
	rl.mu.RUnlock()

	if !exists {
		rl.mu.Lock()
		// Double-check after acquiring write lock
		b, exists = rl.buckets[key]
		if !exists {
			b = &bucket{
				tokens:     rl.rule.Max,
				lastRefill: time.Now(),
			}
			rl.buckets[key] = b
		}
		rl.mu.Unlock()
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill)
	if elapsed >= rl.rule.Window {
		b.tokens = rl.rule.Max
		b.lastRefill = now
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) cleanupLoop() {
	for {
		select {
		case <-rl.cleanup.C:
			rl.mu.Lock()
			now := time.Now()
			for key, b := range rl.buckets {
				b.mu.Lock()
				if now.Sub(b.lastRefill) > rl.rule.Window*2 {
					delete(rl.buckets, key)
				}
				b.mu.Unlock()
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// Stop stops the cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
	rl.cleanup.Stop()
	rl.wg.Wait()
}

// Middleware returns an HTTP middleware that rate limits requests.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			ip = realIP
		}

		if !rl.Allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded","message":"Too many requests. Please try again later."}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
