package scheduler

import "time"

// ScheduleType represents the type of schedule.
type ScheduleType string

const (
	// ScheduleTypeCron represents a cron-based schedule.
	ScheduleTypeCron ScheduleType = "cron"
	// ScheduleTypeInterval represents an interval-based schedule.
	ScheduleTypeInterval ScheduleType = "interval"
	// ScheduleTypeOneTime represents a one-time scheduled execution.
	ScheduleTypeOneTime ScheduleType = "one_time"
)

// Schedule represents a scheduled function execution.
type Schedule struct {
	ID         string         // Unique schedule ID
	Name       string         // Schedule name
	FunctionID string         // Function to invoke
	Type       ScheduleType   // Schedule type (cron, interval, one_time)
	Expression string         // Schedule expression (cron, interval duration, or timestamp)
	Timezone   string         // Timezone for schedule (default "UTC")
	NextRun    *time.Time     // Next scheduled run time
	LastRun    *time.Time     // Last run time
	LastStatus string         // Last execution status
	Enabled    bool           // Whether schedule is enabled
	Config     ScheduleConfig // Schedule configuration
	CreatedAt  time.Time      // When schedule was created
	UpdatedAt  time.Time      // When schedule was last updated
}

// ScheduleConfig contains configuration for a schedule.
type ScheduleConfig struct {
	SkipIfRunning  bool // Skip execution if previous run still active
	MaxOverlap     int  // Maximum concurrent executions (0 = unlimited)
	RetryOnFailure bool // Retry if function fails
	MaxRetries     int  // Maximum retry attempts
	Input          any  // Static input for scheduled runs
}
