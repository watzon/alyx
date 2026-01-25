// Package functions provides serverless function execution via containers.
package functions

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Manifest represents an enhanced function manifest with hooks, schedules, and routes.
type Manifest struct {
	Name         string            `yaml:"name"`
	Runtime      string            `yaml:"runtime"`
	Timeout      string            `yaml:"timeout"`
	Memory       string            `yaml:"memory"`
	Env          map[string]string `yaml:"env"`
	Dependencies []string          `yaml:"dependencies"`
	Routes       []RouteConfig     `yaml:"routes"`
	Hooks        []HookConfig      `yaml:"hooks"`
	Schedules    []ScheduleConfig  `yaml:"schedules"`
	Build        *BuildConfig      `yaml:"build"`
}

// BuildConfig represents build configuration for WASM functions.
type BuildConfig struct {
	Command string   `yaml:"command"` // e.g., "extism-js"
	Args    []string `yaml:"args"`    // e.g., ["src/index.js", "-o", "plugin.wasm"]
	Watch   []string `yaml:"watch"`   // e.g., ["src/**/*.js"]
	Output  string   `yaml:"output"`  // e.g., "plugin.wasm"
}

// RouteConfig represents an HTTP route configuration.
type RouteConfig struct {
	Path    string   `yaml:"path"`
	Methods []string `yaml:"methods"`
}

// HookConfig represents a hook configuration.
type HookConfig struct {
	Type         string              `yaml:"type"`
	Source       string              `yaml:"source"`
	Action       string              `yaml:"action"`
	Mode         string              `yaml:"mode"`
	Config       map[string]any      `yaml:"config"`
	Verification *VerificationConfig `yaml:"verification"`
}

// ScheduleConfig represents a schedule configuration.
type ScheduleConfig struct {
	Name       string         `yaml:"name"`
	Type       string         `yaml:"type"`
	Expression string         `yaml:"expression"`
	Timezone   string         `yaml:"timezone"`
	Config     map[string]any `yaml:"config"`
}

// VerificationConfig represents webhook verification configuration.
type VerificationConfig struct {
	Type   string `yaml:"type"`
	Header string `yaml:"header"`
	Secret string `yaml:"secret"`
}

// Validate validates the manifest structure.
//
//nolint:gocyclo // Validation logic is inherently sequential
func (m *Manifest) Validate() error {
	if m.Name == "" {
		return errors.New("manifest: name is required")
	}

	if m.Runtime != "" {
		if err := validateRuntime(m.Runtime); err != nil {
			return err
		}
	}

	if m.Timeout != "" {
		if parseTimeoutSeconds(m.Timeout) == 0 {
			return fmt.Errorf("manifest: invalid timeout format: %s", m.Timeout)
		}
	}

	if m.Memory != "" {
		if parseMemoryMB(m.Memory) == 0 {
			return fmt.Errorf("manifest: invalid memory format: %s", m.Memory)
		}
	}

	for i, route := range m.Routes {
		if err := route.Validate(); err != nil {
			return fmt.Errorf("manifest: routes[%d]: %w", i, err)
		}
	}

	for i, hook := range m.Hooks {
		if err := hook.Validate(); err != nil {
			return fmt.Errorf("manifest: hooks[%d]: %w", i, err)
		}
	}

	for i, schedule := range m.Schedules {
		if err := schedule.Validate(); err != nil {
			return fmt.Errorf("manifest: schedules[%d]: %w", i, err)
		}
	}

	if m.Build != nil {
		if err := m.Build.Validate(); err != nil {
			return fmt.Errorf("manifest: build: %w", err)
		}
	}

	return nil
}

// Validate validates the route configuration.
func (r *RouteConfig) Validate() error {
	if r.Path == "" {
		return errors.New("path is required")
	}

	if !strings.HasPrefix(r.Path, "/") {
		return fmt.Errorf("path must start with /: %s", r.Path)
	}

	if len(r.Methods) == 0 {
		return errors.New("at least one method is required")
	}

	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "PATCH": true,
		"DELETE": true, "HEAD": true, "OPTIONS": true,
	}

	for _, method := range r.Methods {
		method = strings.ToUpper(method)
		if !validMethods[method] {
			return fmt.Errorf("invalid HTTP method: %s", method)
		}
	}

	return nil
}

