// Copyright 2025 Clyso GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// FailoverSvc switches the routing of a replication scope to the replication destination.
type FailoverSvc interface {
	// FailoverReplication pauses the replication and routes its scope to the replication destination.
	// Unlike SetUserRouting/SetBucketRouting it is allowed while the replication exists.
	// It fails with dom.ErrInvalidArg if the scope is not routed to the replication source (already failed over).
	FailoverReplication(ctx context.Context, id entity.UniversalReplicationID) error
}

var _ FailoverSvc = (*policySvc)(nil)

func (r *policySvc) FailoverReplication(ctx context.Context, id entity.UniversalReplicationID) error {
	return r.FailoverReplicationWithEvent(ctx, id, nil)
}

// FailoverReplicationWithEvent switches the routing of a replication scope to the replication destination
// and, when ev is non-nil and a PostgreSQL control plane is attached, records the failover event in the
// same database transaction.
//
// With a control plane the operation spans PostgreSQL (routing, event; the source of truth) and Redis (failover
// pending flag, paused queues and the immediately visible routing). It is convergent: every step is idempotent and a
// repeated call completes whatever the previous call left undone.
//
//  1. failover_pending is set and the queues are paused (Redis) so that a crashed/failed call is owned and resumable;
//  2. the routing and the event are committed in one PostgreSQL transaction. If that fails definitely (read-only,
//     domain error, database down before the commit) step 1 is rolled back so nothing stays pending; if the outcome is
//     unknown (commit error) the pending flag stays and a retry decides from the stored routing;
//  3. Redis is updated (routing, pending cleared). If this fails the control plane already holds the new routing; the
//     retry sees "routed to the destination and failover pending" and only repeats step 3 (the outbox publisher
//     projects the routing to Redis independently).
func (r *policySvc) FailoverReplicationWithEvent(ctx context.Context, id entity.UniversalReplicationID, ev *controlplane.FailoverEvent) error {
	if err := id.Validate(); err != nil {
		return fmt.Errorf("failover: unable to validate replication id: %w", err)
	}
	routeTo, err := r.getRoutingForReplication(ctx, id)
	if err != nil {
		return fmt.Errorf("failover: unable to get current routing: %w", err)
	}
	info, err := r.GetReplicationPolicyInfoExtended(ctx, id)
	if err != nil {
		return fmt.Errorf("failover: get replication status: %w", err)
	}
	// resume: the control plane already routes to the destination and the pending flag proves that this failover (not
	// an operator) is responsible: only the Redis side is missing
	resume := r.cp != nil && routeTo == id.ToStorage() && info.FailoverPending
	if !resume && routeTo != id.FromStorage() {
		return fmt.Errorf("%w: failover: %s is routed to %s, not to replication source %s", dom.ErrInvalidArg, id.AsString(), routeTo, id.FromStorage())
	}
	if !resume && info.IsPaused && !info.FailoverPending {
		return fmt.Errorf("%w: failover: replication was paused by an operator", dom.ErrFailedPrecondition)
	}
	// Persist ownership before pausing so another process can finish after a crash
	// or Redis failure. Ordinary operator-paused replications remain ineligible.
	if err := r.setFailoverPending(ctx, id, true); err != nil {
		return err
	}
	if err := r.pauseReplicationQueues(ctx, id); err != nil {
		if !resume {
			r.rollbackFailover(ctx, id, info.FailoverPending)
		}
		return fmt.Errorf("failover: %w", err)
	}
	if r.cp != nil && !resume {
		if err := r.failoverInControlPlane(ctx, id, ev); err != nil {
			if !errors.Is(err, controlplane.ErrCommitUncertain) {
				r.rollbackFailover(ctx, id, info.FailoverPending)
			}
			return err
		}
	}
	return r.failoverInRedis(ctx, id)
}

