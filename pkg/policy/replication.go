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

func toCPReplicationID(id entity.UniversalReplicationID) controlplane.ReplicationID {
	if b, ok := id.AsBucketID(); ok {
		return controlplane.ReplicationID{
			User:        b.User,
			FromStorage: b.FromStorage,
			ToStorage:   b.ToStorage,
			FromBucket:  b.FromBucket,
			ToBucket:    b.ToBucket,
		}
	}
	u, _ := id.AsUserID()
	return controlplane.ReplicationID{
		User:        u.User,
		FromStorage: u.FromStorage,
		ToStorage:   u.ToStorage,
	}
}

func toCPStatus(st entity.ReplicationStatus) controlplane.ReplicationStatus {
	return controlplane.ReplicationStatus{
		CreatedAt:   st.CreatedAt,
		Archived:    st.IsArchived,
		ArchivedAt:  st.ArchivedAt,
		AgentURL:    st.AgentURL,
		EventSource: st.EventSource,
	}
}

func fromCPStatus(st controlplane.ReplicationStatus, failoverPending bool) entity.ReplicationStatus {
	return entity.ReplicationStatus{
		CreatedAt:       st.CreatedAt,
		IsArchived:      st.Archived,
		ArchivedAt:      st.ArchivedAt,
		AgentURL:        st.AgentURL,
		EventSource:     st.EventSource,
		FailoverPending: failoverPending,
	}
}

type ReplicationSvc interface {
	// common methods for user and bucket replication policies
	PauseReplication(ctx context.Context, id entity.UniversalReplicationID) error
	ResumeReplication(ctx context.Context, id entity.UniversalReplicationID) error
	GetReplicationPolicyInfoExtended(ctx context.Context, id entity.UniversalReplicationID) (entity.ReplicationStatusExtended, error)
	GetReplicationGeneration(ctx context.Context, id entity.UniversalReplicationID) (int64, error)

	// user replication policies
	ValidateUserReplicationCreation(ctx context.Context, policy entity.UserReplicationPolicy) error
	AddUserReplicationPolicy(ctx context.Context, policy entity.UserReplicationPolicy, opts entity.ReplicationOptions) (int64, error)
	DeleteUserReplication(ctx context.Context, policy entity.UserReplicationPolicy) error
	ListUserReplicationsInfo(ctx context.Context) (map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended, error)
	IsUserReplicationExists(ctx context.Context, user string) (bool, error)

	// bucket replication policies
	ValidateBucketReplicationCreation(ctx context.Context, policy entity.BucketReplicationPolicy) error
	AddBucketReplicationPolicy(ctx context.Context, id entity.BucketReplicationPolicy, opts entity.ReplicationOptions) (int64, error)
	ListBucketReplicationsInfo(ctx context.Context, user string) (map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended, error)
	DeleteBucketReplication(ctx context.Context, id entity.BucketReplicationPolicy) error
}

var _ ReplicationSvc = (*policySvc)(nil)

func (r *policySvc) PauseReplication(ctx context.Context, id entity.UniversalReplicationID) error {
	if err := id.Validate(); err != nil {
		return fmt.Errorf("pause replication: unable to validate replication status id: %w", err)
	}
	// An explicit operator action cancels automatic failover recovery ownership.
	if err := r.setFailoverPending(ctx, id, false); err != nil {
		return err
	}
	return r.pauseReplicationQueues(ctx, id)
}

func (r *policySvc) pauseReplicationQueues(ctx context.Context, id entity.UniversalReplicationID) error {
	for _, queue := range tasks.AllReplicationQueues(id) {
		err := r.queueSvc.Pause(ctx, queue)
		if err != nil {
			if errors.Is(err, dom.ErrNotFound) {
				// since queues created on demand, it is possible that some queues are not created yet
				// for example, event replication queue will be created on the first write request to chorus proxy
				continue
			}
			return fmt.Errorf("pause replication: unable to pause queue %s: %w", queue, err)
		}
	}
	return nil
}

func (r *policySvc) ResumeReplication(ctx context.Context, id entity.UniversalReplicationID) error {
	if err := id.Validate(); err != nil {
		return fmt.Errorf("resume replication: unable to validate replication status id: %w", err)
	}
	if err := r.setFailoverPending(ctx, id, false); err != nil {
		return err
	}
	for _, queue := range tasks.AllReplicationQueues(id) {
		err := r.queueSvc.Resume(ctx, queue)
		if err != nil {
			if errors.Is(err, dom.ErrNotFound) {
				// since queues created on demand, it is possible that some queues are not created yet
				// for example, event replication queue will be created on the first write request to chorus proxy
				continue
			}
			return fmt.Errorf("resume replication: unable to resume queue %s: %w", queue, err)
		}
	}
	return nil
}

