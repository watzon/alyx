package scheduler

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

type RecoveryConfig struct {
	EnableCatchup bool
}

func DefaultRecoveryConfig() *RecoveryConfig {
	catchupEnabled := os.Getenv("ALYX_SCHEDULER_CATCHUP") == "true"
	return &RecoveryConfig{
		EnableCatchup: catchupEnabled,
	}
}

func (s *Scheduler) RecoverSchedules(ctx context.Context, config *RecoveryConfig) error {
	if config == nil {
		config = DefaultRecoveryConfig()
	}

	schedules, err := s.store.List(ctx)
	if err != nil {
		return fmt.Errorf("loading schedules from database: %w", err)
	}

	log.Info().
		Int("count", len(schedules)).
		Bool("catchup_enabled", config.EnableCatchup).
		Msg("Recovering schedules from database")

	for _, schedule := range schedules {
		if err := s.recoverSchedule(ctx, schedule, config); err != nil {
			log.Error().
				Err(err).
				Str("schedule_id", schedule.ID).
				Str("schedule_name", schedule.Name).
				Msg("Failed to recover schedule")
			continue
		}
	}

	return nil
}

func (s *Scheduler) recoverSchedule(ctx context.Context, schedule *Schedule, config *RecoveryConfig) error {
	if !schedule.Enabled {
		log.Debug().
			Str("schedule_id", schedule.ID).
			Str("schedule_name", schedule.Name).
			Msg("Skipping disabled schedule during recovery")
		return nil
	}

	state, err := s.stateStore.Get(ctx, schedule.ID)
	if err != nil {
		return fmt.Errorf("loading schedule state: %w", err)
	}

	if state == nil {
		state = &ScheduleState{
			ScheduleID:     schedule.ID,
			ExecutionCount: 0,
		}
	}

	now := time.Now().UTC()

	if schedule.NextRun != nil && schedule.NextRun.Before(now) {
		missedExecutions, err := s.calculateMissedExecutions(schedule, state, now)
		if err != nil {
			log.Warn().
				Err(err).
				Str("schedule_id", schedule.ID).
				Str("schedule_name", schedule.Name).
				Msg("Failed to calculate missed executions")
		} else if missedExecutions > 0 {
			log.Info().
				Str("schedule_id", schedule.ID).
				Str("schedule_name", schedule.Name).
				Int("missed_count", missedExecutions).
				Msg("Detected missed executions during downtime")

			if config.EnableCatchup {
				if err := s.executeCatchup(ctx, schedule, missedExecutions); err != nil {
					log.Error().
						Err(err).
						Str("schedule_id", schedule.ID).
						Str("schedule_name", schedule.Name).
						Msg("Failed to execute catch-up")
				}
			} else {
				log.Info().
					Str("schedule_id", schedule.ID).
					Str("schedule_name", schedule.Name).
					Msg("Catch-up disabled, skipping missed executions")

				nextRun, calcErr := CalculateNextRun(schedule, now)
				if calcErr == nil {
					schedule.NextRun = &nextRun
					if updateErr := s.store.UpdateNextRun(ctx, schedule.ID, nextRun, now); updateErr != nil {
						log.Error().
							Err(updateErr).
							Str("schedule_id", schedule.ID).
							Msg("Failed to update next_run during recovery")
					}

					state.NextExecutionAt = &nextRun
					state.LastExecutionAt = &now
					if saveErr := s.stateStore.Save(ctx, state); saveErr != nil {
						log.Error().
							Err(saveErr).
							Str("schedule_id", schedule.ID).
							Msg("Failed to save state during recovery")
					}
				}
			}
		}
	}

	if state.NextExecutionAt == nil && schedule.NextRun != nil {
		state.NextExecutionAt = schedule.NextRun
		if err := s.stateStore.Save(ctx, state); err != nil {
			return fmt.Errorf("saving initial state: %w", err)
		}
	}

	log.Debug().
		Str("schedule_id", schedule.ID).
		Str("schedule_name", schedule.Name).
		Time("next_run", *schedule.NextRun).
		Msg("Schedule recovered")

	return nil
}

func (s *Scheduler) calculateMissedExecutions(schedule *Schedule, state *ScheduleState, now time.Time) (int, error) {
	if schedule.NextRun == nil || !schedule.NextRun.Before(now) {
		return 0, nil
	}

	switch schedule.Type {
	case ScheduleTypeOneTime:
		if state.LastExecutionAt != nil {
			return 0, nil
		}
		return 1, nil

	case ScheduleTypeInterval:
		return 1, nil

	case ScheduleTypeCron:
		missedCount := 0
		currentTime := *schedule.NextRun
		parser := NewCronParser()

		for currentTime.Before(now) && missedCount < 1000 {
			missedCount++
			nextTime, err := parser.NextRun(schedule.Expression, schedule.Timezone, currentTime)
			if err != nil {
				return missedCount, fmt.Errorf("calculating next cron run: %w", err)
			}
			currentTime = nextTime
		}

		return missedCount, nil

	default:
		return 0, fmt.Errorf("unknown schedule type: %s", schedule.Type)
	}
}

func (s *Scheduler) executeCatchup(ctx context.Context, schedule *Schedule, missedCount int) error {
	switch schedule.Type {
	case ScheduleTypeOneTime:
		if missedCount > 0 {
			if err := s.processSchedule(ctx, schedule); err != nil {
				return fmt.Errorf("executing one-time catch-up: %w", err)
			}
		}

	case ScheduleTypeInterval:
		if missedCount > 0 {
			if err := s.processSchedule(ctx, schedule); err != nil {
				return fmt.Errorf("executing interval catch-up: %w", err)
			}
		}

	case ScheduleTypeCron:
		execLimit := missedCount
		if execLimit > 100 {
			execLimit = 100
			log.Warn().
				Str("schedule_id", schedule.ID).
				Str("schedule_name", schedule.Name).
				Int("missed_count", missedCount).
				Int("exec_limit", execLimit).
				Msg("Limiting catch-up executions to prevent overload")
		}

		for i := 0; i < execLimit; i++ {
			if err := s.processSchedule(ctx, schedule); err != nil {
				log.Error().
					Err(err).
					Str("schedule_id", schedule.ID).
					Str("schedule_name", schedule.Name).
					Int("execution_number", i+1).
					Msg("Failed to execute catch-up iteration")
				continue
			}

			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}
