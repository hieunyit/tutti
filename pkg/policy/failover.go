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
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/store"
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
func (r *policySvc) FailoverReplicationWithEvent(ctx context.Context, id entity.UniversalReplicationID, ev *controlplane.FailoverEvent) error {
	if err := id.Validate(); err != nil {
		return fmt.Errorf("failover: unable to validate replication id: %w", err)
	}
	routeTo, err := r.getRoutingForReplication(ctx, id)
	if err != nil {
		return fmt.Errorf("failover: unable to get current routing: %w", err)
	}
	if routeTo != id.FromStorage() {
		return fmt.Errorf("%w: failover: %s is routed to %s, not to replication source %s", dom.ErrInvalidArg, id.AsString(), routeTo, id.FromStorage())
	}
	info, err := r.GetReplicationPolicyInfoExtended(ctx, id)
	if err != nil {
		return fmt.Errorf("failover: get replication status: %w", err)
	}
	if info.IsPaused && !info.FailoverPending {
		return fmt.Errorf("%w: failover: replication was paused by an operator", dom.ErrFailedPrecondition)
	}
	// Persist ownership before pausing so another process can finish after a crash
	// or Redis failure. Ordinary operator-paused replications remain ineligible.
	if err := r.setFailoverPending(ctx, id, true); err != nil {
		return err
	}
	if err := r.pauseReplicationQueues(ctx, id); err != nil {
		return fmt.Errorf("failover: %w", err)
	}
	if r.cp != nil {
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
	}
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