func (r *policySvc) GetReplicationPolicyInfoExtended(ctx context.Context, id entity.UniversalReplicationID) (entity.ReplicationStatusExtended, error) {
	if err := id.Validate(); err != nil {
		return entity.ReplicationStatusExtended{}, fmt.Errorf("get replication status: unable to validate replication status id: %w", err)
	}
	tx := r.bucketReplicationStatusStore.TxExecutor()
	statusRes := r.getReplicationStatusInTx(ctx, tx, id)
	switchRes := r.getSwitchInTx(ctx, tx, id)
	_ = tx.Exec(ctx)

	var status entity.ReplicationStatus
	var err error
	if r.cp != nil {
		cpStatus, cpErr := r.cp.Repos().Replications.GetStatus(ctx, toCPReplicationID(id))
		switch {
		case cpErr == nil:
			redisSt, _ := statusRes.Get()
			status = fromCPStatus(cpStatus, redisSt.FailoverPending)
		case !controlplane.IsUnavailable(cpErr):
			return entity.ReplicationStatusExtended{}, cpErr
		default:
			zerolog.Ctx(ctx).Warn().Err(cpErr).Msg("replication: PostgreSQL GetStatus failed, falling back to Redis")
			status, err = statusRes.Get()
			if err != nil {
				return entity.ReplicationStatusExtended{}, err
			}
		}
	} else {
		status, err = statusRes.Get()
		if err != nil {
			return entity.ReplicationStatusExtended{}, err
		}
	}
	switchInfo, err := switchRes.Get()
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return entity.ReplicationStatusExtended{}, err
	}
	var switchInfoPtr *entity.ReplicationSwitchInfo
	if switchInfo.ReplicationIDStr != "" {
		switchInfoPtr = &switchInfo
	}

	return r.fillExtendedReplicationStatus(ctx, id, &status, switchInfoPtr)

}

func (r *policySvc) GetReplicationGeneration(ctx context.Context, id entity.UniversalReplicationID) (int64, error) {
	if r.cp != nil {
		if err := id.Validate(); err != nil {
			return 0, err
		}
		g, err := r.cp.Repos().Replications.Generation(ctx, toCPReplicationID(id))
		if err == nil || !controlplane.IsUnavailable(err) {
			return g, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("replication: PostgreSQL Generation failed, falling back to Redis")
	}
	return r.replicationGenerationStore.Get(ctx, id)
}

func (r *policySvc) getReplicationStatusInTx(ctx context.Context, tx store.Executor[redis.Pipeliner], policy entity.UniversalReplicationID) store.OperationResult[entity.ReplicationStatus] {
	if bucketPolicy, ok := policy.AsBucketID(); ok {
		return r.bucketReplicationStatusStore.WithExecutor(tx).GetOp(ctx, bucketPolicy)
	} else if userPolicy, ok := policy.AsUserID(); ok {
		return r.userReplicationStatusStore.WithExecutor(tx).GetOp(ctx, userPolicy)
	} else {
		// should never happen
		panic(fmt.Sprintf("unknown policy type %s", policy.AsString()))
	}
}

func (r *policySvc) fillExtendedReplicationStatus(ctx context.Context, id entity.UniversalReplicationID, status *entity.ReplicationStatus, switchInfo *entity.ReplicationSwitchInfo) (entity.ReplicationStatusExtended, error) {
	result := entity.ReplicationStatusExtended{
		ReplicationStatus:    status,
		IsPaused:             false,
		Switch:               switchInfo,
		InitMigration:        entity.QueueStats{},
		EventMigration:       entity.QueueStats{},
		InitMigrationListing: entity.QueueStats{},
	}
	// replication is paused if at least one of the queues is paused
	paused := false
	// get initial migration listing queues stats
	var err error
	paused, result.InitMigrationListing, err = r.buildQueueStats(ctx, tasks.InitMigrationListQueue(id))
	if err != nil {
		return entity.ReplicationStatusExtended{}, fmt.Errorf("unable to get init migration queue stats: %w", err)
	}
	result.IsPaused = result.IsPaused || paused

	// get initial migration queues stats
	paused, result.InitMigration, err = r.buildQueueStats(ctx, tasks.InitMigrationCopyQueue(id))
	if err != nil {
		return entity.ReplicationStatusExtended{}, fmt.Errorf("unable to get init migration queue stats: %w", err)
	}
	result.IsPaused = result.IsPaused || paused

	// get event migration queues stats
	paused, result.EventMigration, err = r.buildQueueStats(ctx, tasks.EventMigrationQueue(id))
	if err != nil {
		return entity.ReplicationStatusExtended{}, fmt.Errorf("unable to get event migration queue stats: %w", err)
	}
	result.IsPaused = result.IsPaused || paused

	return result, nil
}

func (r *policySvc) buildQueueStats(ctx context.Context, queue string) (bool, entity.QueueStats, error) {
	stats, err := r.queueSvc.Stats(ctx, queue)
	if errors.Is(err, dom.ErrNotFound) {
		// queue does not exist, so no stats available
		return false, entity.QueueStats{}, nil
	}
	if err != nil {
		return false, entity.QueueStats{}, fmt.Errorf("unable to get queue stats for %s: %w", queue, err)
	}
	return stats.Paused, entity.QueueStats{
		Unprocessed: stats.Unprocessed,
		// asynq's ProcessedTotal counts every finished attempt, including retries
		// and archived (permanently failed) tasks, while FailedTotal counts only
		// the failed attempts. Their difference is the number of tasks that
		// actually succeeded, which is what we report as Done.
		Done:        stats.ProcessedTotal - stats.FailedTotal,
		Archived:    stats.Archived,
		Failed:      stats.FailedTotal,
		Latency:     stats.Latency,
		MemoryUsage: stats.MemoryUsage,
	}, nil
}

func (r *policySvc) validateUserReplicationCreationRepo(ctx context.Context, repos controlplane.Repos, policy entity.UserReplicationPolicy) error {
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("unable to validate user replication policy: %w", err)
	}
	uid := entity.UniversalFromUserReplication(policy)
	cpID := toCPReplicationID(uid)
	_, err := repos.Replications.GetStatus(ctx, cpID)
	switch {
	case err == nil:
		return fmt.Errorf("%w: replication already exists, delete it first", dom.ErrAlreadyExists)
	case errors.Is(err, dom.ErrNotFound):
		break
	default:
		return fmt.Errorf("unable to check existing replication status: %w", err)
	}
	routeTo, err := r.getRoutingForReplicationRepo(ctx, repos.Routing, uid)
	if err != nil {
		return fmt.Errorf("unable to get routing for user replication: %w", err)
	}
	if routeTo != policy.FromStorage {
		return fmt.Errorf("%w: unable to create user %s replication from %s because it is different from routing %s", dom.ErrInvalidArg, policy.User, policy.FromStorage, routeTo)
	}
	if policy.User != "" {
		// per-user exclusivity and routing apply to a real user only, never to storage-level replications
		bucketPolicyExists, err := r.hasBucketReplicationForUserRepo(ctx, repos.Replications, policy.User)
		if err != nil {
			return fmt.Errorf("unable to check existing bucket replications: %w", err)
		}
		if bucketPolicyExists {
			return fmt.Errorf("%w: unable to create user replication when there are existing bucket replications", dom.ErrInvalidArg)
		}
		bucketRoutings, err := repos.Routing.ListBucketRoutes(ctx, policy.User)
		if err != nil {
			return fmt.Errorf("unable to get bucket routings for user replication: %w", err)
		}
		for bucket, routeTo := range bucketRoutings {
			if routeTo != policy.FromStorage {
				return fmt.Errorf("%w: unable to create user %s replication from %s because bucket %s is routed to %s", dom.ErrInvalidArg, policy.User, policy.FromStorage, bucket, routeTo)
			}
		}
	}
	userSwitch, err := r.userReplicationSwitchStore.Get(ctx, policy.User)
	switch {
	case err == nil:
		return fmt.Errorf("%w: cannot have multiple user replications when user replication switch exists: %s", dom.ErrInvalidArg, userSwitch.ReplicationIDStr)
	case errors.Is(err, dom.ErrNotFound):
		break
	default:
		return fmt.Errorf("unable to check existing user replication switch: %w", err)
	}
	return nil
}

