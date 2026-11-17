// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

type Scheduler struct {
	service  Service
	store    controlplane.LifecycleStore
	logger   zerolog.Logger
	interval time.Duration
	wg       sync.WaitGroup
}

type SchedulerOption func(*Scheduler)

func WithSchedulerInterval(d time.Duration) SchedulerOption {
	return func(s *Scheduler) {
		s.interval = d
	}
}

func NewScheduler(service Service, store controlplane.LifecycleStore, logger zerolog.Logger, opts ...SchedulerOption) *Scheduler {
	s := &Scheduler{
		service:  service,
		store:    store,
		logger:   logger.With().Str("component", "lifecycle_scheduler").Logger(),
		interval: time.Minute,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Scheduler) Run(ctx context.Context) {
	s.logger.Info().Dur("interval", s.interval).Msg("lifecycle scheduler started")
	s.checkAndRun(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info().Msg("lifecycle scheduler stopping, waiting for in-flight tasks")
			s.wg.Wait()
			s.logger.Info().Msg("lifecycle scheduler stopped")
			return
		case <-ticker.C:
			s.checkAndRun(ctx)
		}
	}
}

func (s *Scheduler) checkAndRun(ctx context.Context) {
	dueRules, err := s.store.ListDueRules(ctx, time.Now())
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list due lifecycle rules")
		return
	}
	for _, rule := range dueRules {
		if ctx.Err() != nil {
			return
		}
		s.logger.Info().Int64("rule_id", rule.ID).Str("rule_name", rule.Name).Msg("executing due lifecycle rule")
		s.wg.Add(1)
		go func(ruleID int64, ruleName string) {
			defer s.wg.Done()
			stats, err := s.service.ExecuteRule(context.WithValue(ctx, scheduledRunKey{}, true), ruleID)
			if err != nil {
				s.logger.Error().Err(err).Int64("rule_id", ruleID).Msg("lifecycle rule execution failed")
			} else {
				s.logger.Info().
					Int64("rule_id", ruleID).
					Int64("deleted_count", stats.DeletedCount).
					Int64("bytes_freed", stats.BytesFreed).
					Int64("duration_ms", stats.DurationMs).
					Msg("lifecycle rule execution finished")
			}
		}(rule.ID, rule.Name)
	}
}