// rollbackFailover undoes step 1 of a failover that definitely did not reach the control plane: the flag is cleared
// and the queues are resumed (a failover is only allowed on a replication that was not paused by an operator). If the
// rollback itself fails the flag stays set, which keeps the failover resumable.
func (r *policySvc) rollbackFailover(ctx context.Context, id entity.UniversalReplicationID, wasPending bool) {
	if wasPending {
		// a previous attempt owns the pending state; leave it so a later retry can finish
		return
	}
	rctx := context.WithoutCancel(ctx)
	if err := r.setFailoverPending(rctx, id, false); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Str("replication", id.AsString()).Msg("failover: unable to clear failover_pending after a failed attempt; a retry will finish it")
		return
	}
	for _, queue := range tasks.AllReplicationQueues(id) {
		if err := r.queueSvc.Resume(rctx, queue); err != nil && !errors.Is(err, dom.ErrNotFound) {
			zerolog.Ctx(ctx).Warn().Err(err).Str("queue", queue).Msg("failover: unable to resume replication queue after a failed attempt")
		}
	}
}

func (r *policySvc) failoverInControlPlane(ctx context.Context, id entity.UniversalReplicationID, ev *controlplane.FailoverEvent) error {
	user, bucket := policyScope(id)
	if err := r.cp.Tx(ctx, func(repos controlplane.Repos) error {
		curRoute, err := r.getRoutingForReplicationRepo(ctx, repos.Routing, id)
		if err != nil {
			return fmt.Errorf("failover: unable to get current routing: %w", err)
		}
		if curRoute != id.FromStorage() {
			return fmt.Errorf("%w: failover: %s is routed to %s, not to replication source %s", dom.ErrInvalidArg, id.AsString(), curRoute, id.FromStorage())
		}
		if err := repos.Routing.Set(ctx, user, bucket, id.ToStorage()); err != nil {
			return err
		}
		if _, isUser := id.AsUserID(); isUser {
			buckets, err := repos.Routing.ListBucketRoutes(ctx, id.User())
			if err != nil {
				return fmt.Errorf("failover: unable to list bucket routings: %w", err)
			}
			for b, to := range buckets {
				if to == id.FromStorage() {
					if err := repos.Routing.Set(ctx, id.User(), b, id.ToStorage()); err != nil {
						return err
					}
				}
			}
		}
		if ev != nil {
			if _, err := repos.FailoverEvents.Put(ctx, *ev); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return controlplane.WrapWriteError(err)
	}
	return nil
}

// failoverInRedis applies the Redis side: routing to the destination (also of the user's bucket routings that point
// to the failed storage) and failover_pending cleared, in one transaction. Idempotent.
func (r *policySvc) failoverInRedis(ctx context.Context, id entity.UniversalReplicationID) error {
	tx := r.userRoutingStore.TxExecutor()
	r.routeToNewInTx(ctx, tx, id)
	_ = r.setFailoverPendingInTx(ctx, tx, id, false)
	if _, isUser := id.AsUserID(); isUser {
		// explicit bucket routings of the user override the user routing: move those pointing to the failed storage too
		buckets, err := r.bucketRoutingStore.ListRoutesOp(ctx, id.User()).Get()
		if err != nil {
			return fmt.Errorf("failover: unable to list bucket routings: %w", err)
		}
		for bucket, to := range buckets {
			if to == id.FromStorage() {
				_ = r.bucketRoutingStore.WithExecutor(tx).SetOp(ctx, entity.NewBucketRoutingPolicyID(id.User(), bucket), id.ToStorage())
			}
		}
	}
	if err := tx.Exec(ctx); err != nil {
		if r.cp != nil {
			return fmt.Errorf("failover: routing is committed in the control plane but Redis was not updated (repeat the failover to complete it): %w", err)
		}
		return fmt.Errorf("failover: unable to update routing: %w", err)
	}
	return nil
}

func (r *policySvc) setFailoverPending(ctx context.Context, id entity.UniversalReplicationID, pending bool) error {
	tx := r.userRoutingStore.TxExecutor()
	_ = r.setFailoverPendingInTx(ctx, tx, id, pending)
	if err := tx.Exec(ctx); err != nil {
		return fmt.Errorf("failover: update pending state: %w", err)
	}
	return nil
}

func (r *policySvc) setFailoverPendingInTx(ctx context.Context, tx store.Executor[redis.Pipeliner], id entity.UniversalReplicationID, pending bool) store.OperationStatus {
	if userID, ok := id.AsUserID(); ok {
		return r.userReplicationStatusStore.WithExecutor(tx).SetFailoverPendingOp(ctx, userID, pending)
	}
	bucketID, _ := id.AsBucketID()
	return r.bucketReplicationStatusStore.WithExecutor(tx).SetFailoverPendingOp(ctx, bucketID, pending)
}
