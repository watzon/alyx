package hooks

import "time"

type HookType string

const (
	HookTypeDatabase HookType = "database"
	HookTypeWebhook  HookType = "webhook"
	HookTypeSchedule HookType = "schedule"
)

type HookMode string

const (
	HookModeSync  HookMode = "sync"
	HookModeAsync HookMode = "async"
)

type DatabaseAction string

const (
	ActionInsert DatabaseAction = "insert"
	ActionUpdate DatabaseAction = "update"
	ActionDelete DatabaseAction = "delete"
)

type Hook struct {
	ID           string
	Type         HookType
	Source       string
	Action       string
	FunctionName string
	Mode         HookMode
	Enabled      bool
	Config       map[string]any
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
