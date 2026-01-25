package scheduler

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// CronParser wraps robfig/cron for parsing cron expressions.
type CronParser struct {
	parser cron.Parser
}

// NewCronParser creates a new cron parser with standard options.
func NewCronParser() *CronParser {
	return &CronParser{
		parser: cron.NewParser(
			cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
		),
	}
}

// Parse parses a cron expression and returns a schedule.
func (p *CronParser) Parse(expression string) (cron.Schedule, error) {
	schedule, err := p.parser.Parse(expression)
	if err != nil {
		return nil, fmt.Errorf("parsing cron expression: %w", err)
	}
	return schedule, nil
}

// NextRun calculates the next run time for a cron expression in a specific timezone.
func (p *CronParser) NextRun(expression, timezone string, after time.Time) (time.Time, error) {
	schedule, err := p.Parse(expression)
	if err != nil {
		return time.Time{}, err
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("loading timezone: %w", err)
	}

	// Convert after to the target timezone
	afterInTZ := after.In(loc)

	// Get next run time
	next := schedule.Next(afterInTZ)

	return next, nil
}

// ParseInterval parses an interval duration string (e.g., "5m", "1h", "30s").
func ParseInterval(interval string) (time.Duration, error) {
	duration, err := time.ParseDuration(interval)
	if err != nil {
		return 0, fmt.Errorf("parsing interval: %w", err)
	}

	// Disallow sub-second intervals
	if duration < time.Second {
		return 0, fmt.Errorf("interval must be at least 1 second")
	}

	return duration, nil
}

// NextIntervalRun calculates the next run time for an interval schedule.
func NextIntervalRun(interval string, timezone string, after time.Time) (time.Time, error) {
	duration, err := ParseInterval(interval)
	if err != nil {
		return time.Time{}, err
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("loading timezone: %w", err)
	}

	// Convert after to the target timezone
	afterInTZ := after.In(loc)

	// Add interval to after
	next := afterInTZ.Add(duration)

	return next, nil
}

// ParseOneTime parses a one-time schedule timestamp (RFC3339 format).
func ParseOneTime(timestamp string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing one-time timestamp: %w", err)
	}
	return t, nil
}

// CalculateNextRun calculates the next run time for any schedule type.
func CalculateNextRun(schedule *Schedule, after time.Time) (time.Time, error) {
	switch schedule.Type {
	case ScheduleTypeCron:
		parser := NewCronParser()
		return parser.NextRun(schedule.Expression, schedule.Timezone, after)

	case ScheduleTypeInterval:
		return NextIntervalRun(schedule.Expression, schedule.Timezone, after)

	case ScheduleTypeOneTime:
		// One-time schedules don't have a "next" run after the first execution
		if schedule.LastRun != nil {
			return time.Time{}, fmt.Errorf("one-time schedule already executed")
		}
		return ParseOneTime(schedule.Expression)

	default:
		return time.Time{}, fmt.Errorf("unknown schedule type: %s", schedule.Type)
	}
}
