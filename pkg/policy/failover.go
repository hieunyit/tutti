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

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
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
	// Pause first: if the routing update below fails, the next attempt is idempotent.
	if err := r.PauseReplication(ctx, id); err != nil {
		return fmt.Errorf("failover: %w", err)
	}
	tx := r.userRoutingStore.TxExecutor()
	r.routeToNewInTx(ctx, tx, id)
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