func (r *policySvc) ValidateUserReplicationCreation(ctx context.Context, policy entity.UserReplicationPolicy) error {
	if r.cp != nil {
		err := r.validateUserReplicationCreationRepo(ctx, r.cp.Repos(), policy)
		if err == nil || !controlplane.IsUnavailable(err) {
			return err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("replication: PostgreSQL ValidateUserReplicationCreation failed, falling back to Redis")
	}
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("unable to validate user replication policy: %w", err)
	}
	// check if already exists
	_, err := r.userReplicationStatusStore.GetOp(ctx, policy).Get()
	switch {
	case err == nil:
		return fmt.Errorf("%w: replication already exists, delete it first", dom.ErrAlreadyExists)
	case errors.Is(err, dom.ErrNotFound):
		// not found, good
		break
	default:
		return fmt.Errorf("unable to check existing replication status: %w", err)
	}
	// check if routing match
	uid := entity.UniversalFromUserReplication(policy)
	routeTo, err := r.getRoutingForReplication(ctx, uid)
	if err != nil {
		return fmt.Errorf("unable to get routing for user replication: %w", err)
	}
	if routeTo != policy.FromStorage {
		return fmt.Errorf("%w: unable to create user %s replication from %s because it is different from routing %s", dom.ErrInvalidArg, policy.User, policy.FromStorage, routeTo)
	}
	if policy.User != "" {
		// per-user exclusivity and routing apply to a real user only, never to storage-level replications
		// check if there are existing bucket replications for this user
		bucketPolicyExists, err := r.hasBucketReplicationForUser(ctx, policy.User)
		if err != nil {
			return fmt.Errorf("unable to check existing bucket replications: %w", err)
		}
		if bucketPolicyExists {
			return fmt.Errorf("%w: unable to create user replication when there are existing bucket replications", dom.ErrInvalidArg)
		}
		// bucket routing overrides user routing: writes to bucket routed elsewhere would bypass replication
		bucketRoutings, err := r.bucketRoutingStore.ListRoutesOp(ctx, policy.User).Get()
		if err != nil {
			return fmt.Errorf("unable to get bucket routings for user replication: %w", err)
		}
		for bucket, routeTo := range bucketRoutings {
			if routeTo != policy.FromStorage {
				return fmt.Errorf("%w: unable to create user %s replication from %s because bucket %s is routed to %s", dom.ErrInvalidArg, policy.User, policy.FromStorage, bucket, routeTo)
			}
		}
	}

	// check if replication switch already exists for this user
	userSwitch, err := r.userReplicationSwitchStore.Get(ctx, policy.User)
	switch {
	case err == nil:
		return fmt.Errorf("%w: cannot have multiple user replications when user replication switch exists: %s", dom.ErrInvalidArg, userSwitch.ReplicationIDStr)
	case errors.Is(err, dom.ErrNotFound):
		// not found, good
		break
	default:
		return fmt.Errorf("unable to check existing user replication switch: %w", err)
	}
	return nil
}

