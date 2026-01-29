package server

import (
	"testing"
	"time"
)

func TestBruteForceProtector_BasicBlocking(t *testing.T) {
	bfp := NewBruteForceProtector(5, 15*time.Minute)
	defer bfp.Stop()

	key := "test@example.com"

	for i := 0; i < 4; i++ {
		if bfp.IsBlocked(key) {
			t.Errorf("Should not be blocked after %d attempts", i)
		}
		bfp.RecordFailedAttempt(key)
	}

	if bfp.IsBlocked(key) {
		t.Error("Should not be blocked after 4 attempts")
	}

	bfp.RecordFailedAttempt(key)

	if !bfp.IsBlocked(key) {
		t.Error("Should be blocked after 5 attempts")
	}
}

func TestBruteForceProtector_MultipleKeys(t *testing.T) {
	bfp := NewBruteForceProtector(5, 15*time.Minute)
	defer bfp.Stop()

	key1 := "user1@example.com"
	key2 := "user2@example.com"

	for i := 0; i < 5; i++ {
		bfp.RecordFailedAttempt(key1)
	}

	if !bfp.IsBlocked(key1) {
		t.Error("key1 should be blocked after 5 attempts")
	}

	if bfp.IsBlocked(key2) {
		t.Error("key2 should not be blocked")
	}

	for i := 0; i < 3; i++ {
		bfp.RecordFailedAttempt(key2)
	}

	if bfp.IsBlocked(key2) {
		t.Error("key2 should not be blocked after 3 attempts")
	}
}

func TestBruteForceProtector_WindowExpiration(t *testing.T) {
	bfp := NewBruteForceProtector(5, 500*time.Millisecond)
	defer bfp.Stop()

	key := "test@example.com"

	for i := 0; i < 5; i++ {
		bfp.RecordFailedAttempt(key)
	}

	if !bfp.IsBlocked(key) {
		t.Error("Should be blocked after 5 attempts")
	}

	time.Sleep(600 * time.Millisecond)

	if bfp.IsBlocked(key) {
		t.Error("Should not be blocked after window expiration")
	}

	bfp.RecordFailedAttempt(key)
	if bfp.IsBlocked(key) {
		t.Error("Should not be blocked after 1 attempt post-expiration")
	}
}

func TestBruteForceProtector_ClearAttempts(t *testing.T) {
	bfp := NewBruteForceProtector(5, 15*time.Minute)
	defer bfp.Stop()

	key := "test@example.com"

	for i := 0; i < 4; i++ {
		bfp.RecordFailedAttempt(key)
	}

	bfp.ClearAttempts(key)

	if bfp.IsBlocked(key) {
		t.Error("Should not be blocked after clearing attempts")
	}

	for i := 0; i < 3; i++ {
		bfp.RecordFailedAttempt(key)
	}

	if bfp.IsBlocked(key) {
		t.Error("Should not be blocked after 3 new attempts")
	}
}

func TestBruteForceProtector_ConcurrentAccess(t *testing.T) {
	bfp := NewBruteForceProtector(50, 1*time.Second)
	defer bfp.Stop()

	done := make(chan bool)
	key := "concurrent@example.com"

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 5; j++ {
				bfp.RecordFailedAttempt(key)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if !bfp.IsBlocked(key) {
		t.Error("Should be blocked after 50 concurrent attempts")
	}

	bfp.mu.RLock()
	record := bfp.attempts[key]
	bfp.mu.RUnlock()

	if record == nil {
		t.Fatal("Record should exist")
	}

	record.mu.Lock()
	count := record.count
	record.mu.Unlock()

	if count != 50 {
		t.Errorf("Expected 50 attempts, got %d", count)
	}
}

func TestBruteForceProtector_Stop(t *testing.T) {
	bfp := NewBruteForceProtector(5, 1*time.Second)

	bfp.RecordFailedAttempt("test@example.com")

	bfp.Stop()

	time.Sleep(100 * time.Millisecond)

	select {
	case <-bfp.stopCh:
	default:
		t.Error("Stop channel should be closed")
	}
}

func TestBruteForceProtector_CleanupLoop(t *testing.T) {
	bfp := NewBruteForceProtector(5, 100*time.Millisecond)
	defer bfp.Stop()

	bfp.RecordFailedAttempt("key1@example.com")
	bfp.RecordFailedAttempt("key2@example.com")
	bfp.RecordFailedAttempt("key3@example.com")

	bfp.mu.RLock()
	initialCount := len(bfp.attempts)
	bfp.mu.RUnlock()

	if initialCount != 3 {
		t.Errorf("Expected 3 records, got %d", initialCount)
	}

	time.Sleep(300 * time.Millisecond)

	bfp.mu.RLock()
	finalCount := len(bfp.attempts)
	bfp.mu.RUnlock()

	if finalCount != 0 {
		t.Errorf("Expected 0 records after cleanup, got %d", finalCount)
	}
}
