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
)

type RoutingSvc interface {
	SetUserRouting(ctx context.Context, user string, toStorage string) error
	GetUserRoutings(ctx context.Context) (map[string]string, error)
	GetUserRoutingBlocks(ctx context.Context) (map[string]bool, error)
	DeleteUserRouting(ctx context.Context, user string) error
	// UpdateUserRouting atomically changes the target storage of an existing user routing.
	UpdateUserRouting(ctx context.Context, user string, toStorage string) error
	BlockUserRouting(ctx context.Context, user string) error
	UnblockUserRouting(ctx context.Context, user string) error

	SetBucketRouting(ctx context.Context, id entity.BucketRoutingPolicyID, toStorage string) error
	GetBucketRoutings(ctx context.Context, user string) (map[string]string, error)
	GetBucketRoutingBlocks(ctx context.Context, user string) (map[string]bool, error)
	DeleteBucketRouting(ctx context.Context, id entity.BucketRoutingPolicyID) error
	// UpdateBucketRouting atomically changes the target storage of an existing bucket routing.
	UpdateBucketRouting(ctx context.Context, id entity.BucketRoutingPolicyID, toStorage string) error
	BlockBucketRouting(ctx context.Context, id entity.BucketRoutingPolicyID) error
	UnblockBucketRouting(ctx context.Context, id entity.BucketRoutingPolicyID) error
}

var _ RoutingSvc = (*policySvc)(nil)

func validateBucketRoutingID(id entity.BucketRoutingPolicyID) error {
	if id.User == "" || id.Bucket == "" {
		return fmt.Errorf("%w: user and bucket are required", dom.ErrInvalidArg)
	}
	return nil
}

func (r *policySvc) SetBucketRouting(ctx context.Context, id entity.BucketRoutingPolicyID, toStorage string) error {
	if r.cp != nil {
		if err := validateBucketRoutingID(id); err != nil {
			return err
		}
		return controlplane.WrapWriteError(r.cp.Tx(ctx, func(repos controlplane.Repos) error {
			userReplExists, err := r.hasUserReplicationRepo(ctx, repos.Replications, id.User)
			if err != nil {
				return fmt.Errorf("unable to get user-level replications for user %s: %w", id.User, err)
			}
			if userReplExists {
				return fmt.Errorf("%w: cannot set bucket-level routing for user %s and bucket %s because user-level replications exist", dom.ErrInvalidArg, id.User, id.Bucket)
			}
			bucketReplExists, err := r.hasBucketReplicationRepo(ctx, repos.Replications, entity.BucketReplicationPolicyID{
				User:       id.User,
				FromBucket: id.Bucket,
			})
			if err != nil {
				return fmt.Errorf("unable to get bucket-level replications for user %s and bucket %s: %w", id.User, id.Bucket, err)
			}
			if bucketReplExists {
				return fmt.Errorf("%w: cannot set bucket-level routing for user %s and bucket %s because active bucket-level replications exist", dom.ErrInvalidArg, id.User, id.Bucket)
			}
			return repos.Routing.Set(ctx, id.User, id.Bucket, toStorage)
		}))
	}
	//check if there are no active user-level replications for the user
	userReplExists, err := r.hasUserReplication(ctx, id.User)
	if err != nil {
		return fmt.Errorf("unable to get user-level replications for user %s: %w", id.User, err)
	}
	if userReplExists {
		return fmt.Errorf("%w: cannot set bucket-level routing for user %s and bucket %s because user-level replications exist", dom.ErrInvalidArg, id.User, id.Bucket)
	}
	// check if there are no active bucket-level replications
	bucketReplExists, err := r.hasBucketReplication(ctx, entity.BucketReplicationPolicyID{
		User:       id.User,
		FromBucket: id.Bucket,
	})
	if err != nil {
		return fmt.Errorf("unable to get bucket-level replications for user %s and bucket %s: %w", id.User, id.Bucket, err)
	}
	if bucketReplExists {
		return fmt.Errorf("%w: cannot set bucket-level routing for user %s and bucket %s because active bucket-level replications exist", dom.ErrInvalidArg, id.User, id.Bucket)
	}
	// add the routing
	return r.bucketRoutingStore.SetOp(ctx, id, toStorage).Get()
}