func (r *policySvc) AddUserReplicationPolicy(ctx context.Context, policy entity.UserReplicationPolicy, opts entity.ReplicationOptions) (int64, error) {
	if r.cp != nil {
		var gen int64
		err := r.cp.Tx(ctx, func(repos controlplane.Repos) error {
			if err := r.validateUserReplicationCreationRepo(ctx, repos, policy); err != nil {
				return err
			}
			uid := entity.UniversalFromUserReplication(policy)
			cpID := toCPReplicationID(uid)
			if policy.User != "" { // storage-level replications are not bound to routing
				if err := repos.Routing.Set(ctx, policy.User, "", policy.FromStorage); err != nil {
					return err
				}
			}
			if err := repos.Replications.Add(ctx, cpID, toCPStatus(entity.StatusFromOptions(opts))); err != nil {
				return err
			}
			var err error
			gen, err = repos.Replications.IncrementGeneration(ctx, cpID)
			return err
		})
		return gen, controlplane.WrapWriteError(err)
	}
	if err := r.ValidateUserReplicationCreation(ctx, policy); err != nil {
		return 0, err
	}
	uid := entity.UniversalFromUserReplication(policy)
	tx := r.userReplicationPolicyStore.TxExecutor()
	// 1. set explicit user-level routing (storage-level replications are not bound to routing)
	if policy.User != "" {
		_ = r.userRoutingStore.WithExecutor(tx).SetOp(ctx, policy.User, policy.FromStorage)
	}
	// 2. add replication policy
	_ = r.userReplicationPolicyStore.WithExecutor(tx).AddOp(ctx, policy)
	// 3. add replication status
	_ = r.userReplicationStatusStore.WithExecutor(tx).AddOp(ctx, policy, entity.StatusFromOptions(opts))
	// 4. start new replication generation
	generation := r.replicationGenerationStore.WithExecutor(tx).IncrementOp(ctx, uid)

	if err := tx.Exec(ctx); err != nil {
		return 0, fmt.Errorf("unable to execute transaction: %w", err)
	}
	return generation.Get()
}

func (r *policySvc) DeleteUserReplication(ctx context.Context, policy entity.UserReplicationPolicy) error {
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("unable to validate user replication policy: %w", err)
	}
	// cannot delete if switch exists
	_, err := r.userReplicationSwitchStore.Get(ctx, policy.User)
	switch {
	case err == nil:
		return fmt.Errorf("%w: cannot delete user replication with replication switch. Delete the switch first", dom.ErrInvalidArg)
	case errors.Is(err, dom.ErrNotFound):
		// not found, good
		break
	default:
		return fmt.Errorf("unable to check existing user replication switch: %w", err)
	}
	uid := entity.UniversalFromUserReplication(policy)
	if r.cp != nil {
		cpID := toCPReplicationID(uid)
		var notFound bool
		err := r.cp.Tx(ctx, func(repos controlplane.Repos) error {
			if _, err := repos.Replications.GetStatus(ctx, cpID); errors.Is(err, dom.ErrNotFound) {
				notFound = true
				return nil
			} else if err != nil {
				return err
			}
			if err := repos.Replications.Remove(ctx, cpID); err != nil {
				return err
			}
			if err := repos.Replications.DeleteStatus(ctx, cpID); err != nil {
				return err
			}
			_, err := repos.Replications.IncrementGeneration(ctx, cpID)
			return err
		})
		if err != nil {
			return controlplane.WrapWriteError(err)
		}
		if notFound {
			if err := tasks.StopAndPurgeReplicationQueues(ctx, r.queueSvc, uid); err != nil {
				return fmt.Errorf("unable to remove queues of deleted replication: %w", err)
			}
			return nil
		}
		if err := tasks.StopAndPurgeReplicationQueues(ctx, r.queueSvc, uid); err != nil {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("unable to remove queues of deleted replication, retry delete to remove them")
		}
		return nil
	}
	// check if policy exists
	_, err = r.userReplicationStatusStore.GetOp(ctx, policy).Get()
	if errors.Is(err, dom.ErrNotFound) {
		if err := tasks.StopAndPurgeReplicationQueues(ctx, r.queueSvc, uid); err != nil {
			return fmt.Errorf("unable to remove queues of deleted replication: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to get existing replication status: %w", err)
	}

	// delete
	tx := r.userReplicationPolicyStore.TxExecutor()
	// 1. delete replication policy
	_ = r.userReplicationPolicyStore.WithExecutor(tx).RemoveOp(ctx, policy)
	// 2. delete replication status
	_ = r.userReplicationStatusStore.WithExecutor(tx).DeleteOp(ctx, policy)
	// we do NOT delete user-level routing.
	// user may be deleting replication after successful switch.
	// or in case if routing was set manually.
	// 3. start new replication generation to drop already created tasks
	_ = r.replicationGenerationStore.WithExecutor(tx).IncrementOp(ctx, uid)

	if err := tx.Exec(ctx); err != nil {
		return fmt.Errorf("unable to execute transaction: %w", err)
	}
	if err := tasks.StopAndPurgeReplicationQueues(ctx, r.queueSvc, uid); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("unable to remove queues of deleted replication, retry delete to remove them")
	}
	return nil
}

func (r *policySvc) validateBucketReplicationCreationRepo(ctx context.Context, repos controlplane.Repos, policy entity.BucketReplicationPolicy) error {
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("unable to validate bucket replication policy: %w", err)
	}
	uid := entity.UniversalFromBucketReplication(policy)
	cpID := toCPReplicationID(uid)
	_, err := repos.Replications.GetStatus(ctx, cpID)
	switch {
	case err == nil:
		return fmt.Errorf("%w: replication already exists, delete it first", dom.ErrAlreadyExists)
	case errors.Is(err, dom.ErrNotFound):
		break
	default:
		return fmt.Errorf("unable to check existing replication status: %w", err)
	}
	routeTo, err := r.getRoutingForReplicationRepo(ctx, repos.Routing, uid)
	if err != nil {
		return fmt.Errorf("unable to get routing for bucket replication: %w", err)
	}
	if routeTo != policy.FromStorage {
		return fmt.Errorf("%w: unable to create bucket %s replication from %s because it is different from routing %s", dom.ErrInvalidArg, policy.User, policy.FromStorage, routeTo)
	}
	if policy.User != "" {
		// per-user exclusivity applies to a real user only
		userReplicationExists, err := r.hasUserReplicationRepo(ctx, repos.Replications, policy.User)
		if err != nil {
			return fmt.Errorf("unable to check existing user replications: %w", err)
		}
		if userReplicationExists {
			return fmt.Errorf("%w: unable to create bucket replication when there are existing user replications", dom.ErrInvalidArg)
		}
	}
	userSwitch, err := r.bucketReplicationSwitchStore.Get(ctx, policy.LookupID())
	switch {
	case err == nil:
		return fmt.Errorf("%w: cannot have multiple bucket replications when bucket replication switch exists: %s", dom.ErrInvalidArg, userSwitch.ReplicationIDStr)
	case errors.Is(err, dom.ErrNotFound):
		break
	default:
		return fmt.Errorf("unable to check existing bucket replication switch: %w", err)
	}
	inUse, err := repos.Replications.IsDestinationInUse(ctx, policy.User, policy.ToStorage, policy.ToBucket)
	if err != nil {
		return fmt.Errorf("unable to check existing replications for destination bucket: %w", err)
	}
	if inUse {
		return fmt.Errorf("%w: cannot use bucket %s as destination because it is already used in another replication", dom.ErrInvalidArg, policy.ToBucket)
	}
	return nil
}

