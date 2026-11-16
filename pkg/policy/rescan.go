// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/rescan"
	"github.com/hieunyit/tutti/pkg/store"
)

// rescanStore returns the rescan state store. The state lives in the queue Redis database like the initial copy
// byte counters (the worker writes it there). nil when the queue service exposes no client.
func (r *policySvc) rescanStore() *store.RescanStore {
	qr, ok := r.queueSvc.(interface{ RedisClient() redis.Cmdable })
	if !ok || qr == nil {
		return nil
	}
	rc := qr.RedisClient()
	if rc == nil {
		return nil
	}
	return store.NewRescanStore(rc)
}

// dropRescanState removes the scan state of a generation of a deleted replication.
func (r *policySvc) dropRescanState(ctx context.Context, id entity.UniversalReplicationID, generation int64) {
	rs := r.rescanStore()
	if rs == nil || generation <= 0 {
		return
	}
	if err := rs.Delete(context.WithoutCancel(ctx), id.AsString(), generation); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Str("replication", id.AsString()).Msg("unable to remove rescan state of deleted replication")
	}
}

// dropRescanStateOnDelete schedules the cleanup of the current generation. Call it before the delete bumps the generation.
func (r *policySvc) dropRescanStateOnDelete(ctx context.Context, id entity.UniversalReplicationID) func() {
	gen, err := r.GetReplicationGeneration(ctx, id)
	if err != nil || gen <= 0 {
		return func() {}
	}
	return func() { r.dropRescanState(ctx, id, gen) }
}

// getStatus returns the configured status of a replication (control plane first, Redis fallback).
func (r *policySvc) getStatus(ctx context.Context, id entity.UniversalReplicationID) (entity.ReplicationStatus, error) {
	if r.cp != nil {
		st, err := r.cp.Repos().Replications.GetStatus(ctx, toCPReplicationID(id))
		if err == nil {
			return fromCPStatus(st, false), nil
		}
		if !controlplane.IsUnavailable(err) {
			return entity.ReplicationStatus{}, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("replication: PostgreSQL GetStatus failed, falling back to Redis")
	}
	tx := r.bucketReplicationStatusStore.TxExecutor()
	res := r.getReplicationStatusInTx(ctx, tx, id)
	_ = tx.Exec(ctx)
	return res.Get()
}

// rescanInfo summarizes the rescan of a replication for the API. Best effort: runtime state is optional.
func (r *policySvc) rescanInfo(ctx context.Context, id entity.UniversalReplicationID, status *entity.ReplicationStatus, paused, initDone bool) entity.RescanInfo {
	info := entity.RescanInfo{Interval: status.RescanInterval()}
	rs := r.rescanStore()
	if rs == nil {
		return info
	}
	gen, err := r.GetReplicationGeneration(ctx, id)
	if err != nil || gen <= 0 {
		return info
	}
	now := time.Now()
	st, err := rs.Get(ctx, id.AsString(), gen, now)
	if err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).Msg("unable to read rescan state")
		return info
	}
	info.Running = st.Running
	info.LastError = st.LastError
	info.LastObjects = st.LastObjects
	if !st.LastStart.IsZero() {
		t := st.LastStart
		info.LastScanAt = &t
	}
	if info.Interval > 0 && !paused && initDone && !st.Running && !status.IsArchived {
		next := store.RescanDue(info.Interval, st, now)
		info.NextScanAt = &next
	}
	return info
}

// ListRescanTargets returns the replications that may be scanned now: rescan interval set, not archived, not
// paused (no queue of the replication is paused), initial copy done. users are the users whose bucket-level
// replications are inspected in addition to storage-level ones (empty user) and user-level replications.
func (r *policySvc) ListRescanTargets(ctx context.Context, users []string) ([]rescan.Target, error) {
	var res []rescan.Target
	consider := func(id entity.UniversalReplicationID, st entity.ReplicationStatus) error {
		if st.RescanSeconds <= 0 || st.IsArchived || st.FailoverPending {
			return nil
		}
		ext, err := r.fillExtendedReplicationStatus(ctx, id, &st, nil)
		if err != nil {
			return err
		}
		if ext.IsPaused || !ext.InitDone() {
			return nil
		}
		gen, err := r.GetReplicationGeneration(ctx, id)
		if err != nil {
			return err
		}
		if gen <= 0 {
			return nil
		}
		res = append(res, rescan.Target{ID: id, Generation: gen, Interval: st.RescanInterval(), Since: st.Since, CreatedAt: st.CreatedAt})
		return nil
	}
	seen := map[string]bool{}
	for _, user := range append(append([]string(nil), users...), "") {
		if seen[user] {
			continue
		}
		seen[user] = true
		statuses, err := r.listBucketStatuses(ctx, user)
		if err != nil {
			if errors.Is(err, dom.ErrNotFound) {
				continue
			}
			return nil, err
		}
		for id, st := range statuses {
			if err := consider(entity.UniversalFromBucketReplication(id), st); err != nil {
				return nil, err
			}
		}
	}
	statuses, err := r.listUserStatuses(ctx)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return nil, err
	}
	for id, st := range statuses {
		if err := consider(entity.UniversalFromUserReplication(id), st); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// RescanReplicationNow starts a scan of the replication immediately ("Sync now"). It fails with
// dom.ErrFailedPrecondition when a scan is already running, the replication is paused or its initial copy is not
// finished. It works whether or not the periodic rescan is enabled.
func (r *policySvc) RescanReplicationNow(ctx context.Context, id entity.UniversalReplicationID) error {
	if err := id.Validate(); err != nil {
		return fmt.Errorf("rescan replication: unable to validate replication id: %w", err)
	}
	info, err := r.GetReplicationPolicyInfoExtended(ctx, id)
	if err != nil {
		return err
	}
	if info.IsArchived {
		return fmt.Errorf("%w: replication %s is archived", dom.ErrNotFound, id.AsString())
	}
	if info.IsPaused {
		return fmt.Errorf("%w: replication is paused, resume it before starting a sync", dom.ErrFailedPrecondition)
	}
	if !info.InitDone() {
		return fmt.Errorf("%w: the initial copy is not finished yet, a sync can start when it is done", dom.ErrFailedPrecondition)
	}
	rs := r.rescanStore()
	if rs == nil {
		return fmt.Errorf("%w: rescan needs the worker Redis", dom.ErrFailedPrecondition)
	}
	gen, err := r.GetReplicationGeneration(ctx, id)
	if err != nil {
		return err
	}
	starter := &rescan.Starter{Store: rs, Queue: r.queueSvc}
	started, _, err := starter.Start(ctx, rescan.Target{ID: id, Generation: gen, Interval: info.RescanInterval(), Since: info.Since, CreatedAt: info.CreatedAt})
	if err != nil {
		return err
	}
	if !started {
		return fmt.Errorf("%w: a sync is already running", dom.ErrFailedPrecondition)
	}
	metrics.RescanStarted()
	return nil
}
