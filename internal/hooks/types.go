package hooks

import "time"

// HookMode represents the execution mode for a hook.
type HookMode string

const (
	// HookModeSync executes the hook synchronously and waits for completion.
	HookModeSync HookMode = "sync"
	// HookModeAsync executes the hook asynchronously in the background.
	HookModeAsync HookMode = "async"
)

// Hook represents a function hook registration.
type Hook struct {
	ID          string     // Unique hook ID
	Name        string     // Hook name
	FunctionID  string     // Function to invoke
	EventType   string     // Event type to match (from events.EventType)
	EventSource string     // Event source to match (wildcard "*" supported)
	EventAction string     // Event action to match (wildcard "*" supported)
	Mode        HookMode   // Execution mode (sync or async)
	Priority    int        // Execution priority (higher = earlier)
	Config      HookConfig // Hook configuration
	Enabled     bool       // Whether hook is enabled
	CreatedAt   time.Time  // When hook was created
	UpdatedAt   time.Time  // When hook was last updated
}

// HookConfig contains configuration for a hook.
type HookConfig struct {
	OnFailure string        // What to do on failure: "reject" or "continue"
	Timeout   time.Duration // Timeout for sync hooks (default 5s)
}