func (r *policySvc) ValidateBucketReplicationCreation(ctx context.Context, policy entity.BucketReplicationPolicy) error {
	if r.cp != nil {
		err := r.validateBucketReplicationCreationRepo(ctx, r.cp.Repos(), policy)
		if err == nil || !controlplane.IsUnavailable(err) {
			return err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("replication: PostgreSQL ValidateBucketReplicationCreation failed, falling back to Redis")
	}
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("unable to validate bucket replication policy: %w", err)
	}
	// check if already exists
	_, err := r.bucketReplicationStatusStore.GetOp(ctx, policy).Get()
	switch {
	case err == nil:
		return fmt.Errorf("%w: replication already exists, delete it first", dom.ErrAlreadyExists)
	case errors.Is(err, dom.ErrNotFound):
		// not found, good
		break
	default:
		return fmt.Errorf("unable to check existing replication status: %w", err)
	}
	// check if routing match
	uid := entity.UniversalFromBucketReplication(policy)
	routeTo, err := r.getRoutingForReplication(ctx, uid)
	if err != nil {
		return fmt.Errorf("unable to get routing for bucket replication: %w", err)
	}
	if routeTo != policy.FromStorage {
		return fmt.Errorf("%w: unable to create bucket %s replication from %s because it is different from routing %s", dom.ErrInvalidArg, policy.User, policy.FromStorage, routeTo)
	}
	// check if there are existing user replications for this user (a real user only, never storage-level)
	if policy.User != "" {
		userReplicationExists, err := r.hasUserReplication(ctx, policy.User)
		if err != nil {
			return fmt.Errorf("unable to check existing user replications: %w", err)
		}
		if userReplicationExists {
			return fmt.Errorf("%w: unable to create bucket replication when there are existing user replications", dom.ErrInvalidArg)
		}
	}

	// check if replication switch already exists
	userSwitch, err := r.bucketReplicationSwitchStore.Get(ctx, policy.LookupID())
	switch {
	case err == nil:
		return fmt.Errorf("%w: cannot have multiple bucket replications when bucket replication switch exists: %s", dom.ErrInvalidArg, userSwitch.ReplicationIDStr)
	case errors.Is(err, dom.ErrNotFound):
		// not found, good
		break
	default:
		return fmt.Errorf("unable to check existing bucket replication switch: %w", err)
	}
	// check if bucket already used as destination to avoid many-to-one writes from other replications
	// we check it only for bucket-level replications because because of custom destination bucket name
	inUse, err := r.bucketReplicationPolicyStore.IsDestinationInUse(ctx, policy)
	if err != nil {
		return fmt.Errorf("unable to check existing replications for destination bucket: %w", err)
	}
	if inUse {
		return fmt.Errorf("%w: cannot use bucket %s as destination because it is already used in another replication", dom.ErrInvalidArg, policy.ToBucket)
	}
	return nil
}

func (r *policySvc) AddBucketReplicationPolicy(ctx context.Context, policy entity.BucketReplicationPolicy, opts entity.ReplicationOptions) (int64, error) {
	if r.cp != nil {
		var gen int64
		err := r.cp.Tx(ctx, func(repos controlplane.Repos) error {
			if err := r.validateBucketReplicationCreationRepo(ctx, repos, policy); err != nil {
				return err
			}
			uid := entity.UniversalFromBucketReplication(policy)
			cpID := toCPReplicationID(uid)
			if policy.User != "" { // storage-level replications are not bound to routing
				if err := repos.Routing.Set(ctx, policy.User, policy.FromBucket, policy.FromStorage); err != nil {
					return err
				}
			}
			if err := repos.Replications.Add(ctx, cpID, toCPStatus(entity.StatusFromOptions(opts))); err != nil {
				return err
			}
			if policy.ToBucket != policy.FromBucket {
				if err := repos.Routing.Block(ctx, policy.User, policy.ToBucket); err != nil {
					return err
				}
			}
			var err error
			gen, err = repos.Replications.IncrementGeneration(ctx, cpID)
			return err
		})
		return gen, controlplane.WrapWriteError(err)
	}
	if err := r.ValidateBucketReplicationCreation(ctx, policy); err != nil {
		return 0, err
	}
	uid := entity.UniversalFromBucketReplication(policy)
	tx := r.bucketReplicationPolicyStore.TxExecutor()
	// 1. set explicit bucket-level routing (storage-level replications are not bound to routing)
	if policy.User != "" {
		_ = r.bucketRoutingStore.WithExecutor(tx).SetOp(ctx, policy.RoutingID(), policy.FromStorage)
	}
	// 2. add replication policy
	_ = r.bucketReplicationPolicyStore.WithExecutor(tx).AddOp(ctx, policy)
	// 3. add replication status
	_ = r.bucketReplicationStatusStore.WithExecutor(tx).AddOp(ctx, policy, entity.StatusFromOptions(opts))
	// 4. block routing for custom destination bucket to avoid many-to-one writes from proxy
	if policy.ToBucket != policy.FromBucket {
		_ = r.bucketRoutingStore.WithExecutor(tx).BlockOp(ctx, entity.NewBucketRoutingPolicyID(policy.User, policy.ToBucket))
	}
	// 5. start new replication generation
	generation := r.replicationGenerationStore.WithExecutor(tx).IncrementOp(ctx, uid)

	if err := tx.Exec(ctx); err != nil {
		return 0, fmt.Errorf("unable to execute transaction: %w", err)
	}
	return generation.Get()
}

