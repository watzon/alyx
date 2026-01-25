// Package functions provides serverless function execution via containers.
package functions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gobwas/glob"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

const (
	defaultDebounceDuration = 100 * time.Millisecond
)

// SourceWatcher watches source files and triggers builds when changes are detected.
type SourceWatcher struct {
	registry         *Registry
	watcher          *fsnotify.Watcher
	debounceDuration time.Duration
	debounceTimers   map[string]*time.Timer
	mu               sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
}

// NewSourceWatcher creates a new source file watcher.
func NewSourceWatcher(registry *Registry) (*SourceWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating fsnotify watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &SourceWatcher{
		registry:         registry,
		watcher:          watcher,
		debounceDuration: defaultDebounceDuration,
		debounceTimers:   make(map[string]*time.Timer),
		ctx:              ctx,
		cancel:           cancel,
	}, nil
}

// SetDebounceDuration sets the debounce duration for file change events.
func (sw *SourceWatcher) SetDebounceDuration(d time.Duration) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.debounceDuration = d
}

// Start begins watching source files for all functions with build configurations.
func (sw *SourceWatcher) Start() error {
	functions := sw.registry.List()

	for _, fn := range functions {
		funcDir := filepath.Dir(fn.Path)

		manifestPath := filepath.Join(funcDir, "manifest.yaml")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}

		manifest, err := sw.loadManifest(manifestPath)
		if err != nil {
			log.Warn().Err(err).Str("function", fn.Name).Msg("Failed to load manifest")
			continue
		}

		if manifest.Build == nil {
			continue
		}

		if err := sw.addWatchPatterns(funcDir, manifest.Build.Watch); err != nil {
			log.Warn().Err(err).Str("function", fn.Name).Msg("Failed to add watch patterns")
			continue
		}

		log.Debug().
			Str("function", fn.Name).
			Strs("patterns", manifest.Build.Watch).
			Msg("Watching source files")
	}

	sw.wg.Add(1)
	go sw.eventLoop()

	return nil
}

// Stop stops the watcher and cleans up resources.
func (sw *SourceWatcher) Stop() error {
	sw.cancel()
	sw.wg.Wait()

	sw.mu.Lock()
	for _, timer := range sw.debounceTimers {
		timer.Stop()
	}
	sw.mu.Unlock()

	return sw.watcher.Close()
}

func (sw *SourceWatcher) loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("validating manifest: %w", err)
	}

	return &manifest, nil
}

func (sw *SourceWatcher) addWatchPatterns(funcDir string, patterns []string) error {
	if len(patterns) == 0 {
		return sw.watcher.Add(funcDir)
	}

	for _, pattern := range patterns {
		absPattern := filepath.Join(funcDir, pattern)
		baseDir := extractBaseDir(absPattern)

		if err := sw.watcher.Add(baseDir); err != nil {
			return fmt.Errorf("adding watch for %s: %w", baseDir, err)
		}
	}

	return nil
}

func extractBaseDir(pattern string) string {
	dir := filepath.Dir(pattern)
	for {
		if !containsWildcard(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dir
}

func containsWildcard(path string) bool {
	return filepath.Base(path) == "**" ||
		filepath.Base(path) == "*" ||
		containsChar(path, '*') ||
		containsChar(path, '?') ||
		containsChar(path, '[')
}

func containsChar(s string, c rune) bool {
	for _, r := range s {
		if r == c {
			return true
		}
	}
	return false
}

func (sw *SourceWatcher) eventLoop() {
	defer sw.wg.Done()

	for {
		select {
		case <-sw.ctx.Done():
			return

		case event, ok := <-sw.watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Write == fsnotify.Write ||
				event.Op&fsnotify.Create == fsnotify.Create {
				sw.handleEvent(event)
			}

		case err, ok := <-sw.watcher.Errors:
			if !ok {
				return
			}
			log.Error().Err(err).Msg("File watcher error")
		}
	}
}

func (sw *SourceWatcher) handleEvent(event fsnotify.Event) {
	fn, patterns := sw.findFunctionForFile(event.Name)
	if fn == nil {
		return
	}

	if !sw.matchesPattern(event.Name, filepath.Dir(fn.Path), patterns) {
		return
	}

	log.Debug().
		Str("file", event.Name).
		Str("function", fn.Name).
		Msg("Source file changed")

	sw.debounceBuild(fn)
}

func (sw *SourceWatcher) findFunctionForFile(filePath string) (*FunctionDef, []string) {
	functions := sw.registry.List()

	for _, fn := range functions {
		funcDir := filepath.Dir(fn.Path)

		relPath, err := filepath.Rel(funcDir, filePath)
		if err != nil || filepath.IsAbs(relPath) || len(relPath) > 0 && relPath[0] == '.' {
			continue
		}

		manifestPath := filepath.Join(funcDir, "manifest.yaml")
		manifest, err := sw.loadManifest(manifestPath)
		if err != nil || manifest.Build == nil {
			continue
		}

		return fn, manifest.Build.Watch
	}

	return nil, nil
}

func (sw *SourceWatcher) matchesPattern(filePath, funcDir string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}

	relPath, err := filepath.Rel(funcDir, filePath)
	if err != nil {
		return false
	}

	for _, pattern := range patterns {
		matcher, err := glob.Compile(pattern, '/')
		if err != nil {
			log.Warn().Err(err).Str("pattern", pattern).Msg("Invalid glob pattern")
			continue
		}

		if matcher.Match(relPath) {
			return true
		}
	}

	return false
}

func (sw *SourceWatcher) debounceBuild(fn *FunctionDef) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if timer, exists := sw.debounceTimers[fn.Name]; exists {
		timer.Stop()
	}

	sw.debounceTimers[fn.Name] = time.AfterFunc(sw.debounceDuration, func() {
		sw.executeBuild(fn)
	})
}

func (sw *SourceWatcher) executeBuild(fn *FunctionDef) {
	funcDir := filepath.Dir(fn.Path)
	manifestPath := filepath.Join(funcDir, "manifest.yaml")

	manifest, err := sw.loadManifest(manifestPath)
	if err != nil {
		log.Error().Err(err).Str("function", fn.Name).Msg("Failed to load manifest for build")
		return
	}

	if manifest.Build == nil {
		return
	}

	log.Info().
		Str("function", fn.Name).
		Str("command", manifest.Build.Command).
		Msg("Building function")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, manifest.Build.Command, manifest.Build.Args...)
	cmd.Dir = funcDir

	output, err := cmd.CombinedOutput()

	if err != nil {
		log.Error().
			Err(err).
			Str("function", fn.Name).
			Str("output", string(output)).
			Msg("Build failed")
		return
	}

	log.Info().
		Str("function", fn.Name).
		Str("output", string(output)).
		Msg("Build succeeded")
}