func (r *policySvc) SetUserRouting(ctx context.Context, user string, toStorage string) error {
	if r.cp != nil {
		return controlplane.WrapWriteError(r.cp.Tx(ctx, func(repos controlplane.Repos) error {
			userReplExists, err := r.hasUserReplicationRepo(ctx, repos.Replications, user)
			if err != nil {
				return fmt.Errorf("unable to get user-level replications for user %s: %w", user, err)
			}
			if userReplExists {
				return fmt.Errorf("%w: cannot set user-level routing for user %s because active user-level replications exist", dom.ErrInvalidArg, user)
			}
			return repos.Routing.Set(ctx, user, "", toStorage)
		}))
	}
	//check if there are no active user-level replications
	userReplExists, err := r.hasUserReplication(ctx, user)
	if err != nil {
		return fmt.Errorf("unable to get user-level replications for user %s: %w", user, err)
	}
	if userReplExists {
		return fmt.Errorf("%w: cannot set user-level routing for user %s because active user-level replications exist", dom.ErrInvalidArg, user)
	}
	// add the routing
	return r.userRoutingStore.SetOp(ctx, user, toStorage).Get()
}

// UpdateBucketRouting has the same replication conflict checks as SetBucketRouting, but requires an existing routing
// and replaces its target in one atomic Redis operation (no window where the bucket falls back to the main storage).
func (r *policySvc) UpdateBucketRouting(ctx context.Context, id entity.BucketRoutingPolicyID, toStorage string) error {
	if r.cp != nil {
		if err := validateBucketRoutingID(id); err != nil {
			return err
		}
		return controlplane.WrapWriteError(r.cp.Tx(ctx, func(repos controlplane.Repos) error {
			userReplExists, err := r.hasUserReplicationRepo(ctx, repos.Replications, id.User)
			if err != nil {
				return fmt.Errorf("unable to get user-level replications for user %s: %w", id.User, err)
			}
			if userReplExists {
				return fmt.Errorf("%w: cannot update bucket-level routing for user %s and bucket %s because user-level replications exist", dom.ErrInvalidArg, id.User, id.Bucket)
			}
			bucketReplExists, err := r.hasBucketReplicationRepo(ctx, repos.Replications, entity.BucketReplicationPolicyID{
				User:       id.User,
				FromBucket: id.Bucket,
			})
			if err != nil {
				return fmt.Errorf("unable to get bucket-level replications for user %s and bucket %s: %w", id.User, id.Bucket, err)
			}
			if bucketReplExists {
				return fmt.Errorf("%w: cannot update bucket-level routing for user %s and bucket %s because active bucket-level replications exist", dom.ErrInvalidArg, id.User, id.Bucket)
			}
			return repos.Routing.Update(ctx, id.User, id.Bucket, toStorage)
		}))
	}
	userReplExists, err := r.hasUserReplication(ctx, id.User)
	if err != nil {
		return fmt.Errorf("unable to get user-level replications for user %s: %w", id.User, err)
	}
	if userReplExists {
		return fmt.Errorf("%w: cannot update bucket-level routing for user %s and bucket %s because user-level replications exist", dom.ErrInvalidArg, id.User, id.Bucket)
	}
	bucketReplExists, err := r.hasBucketReplication(ctx, entity.BucketReplicationPolicyID{
		User:       id.User,
		FromBucket: id.Bucket,
	})
	if err != nil {
		return fmt.Errorf("unable to get bucket-level replications for user %s and bucket %s: %w", id.User, id.Bucket, err)
	}
	if bucketReplExists {
		return fmt.Errorf("%w: cannot update bucket-level routing for user %s and bucket %s because active bucket-level replications exist", dom.ErrInvalidArg, id.User, id.Bucket)
	}
	return r.bucketRoutingStore.UpdateOp(ctx, id, toStorage)
}