func (r *policySvc) DeleteBucketReplication(ctx context.Context, policy entity.BucketReplicationPolicy) error {
	if err := policy.Validate(); err != nil {
		return fmt.Errorf("unable to validate bucket replication policy: %w", err)
	}
	// cannot delete if switch exists
	_, err := r.bucketReplicationSwitchStore.Get(ctx, policy.LookupID())
	switch {
	case err == nil:
		return fmt.Errorf("%w: cannot delete bucket replication with replication switch. Delete the switch first", dom.ErrInvalidArg)
	case errors.Is(err, dom.ErrNotFound):
		// not found, good
		break
	default:
		return fmt.Errorf("unable to check existing bucket replication switch: %w", err)
	}
	uid := entity.UniversalFromBucketReplication(policy)
	if r.cp != nil {
		cpID := toCPReplicationID(uid)
		var notFound bool
		err := r.cp.Tx(ctx, func(repos controlplane.Repos) error {
			if _, err := repos.Replications.GetStatus(ctx, cpID); errors.Is(err, dom.ErrNotFound) {
				notFound = true
				return nil
			} else if err != nil {
				return err
			}
			if err := repos.Replications.Remove(ctx, cpID); err != nil {
				return err
			}
			if err := repos.Replications.DeleteStatus(ctx, cpID); err != nil {
				return err
			}
			if policy.ToBucket != policy.FromBucket {
				if err := repos.Routing.Unblock(ctx, policy.User, policy.ToBucket); err != nil {
					return err
				}
			}
			_, err := repos.Replications.IncrementGeneration(ctx, cpID)
			return err
		})
		if err != nil {
			return controlplane.WrapWriteError(err)
		}
		if notFound {
			if err := tasks.StopAndPurgeReplicationQueues(ctx, r.queueSvc, uid); err != nil {
				return fmt.Errorf("unable to remove queues of deleted replication: %w", err)
			}
			return nil
		}
		if err := tasks.StopAndPurgeReplicationQueues(ctx, r.queueSvc, uid); err != nil {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("unable to remove queues of deleted replication, retry delete to remove them")
		}
		return nil
	}
	// check if policy exists
	_, err = r.bucketReplicationStatusStore.GetOp(ctx, policy).Get()
	if errors.Is(err, dom.ErrNotFound) {
		if err := tasks.StopAndPurgeReplicationQueues(ctx, r.queueSvc, uid); err != nil {
			return fmt.Errorf("unable to remove queues of deleted replication: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to get existing replication status: %w", err)
	}

	// delete
	tx := r.bucketReplicationPolicyStore.TxExecutor()
	// 1. delete replication policy
	_ = r.bucketReplicationPolicyStore.WithExecutor(tx).RemoveOp(ctx, policy)
	// 2. delete replication status
	_ = r.bucketReplicationStatusStore.WithExecutor(tx).DeleteOp(ctx, policy)
	// 3. unbock routing for custom destination bucket
	if policy.ToBucket != policy.FromBucket {
		_ = r.bucketRoutingStore.WithExecutor(tx).UnblockOp(ctx, entity.NewBucketRoutingPolicyID(policy.User, policy.ToBucket))
	}
	// 4. start new replication generation to drop already created tasks
	_ = r.replicationGenerationStore.WithExecutor(tx).IncrementOp(ctx, uid)

	if err := tx.Exec(ctx); err != nil {
		return fmt.Errorf("unable to execute transaction: %w", err)
	}
	if err := tasks.StopAndPurgeReplicationQueues(ctx, r.queueSvc, uid); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("unable to remove queues of deleted replication, retry delete to remove them")
	}
	return nil
}

