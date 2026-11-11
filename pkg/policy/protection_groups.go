// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"slices"
)

type protectionBackend struct{ p *policySvc }

func (r *policySvc) ProtectionBackend() protectiongroups.Backend { return &protectionBackend{p: r} }

func (b *protectionBackend) Validate(ctx context.Context, g protectiongroups.Group, adopt bool) error {
	p := b.p
	if g.User == "" {
		// Infrastructure group: runs under ServiceUser using storage connection accounts.
		if p.sharedAccount != nil {
			for _, m := range g.Members {
				if !p.sharedAccount(m, protectiongroups.ServiceUser) {
					return fmt.Errorf("%w: storage %s has no connection account; protection group requires connection accounts on all members", dom.ErrFailedPrecondition, m)
				}
			}
		}
		allowed := map[string]bool{}
		for _, id := range g.Replications(g.Primary) {
			allowed[id.AsString()] = true
		}
		ur, err := p.ListUserReplicationsInfo(ctx)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return err
		}
		br, err := p.ListBucketReplicationsInfo(ctx, protectiongroups.ServiceUser)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return err
		}
		check := func(id entity.UniversalReplicationID, st entity.ReplicationStatusExtended) error {
			if st.IsArchived {
				return nil
			}
			if st.Switch != nil || st.IsPaused {
				return fmt.Errorf("%w: replication is paused or switching", dom.ErrFailedPrecondition)
			}
			if !adopt || !allowed[id.AsString()] {
				return fmt.Errorf("%w: unmanaged replication %s conflicts; use explicit adoption for a matching topology", dom.ErrFailedPrecondition, id.AsString())
			}
			return nil
		}
		for id, st := range ur {
			if id.User == protectiongroups.ServiceUser {
				if err := check(entity.UniversalFromUserReplication(id), st); err != nil {
					return err
				}
			}
		}
		for id, st := range br {
			if len(g.Buckets) == 0 || g.Covers(protectiongroups.ServiceUser, id.FromBucket) {
				if err := check(entity.UniversalFromBucketReplication(id), st); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// Legacy user group: adopts only a single coherent star rooted at the current routing
	routes, err := p.GetBucketRoutings(ctx, g.User)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return err
	}
	users, err := p.GetUserRoutings(ctx)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return err
	}
	route := users[g.User]
	if route == "" {
		route = p.mainStorage
	}
	allowed := map[string]bool{}
	for _, id := range g.Replications(g.Primary) {
		allowed[id.AsString()] = true
	}
	if len(g.Buckets) == 0 {
		if route != g.Primary {
			return fmt.Errorf("%w: group primary must match current routing", dom.ErrFailedPrecondition)
		}
		for _, r := range routes {
			if r != g.Primary {
				return fmt.Errorf("%w: conflicting bucket routing", dom.ErrFailedPrecondition)
			}
		}
	} else {
		for _, bucket := range g.Buckets {
			effective := route
			if r := routes[bucket]; r != "" {
				effective = r
			}
			if effective != g.Primary {
				return fmt.Errorf("%w: bucket %s is routed elsewhere", dom.ErrFailedPrecondition, bucket)
			}
		}
	}
	ur, err := p.ListUserReplicationsInfo(ctx)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return err
	}
	br, err := p.ListBucketReplicationsInfo(ctx, g.User)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return err
	}
	check := func(id entity.UniversalReplicationID, st entity.ReplicationStatusExtended) error {
		if st.IsArchived {
			return nil
		}
		if st.Switch != nil || st.IsPaused {
			return fmt.Errorf("%w: replication is paused or switching", dom.ErrFailedPrecondition)
		}
		if !adopt || !allowed[id.AsString()] {
			return fmt.Errorf("%w: unmanaged replication %s conflicts; use explicit adoption for a matching topology", dom.ErrFailedPrecondition, id.AsString())
		}
		return nil
	}
	for id, st := range ur {
		if id.User == g.User {
			if err := check(entity.UniversalFromUserReplication(id), st); err != nil {
				return err
			}
		}
	}
	for id, st := range br {
		if len(g.Buckets) == 0 || g.Covers(g.User, id.FromBucket) {
			if err := check(entity.UniversalFromBucketReplication(id), st); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *protectionBackend) Ensure(ctx context.Context, g protectiongroups.Group, id entity.UniversalReplicationID) (int64, error) {
	ctx = protectiongroups.Managed(ctx)
	p := b.p
	info, err := p.GetReplicationPolicyInfoExtended(ctx, id)
	if err == nil {
		if info.IsArchived {
			return 0, fmt.Errorf("%w: archived managed replication requires operator recovery", dom.ErrFailedPrecondition)
		}
		if info.Switch != nil {
			return 0, fmt.Errorf("%w: legacy switch conflicts with protection group", dom.ErrFailedPrecondition)
		}
		if info.IsPaused {
			if err = p.ResumeReplication(ctx, id); err != nil {
				return 0, err
			}
		}
		return p.GetReplicationGeneration(ctx, id)
	}
	if !errors.Is(err, dom.ErrNotFound) {
		return 0, err
	}
	var gen int64
	if u, ok := id.AsUserID(); ok {
		gen, err = p.AddUserReplicationPolicy(ctx, u, entity.ReplicationOptions{})
	} else {
		v, _ := id.AsBucketID()
		gen, err = p.AddBucketReplicationPolicy(ctx, v, entity.ReplicationOptions{})
	}
	return gen, err
}
func (b *protectionBackend) Seed(ctx context.Context, id entity.UniversalReplicationID, generation int64) error {
	var task tasks.ReplicationTask
	if _, ok := id.AsUserID(); ok {
		task = &tasks.MigrateS3UserPayload{}
	} else {
		v, _ := id.AsBucketID()
		task = &tasks.BucketCreatePayload{Bucket: v.FromBucket}
	}
	task.SetReplicationID(id, generation)
	return b.p.queueSvc.EnqueueTask(ctx, task)
}
func (b *protectionBackend) Replication(ctx context.Context, id entity.UniversalReplicationID) (entity.ReplicationStatusExtended, error) {
	return b.p.GetReplicationPolicyInfoExtended(ctx, id)
}
func (b *protectionBackend) Pause(ctx context.Context, g protectiongroups.Group) error {
	for _, id := range g.Replications(g.Primary) {
		if err := b.p.queueSvc.Stop(ctx, tasks.InitMigrationListQueue(id), tasks.InitMigrationCopyQueue(id), tasks.EventMigrationQueue(id)); err != nil {
			return err
		}
	}
	return nil
}

// commitPendingKey records, per group, the primaries whose topology change is committed in Redis but not yet
// in PostgreSQL (field = group ID, value = JSON pendingCommit). It is written in the same Redis transaction as the
// promotion and cleared once PostgreSQL has the change, so a failed or interrupted PostgreSQL write is retried
// (SyncControlPlane) instead of leaving PostgreSQL silently behind.
const commitPendingKey = "tutti:protection:cp-commit-pending"

type pendingCommit struct {
	// OldPrimaries are the previous primaries whose replications must be removed in PostgreSQL.
	OldPrimaries []string `json:"oldPrimaries,omitempty"`
}

// Commit replaces routing, removes the old topology and records the group. Redis is the commit point (WATCH on
// the group and its writer registries); PostgreSQL follows afterwards and is retried until it succeeds (see
// commitPendingKey). Writing PostgreSQL first would let its outbox project a routing change to Redis even when the
// Redis transaction then fails.
func (b *protectionBackend) Commit(ctx context.Context, before protectiongroups.Group, after *protectiongroups.Group) error {
	ctx = protectiongroups.Managed(ctx)
	p := b.p
	// Watch both group metadata and writer registries. Admission cannot race a promotion
	// because the same watched group is frozen before this commit.
	writers := "tutti:protection:writers:" + before.ID
	next := *after
	next.Revision = before.Revision + 1
	err := p.client.Watch(ctx, func(tx *redis.Tx) error {
		raw, err := tx.HGet(ctx, protectiongroups.GroupsKey, before.ID).Result()
		if err != nil {
			return err
		}
		var current protectiongroups.Group
		if err = json.Unmarshal([]byte(raw), &current); err != nil {
			return err
		}
		if current.Revision != before.Revision || !current.Frozen {
			return fmt.Errorf("%w: group promotion lost ownership", dom.ErrFailedPrecondition)
		}
		for _, key := range []string{writers, writers + ":workers"} {
			n, err := tx.HLen(ctx, key).Result()
			if err != nil {
				return err
			}
			if n != 0 {
				return fmt.Errorf("%w: outstanding writes prevent promotion", dom.ErrFailedPrecondition)
			}
		}
		rawNext, err := json.Marshal(next)
		if err != nil {
			return err
		}
		promRaw, err := json.Marshal(next.Promotion())
		if err != nil {
			return err
		}
		var pending []byte
		if p.cp != nil {
			var pc pendingCommit
			if prev, err := tx.HGet(ctx, commitPendingKey, before.ID).Result(); err == nil {
				_ = json.Unmarshal([]byte(prev), &pc)
			} else if !errors.Is(err, redis.Nil) {
				return err
			}
			if before.Primary != after.Primary && !slices.Contains(pc.OldPrimaries, before.Primary) {
				pc.OldPrimaries = append(pc.OldPrimaries, before.Primary)
			}
			if pending, err = json.Marshal(pc); err != nil {
				return err
			}
		}
		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			if before.Primary != after.Primary {
				for _, id := range before.Replications(before.Primary) {
					if u, ok := id.AsUserID(); ok {
						store.NewUserReplicationPolicyStore(pipe).RemoveOp(ctx, u)
						store.NewUserReplicationStatusStore(pipe).DeleteOp(ctx, u)
					} else {
						v, _ := id.AsBucketID()
						store.NewBucketReplicationPolicyStore(pipe).RemoveOp(ctx, v)
						store.NewBucketReplicationStatusStore(pipe).DeleteOp(ctx, v)
					}
					store.NewReplicationGenerationStore(pipe).IncrementOp(ctx, id)
				}
			}
			if after.User != "" {
				if len(after.Buckets) == 0 {
					store.NewUserRoutingStore(pipe).SetOp(ctx, after.User, after.Primary)
					// Creation validation established that every explicit bucket route pointed at the same primary.
					pipe.Del(ctx, "p:route_bucket:"+after.User)
				} else {
					for _, bucket := range after.Buckets {
						store.NewBucketRoutingStore(pipe).SetOp(ctx, entity.NewBucketRoutingPolicyID(after.User, bucket), after.Primary)
					}
				}
			}
			pipe.HSet(ctx, protectiongroups.GroupsKey, after.ID, rawNext)
			pipe.HSet(ctx, protectiongroups.PromotionKey, after.ID, promRaw)
			pipe.HSet(ctx, protectiongroups.ScopeIndexKey, "p:"+after.ID, after.Primary)
			if pending != nil {
				pipe.HSet(ctx, commitPendingKey, after.ID, pending)
			}
			// Redis Tx clients do not necessarily inherit hooks from their parent. Include the
			// snapshot version explicitly in this authoritative topology transaction.
			pipe.Incr(ctx, SnapshotVersionKey)
			pipe.Publish(ctx, SnapshotChannel, "changed")
			return nil
		})
		return err
	}, protectiongroups.GroupsKey, writers, writers+":workers", commitPendingKey)
	if err != nil {
		return err
	}
	after.Revision = next.Revision
	if p.cp != nil {
		if err := b.SyncControlPlane(ctx, next); err != nil {
			// committed in Redis: PostgreSQL is retried by the reconciler (SyncControlPlane)
			zerolog.Ctx(ctx).Warn().Err(err).Str("group", after.ID).Msg("protection group commit not yet in PostgreSQL; will retry")
		}
	}
	if before.Primary != after.Primary {
		for _, id := range before.Replications(before.Primary) {
			if err := tasks.StopAndPurgeReplicationQueues(ctx, p.queueSvc, id); err != nil {
				return err
			}
		}
	} else {
		for _, id := range after.Replications(after.Primary) {
			if _, err := p.GetReplicationPolicyInfoExtended(ctx, id); errors.Is(err, dom.ErrNotFound) {
				continue
			} else if err != nil {
				return err
			}
			if err := p.ResumeReplication(ctx, id); err != nil {
				return err
			}
		}
	}
	return nil
}

