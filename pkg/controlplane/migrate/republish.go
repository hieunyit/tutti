// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package migrate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
)

// RepublishOptions configures Republish.
type RepublishOptions struct {
	// DryRun computes the diff between PostgreSQL and Redis without enqueuing outbox entries or writing to Redis.
	DryRun bool
	// Force allows overwriting Redis entries that differ from PostgreSQL (and, with Prune, removing entries that PostgreSQL
	// does not know). Without it Republish refuses when Redis holds anything that would be overwritten, because that may
	// be newer than PostgreSQL (for example after PostgreSQL was restored from an old backup).
	Force bool
	// Prune removes the Redis entries that have no PostgreSQL row, making Redis an exact copy of PostgreSQL for the
	// projected families. Without it republish only adds and overwrites (extras are reported as warnings).
	Prune bool
	// DrainTimeout bounds the wait for the outbox to drain (default 5 minutes). Another publisher (a worker) may be
	// draining it; Republish waits for it instead of giving up.
	DrainTimeout time.Duration
}

// skipFamilies are not republished. Protection groups keep runtime fields (promotion, fencing, scope index) in the
// same Redis documents as the desired spec, so they cannot be rebuilt from PostgreSQL without resetting live state.
var skipFamilies = map[Family]bool{FamProtectionGroups: true}

func dropSkipped(s Snapshot) {
	for f, skip := range skipFamilies {
		if skip {
			s[f] = map[string]string{}
		}
	}
}

