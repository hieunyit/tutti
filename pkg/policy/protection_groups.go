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
)

type protectionBackend struct{ p *policySvc }

func (r *policySvc) ProtectionBackend() protectiongroups.Backend { return &protectionBackend{p: r} }

func (b *protectionBackend) Validate(ctx context.Context, g protectiongroups.Group, adopt bool) error {
	p := b.p
	// A group adopts only a single coherent star rooted at the current routing, never
	// silently moves data/routing or steals arbitrary legacy replications.
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

func (b *protectionBackend) Commit(ctx context.Context, before protectiongroups.Group, after *protectiongroups.Group) error {
	ctx = protectiongroups.Managed(ctx)
	p := b.p
	// Watch both group metadata and writer registries. Admission cannot race a promotion
	// because the same watched group is frozen before this commit.
	writers := "tutti:protection:writers:" + before.ID
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
		next := *after
		next.Revision = before.Revision + 1
		rawNext, err := json.Marshal(next)
		if err != nil {
			return err
		}
		promRaw, err := json.Marshal(next.Promotion())
		if err != nil {
			return err
		}
		if p.cp != nil {
			if err := p.cp.Tx(ctx, func(repos controlplane.Repos) error {
				if before.Primary != after.Primary {
					for _, id := range before.Replications(before.Primary) {
						cpID := toCPReplicationID(id)
						if err := repos.Replications.Remove(ctx, cpID); err != nil {
							return err
						}
						if err := repos.Replications.DeleteStatus(ctx, cpID); err != nil {
							return err
						}
						if _, err := repos.Replications.IncrementGeneration(ctx, cpID); err != nil {
							return err
						}
					}
				}
				if len(after.Buckets) == 0 {
					if err := repos.Routing.Set(ctx, after.User, "", after.Primary); err != nil {
						return err
					}
					bRoutes, err := repos.Routing.ListBucketRoutes(ctx, after.User)
					if err != nil {
						return err
					}
					for b := range bRoutes {
						if err := repos.Routing.DeleteRoute(ctx, after.User, b); err != nil {
							return err
						}
					}
				} else {
					for _, bucket := range after.Buckets {
						if err := repos.Routing.Set(ctx, after.User, bucket, after.Primary); err != nil {
							return err
						}
					}
				}
				_, err := repos.ProtectionGroups.Put(ctx, controlplane.RawDoc{ID: after.ID, User: after.User, JSON: rawNext})
				return err
			}); err != nil {
				return controlplane.WrapWriteError(err)
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
			if len(after.Buckets) == 0 {
				store.NewUserRoutingStore(pipe).SetOp(ctx, after.User, after.Primary)
				// Creation validation established that every explicit bucket route pointed at the same primary.
				pipe.Del(ctx, "p:route_bucket:"+after.User)
			} else {
				for _, bucket := range after.Buckets {
					store.NewBucketRoutingStore(pipe).SetOp(ctx, entity.NewBucketRoutingPolicyID(after.User, bucket), after.Primary)
				}
			}
			pipe.HSet(ctx, protectiongroups.GroupsKey, after.ID, rawNext)
			pipe.HSet(ctx, protectiongroups.PromotionKey, after.ID, promRaw)
			// Redis Tx clients do not necessarily inherit hooks from their parent. Include the
			// snapshot version explicitly in this authoritative topology transaction.
			pipe.Incr(ctx, SnapshotVersionKey)
			pipe.Publish(ctx, SnapshotChannel, "changed")
			return nil
		})
		return err
	}, protectiongroups.GroupsKey, writers, writers+":workers")
	if err != nil {
		return err
	}
	after.Revision = before.Revision + 1
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
