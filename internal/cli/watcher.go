package cli

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

// EventType represents the type of file change event.
type EventType int

const (
	EventCreated EventType = iota
	EventModified
	EventDeleted
	EventRenamed
)

// FileEvent represents a file change event.
type FileEvent struct {
	Type EventType
	Path string
	Name string
}

// String returns a human-readable string for the event type.
func (e EventType) String() string {
	switch e {
	case EventCreated:
		return "created"
	case EventModified:
		return "modified"
	case EventDeleted:
		return "deleted"
	case EventRenamed:
		return "renamed"
	default:
		return "unknown"
	}
}

// Watcher watches files and directories for changes.
type Watcher struct {
	watcher   *fsnotify.Watcher
	debounce  time.Duration
	handlers  map[string][]WatchHandler
	mu        sync.RWMutex
	wg        sync.WaitGroup
	events    chan FileEvent
	done      chan struct{}
	pendingMu sync.Mutex
	pending   map[string]*time.Timer
}

// WatchHandler is called when a file change is detected.
type WatchHandler func(event FileEvent)

// WatcherOption configures the watcher.
type WatcherOption func(*Watcher)

// WithDebounce sets the debounce duration for file events.
// Multiple events for the same file within this duration will be coalesced.
func WithDebounce(d time.Duration) WatcherOption {
	return func(w *Watcher) {
		w.debounce = d
	}
}

// NewWatcher creates a new file watcher.
func NewWatcher(opts ...WatcherOption) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher:  fsWatcher,
		debounce: 100 * time.Millisecond,
		handlers: make(map[string][]WatchHandler),
		events:   make(chan FileEvent, 100),
		done:     make(chan struct{}),
		pending:  make(map[string]*time.Timer),
	}

	for _, opt := range opts {
		opt(w)
	}

	return w, nil
}

// Watch adds a file or directory to the watch list with an optional handler.
// The pattern can be a file path, directory path, or glob pattern.
func (w *Watcher) Watch(pattern string, handler WatchHandler) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Register the handler for this pattern
	w.handlers[pattern] = append(w.handlers[pattern], handler)

	// Add to fsnotify watcher
	return w.watcher.Add(pattern)
}

// WatchDir watches a directory recursively.
func (w *Watcher) WatchDir(dir string, handler WatchHandler) error {
	w.mu.Lock()
	w.handlers[dir] = append(w.handlers[dir], handler)
	w.mu.Unlock()

	return w.watcher.Add(dir)
}

// Start begins watching for file changes.
func (w *Watcher) Start(ctx context.Context) {
	w.wg.Add(2)

	go func() {
		defer w.wg.Done()
		w.processLoop(ctx)
	}()

	go func() {
		defer w.wg.Done()
		w.dispatchLoop(ctx)
	}()
}

// Stop stops the watcher.
func (w *Watcher) Stop() error {
	close(w.done)
	w.wg.Wait()
	return w.watcher.Close()
}

// processLoop reads fsnotify events and converts them to FileEvents.
func (w *Watcher) processLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.done:
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleFSEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Error().Err(err).Msg("Watcher error")
		}
	}
}

// handleFSEvent converts an fsnotify event to a FileEvent and debounces it.
func (w *Watcher) handleFSEvent(event fsnotify.Event) {
	var eventType EventType
	switch {
	case event.Op&fsnotify.Create != 0:
		eventType = EventCreated
	case event.Op&fsnotify.Write != 0:
		eventType = EventModified
	case event.Op&fsnotify.Remove != 0:
		eventType = EventDeleted
	case event.Op&fsnotify.Rename != 0:
		eventType = EventRenamed
	default:
		return
	}

	fileEvent := FileEvent{
		Type: eventType,
		Path: event.Name,
		Name: filepath.Base(event.Name),
	}

	// Debounce the event
	w.pendingMu.Lock()
	defer w.pendingMu.Unlock()

	// Cancel any pending event for this file
	if timer, exists := w.pending[event.Name]; exists {
		timer.Stop()
	}

	// Schedule a new event
	w.pending[event.Name] = time.AfterFunc(w.debounce, func() {
		w.pendingMu.Lock()
		delete(w.pending, event.Name)
		w.pendingMu.Unlock()

		select {
		case w.events <- fileEvent:
		default:
			log.Warn().Str("path", event.Name).Msg("Event channel full, dropping event")
		}
	})
}

// dispatchLoop dispatches file events to registered handlers.
func (w *Watcher) dispatchLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.done:
			return
		case event := <-w.events:
			w.dispatchEvent(event)
		}
	}
}

// dispatchEvent finds matching handlers and calls them.
func (w *Watcher) dispatchEvent(event FileEvent) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Find all handlers that match this event
	for pattern, handlers := range w.handlers {
		if matchesPattern(event.Path, pattern) {
			for _, handler := range handlers {
				handler(event)
			}
		}
	}
}

// matchesPattern checks if a path matches a pattern.
func matchesPattern(path, pattern string) bool {
	if path == pattern {
		return true
	}

	if strings.HasPrefix(path, pattern+string(filepath.Separator)) {
		return true
	}

	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}

	return filepath.Dir(path) == pattern
}

