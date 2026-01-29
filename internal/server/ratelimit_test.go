package server

import (
	"testing"
	"time"

	"github.com/watzon/alyx/internal/config"
)

func TestRateLimiter_Allow(t *testing.T) {
	rule := config.RateLimitRule{
		Max:    3,
		Window: 1 * time.Second,
	}

	rl := NewRateLimiter(rule)
	defer rl.Stop()

	key := "test-key"

	for i := 0; i < 3; i++ {
		if !rl.Allow(key) {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	if rl.Allow(key) {
		t.Error("4th request should be blocked")
	}

	time.Sleep(1100 * time.Millisecond)

	if !rl.Allow(key) {
		t.Error("Request after window should be allowed")
	}
}

func TestRateLimiter_MultipleKeys(t *testing.T) {
	rule := config.RateLimitRule{
		Max:    2,
		Window: 1 * time.Second,
	}

	rl := NewRateLimiter(rule)
	defer rl.Stop()

	if !rl.Allow("key1") || !rl.Allow("key1") {
		t.Error("key1 should allow 2 requests")
	}

	if !rl.Allow("key2") || !rl.Allow("key2") {
		t.Error("key2 should allow 2 requests")
	}

	if rl.Allow("key1") {
		t.Error("key1 should be blocked")
	}
	if rl.Allow("key2") {
		t.Error("key2 should be blocked")
	}
}

func TestRateLimiter_WindowRefill(t *testing.T) {
	rule := config.RateLimitRule{
		Max:    2,
		Window: 500 * time.Millisecond,
	}

	rl := NewRateLimiter(rule)
	defer rl.Stop()

	key := "test-key"

	if !rl.Allow(key) || !rl.Allow(key) {
		t.Error("First 2 requests should be allowed")
	}

	if rl.Allow(key) {
		t.Error("3rd request should be blocked")
	}

	time.Sleep(600 * time.Millisecond)

	if !rl.Allow(key) || !rl.Allow(key) {
		t.Error("Requests after window should be allowed")
	}

	if rl.Allow(key) {
		t.Error("Exceeding limit again should be blocked")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rule := config.RateLimitRule{
		Max:    5,
		Window: 100 * time.Millisecond,
	}

	rl := NewRateLimiter(rule)
	defer rl.Stop()

	rl.Allow("key1")
	rl.Allow("key2")
	rl.Allow("key3")

	rl.mu.RLock()
	initialCount := len(rl.buckets)
	rl.mu.RUnlock()

	if initialCount != 3 {
		t.Errorf("Expected 3 buckets, got %d", initialCount)
	}

	time.Sleep(300 * time.Millisecond)

	rl.mu.RLock()
	finalCount := len(rl.buckets)
	rl.mu.RUnlock()

	if finalCount != 0 {
		t.Errorf("Expected 0 buckets after cleanup, got %d", finalCount)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rule := config.RateLimitRule{
		Max:    100,
		Window: 1 * time.Second,
	}

	rl := NewRateLimiter(rule)
	defer rl.Stop()

	done := make(chan bool)
	key := "concurrent-key"

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				rl.Allow(key)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	rl.mu.RLock()
	b := rl.buckets[key]
	rl.mu.RUnlock()

	if b == nil {
		t.Fatal("Bucket should exist")
	}

	b.mu.Lock()
	tokens := b.tokens
	b.mu.Unlock()

	if tokens != 0 {
		t.Errorf("Expected 0 tokens remaining, got %d", tokens)
	}
}

func TestRateLimiter_Stop(t *testing.T) {
	rule := config.RateLimitRule{
		Max:    5,
		Window: 1 * time.Second,
	}

	rl := NewRateLimiter(rule)

	rl.Allow("test-key")

	rl.Stop()

	time.Sleep(100 * time.Millisecond)

	select {
	case <-rl.stopCh:
	default:
		t.Error("Stop channel should be closed")
	}
}
