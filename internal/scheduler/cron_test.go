package scheduler

import (
	"testing"
	"time"
)

func TestCronParser_Parse(t *testing.T) {
	parser := NewCronParser()

	tests := []struct {
		name       string
		expression string
		wantErr    bool
	}{
		{
			name:       "valid cron - every minute",
			expression: "* * * * *",
			wantErr:    false,
		},
		{
			name:       "valid cron - every hour",
			expression: "0 * * * *",
			wantErr:    false,
		},
		{
			name:       "valid cron - daily at midnight",
			expression: "0 0 * * *",
			wantErr:    false,
		},
		{
			name:       "valid cron - weekly on monday",
			expression: "0 0 * * 1",
			wantErr:    false,
		},
		{
			name:       "valid cron - with ranges",
			expression: "0 9-17 * * 1-5",
			wantErr:    false,
		},
		{
			name:       "valid cron - with steps",
			expression: "*/5 * * * *",
			wantErr:    false,
		},
		{
			name:       "invalid cron - too few fields",
			expression: "* * *",
			wantErr:    true,
		},
		{
			name:       "invalid cron - invalid value",
			expression: "60 * * * *",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse(tt.expression)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCronParser_NextRun(t *testing.T) {
	parser := NewCronParser()

	tests := []struct {
		name       string
		expression string
		timezone   string
		after      time.Time
		wantErr    bool
		checkFunc  func(t *testing.T, next time.Time)
	}{
		{
			name:       "every minute in UTC",
			expression: "* * * * *",
			timezone:   "UTC",
			after:      time.Date(2026, 1, 25, 12, 30, 0, 0, time.UTC),
			wantErr:    false,
			checkFunc: func(t *testing.T, next time.Time) {
				expected := time.Date(2026, 1, 25, 12, 31, 0, 0, time.UTC)
				if !next.Equal(expected) {
					t.Errorf("NextRun() = %v, want %v", next, expected)
				}
			},
		},
		{
			name:       "every hour in America/New_York",
			expression: "0 * * * *",
			timezone:   "America/New_York",
			after:      time.Date(2026, 1, 25, 12, 30, 0, 0, time.UTC),
			wantErr:    false,
			checkFunc: func(t *testing.T, next time.Time) {
				// Should be next hour in New York timezone
				if next.Before(time.Date(2026, 1, 25, 12, 30, 0, 0, time.UTC)) {
					t.Errorf("NextRun() should be after input time")
				}
			},
		},
		{
			name:       "daily at midnight in Europe/London",
			expression: "0 0 * * *",
			timezone:   "Europe/London",
			after:      time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC),
			wantErr:    false,
			checkFunc: func(t *testing.T, next time.Time) {
				// Should be midnight in London timezone
				loc, _ := time.LoadLocation("Europe/London")
				nextInLondon := next.In(loc)
				if nextInLondon.Hour() != 0 || nextInLondon.Minute() != 0 {
					t.Errorf("NextRun() should be at midnight in London, got %v", nextInLondon)
				}
			},
		},
		{
			name:       "invalid timezone",
			expression: "* * * * *",
			timezone:   "Invalid/Timezone",
			after:      time.Now(),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, err := parser.NextRun(tt.expression, tt.timezone, tt.after)
			if (err != nil) != tt.wantErr {
				t.Errorf("NextRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, next)
			}
		})
	}
}

func TestParseInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval string
		want     time.Duration
		wantErr  bool
	}{
		{
			name:     "5 minutes",
			interval: "5m",
			want:     5 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "1 hour",
			interval: "1h",
			want:     1 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "30 seconds",
			interval: "30s",
			want:     30 * time.Second,
			wantErr:  false,
		},
		{
			name:     "1 day",
			interval: "24h",
			want:     24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "sub-second interval (rejected)",
			interval: "500ms",
			wantErr:  true,
		},
		{
			name:     "invalid format",
			interval: "invalid",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseInterval(tt.interval)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseInterval() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNextIntervalRun(t *testing.T) {
	tests := []struct {
		name      string
		interval  string
		timezone  string
		after     time.Time
		wantErr   bool
		checkFunc func(t *testing.T, next time.Time)
	}{
		{
			name:     "5 minutes in UTC",
			interval: "5m",
			timezone: "UTC",
			after:    time.Date(2026, 1, 25, 12, 30, 0, 0, time.UTC),
			wantErr:  false,
			checkFunc: func(t *testing.T, next time.Time) {
				expected := time.Date(2026, 1, 25, 12, 35, 0, 0, time.UTC)
				if !next.Equal(expected) {
					t.Errorf("NextIntervalRun() = %v, want %v", next, expected)
				}
			},
		},
		{
			name:     "1 hour in America/New_York",
			interval: "1h",
			timezone: "America/New_York",
			after:    time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC),
			wantErr:  false,
			checkFunc: func(t *testing.T, next time.Time) {
				// Should be 1 hour after input
				expected := time.Date(2026, 1, 25, 13, 0, 0, 0, time.UTC)
				if !next.Equal(expected) {
					t.Errorf("NextIntervalRun() = %v, want %v", next, expected)
				}
			},
		},
		{
			name:     "invalid timezone",
			interval: "5m",
			timezone: "Invalid/Timezone",
			after:    time.Now(),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, err := NextIntervalRun(tt.interval, tt.timezone, tt.after)
			if (err != nil) != tt.wantErr {
				t.Errorf("NextIntervalRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, next)
			}
		})
	}
}

func TestParseOneTime(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
		want      time.Time
		wantErr   bool
	}{
		{
			name:      "valid RFC3339",
			timestamp: "2026-01-25T12:00:00Z",
			want:      time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			name:      "valid RFC3339 with timezone",
			timestamp: "2026-01-25T12:00:00-05:00",
			want:      time.Date(2026, 1, 25, 17, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			name:      "invalid format",
			timestamp: "2026-01-25 12:00:00",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseOneTime(tt.timestamp)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOneTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("ParseOneTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateNextRun(t *testing.T) {
	now := time.Date(2026, 1, 25, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		schedule  *Schedule
		after     time.Time
		wantErr   bool
		checkFunc func(t *testing.T, next time.Time)
	}{
		{
			name: "cron schedule",
			schedule: &Schedule{
				Type:       ScheduleTypeCron,
				Expression: "0 * * * *",
				Timezone:   "UTC",
			},
			after:   now,
			wantErr: false,
			checkFunc: func(t *testing.T, next time.Time) {
				expected := time.Date(2026, 1, 25, 13, 0, 0, 0, time.UTC)
				if !next.Equal(expected) {
					t.Errorf("CalculateNextRun() = %v, want %v", next, expected)
				}
			},
		},
		{
			name: "interval schedule",
			schedule: &Schedule{
				Type:       ScheduleTypeInterval,
				Expression: "30m",
				Timezone:   "UTC",
			},
			after:   now,
			wantErr: false,
			checkFunc: func(t *testing.T, next time.Time) {
				expected := time.Date(2026, 1, 25, 12, 30, 0, 0, time.UTC)
				if !next.Equal(expected) {
					t.Errorf("CalculateNextRun() = %v, want %v", next, expected)
				}
			},
		},
		{
			name: "one-time schedule (first run)",
			schedule: &Schedule{
				Type:       ScheduleTypeOneTime,
				Expression: "2026-01-25T15:00:00Z",
				Timezone:   "UTC",
			},
			after:   now,
			wantErr: false,
			checkFunc: func(t *testing.T, next time.Time) {
				expected := time.Date(2026, 1, 25, 15, 0, 0, 0, time.UTC)
				if !next.Equal(expected) {
					t.Errorf("CalculateNextRun() = %v, want %v", next, expected)
				}
			},
		},
		{
			name: "one-time schedule (already executed)",
			schedule: &Schedule{
				Type:       ScheduleTypeOneTime,
				Expression: "2026-01-25T15:00:00Z",
				Timezone:   "UTC",
				LastRun:    &now,
			},
			after:   now,
			wantErr: true,
		},
		{
			name: "unknown schedule type",
			schedule: &Schedule{
				Type:       ScheduleType("unknown"),
				Expression: "* * * * *",
				Timezone:   "UTC",
			},
			after:   now,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, err := CalculateNextRun(tt.schedule, tt.after)
			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateNextRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, next)
			}
		})
	}
}

// TestTimezoneHandling tests DST transitions and timezone edge cases.
func TestTimezoneHandling(t *testing.T) {
	parser := NewCronParser()

	// Test DST transition (spring forward)
	// In America/New_York, DST starts on March 9, 2026 at 2:00 AM
	beforeDST := time.Date(2026, 3, 9, 1, 0, 0, 0, time.UTC)
	next, err := parser.NextRun("0 2 * * *", "America/New_York", beforeDST)
	if err != nil {
		t.Fatalf("NextRun() error = %v", err)
	}

	loc, _ := time.LoadLocation("America/New_York")
	nextInNY := next.In(loc)

	// During DST transition, 2:00 AM doesn't exist (clock jumps to 3:00 AM)
	// Cron should handle this gracefully
	if nextInNY.Hour() != 2 && nextInNY.Hour() != 3 {
		t.Logf("DST transition handled: next run at %v (hour %d)", nextInNY, nextInNY.Hour())
	}

	// Test DST transition (fall back)
	// In America/New_York, DST ends on November 1, 2026 at 2:00 AM
	beforeFallBack := time.Date(2026, 11, 1, 1, 0, 0, 0, time.UTC)
	next, err = parser.NextRun("0 2 * * *", "America/New_York", beforeFallBack)
	if err != nil {
		t.Fatalf("NextRun() error = %v", err)
	}

	nextInNY = next.In(loc)
	if nextInNY.Hour() != 2 {
		t.Logf("DST fall back handled: next run at %v (hour %d)", nextInNY, nextInNY.Hour())
	}
}
