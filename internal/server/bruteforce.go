package server

import (
	"sync"
	"time"
)

// BruteForceProtector tracks failed login attempts and blocks users temporarily.
type BruteForceProtector struct {
	mu        sync.RWMutex
	attempts  map[string]*attemptRecord
	threshold int
	window    time.Duration
	cleanup   *time.Ticker
	wg        sync.WaitGroup
	stopCh    chan struct{}
}

type attemptRecord struct {
	count        int
	firstAttempt time.Time
	mu           sync.Mutex
}

// NewBruteForceProtector creates a new brute force protector with the given threshold and window.
func NewBruteForceProtector(threshold int, window time.Duration) *BruteForceProtector {
	bfp := &BruteForceProtector{
		attempts:  make(map[string]*attemptRecord),
		threshold: threshold,
		window:    window,
		cleanup:   time.NewTicker(window * 2),
		stopCh:    make(chan struct{}),
	}

	bfp.wg.Add(1)
	go func() {
		defer bfp.wg.Done()
		bfp.cleanupLoop()
	}()

	return bfp
}

// RecordFailedAttempt increments the failed attempt counter for the given key.
func (bfp *BruteForceProtector) RecordFailedAttempt(key string) {
	bfp.mu.RLock()
	record, exists := bfp.attempts[key]
	bfp.mu.RUnlock()

	if !exists {
		bfp.mu.Lock()
		// Double-check after acquiring write lock
		record, exists = bfp.attempts[key]
		if !exists {
			record = &attemptRecord{
				count:        0,
				firstAttempt: time.Now(),
			}
			bfp.attempts[key] = record
		}
		bfp.mu.Unlock()
	}

	record.mu.Lock()
	defer record.mu.Unlock()

	now := time.Now()
	if now.Sub(record.firstAttempt) >= bfp.window {
		record.count = 1
		record.firstAttempt = now
	} else {
		record.count++
	}
}

// IsBlocked checks if the given key is currently blocked due to too many failed attempts.
func (bfp *BruteForceProtector) IsBlocked(key string) bool {
	bfp.mu.RLock()
	record, exists := bfp.attempts[key]
	bfp.mu.RUnlock()

	if !exists {
		return false
	}

	record.mu.Lock()
	defer record.mu.Unlock()

	now := time.Now()
	if now.Sub(record.firstAttempt) >= bfp.window {
		return false
	}

	return record.count >= bfp.threshold
}

// ClearAttempts resets the attempt counter for the given key (e.g., after successful login).
func (bfp *BruteForceProtector) ClearAttempts(key string) {
	bfp.mu.Lock()
	defer bfp.mu.Unlock()
	delete(bfp.attempts, key)
}

func (bfp *BruteForceProtector) cleanupLoop() {
	for {
		select {
		case <-bfp.cleanup.C:
			bfp.mu.Lock()
			now := time.Now()
			for key, record := range bfp.attempts {
				record.mu.Lock()
				if now.Sub(record.firstAttempt) > bfp.window*2 {
					delete(bfp.attempts, key)
				}
				record.mu.Unlock()
			}
			bfp.mu.Unlock()
		case <-bfp.stopCh:
			return
		}
	}
}

// Stop stops the cleanup goroutine.
func (bfp *BruteForceProtector) Stop() {
	close(bfp.stopCh)
	bfp.cleanup.Stop()
	bfp.wg.Wait()
}