// Republish rebuilds the Redis control-plane projection from PostgreSQL when Redis has been lost or drifted.
//
// It first waits for the outbox to drain (pending changes are the normal reason for a lagging Redis), compares both
// sides and refuses to overwrite differing Redis data unless Force is set. Then it enqueues a snapshot of all
// PostgreSQL rows (without touching row versions or audit_log; with Prune also the removal of Redis-only entries),
// waits until the publisher has applied everything enqueued (it may be a worker's publisher: Busy is not an error)
// and only then verifies that Redis and PostgreSQL agree. Protection groups are not touched.
func Republish(ctx context.Context, store *pg.Store, src Sources, opt RepublishOptions) (*Report, error) {
	if opt.DrainTimeout <= 0 {
		opt.DrainTimeout = 5 * time.Minute
	}
	pub := pg.NewPublisher(store, src.Config, controlplane.OutboxConfig{BatchSize: 500}, zerolog.Nop())
	pub.MetaRedis = src.Meta
	rep := &Report{StartedAt: time.Now().UTC(), DryRun: opt.DryRun, Families: map[Family]*FamilyReport{}}

	if !opt.DryRun {
		// changes committed before the republish but not yet projected are not drift
		if hw, err := store.OutboxHighWater(ctx); err == nil {
			if err := drain(ctx, store, pub, hw, opt.DrainTimeout); err != nil {
				rep.Errors = append(rep.Errors, "waiting for the outbox to drain before comparing: "+err.Error())
				return rep, nil
			}
		}
	}

	pgData, err := ReadPG(ctx, store.Repos())
	if err != nil {
		return nil, fmt.Errorf("reading PostgreSQL: %w", err)
	}
	beforeRedis, err := ReadRedis(ctx, src)
	if err != nil {
		return nil, fmt.Errorf("reading Redis: %w", err)
	}
	pgSnap := pgData.Canonical()
	beforeSnap := beforeRedis.Canonical()
	dropSkipped(pgSnap)
	dropSkipped(beforeSnap)
	pgSum := pgSnap.Summarize()
	beforeSum := beforeSnap.Summarize()
	rep.Warnings = append(rep.Warnings, beforeRedis.Warnings...)
	rep.Warnings = append(rep.Warnings, "protection groups are not republished: their runtime state lives in Redis")
	if !opt.DryRun {
		// Groups lost from Redis are restored from their PostgreSQL recovery seed, frozen and re-initializing so every
		// member is re-verified; groups still in Redis are left untouched. Also rebuilds the scope index.
		if err := protectiongroups.NewStore(src.Config).WithControlPlane(store).Recover(ctx); err != nil {
			rep.Errors = append(rep.Errors, "protection groups recovery: "+err.Error())
		}
	}

	var prune []pg.PruneItem
	var updated, extra int
	for _, f := range Families {
		fr := &FamilyReport{Source: beforeSum[f], Target: pgSum[f]}
		for k, v := range pgSnap[f] {
			cur, ok := beforeSnap[f][k]
			switch {
			case !ok:
				fr.Created++
			case cur != v:
				fr.Updated++
				updated++
			default:
				fr.Unchanged++
			}
		}
		for k := range beforeSnap[f] {
			if _, ok := pgSnap[f][k]; !ok {
				fr.ExtraInTarget++
				if f != FamDynamicSalt && f != FamConsoleLogins {
					extra++
					prune = append(prune, pg.PruneItem{Family: string(f), Key: k})
				}
			}
		}
		rep.Families[f] = fr
	}
	if extra > 0 && !opt.Prune {
		rep.Warnings = append(rep.Warnings, fmt.Sprintf("%d Redis entries have no PostgreSQL row and are left in place (pass --prune to remove them); republish without --prune is additive, not a full rebuild", extra))
	}

	if opt.DryRun {
		rep.OK = len(rep.Errors) == 0
		return rep, nil
	}
	if !opt.Force && (updated > 0 || (opt.Prune && extra > 0)) {
		rep.Errors = append(rep.Errors, fmt.Sprintf("refusing to overwrite Redis: %d entries differ from PostgreSQL and %d exist only in Redis; they may be newer than PostgreSQL (for example after a restore from an old backup). Review a --dry-run and re-run with --force", updated, extra))
		rep.OK = false
		return rep, nil
	}

	if _, err := store.EnqueueSnapshot(ctx); err != nil {
		rep.Errors = append(rep.Errors, "enqueue snapshot: "+err.Error())
		return rep, nil
	}
	if opt.Prune {
		if _, err := store.EnqueuePrune(ctx, prune); err != nil {
			rep.Errors = append(rep.Errors, "enqueue prune: "+err.Error())
			return rep, nil
		}
	}
	hw, err := store.OutboxHighWater(ctx)
	if err != nil {
		rep.Errors = append(rep.Errors, "outbox high-water: "+err.Error())
		return rep, nil
	}
	if err := drain(ctx, store, pub, hw, opt.DrainTimeout); err != nil {
		rep.Errors = append(rep.Errors, "publish outbox: "+err.Error())
		return rep, nil
	}

	afterRedis, err := ReadRedis(ctx, src)
	if err != nil {
		rep.Errors = append(rep.Errors, "re-reading Redis: "+err.Error())
		return rep, nil
	}
	afterSnap := afterRedis.Canonical()
	dropSkipped(afterSnap)
	afterSum := afterSnap.Summarize()
	for _, f := range Families {
		rep.Families[f].Source = afterSum[f]
		pgAfter := pgSum[f]
		rep.Families[f].After = &pgAfter
	}
	// Without --prune, Redis-only entries stay by design (already reported as a warning).
	for _, d := range Compare(pgSnap, afterSnap) {
		if d.Kind == "missing_in_source" && (!opt.Prune || d.Family == FamDynamicSalt || d.Family == FamConsoleLogins) {
			continue
		}
		rep.Mismatches = append(rep.Mismatches, d)
	}
	rep.OK = len(rep.Errors) == 0 && len(rep.Mismatches) == 0
	return rep, nil
}

// drain publishes until every outbox entry up to highWater is applied. A publisher that holds the lock (a worker) is
// waited for, failed entries are retried after their backoff; ctx or the timeout ends the wait with an error.
func drain(ctx context.Context, store *pg.Store, pub *pg.Publisher, highWater int64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		n, err := store.PendingUpTo(ctx, highWater)
		if err != nil {
			return fmt.Errorf("check outbox pending: %w", err)
		}
		if n == 0 {
			return nil
		}
		res, err := pub.RunOnce(ctx)
		switch {
		case err != nil:
			lastErr = err
		case res.Failed > 0:
			lastErr = fmt.Errorf("%d entries failed to apply to Redis", res.Failed)
		case res.Busy:
			lastErr = fmt.Errorf("another publisher is draining the outbox")
		default:
			lastErr = nil
		}
		if lastErr == nil && res.Published > 0 {
			continue // more may be waiting, do not sleep
		}
		if time.Now().After(deadline) {
			msg := fmt.Sprintf("%d outbox entries still pending after %s", n, timeout)
			if lastErr != nil {
				msg += ": " + lastErr.Error()
			}
			return errors.New(msg)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}
