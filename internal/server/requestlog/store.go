// Package requestlog provides an in-memory ring buffer for HTTP request logs.
package requestlog

import (
	"strings"
	"sync"
	"time"
)

// Entry represents a single HTTP request log entry.
type Entry struct {
	ID         string            `json:"id"`
	Timestamp  time.Time         `json:"timestamp"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Query      string            `json:"query,omitempty"`
	Status     int               `json:"status"`
	Duration   time.Duration     `json:"duration"`
	DurationMS float64           `json:"duration_ms"`
	BytesIn    int64             `json:"bytes_in"`
	BytesOut   int64             `json:"bytes_out"`
	ClientIP   string            `json:"client_ip"`
	UserAgent  string            `json:"user_agent,omitempty"`
	UserID     string            `json:"user_id,omitempty"`
	Error      string            `json:"error,omitempty"`
	ErrorCode  string            `json:"error_code,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// Store is a thread-safe ring buffer for request logs.
type Store struct {
	mu       sync.RWMutex
	entries  []Entry
	capacity int
	head     int
	count    int
}

// NewStore creates a new request log store with the given capacity.
func NewStore(capacity int) *Store {
	if capacity <= 0 {
		capacity = 1000
	}
	return &Store{
		entries:  make([]Entry, capacity),
		capacity: capacity,
	}
}

// Add appends a new entry to the store.
func (s *Store) Add(entry Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[s.head] = entry
	s.head = (s.head + 1) % s.capacity
	if s.count < s.capacity {
		s.count++
	}
}

// FilterOptions specifies criteria for filtering log entries.
type FilterOptions struct {
	Method            string
	Path              string
	ExcludePathPrefix string
	Status            int
	MinStatus         int
	MaxStatus         int
	UserID            string
	Since             time.Time
	Until             time.Time
	Limit             int
	Offset            int
}

// ListResult contains the result of listing log entries.
type ListResult struct {
	Entries []Entry `json:"entries"`
	Total   int     `json:"total"`
	Limit   int     `json:"limit"`
	Offset  int     `json:"offset"`
}

// List returns log entries matching the filter options.
// Entries are returned in reverse chronological order (newest first).
func (s *Store) List(opts FilterOptions) ListResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	if opts.Limit > 1000 {
		opts.Limit = 1000
	}

	var filtered []Entry

	// Iterate in reverse order (newest first)
	for i := 0; i < s.count; i++ {
		idx := (s.head - 1 - i + s.capacity) % s.capacity
		entry := s.entries[idx]

		if s.matchesFilter(entry, opts) {
			filtered = append(filtered, entry)
		}
	}

	total := len(filtered)

	// Apply pagination
	start := opts.Offset
	if start > total {
		start = total
	}
	end := start + opts.Limit
	if end > total {
		end = total
	}

	return ListResult{
		Entries: filtered[start:end],
		Total:   total,
		Limit:   opts.Limit,
		Offset:  opts.Offset,
	}
}

func (s *Store) matchesFilter(entry Entry, opts FilterOptions) bool {
	return s.matchesStringFilters(entry, opts) &&
		s.matchesStatusFilters(entry, opts) &&
		s.matchesTimeFilters(entry, opts)
}

func (s *Store) matchesStringFilters(entry Entry, opts FilterOptions) bool {
	if opts.Method != "" && entry.Method != opts.Method {
		return false
	}
	if opts.Path != "" && entry.Path != opts.Path {
		return false
	}
	if opts.ExcludePathPrefix != "" && strings.HasPrefix(entry.Path, opts.ExcludePathPrefix) {
		return false
	}
	if opts.UserID != "" && entry.UserID != opts.UserID {
		return false
	}
	return true
}

func (s *Store) matchesStatusFilters(entry Entry, opts FilterOptions) bool {
	if opts.Status != 0 && entry.Status != opts.Status {
		return false
	}
	if opts.MinStatus != 0 && entry.Status < opts.MinStatus {
		return false
	}
	if opts.MaxStatus != 0 && entry.Status > opts.MaxStatus {
		return false
	}
	return true
}

func (s *Store) matchesTimeFilters(entry Entry, opts FilterOptions) bool {
	if !opts.Since.IsZero() && entry.Timestamp.Before(opts.Since) {
		return false
	}
	if !opts.Until.IsZero() && entry.Timestamp.After(opts.Until) {
		return false
	}
	return true
}

// Count returns the number of entries currently in the store.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.count
}

// Clear removes all entries from the store.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make([]Entry, s.capacity)
	s.head = 0
	s.count = 0
}

// Stats returns statistics about the log store.
type Stats struct {
	Capacity int `json:"capacity"`
	Count    int `json:"count"`
}

// Stats returns current store statistics.
func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Stats{
		Capacity: s.capacity,
		Count:    s.count,
	}
}