func (r *policySvc) listBucketStatuses(ctx context.Context, user string) (map[entity.BucketReplicationPolicy]entity.ReplicationStatus, error) {
	if r.cp != nil {
		// ListStatuses("") lists every user: the empty user (storage-level) is filtered below
		cpStatuses, err := r.cp.Repos().Replications.ListStatuses(ctx, user)
		if err == nil {
			redisStatuses, _ := r.bucketReplicationStatusStore.List(ctx, user)
			statuses := make(map[entity.BucketReplicationPolicy]entity.ReplicationStatus)
			for id, st := range cpStatuses {
				if id.IsUserLevel() || id.User != user {
					continue
				}
				bp := entity.BucketReplicationPolicy{
					User:        id.User,
					FromStorage: id.FromStorage,
					FromBucket:  id.FromBucket,
					ToStorage:   id.ToStorage,
					ToBucket:    id.ToBucket,
				}
				var pending bool
				if rst, ok := redisStatuses[bp]; ok {
					pending = rst.FailoverPending
				}
				statuses[bp] = fromCPStatus(st, pending)
			}
			if len(statuses) == 0 {
				return nil, fmt.Errorf("%w: no bucket replication policies found for user %s", dom.ErrNotFound, user)
			}
			return statuses, nil
		}
		if !controlplane.IsUnavailable(err) {
			return nil, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Str("user", user).Msg("replication: PostgreSQL ListStatuses failed, falling back to Redis")
	}
	return r.bucketReplicationStatusStore.List(ctx, user)
}

func (r *policySvc) listUserStatuses(ctx context.Context) (map[entity.UserReplicationPolicy]entity.ReplicationStatus, error) {
	if r.cp != nil {
		cpStatuses, err := r.cp.Repos().Replications.ListStatuses(ctx, "")
		if err == nil {
			redisStatuses, _ := r.userReplicationStatusStore.List(ctx)
			statuses := make(map[entity.UserReplicationPolicy]entity.ReplicationStatus)
			for id, st := range cpStatuses {
				if !id.IsUserLevel() {
					continue
				}
				up := entity.UserReplicationPolicy{
					User:        id.User,
					FromStorage: id.FromStorage,
					ToStorage:   id.ToStorage,
				}
				var pending bool
				if rst, ok := redisStatuses[up]; ok {
					pending = rst.FailoverPending
				}
				statuses[up] = fromCPStatus(st, pending)
			}
			if len(statuses) == 0 {
				return nil, fmt.Errorf("%w: no user replication policies found", dom.ErrNotFound)
			}
			return statuses, nil
		}
		if !controlplane.IsUnavailable(err) {
			return nil, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("replication: PostgreSQL ListStatuses failed, falling back to Redis")
	}
	return r.userReplicationStatusStore.List(ctx)
}

func (r *policySvc) ListBucketReplicationsInfo(ctx context.Context, user string) (map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended, error) {
	// get all statuses
	statuses, err := r.listBucketStatuses(ctx, user)
	if err != nil {
		return nil, err
	}
	// fetch switches in batch
	batch := r.bucketReplicationSwitchStore.TxExecutor()
	inBatch := r.bucketReplicationSwitchStore.WithExecutor(batch)
	switches := make(map[entity.BucketReplicationPolicy]store.OperationResult[entity.ReplicationSwitchInfo], len(statuses))
	for policy := range statuses {
		switches[policy] = inBatch.GetOp(ctx, policy.LookupID())
	}
	_ = batch.Exec(ctx)

	// fill results
	result := make(map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended, len(statuses))
	for policy, status := range statuses {
		switchInfo, err := switches[policy].Get()
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, fmt.Errorf("unable to get existing bucket replication switch: %w", err)
		}
		var switchInfoPtr *entity.ReplicationSwitchInfo
		if err == nil {
			switchInfoPtr = &switchInfo
		}
		st := status
		extendedStatus, err := r.fillExtendedReplicationStatus(ctx, entity.UniversalFromBucketReplication(policy), &st, switchInfoPtr)
		if err != nil {
			return nil, fmt.Errorf("unable to fill extended status for policy %#v: %w", policy, err)
		}
		result[policy] = extendedStatus
	}
	return result, nil
}

func (r *policySvc) ListUserReplicationsInfo(ctx context.Context) (map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended, error) {
	// get all statuses
	statuses, err := r.listUserStatuses(ctx)
	if err != nil {
		return nil, err
	}
	// fetch switches in batch
	batch := r.userReplicationSwitchStore.TxExecutor()
	inBatch := r.userReplicationSwitchStore.WithExecutor(batch)
	switches := make(map[entity.UserReplicationPolicy]store.OperationResult[entity.ReplicationSwitchInfo], len(statuses))
	for policy := range statuses {
		switches[policy] = inBatch.GetOp(ctx, policy.User)
	}
	_ = batch.Exec(ctx)

	// fill results
	result := make(map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended, len(statuses))
	for policy, status := range statuses {
		switchInfo, err := switches[policy].Get()
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, fmt.Errorf("unable to get existing user replication switch: %w", err)
		}
		var switchInfoPtr *entity.ReplicationSwitchInfo
		if err == nil {
			switchInfoPtr = &switchInfo
		}
		st := status
		extendedStatus, err := r.fillExtendedReplicationStatus(ctx, entity.UniversalFromUserReplication(policy), &st, switchInfoPtr)
		if err != nil {
			return nil, fmt.Errorf("unable to fill extended status for policy %#v: %w", policy, err)
		}
		result[policy] = extendedStatus
	}
	return result, nil
}

func (r *policySvc) IsUserReplicationExists(ctx context.Context, user string) (bool, error) {
	if r.cp != nil {
		statuses, err := r.cp.Repos().Replications.ListStatuses(ctx, user)
		if err == nil {
			for id := range statuses {
				if id.IsUserLevel() && id.User == user {
					return true, nil
				}
			}
			return false, nil
		}
		if !controlplane.IsUnavailable(err) {
			return false, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Str("user", user).Msg("replication: PostgreSQL IsUserReplicationExists failed, falling back to Redis")
	}
	return r.userReplicationStatusStore.Exists(ctx, user)
}

func (r *policySvc) hasUserReplicationRepo(ctx context.Context, replRepo controlplane.Replications, user string) (bool, error) {
	if user == "" {
		return false, fmt.Errorf("%w: user is required", dom.ErrInvalidArg)
	}
	policies, err := replRepo.Policies(ctx, user, "")
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return false, err
	}
	if len(policies) != 0 {
		return true, nil
	}
	return r.userReplicationSwitchStore.IsZeroDowntimeActiveOp(ctx, user).Get()
}