// SchemaWatcher is a specialized watcher for schema file changes.
type SchemaWatcher struct {
	watcher    *Watcher
	schemaPath string
	onChange   func(path string)
}

// NewSchemaWatcher creates a watcher for schema file changes.
const watchDebounce = 200 * time.Millisecond

func NewSchemaWatcher(schemaPath string, onChange func(path string)) (*SchemaWatcher, error) {
	w, err := NewWatcher(WithDebounce(watchDebounce))
	if err != nil {
		return nil, err
	}

	sw := &SchemaWatcher{
		watcher:    w,
		schemaPath: schemaPath,
		onChange:   onChange,
	}

	if err := w.Watch(schemaPath, func(event FileEvent) {
		if event.Type == EventModified || event.Type == EventCreated {
			log.Debug().Str("event", event.Type.String()).Str("path", event.Path).Msg("Schema file changed")
			if sw.onChange != nil {
				sw.onChange(event.Path)
			}
		}
	}); err != nil {
		_ = w.Stop()
		return nil, err
	}

	return sw, nil
}

// Start begins watching for schema changes.
func (sw *SchemaWatcher) Start(ctx context.Context) {
	sw.watcher.Start(ctx)
}

// Stop stops the schema watcher.
func (sw *SchemaWatcher) Stop() error {
	return sw.watcher.Stop()
}

// FunctionWatcher is a specialized watcher for function file changes.
type FunctionWatcher struct {
	watcher       *Watcher
	functionsPath string
	onChange      func(path string, eventType EventType)
}

// NewFunctionWatcher creates a watcher for function file changes.
func NewFunctionWatcher(functionsPath string, onChange func(path string, eventType EventType)) (*FunctionWatcher, error) {
	w, err := NewWatcher(WithDebounce(watchDebounce))
	if err != nil {
		return nil, err
	}

	fw := &FunctionWatcher{
		watcher:       w,
		functionsPath: functionsPath,
		onChange:      onChange,
	}

	if err := w.WatchDir(functionsPath, func(event FileEvent) {
		if isFunctionFile(event.Path) {
			log.Debug().
				Str("event", event.Type.String()).
				Str("path", event.Path).
				Msg("Function file changed")
			if fw.onChange != nil {
				fw.onChange(event.Path, event.Type)
			}
		}
	}); err != nil {
		_ = w.Stop()
		return nil, err
	}

	return fw, nil
}

// Start begins watching for function changes.
func (fw *FunctionWatcher) Start(ctx context.Context) {
	fw.watcher.Start(ctx)
}

// Stop stops the function watcher.
func (fw *FunctionWatcher) Stop() error {
	return fw.watcher.Stop()
}

// isFunctionFile returns true if the path is a function file.
func isFunctionFile(path string) bool {
	ext := filepath.Ext(path)
	switch ext {
	case ".js", ".ts", ".mjs", ".cjs":
		return true
	case ".py":
		return true
	case ".go":
		return true
	default:
		return false
	}
}

// DevWatcher combines schema and function watching for development mode.
type DevWatcher struct {
	schemaWatcher   *SchemaWatcher
	functionWatcher *FunctionWatcher
}

// DevWatcherConfig configures the development watcher.
type DevWatcherConfig struct {
	SchemaPath       string
	FunctionsPath    string
	OnSchemaChange   func(path string)
	OnFunctionChange func(path string, eventType EventType)
}

// NewDevWatcher creates a combined watcher for development mode.
func NewDevWatcher(cfg DevWatcherConfig) (*DevWatcher, error) {
	var schemaWatcher *SchemaWatcher
	var functionWatcher *FunctionWatcher
	var err error

	if cfg.SchemaPath != "" {
		schemaWatcher, err = NewSchemaWatcher(cfg.SchemaPath, cfg.OnSchemaChange)
		if err != nil {
			return nil, err
		}
	}

	if cfg.FunctionsPath != "" {
		functionWatcher, err = NewFunctionWatcher(cfg.FunctionsPath, cfg.OnFunctionChange)
		if err != nil {
			if schemaWatcher != nil {
				_ = schemaWatcher.Stop()
			}
			return nil, err
		}
	}

	return &DevWatcher{
		schemaWatcher:   schemaWatcher,
		functionWatcher: functionWatcher,
	}, nil
}

// Start begins all watchers.
func (dw *DevWatcher) Start(ctx context.Context) {
	if dw.schemaWatcher != nil {
		dw.schemaWatcher.Start(ctx)
	}
	if dw.functionWatcher != nil {
		dw.functionWatcher.Start(ctx)
	}
}

// Stop stops all watchers.
func (dw *DevWatcher) Stop() error {
	var err error
	if dw.schemaWatcher != nil {
		if stopErr := dw.schemaWatcher.Stop(); stopErr != nil {
			err = stopErr
		}
	}
	if dw.functionWatcher != nil {
		if stopErr := dw.functionWatcher.Stop(); stopErr != nil {
			err = stopErr
		}
	}
	return err
}
