// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package migrate

import (
	"context"
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
}

// Republish rebuilds the Redis control-plane projection from PostgreSQL when Redis has been lost or drifted.
// In non-dry-run mode it enqueues an outbox snapshot of all PostgreSQL rows (without touching row versions or
// audit_log), drains the outbox through pg.Publisher into src, rebuilds the protection-group scope index if
// needed, and verifies that Redis and PostgreSQL agree.
func Republish(ctx context.Context, store *pg.Store, src Sources, opt RepublishOptions) (*Report, error) {
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
	pgSum := pgSnap.Summarize()
	beforeSum := beforeSnap.Summarize()

	rep := &Report{
		StartedAt: time.Now().UTC(),
		DryRun:    opt.DryRun,
		Families:  map[Family]*FamilyReport{},
		Warnings:  append([]string(nil), beforeRedis.Warnings...),
	}
	for _, f := range Families {
		fr := &FamilyReport{
			Source: beforeSum[f],
			Target: pgSum[f],
		}
		for k, v := range pgSnap[f] {
			cur, ok := beforeSnap[f][k]
			switch {
			case !ok:
				fr.Created++
			case cur != v:
				fr.Updated++
			default:
				fr.Unchanged++
			}
		}
		for k := range beforeSnap[f] {
			if _, ok := pgSnap[f][k]; !ok {
				fr.ExtraInTarget++
			}
		}
		rep.Families[f] = fr
	}

	if opt.DryRun {
		rep.OK = len(rep.Errors) == 0
		return rep, nil
	}

	if _, err := store.EnqueueSnapshot(ctx); err != nil {
		rep.Errors = append(rep.Errors, "enqueue snapshot: "+err.Error())
		return rep, nil
	}

	pub := pg.NewPublisher(store, src.Config, controlplane.OutboxConfig{BatchSize: 500}, zerolog.Nop())
	pub.MetaRedis = src.Meta
	for {
		res, err := pub.RunOnce(ctx)
		if err != nil {
			rep.Errors = append(rep.Errors, "publish outbox: "+err.Error())
			return rep, nil
		}
		if res.Failed > 0 {
			rep.Errors = append(rep.Errors, fmt.Sprintf("publish outbox: %d entries failed to apply to Redis", res.Failed))
			return rep, nil
		}
		n, _, err := pub.Pending(ctx)
		if err != nil {
			rep.Errors = append(rep.Errors, "check outbox pending: "+err.Error())
			return rep, nil
		}
		if n == 0 || res.Published == 0 {
			break
		}
	}

	if len(pgData.ProtectionGroups) > 0 {
		if err := protectiongroups.NewStore(src.Config).EnsureIndex(ctx); err != nil {
			rep.Warnings = append(rep.Warnings, "rebuilding protection group scope index: "+err.Error())
		}
	}

	afterRedis, err := ReadRedis(ctx, src)
	if err != nil {
		rep.Errors = append(rep.Errors, "re-reading Redis: "+err.Error())
		return rep, nil
	}
	afterSnap := afterRedis.Canonical()
	afterSum := afterSnap.Summarize()
	for _, f := range Families {
		sum := afterSum[f]
		rep.Families[f].Source = sum
		pgAfter := pgSum[f]
		rep.Families[f].After = &pgAfter
	}
	// Ignore extra keys that existed in Redis before republish (republish is additive like migrate).
	for _, d := range Compare(pgSnap, afterSnap) {
		if d.Kind != "missing_in_source" {
			rep.Mismatches = append(rep.Mismatches, d)
		}
	}
	rep.OK = len(rep.Errors) == 0 && len(rep.Mismatches) == 0
	return rep, nil
}