// clearPendingScript deletes the pending marker only if it is unchanged (a newer promotion keeps its own).
var clearPendingScript = redis.NewScript(`
if redis.call('hget',KEYS[1],ARGV[1])==ARGV[2] then return redis.call('hdel',KEYS[1],ARGV[1]) end
return 0
`)

// SyncControlPlane applies a promotion that is committed in Redis but still pending in PostgreSQL: it removes
// the replications of the recorded old primaries (only those whose source still is that old primary, so a
// replication the reconciler created since from the new primary is never touched), bumps their generations,
// sets the routing to g's (current) primary and stores the group's PostgreSQL copy, in one PostgreSQL
// transaction. It is idempotent and a no-op without a pending marker. g must be the current Redis group.
func (b *protectionBackend) SyncControlPlane(ctx context.Context, g protectiongroups.Group) error {
	p := b.p
	if p.cp == nil {
		return nil
	}
	marker, err := p.client.HGet(ctx, commitPendingKey, g.ID).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return err
	}
	var pc pendingCommit
	if err := json.Unmarshal([]byte(marker), &pc); err != nil {
		return err
	}
	doc, err := protectiongroups.ControlPlaneDocument(g)
	if err != nil {
		return err
	}
	err = p.cp.Tx(ctx, func(repos controlplane.Repos) error {
		for _, old := range pc.OldPrimaries {
			if old == g.Primary {
				continue // back on an earlier primary: its replications are the current ones
			}
			for _, id := range g.Replications(old) {
				cpID := toCPReplicationID(id)
				pols, err := repos.Replications.Policies(ctx, cpID.User, cpID.FromBucket)
				if err != nil && !errors.Is(err, dom.ErrNotFound) {
					return err
				}
				for _, pol := range pols {
					if pol.FromStorage == cpID.FromStorage && pol.ToStorage == cpID.ToStorage && pol.ToBucket == cpID.ToBucket {
						if err := repos.Replications.Remove(ctx, cpID); err != nil {
							return err
						}
					}
				}
				if err := repos.Replications.DeleteStatus(ctx, cpID); err != nil {
					return err
				}
				if _, err := repos.Replications.IncrementGeneration(ctx, cpID); err != nil {
					return err
				}
			}
		}
		if g.User != "" {
			if len(g.Buckets) == 0 {
				if err := repos.Routing.Set(ctx, g.User, "", g.Primary); err != nil {
					return err
				}
				bRoutes, err := repos.Routing.ListBucketRoutes(ctx, g.User)
				if err != nil {
					return err
				}
				for bucket := range bRoutes {
					if err := repos.Routing.DeleteRoute(ctx, g.User, bucket); err != nil {
						return err
					}
				}
			} else {
				for _, bucket := range g.Buckets {
					if err := repos.Routing.Set(ctx, g.User, bucket, g.Primary); err != nil {
						return err
					}
				}
			}
		}
		_, err := repos.ProtectionGroups.Put(ctx, controlplane.RawDoc{ID: g.ID, User: g.User, JSON: doc})
		return err
	})
	if err != nil {
		return controlplane.WrapWriteError(err)
	}
	return clearPendingScript.Run(ctx, p.client, []string{commitPendingKey}, g.ID, marker).Err()
}