// Validate validates the hook configuration.
func (h *HookConfig) Validate() error {
	if h.Type == "" {
		return errors.New("type is required")
	}

	validTypes := map[string]bool{
		"database": true,
		"auth":     true,
		"webhook":  true,
	}

	if !validTypes[h.Type] {
		return fmt.Errorf("invalid hook type: %s (must be database, auth, or webhook)", h.Type)
	}

	// Database and auth hooks require source and action
	if h.Type == "database" || h.Type == "auth" {
		if h.Source == "" {
			return fmt.Errorf("%s hook requires source", h.Type)
		}
		if h.Action == "" {
			return fmt.Errorf("%s hook requires action", h.Type)
		}
	}

	// Webhook hooks require verification config
	if h.Type == "webhook" {
		if h.Verification == nil {
			return errors.New("webhook hook requires verification config")
		}
		if err := h.Verification.Validate(); err != nil {
			return fmt.Errorf("verification: %w", err)
		}
	}

	if h.Mode != "" {
		validModes := map[string]bool{
			"sync":  true,
			"async": true,
		}
		if !validModes[h.Mode] {
			return fmt.Errorf("invalid mode: %s (must be sync or async)", h.Mode)
		}
	}

	// Validate timeout if present in config
	if h.Config != nil {
		if timeout, ok := h.Config["timeout"].(string); ok {
			if _, err := time.ParseDuration(timeout); err != nil {
				return fmt.Errorf("invalid timeout duration: %s", timeout)
			}
		}
	}

	return nil
}

// Validate validates the schedule configuration.
func (s *ScheduleConfig) Validate() error {
	if s.Name == "" {
		return errors.New("name is required")
	}

	if s.Type == "" {
		return errors.New("type is required")
	}

	validTypes := map[string]bool{
		"cron":     true,
		"interval": true,
		"one_time": true,
	}

	if !validTypes[s.Type] {
		return fmt.Errorf("invalid schedule type: %s (must be cron, interval, or one_time)", s.Type)
	}

	if s.Expression == "" {
		return errors.New("expression is required")
	}

	// Validate expression format based on type
	switch s.Type {
	case "interval":
		if _, err := time.ParseDuration(s.Expression); err != nil {
			return fmt.Errorf("invalid interval expression: %s", s.Expression)
		}
	case "one_time":
		if _, err := time.Parse(time.RFC3339, s.Expression); err != nil {
			return fmt.Errorf("invalid one_time expression (must be RFC3339): %s", s.Expression)
		}
	case "cron":
		// Cron validation is complex, defer to scheduler package
		// Just check it's not empty
		if strings.TrimSpace(s.Expression) == "" {
			return errors.New("cron expression cannot be empty")
		}
	}

	// Validate timezone if present
	if s.Timezone != "" {
		if _, err := time.LoadLocation(s.Timezone); err != nil {
			return fmt.Errorf("invalid timezone: %s", s.Timezone)
		}
	}

	return nil
}

// Validate validates the verification configuration.
func (v *VerificationConfig) Validate() error {
	if v.Type == "" {
		return errors.New("type is required")
	}

	validTypes := map[string]bool{
		"hmac-sha256": true,
		"hmac-sha1":   true,
	}

	if !validTypes[v.Type] {
		return fmt.Errorf("invalid verification type: %s (must be hmac-sha256 or hmac-sha1)", v.Type)
	}

	if v.Header == "" {
		return errors.New("header is required")
	}

	if v.Secret == "" {
		return errors.New("secret is required")
	}

	return nil
}

func validateRuntime(runtime string) error {
	validRuntimes := map[string]bool{
		"node":   true,
		"python": true,
		"go":     true,
		"deno":   true,
		"bun":    true,
	}

	if !validRuntimes[runtime] {
		return fmt.Errorf("invalid runtime: %s (must be node, python, go, deno, or bun)", runtime)
	}

	return nil
}

// Validate validates the build configuration.
func (b *BuildConfig) Validate() error {
	if b.Command == "" {
		return errors.New("command is required")
	}

	if b.Output == "" {
		return errors.New("output is required")
	}

	return nil
}