// UpdateUserRouting has the same replication conflict checks as SetUserRouting, but requires an existing routing
// and replaces its target in one atomic Redis operation.
func (r *policySvc) UpdateUserRouting(ctx context.Context, user string, toStorage string) error {
	if r.cp != nil {
		return controlplane.WrapWriteError(r.cp.Tx(ctx, func(repos controlplane.Repos) error {
			userReplExists, err := r.hasUserReplicationRepo(ctx, repos.Replications, user)
			if err != nil {
				return fmt.Errorf("unable to get user-level replications for user %s: %w", user, err)
			}
			if userReplExists {
				return fmt.Errorf("%w: cannot update user-level routing for user %s because active user-level replications exist", dom.ErrInvalidArg, user)
			}
			return repos.Routing.Update(ctx, user, "", toStorage)
		}))
	}
	userReplExists, err := r.hasUserReplication(ctx, user)
	if err != nil {
		return fmt.Errorf("unable to get user-level replications for user %s: %w", user, err)
	}
	if userReplExists {
		return fmt.Errorf("%w: cannot update user-level routing for user %s because active user-level replications exist", dom.ErrInvalidArg, user)
	}
	return r.userRoutingStore.UpdateOp(ctx, user, toStorage)
}

func (r *policySvc) DeleteBucketRouting(ctx context.Context, id entity.BucketRoutingPolicyID) error {
	if r.cp != nil {
		if err := validateBucketRoutingID(id); err != nil {
			return err
		}
		return controlplane.WrapWriteError(r.cp.Tx(ctx, func(repos controlplane.Repos) error {
			bucketReplExists, err := r.hasBucketReplicationRepo(ctx, repos.Replications, entity.BucketReplicationPolicyID{
				User:       id.User,
				FromBucket: id.Bucket,
			})
			if err != nil {
				return fmt.Errorf("unable to get bucket-level replications for user %s and bucket %s: %w", id.User, id.Bucket, err)
			}
			if bucketReplExists {
				return fmt.Errorf("%w: cannot delete bucket-level routing for user %s and bucket %s because dependent bucket-level replications exist", dom.ErrInvalidArg, id.User, id.Bucket)
			}
			return repos.Routing.DeleteRoute(ctx, id.User, id.Bucket)
		}))
	}
	// check if there are no dependent bucket-level replications and it is safe to delete
	bucketReplExists, err := r.hasBucketReplication(ctx, entity.BucketReplicationPolicyID{
		User:       id.User,
		FromBucket: id.Bucket,
	})
	if err != nil {
		return fmt.Errorf("unable to get bucket-level replications for user %s and bucket %s: %w", id.User, id.Bucket, err)
	}
	if bucketReplExists {
		return fmt.Errorf("%w: cannot delete bucket-level routing for user %s and bucket %s because dependent bucket-level replications exist", dom.ErrInvalidArg, id.User, id.Bucket)
	}
	return r.bucketRoutingStore.DeleteRouteOp(ctx, id).Get()
}

func (r *policySvc) DeleteUserRouting(ctx context.Context, user string) error {
	if r.cp != nil {
		if user == "" {
			return fmt.Errorf("%w: user is required", dom.ErrInvalidArg)
		}
		return controlplane.WrapWriteError(r.cp.Tx(ctx, func(repos controlplane.Repos) error {
			userReplExists, err := r.hasUserReplicationRepo(ctx, repos.Replications, user)
			if err != nil {
				return fmt.Errorf("unable to get user-level replications for user %s: %w", user, err)
			}
			if userReplExists {
				return fmt.Errorf("%w: cannot delete user-level routing for user %s because dependent user-level replications exist", dom.ErrInvalidArg, user)
			}
			return repos.Routing.DeleteRoute(ctx, user, "")
		}))
	}
	// check if there are no active user-level replications and it is safe to delete
	userReplExists, err := r.hasUserReplication(ctx, user)
	if err != nil {
		return fmt.Errorf("unable to get user-level replications for user %s: %w", user, err)
	}
	if userReplExists {
		return fmt.Errorf("%w: cannot delete user-level routing for user %s because dependent user-level replications exist", dom.ErrInvalidArg, user)
	}
	return r.userRoutingStore.DeleteRouteOp(ctx, user).Get()
}