func (r *policySvc) resolveBaseStorage(ctx context.Context, user, bucket string) (string, error) {
	if bucket != "" && user != "" {
		bucketRouteID := entity.NewBucketRoutingPolicyID(user, bucket)
		bRoute, err := r.bucketRoutingStore.GetOp(ctx, bucketRouteID).Get()
		if err == nil {
			return bRoute, nil
		}
		if !errors.Is(err, dom.ErrNotFound) {
			return "", err
		}
	}
	if user != "" {
		uRoute, err := r.userRoutingStore.GetOp(ctx, user).Get()
		if err == nil {
			return uRoute, nil
		}
		if !errors.Is(err, dom.ErrNotFound) {
			return "", err
		}
	}
	return r.mainStorage, nil
}

// ProtectionGroupOf resolves the protection group managing (user, bucket) for control loops such as the legacy
// failover controller, which are not serving a request and so have not validated the snapshot's config
// version: the snapshot is refreshed first and used only when it is current, otherwise the index is read from
// Redis (rebuilt when missing). An error means the scope is unknown; callers must fail closed.
func (r *policySvc) ProtectionGroupOf(ctx context.Context, user, bucket string) (string, error) {
	baseStorage, err := r.resolveBaseStorage(ctx, user, bucket)
	if err != nil {
		return "", fmt.Errorf("%w: unable to resolve base storage for protection group lookup: %w", dom.ErrUnavailable, err)
	}

	lookup := func(idx map[string]string) (string, bool) {
		return protectiongroups.LookupEffectiveScope(idx, user, bucket, baseStorage)
	}

	if r.snap != nil {
		if err := r.snap.refresh(ctx, false); err == nil {
			if idx, err := r.snap.protectionScope(ctx); err == nil {
				if group, ok := lookup(idx); ok {
					return group, nil
				}
			}
		}
	}
	idx, err := protectiongroups.ScopeFields(ctx, r.client, user, bucket, baseStorage)
	if err != nil {
		return "", fmt.Errorf("%w: protection group scope index unavailable: %w", dom.ErrUnavailable, err)
	}
	if group, ok := lookup(idx); ok {
		return group, nil
	}
	if err := protectiongroups.NewStore(r.client).EnsureIndex(ctx); err != nil {
		return "", fmt.Errorf("%w: protection group scope index unavailable: %w", dom.ErrUnavailable, err)
	}
	idx, err = protectiongroups.ScopeFields(ctx, r.client, user, bucket, baseStorage)
	if err != nil {
		return "", fmt.Errorf("%w: protection group scope index unavailable: %w", dom.ErrUnavailable, err)
	}
	if group, ok := lookup(idx); ok {
		return group, nil
	}
	return "", fmt.Errorf("%w: protection group scope index is incomplete", dom.ErrUnavailable)
}