func (r *policySvc) hasBucketReplicationRepo(ctx context.Context, replRepo controlplane.Replications, id entity.BucketReplicationPolicyID) (bool, error) {
	if id.User == "" || id.FromBucket == "" {
		return false, fmt.Errorf("%w: user and bucket are required", dom.ErrInvalidArg)
	}
	policies, err := replRepo.Policies(ctx, id.User, id.FromBucket)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return false, err
	}
	if len(policies) != 0 {
		return true, nil
	}
	return r.bucketReplicationSwitchStore.IsZeroDowntimeActiveOp(ctx, id).Get()
}

func (r *policySvc) hasBucketReplicationForUserRepo(ctx context.Context, replRepo controlplane.Replications, user string) (bool, error) {
	if user == "" {
		return false, fmt.Errorf("%w: user is required", dom.ErrInvalidArg)
	}
	exists, err := replRepo.BucketReplicationExistsForUser(ctx, user)
	if err != nil || exists {
		return exists, err
	}
	return r.bucketReplicationSwitchStore.IsZeroDowntimeActiveForUserOp(ctx, user).Get()
}

// Zero-downtime switch removes replication policy when it starts,
// but the replication is active until the switch is done.
func (r *policySvc) hasUserReplication(ctx context.Context, user string) (bool, error) {
	if r.cp != nil {
		exists, err := r.hasUserReplicationRepo(ctx, r.cp.Repos().Replications, user)
		if err == nil || !controlplane.IsUnavailable(err) {
			return exists, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Str("user", user).Msg("replication: PostgreSQL hasUserReplication failed, falling back to Redis")
	}
	policies, err := r.userReplicationPolicyStore.Get(ctx, user)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return false, err
	}
	if len(policies) != 0 {
		return true, nil
	}
	return r.userReplicationSwitchStore.IsZeroDowntimeActiveOp(ctx, user).Get()
}

func (r *policySvc) hasBucketReplication(ctx context.Context, id entity.BucketReplicationPolicyID) (bool, error) {
	if r.cp != nil {
		exists, err := r.hasBucketReplicationRepo(ctx, r.cp.Repos().Replications, id)
		if err == nil || !controlplane.IsUnavailable(err) {
			return exists, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("replication: PostgreSQL hasBucketReplication failed, falling back to Redis")
	}
	policies, err := r.bucketReplicationPolicyStore.Get(ctx, id)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return false, err
	}
	if len(policies) != 0 {
		return true, nil
	}
	return r.bucketReplicationSwitchStore.IsZeroDowntimeActiveOp(ctx, id).Get()
}

func (r *policySvc) hasBucketReplicationForUser(ctx context.Context, user string) (bool, error) {
	if r.cp != nil {
		exists, err := r.hasBucketReplicationForUserRepo(ctx, r.cp.Repos().Replications, user)
		if err == nil || !controlplane.IsUnavailable(err) {
			return exists, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Str("user", user).Msg("replication: PostgreSQL hasBucketReplicationForUser failed, falling back to Redis")
	}
	exists, err := r.bucketReplicationPolicyStore.ExistsForUser(ctx, user)
	if err != nil || exists {
		return exists, err
	}
	return r.bucketReplicationSwitchStore.IsZeroDowntimeActiveForUserOp(ctx, user).Get()
}

func (r *policySvc) archieveReplicationStatusInTx(ctx context.Context, tx store.Executor[redis.Pipeliner], policy entity.UniversalReplicationID) {
	if bucketPolicy, ok := policy.AsBucketID(); ok {
		_ = r.bucketReplicationStatusStore.WithExecutor(tx).ArchieveOp(ctx, bucketPolicy)
	} else if userPolicy, ok := policy.AsUserID(); ok {
		_ = r.userReplicationStatusStore.WithExecutor(tx).ArchieveOp(ctx, userPolicy)
	} else {
		// should never happen
		panic(fmt.Sprintf("unknown policy type %s", policy.AsString()))
	}
}

func (r *policySvc) removeReplicationPolicyInTx(ctx context.Context, tx store.Executor[redis.Pipeliner], policy entity.UniversalReplicationID) {
	if bucketPolicy, ok := policy.AsBucketID(); ok {
		_ = r.bucketReplicationPolicyStore.WithExecutor(tx).RemoveOp(ctx, bucketPolicy)
	} else if userPolicy, ok := policy.AsUserID(); ok {
		_ = r.userReplicationPolicyStore.WithExecutor(tx).RemoveOp(ctx, userPolicy)
	} else {
		// should never happen
		panic(fmt.Sprintf("unknown policy type %s", policy.AsString()))
	}
}

func (r *policySvc) createReplicationPolicyInTx(ctx context.Context, tx store.Executor[redis.Pipeliner], policy entity.UniversalReplicationID, status entity.ReplicationStatus) {
	if bucketPolicy, ok := policy.AsBucketID(); ok {
		_ = r.bucketReplicationPolicyStore.WithExecutor(tx).AddOp(ctx, bucketPolicy)
		_ = r.bucketReplicationStatusStore.WithExecutor(tx).AddOp(ctx, bucketPolicy, status)
	} else if userPolicy, ok := policy.AsUserID(); ok {
		_ = r.userReplicationPolicyStore.WithExecutor(tx).AddOp(ctx, userPolicy)
		_ = r.userReplicationStatusStore.WithExecutor(tx).AddOp(ctx, userPolicy, status)
	} else {
		// should never happen
		panic(fmt.Sprintf("unknown policy type %s", policy.AsString()))
	}
}
