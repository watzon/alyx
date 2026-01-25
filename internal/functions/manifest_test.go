package functions

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestManifest_Validate(t *testing.T) {
	tests := []struct {
		name      string
		manifest  *Manifest
		expectErr bool
	}{
		{
			name: "valid minimal manifest",
			manifest: &Manifest{
				Name:    "test-function",
				Runtime: "node",
			},
			expectErr: false,
		},
		{
			name: "valid full manifest",
			manifest: &Manifest{
				Name:    "test-function",
				Runtime: "node",
				Timeout: "30s",
				Memory:  "256mb",
				Routes: []RouteConfig{
					{Path: "/api/test", Methods: []string{"GET", "POST"}},
				},
				Hooks: []HookConfig{
					{Type: "database", Source: "users", Action: "insert", Mode: "async"},
				},
				Schedules: []ScheduleConfig{
					{Name: "daily", Type: "cron", Expression: "0 0 * * *"},
				},
			},
			expectErr: false,
		},
		{
			name: "missing name",
			manifest: &Manifest{
				Runtime: "node",
			},
			expectErr: true,
		},
		{
			name: "invalid runtime",
			manifest: &Manifest{
				Name:    "test",
				Runtime: "ruby",
			},
			expectErr: true,
		},
		{
			name: "invalid timeout",
			manifest: &Manifest{
				Name:    "test",
				Timeout: "invalid",
			},
			expectErr: true,
		},
		{
			name: "invalid memory",
			manifest: &Manifest{
				Name:   "test",
				Memory: "invalid",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRouteConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		route     *RouteConfig
		expectErr bool
	}{
		{
			name:      "valid route",
			route:     &RouteConfig{Path: "/api/test", Methods: []string{"GET"}},
			expectErr: false,
		},
		{
			name:      "multiple methods",
			route:     &RouteConfig{Path: "/api/test", Methods: []string{"GET", "POST", "PUT"}},
			expectErr: false,
		},
		{
			name:      "missing path",
			route:     &RouteConfig{Methods: []string{"GET"}},
			expectErr: true,
		},
		{
			name:      "path without leading slash",
			route:     &RouteConfig{Path: "api/test", Methods: []string{"GET"}},
			expectErr: true,
		},
		{
			name:      "missing methods",
			route:     &RouteConfig{Path: "/api/test"},
			expectErr: true,
		},
		{
			name:      "invalid method",
			route:     &RouteConfig{Path: "/api/test", Methods: []string{"INVALID"}},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.route.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestHookConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		hook      *HookConfig
		expectErr bool
	}{
		{
			name:      "valid database hook",
			hook:      &HookConfig{Type: "database", Source: "users", Action: "insert", Mode: "async"},
			expectErr: false,
		},
		{
			name:      "valid auth hook",
			hook:      &HookConfig{Type: "auth", Source: "signup", Action: "after", Mode: "sync"},
			expectErr: false,
		},
		{
			name: "valid webhook hook",
			hook: &HookConfig{
				Type: "webhook",
				Verification: &VerificationConfig{
					Type:   "hmac-sha256",
					Header: "X-Signature",
					Secret: "secret",
				},
			},
			expectErr: false,
		},
		{
			name:      "missing type",
			hook:      &HookConfig{Source: "users", Action: "insert"},
			expectErr: true,
		},
		{
			name:      "invalid type",
			hook:      &HookConfig{Type: "invalid", Source: "users", Action: "insert"},
			expectErr: true,
		},
		{
			name:      "database hook missing source",
			hook:      &HookConfig{Type: "database", Action: "insert"},
			expectErr: true,
		},
		{
			name:      "database hook missing action",
			hook:      &HookConfig{Type: "database", Source: "users"},
			expectErr: true,
		},
		{
			name:      "webhook hook missing verification",
			hook:      &HookConfig{Type: "webhook"},
			expectErr: true,
		},
		{
			name:      "invalid mode",
			hook:      &HookConfig{Type: "database", Source: "users", Action: "insert", Mode: "invalid"},
			expectErr: true,
		},
		{
			name: "invalid timeout in config",
			hook: &HookConfig{
				Type:   "database",
				Source: "users",
				Action: "insert",
				Config: map[string]any{"timeout": "invalid"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.hook.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestScheduleConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		schedule  *ScheduleConfig
		expectErr bool
	}{
		{
			name:      "valid cron schedule",
			schedule:  &ScheduleConfig{Name: "daily", Type: "cron", Expression: "0 0 * * *"},
			expectErr: false,
		},
		{
			name:      "valid interval schedule",
			schedule:  &ScheduleConfig{Name: "every-5m", Type: "interval", Expression: "5m"},
			expectErr: false,
		},
		{
			name:      "valid one_time schedule",
			schedule:  &ScheduleConfig{Name: "once", Type: "one_time", Expression: time.Now().Add(time.Hour).Format(time.RFC3339)},
			expectErr: false,
		},
		{
			name:      "valid with timezone",
			schedule:  &ScheduleConfig{Name: "daily", Type: "cron", Expression: "0 0 * * *", Timezone: "America/New_York"},
			expectErr: false,
		},
		{
			name:      "missing name",
			schedule:  &ScheduleConfig{Type: "cron", Expression: "0 0 * * *"},
			expectErr: true,
		},
		{
			name:      "missing type",
			schedule:  &ScheduleConfig{Name: "daily", Expression: "0 0 * * *"},
			expectErr: true,
		},
		{
			name:      "invalid type",
			schedule:  &ScheduleConfig{Name: "daily", Type: "invalid", Expression: "0 0 * * *"},
			expectErr: true,
		},
		{
			name:      "missing expression",
			schedule:  &ScheduleConfig{Name: "daily", Type: "cron"},
			expectErr: true,
		},
		{
			name:      "invalid interval expression",
			schedule:  &ScheduleConfig{Name: "test", Type: "interval", Expression: "invalid"},
			expectErr: true,
		},
		{
			name:      "invalid one_time expression",
			schedule:  &ScheduleConfig{Name: "test", Type: "one_time", Expression: "invalid"},
			expectErr: true,
		},
		{
			name:      "invalid timezone",
			schedule:  &ScheduleConfig{Name: "daily", Type: "cron", Expression: "0 0 * * *", Timezone: "Invalid/Timezone"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schedule.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestVerificationConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		verify    *VerificationConfig
		expectErr bool
	}{
		{
			name:      "valid hmac-sha256",
			verify:    &VerificationConfig{Type: "hmac-sha256", Header: "X-Signature", Secret: "secret"},
			expectErr: false,
		},
		{
			name:      "valid hmac-sha1",
			verify:    &VerificationConfig{Type: "hmac-sha1", Header: "X-Hub-Signature", Secret: "secret"},
			expectErr: false,
		},
		{
			name:      "missing type",
			verify:    &VerificationConfig{Header: "X-Signature", Secret: "secret"},
			expectErr: true,
		},
		{
			name:      "invalid type",
			verify:    &VerificationConfig{Type: "invalid", Header: "X-Signature", Secret: "secret"},
			expectErr: true,
		},
		{
			name:      "missing header",
			verify:    &VerificationConfig{Type: "hmac-sha256", Secret: "secret"},
			expectErr: true,
		},
		{
			name:      "missing secret",
			verify:    &VerificationConfig{Type: "hmac-sha256", Header: "X-Signature"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.verify.Validate()
			if tt.expectErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestManifest_YAML_Parsing(t *testing.T) {
	yamlContent := `
name: on-user-created
runtime: node
timeout: 30s
memory: 256mb

routes:
  - path: /api/users/sync
    methods: [POST]
  - path: /api/users/{id}/avatar
    methods: [GET, PUT]

hooks:
  - type: database
    source: users
    action: insert
    mode: async
    
  - type: auth
    source: signup
    action: after
    mode: sync
    config:
      on_failure: reject
      timeout: 5s
    
  - type: webhook
    verification:
      type: hmac-sha256
      header: Stripe-Signature
      secret: ${STRIPE_WEBHOOK_SECRET}

schedules:
  - name: daily-cleanup
    type: cron
    expression: "0 2 * * *"
    timezone: America/New_York
    config:
      skip_if_running: true
      
  - name: health-check
    type: interval
    expression: "5m"

env:
  OPENAI_API_KEY: ${OPENAI_API_KEY}
`

	var manifest Manifest
	if err := yaml.Unmarshal([]byte(yamlContent), &manifest); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	if manifest.Name != "on-user-created" {
		t.Errorf("expected name 'on-user-created', got %s", manifest.Name)
	}

	if manifest.Runtime != "node" {
		t.Errorf("expected runtime 'node', got %s", manifest.Runtime)
	}

	if len(manifest.Routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(manifest.Routes))
	}

	if len(manifest.Hooks) != 3 {
		t.Errorf("expected 3 hooks, got %d", len(manifest.Hooks))
	}

	if len(manifest.Schedules) != 2 {
		t.Errorf("expected 2 schedules, got %d", len(manifest.Schedules))
	}

	if manifest.Env["OPENAI_API_KEY"] != "${OPENAI_API_KEY}" {
		t.Errorf("expected env var, got %s", manifest.Env["OPENAI_API_KEY"])
	}
}

func TestManifest_BackwardCompatibility(t *testing.T) {
	legacyYAML := `
name: legacy-function
runtime: python
timeout: 60s
memory: 512mb
env:
  API_KEY: secret
dependencies:
  - requests
  - flask
`

	var manifest Manifest
	if err := yaml.Unmarshal([]byte(legacyYAML), &manifest); err != nil {
		t.Fatalf("failed to parse legacy YAML: %v", err)
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	if manifest.Name != "legacy-function" {
		t.Errorf("expected name 'legacy-function', got %s", manifest.Name)
	}

	if len(manifest.Routes) != 0 {
		t.Errorf("expected 0 routes, got %d", len(manifest.Routes))
	}

	if len(manifest.Hooks) != 0 {
		t.Errorf("expected 0 hooks, got %d", len(manifest.Hooks))
	}

	if len(manifest.Schedules) != 0 {
		t.Errorf("expected 0 schedules, got %d", len(manifest.Schedules))
	}
}
