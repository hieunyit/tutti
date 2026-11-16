// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/rescan"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// DefaultRescanTick is how often the scheduler looks for replications whose rescan is due.
const DefaultRescanTick = 30 * time.Second

// RescanTargets returns the replications that may be scanned now: rescan interval set, not archived, not paused,
// initial copy done.
type RescanTargets func(ctx context.Context) ([]rescan.Target, error)

// RescanScheduler starts due rescans. Every worker process runs one; the scan itself is owned by a Redis state
// (store.RescanStore.Start is atomic), so only one scan per replication runs cluster wide.
type RescanScheduler struct {
	Targets RescanTargets
	Store   *store.RescanStore
	Queue   tasks.QueueService
	Tick    time.Duration
	Now     func() time.Time
}

func (r *RescanScheduler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// Run ticks until ctx ends.
func (r *RescanScheduler) Run(ctx context.Context) {
	tick := r.Tick
	if tick <= 0 {
		tick = DefaultRescanTick
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.RunOnce(ctx)
		}
	}
}

// RunOnce processes all targets once and returns the number of scans started.
func (r *RescanScheduler) RunOnce(ctx context.Context) int {
	logger := zerolog.Ctx(ctx)
	targets, err := r.Targets(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("rescan scheduler: unable to list replications")
		return 0
	}
	starter := &rescan.Starter{Store: r.Store, Queue: r.Queue, Now: r.Now}
	started := 0
	for _, t := range targets {
		now := r.now()
		id := t.ID.AsString()
		st, err := r.Store.Get(ctx, id, t.Generation, now)
		if err != nil {
			logger.Warn().Err(err).Str("replication", id).Msg("rescan scheduler: unable to read state")
			continue
		}
		if st.RunID != "" {
			// A scan owns the replication. While its queues still hold tasks it is alive even without progress
			// (long copies, a busy copy queue): keep its lease so that it is not superseded.
			if n, err := r.Queue.UnprocessedCount(ctx, true, tasks.RescanListQueue(t.ID), tasks.RescanCopyQueue(t.ID)); err == nil && n > 0 {
				_, _ = r.Store.Renew(ctx, id, t.Generation, st.RunID, now)
				continue
			}
			if st.Running {
				continue
			}
		}
		if st.Armed.IsZero() {
			if err := r.Store.Arm(ctx, id, t.Generation, now); err != nil {
				logger.Warn().Err(err).Str("replication", id).Msg("rescan scheduler: unable to arm")
				continue
			}
			st.Armed = now
		}
		if now.Before(store.RescanDue(t.Interval, st, now)) {
			continue
		}
		ok, runID, err := starter.Start(ctx, t)
		if err != nil {
			logger.Warn().Err(err).Str("replication", id).Msg("rescan scheduler: unable to start scan")
			continue
		}
		if ok {
			started++
			metrics.RescanStarted()
			logger.Info().Str("replication", id).Str("run", runID).Msg("rescan: started")
		}
	}
	return started
}
