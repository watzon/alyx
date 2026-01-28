// Package functions provides serverless function execution.
package functions

// BuildConfig represents build configuration for compiled functions.
type BuildConfig struct {
	Command string   `yaml:"command" json:"command"` // e.g., "extism-js"
	Args    []string `yaml:"args" json:"args"`       // e.g., ["src/index.js", "-o", "plugin.wasm"]
	Watch   []string `yaml:"watch" json:"watch"`     // e.g., ["src/**/*.js"]
	Output  string   `yaml:"output" json:"output"`   // e.g., "plugin.wasm"
}

// RouteConfig represents an HTTP route configuration.
type RouteConfig struct {
	Path    string   `yaml:"path" json:"path"`
	Methods []string `yaml:"methods" json:"methods"`
}

// HookConfig represents a hook configuration.
type HookConfig struct {
	Type         string              `yaml:"type" json:"type"`
	Source       string              `yaml:"source" json:"source"`
	Action       string              `yaml:"action" json:"action"`
	Mode         string              `yaml:"mode" json:"mode"`
	Config       map[string]any      `yaml:"config" json:"config"`
	Verification *VerificationConfig `yaml:"verification" json:"verification,omitempty"`
}

// ScheduleConfig represents a schedule configuration.
type ScheduleConfig struct {
	Name       string         `yaml:"name" json:"name"`
	Type       string         `yaml:"type" json:"type"`
	Expression string         `yaml:"expression" json:"expression"`
	Timezone   string         `yaml:"timezone" json:"timezone"`
	Config     map[string]any `yaml:"config" json:"config"`
	Input      map[string]any `yaml:"input" json:"input"`
}

// VerificationConfig represents webhook verification configuration.
type VerificationConfig struct {
	Type   string `yaml:"type" json:"type"`
	Header string `yaml:"header" json:"header"`
	Secret string `yaml:"secret" json:"secret"`
}