func (r *policySvc) GetBucketRoutings(ctx context.Context, user string) (map[string]string, error) {
	if r.cp != nil {
		if user == "" {
			return nil, fmt.Errorf("%w: user is required", dom.ErrInvalidArg)
		}
		routes, err := r.cp.Repos().Routing.ListBucketRoutes(ctx, user)
		if err == nil {
			return routes, nil
		}
		if !controlplane.IsUnavailable(err) {
			return nil, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Str("user", user).Msg("routing: PostgreSQL ListBucketRoutes failed, falling back to Redis")
	}
	return r.bucketRoutingStore.ListRoutesOp(ctx, user).Get()
}

func (r *policySvc) GetUserRoutings(ctx context.Context) (map[string]string, error) {
	if r.cp != nil {
		routes, err := r.cp.Repos().Routing.ListUserRoutes(ctx)
		if err == nil {
			return routes, nil
		}
		if !controlplane.IsUnavailable(err) {
			return nil, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("routing: PostgreSQL ListUserRoutes failed, falling back to Redis")
	}
	return r.userRoutingStore.ListRoutesOp(ctx).Get()
}

func (r *policySvc) BlockBucketRouting(ctx context.Context, id entity.BucketRoutingPolicyID) error {
	if r.cp != nil {
		if err := validateBucketRoutingID(id); err != nil {
			return err
		}
		return controlplane.WrapWriteError(r.cp.Repos().Routing.Block(ctx, id.User, id.Bucket))
	}
	return r.bucketRoutingStore.BlockOp(ctx, id).Get()
}

func (r *policySvc) BlockUserRouting(ctx context.Context, user string) error {
	if r.cp != nil {
		return controlplane.WrapWriteError(r.cp.Repos().Routing.Block(ctx, user, ""))
	}
	return r.userRoutingStore.BlockOp(ctx, user).Get()
}

func (r *policySvc) UnblockBucketRouting(ctx context.Context, id entity.BucketRoutingPolicyID) error {
	if r.cp != nil {
		if err := validateBucketRoutingID(id); err != nil {
			return err
		}
		return controlplane.WrapWriteError(r.cp.Repos().Routing.Unblock(ctx, id.User, id.Bucket))
	}
	return r.bucketRoutingStore.UnblockOp(ctx, id).Get()
}

func (r *policySvc) UnblockUserRouting(ctx context.Context, user string) error {
	if r.cp != nil {
		return controlplane.WrapWriteError(r.cp.Repos().Routing.Unblock(ctx, user, ""))
	}
	return r.userRoutingStore.UnblockOp(ctx, user).Get()
}

func (r *policySvc) GetBucketRoutingBlocks(ctx context.Context, user string) (map[string]bool, error) {
	if r.cp != nil {
		if user == "" {
			return nil, fmt.Errorf("%w: user is required", dom.ErrInvalidArg)
		}
		blocks, err := r.cp.Repos().Routing.ListBucketBlocks(ctx, user)
		if err == nil {
			return blocks, nil
		}
		if !controlplane.IsUnavailable(err) {
			return nil, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Str("user", user).Msg("routing: PostgreSQL ListBucketBlocks failed, falling back to Redis")
	}
	return r.bucketRoutingStore.ListBlocksOp(ctx, user).Get()
}

func (r *policySvc) GetUserRoutingBlocks(ctx context.Context) (map[string]bool, error) {
	if r.cp != nil {
		blocks, err := r.cp.Repos().Routing.ListUserBlocks(ctx)
		if err == nil {
			return blocks, nil
		}
		if !controlplane.IsUnavailable(err) {
			return nil, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("routing: PostgreSQL ListUserBlocks failed, falling back to Redis")
	}
	return r.userRoutingStore.ListBlocksOp(ctx).Get()
}

func (r *policySvc) unblockRouteInTx(ctx context.Context, tx store.Executor[redis.Pipeliner], policy entity.UniversalReplicationID) {
	if bucketPolicy, ok := policy.AsBucketID(); ok {
		_ = r.bucketRoutingStore.WithExecutor(tx).UnblockOp(ctx, bucketPolicy.RoutingID())
	} else if userPolicy, ok := policy.AsUserID(); ok {
		_ = r.userRoutingStore.WithExecutor(tx).UnblockOp(ctx, userPolicy.RoutingID())
	} else {
		// should never happen
		panic(fmt.Sprintf("unknown policy type %s", policy.AsString()))
	}
}

func (r *policySvc) blockRouteInTx(ctx context.Context, tx store.Executor[redis.Pipeliner], policy entity.UniversalReplicationID) {
	if bucketPolicy, ok := policy.AsBucketID(); ok {
		_ = r.bucketRoutingStore.WithExecutor(tx).BlockOp(ctx, bucketPolicy.RoutingID())
	} else if userPolicy, ok := policy.AsUserID(); ok {
		_ = r.userRoutingStore.WithExecutor(tx).BlockOp(ctx, userPolicy.RoutingID())
	} else {
		// should never happen
		panic(fmt.Sprintf("unknown policy type %s", policy.AsString()))
	}
}

func (r *policySvc) routeToNewInTx(ctx context.Context, tx store.Executor[redis.Pipeliner], policy entity.UniversalReplicationID) {
	if bucketPolicy, ok := policy.AsBucketID(); ok {
		_ = r.bucketRoutingStore.WithExecutor(tx).SetOp(ctx, bucketPolicy.RoutingID(), bucketPolicy.ToStorage)
	} else if userPolicy, ok := policy.AsUserID(); ok {
		_ = r.userRoutingStore.WithExecutor(tx).SetOp(ctx, userPolicy.RoutingID(), userPolicy.ToStorage)
	} else {
		// should never happen
		panic(fmt.Sprintf("unknown policy type %s", policy.AsString()))
	}
}

func (r *policySvc) routeToOldInTx(ctx context.Context, tx store.Executor[redis.Pipeliner], policy entity.UniversalReplicationID) {
	if bucketPolicy, ok := policy.AsBucketID(); ok {
		_ = r.bucketRoutingStore.WithExecutor(tx).SetOp(ctx, bucketPolicy.RoutingID(), bucketPolicy.FromStorage)
	} else if userPolicy, ok := policy.AsUserID(); ok {
		_ = r.userRoutingStore.WithExecutor(tx).SetOp(ctx, userPolicy.RoutingID(), userPolicy.FromStorage)
	} else {
		// should never happen
		panic(fmt.Sprintf("unknown policy type %s", policy.AsString()))
	}
}

func (r *policySvc) getRoutingForReplicationRepo(ctx context.Context, routingRepo controlplane.Routing, repliction entity.UniversalReplicationID) (string, error) {
	if bucketPolicy, ok := repliction.AsBucketID(); ok {
		routeTo, err := routingRepo.Get(ctx, bucketPolicy.User, bucketPolicy.FromBucket)
		switch {
		case errors.Is(err, dom.ErrNotFound):
			break
		default:
			return routeTo, err
		}
	}
	userRouteTo, err := routingRepo.Get(ctx, repliction.User(), "")
	switch {
	case errors.Is(err, dom.ErrNotFound):
		return r.mainStorage, nil
	default:
		return userRouteTo, err
	}
}

func (r *policySvc) getRoutingForReplication(ctx context.Context, repliction entity.UniversalReplicationID) (string, error) {
	if r.cp != nil {
		routeTo, err := r.getRoutingForReplicationRepo(ctx, r.cp.Repos().Routing, repliction)
		if err == nil || !controlplane.IsUnavailable(err) {
			return routeTo, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("routing: PostgreSQL getRoutingForReplication failed, falling back to Redis")
	}
	if bucketPolicy, ok := repliction.AsBucketID(); ok {
		routeTo, err := r.bucketRoutingStore.GetOp(ctx, bucketPolicy.RoutingID()).Get()
		switch {
		case errors.Is(err, dom.ErrNotFound):
			// try user level routing
			break
		default:
			return routeTo, err
		}
	}
	userRouteTo, err := r.userRoutingStore.GetOp(ctx, repliction.User()).Get()
	switch {
	case errors.Is(err, dom.ErrNotFound):
		// no user level routing, return main
		return r.mainStorage, nil
	default:
		return userRouteTo, err
	}
}
