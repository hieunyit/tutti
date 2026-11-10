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

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/validate"
)

type ContextSvc interface {
	// Sets active policies to proxy request context.
	BuildProxyContext(ctx context.Context, user, bucket string) (context.Context, error)
	// Same as BuildProxyContext but for requests where bucket is not known (e.g. ListBuckets).
	BuildProxyNoBucketContext(ctx context.Context, user string) (context.Context, error)
	// Sets active policies to agent bucket notification request context.
	BuildAgentContext(ctx context.Context, user, bucket string) (context.Context, error)
}

var _ ContextSvc = (*policySvc)(nil)

func (r *policySvc) BuildProxyContext(ctx context.Context, user, bucket string) (context.Context, error) {
	bucketRouteID := entity.NewBucketRoutingPolicyID(user, bucket)
	if err := validate.BucketRoutingPolicyID(bucketRouteID); err != nil {
		return ctx, fmt.Errorf("unable to validate bucket routing policy id: %w", err)
	}
	bucketReplID := entity.NewBucketReplicationPolicyID(user, bucket)
	if err := validate.BucketReplicationPolicyID(bucketReplID); err != nil {
		return ctx, fmt.Errorf("unable to validate bucket replication policy id: %w", err)
	}
	return r.proxyContext(ctx, user, bucket)
}

func (r *policySvc) BuildProxyNoBucketContext(ctx context.Context, user string) (context.Context, error) {
	return r.proxyContext(ctx, user, "")
}

// proxyContext resolves the routing/replication data of the request, from the in-memory snapshot when it
// is enabled (see snapshot.go) and straight from Redis otherwise, and sets it to the context.
func (r *policySvc) proxyContext(ctx context.Context, user, bucket string) (context.Context, error) {
	var (
		data *proxyData
		err  error
	)
	if r.snap != nil {
		data, err = r.snap.lookup(ctx, r, user, bucket)
	} else {
		data, err = r.loadProxyData(ctx, user, bucket)
	}
	if err != nil {
		return ctx, err
	}
	return data.apply(ctx)
}

// proxyData is everything the proxy context needs for one (user, bucket) pair. It is immutable once built,
// so it can be shared between requests by the snapshot.
type proxyData struct {
	// routeErr is dom.ErrRoutingBlock when routing is blocked; no other error is ever stored.
	routeErr     error
	routeTo      string
	activeSwitch *entity.ReplicationSwitchInfo
	replications []entity.UniversalReplicationID
	generations  map[entity.UniversalReplicationID]int64
	// sharedGroup is the protection group of ANOTHER identity managing the bucket on the routed storage of a
	// user on a shared connection account: its writes are refused by the write fence.
	sharedGroup string
}

func (d *proxyData) apply(ctx context.Context) (context.Context, error) {
	if d.routeErr != nil {
		return ctx, d.routeErr
	}
	ctx = xctx.SetRoutingPolicy(ctx, d.routeTo)
	ctx = xctx.SetSharedProtectionGroup(ctx, d.sharedGroup)
	if d.activeSwitch != nil {
		sw := *d.activeSwitch
		ctx = xctx.SetInProgressZeroDowntime(ctx, sw)
	}
	if len(d.replications) != 0 {
		// copy: the data may be shared between requests
		ctx = xctx.SetReplications(ctx, append([]entity.UniversalReplicationID(nil), d.replications...))
	}
	if len(d.generations) != 0 {
		gens := make(map[entity.UniversalReplicationID]int64, len(d.generations))
		for k, v := range d.generations {
			gens[k] = v
		}
		ctx = xctx.SetReplicationGenerations(ctx, gens)
	}
	return ctx, nil
}

// loadProxyData reads from Redis (one transaction):
//  1. user level routing
//  2. bucket level routing
//  3. user level active zero downtime switch
//  4. bucket level active zero downtime switch
//  5. user replication policies
//  6. bucket replication policies
//  7. replication generations
//
// bucket == "" means a request without bucket (e.g. ListBuckets): only the user level data is read.
func (r *policySvc) loadProxyData(ctx context.Context, user, bucket string) (*proxyData, error) {
	data, err := r.loadProxyDataForUser(ctx, user, bucket)
	if err != nil || data.routeErr != nil || bucket == "" || r.sharedAccount == nil || !r.sharedAccount(data.routeTo, user) {
		return data, err
	}
	// The user reaches the routed storage through its shared connection account, so its buckets there are the
	// same physical buckets other identities see. A bucket managed by another identity's protection group must
	// not be written outside that group: the request keeps the user's own routing and replications (never the
	// group owner's) and the write fence refuses its writes (protectiongroups.Store.Middleware).
	group, err := r.sharedProtectionGroup(ctx, data.routeTo, user, bucket)
	if err != nil {
		// never cache an unknown protection scope: the request fails closed and the next one retries
		return nil, err
	}
	data.sharedGroup = group
	return data, nil
}

// sharedProtectionGroup returns the protection group of another identity that manages bucket on storage (see
// protectiongroups.LookupSharedGroup). An unreadable or incomplete index is an error, never "unprotected".
func (r *policySvc) sharedProtectionGroup(ctx context.Context, storage, user, bucket string) (string, error) {
	read := func() (map[string]string, error) {
		if r.snap != nil {
			return r.snap.protectionScope(ctx)
		}
		// without the snapshot: only the fields this lookup needs (O(1), not the whole index)
		return protectiongroups.SharedGroupFields(ctx, r.client, storage, bucket)
	}
	idx, err := read()
	if err != nil {
		return "", fmt.Errorf("%w: protection group scope index unavailable: %w", dom.ErrUnavailable, err)
	}
	group, ok, err := protectiongroups.LookupSharedGroup(idx, storage, user, bucket)
	if err != nil || ok {
		return group, err
	}
	// Missing or older index format: rebuild it. The rebuild bumps the config version, so the snapshot
	// reads the new index with its next state; read Redis directly for this request.
	if err := protectiongroups.NewStore(r.client).EnsureIndex(ctx); err != nil {
		return "", fmt.Errorf("%w: protection group scope index unavailable: %w", dom.ErrUnavailable, err)
	}
	idx, err = r.client.HGetAll(ctx, protectiongroups.ScopeIndexKey).Result()
	if err != nil {
		return "", fmt.Errorf("%w: protection group scope index unavailable: %w", dom.ErrUnavailable, err)
	}
	group, ok, err = protectiongroups.LookupSharedGroup(idx, storage, user, bucket)
	if err == nil && !ok {
		err = fmt.Errorf("%w: protection group scope index is incomplete", dom.ErrUnavailable)
	}
	return group, err
}

func (r *policySvc) loadProxyDataForUser(ctx context.Context, user, bucket string) (*proxyData, error) {
	tx := r.userRoutingStore.TxExecutor()
	userRoutingResult := r.userRoutingStore.WithExecutor(tx).GetOp(ctx, user)
	userActiveSwitchResult := r.userReplicationSwitchStore.WithExecutor(tx).IsZeroDowntimeActiveOp(ctx, user)
	userReplications := r.userReplicationPolicyStore.WithExecutor(tx).GetOp(ctx, user)
	userGenerations := r.replicationGenerationStore.WithExecutor(tx).GetUserOp(ctx, user)

	bucketReplID := entity.NewBucketReplicationPolicyID(user, bucket)
	var (
		// no bucket: no bucket routing, switch, replications and generations
		bucketRoutingResult      store.OperationResult[string]                                              = store.NewRedisFailedOperationResult[string](dom.ErrNotFound)
		bucketActiveSwitchResult store.OperationResult[bool]                                                = store.NewRedisFailedOperationResult[bool](dom.ErrNotFound)
		bucketReplications       store.OperationResult[[]entity.BucketReplicationPolicy]                    = store.NewRedisFailedOperationResult[[]entity.BucketReplicationPolicy](dom.ErrNotFound)
		bucketGenerations        store.OperationResult[map[entity.BucketReplicationPolicyDestination]int64] = store.NewRedisFailedOperationResult[map[entity.BucketReplicationPolicyDestination]int64](dom.ErrNotFound)
	)
	if bucket != "" {
		bucketRouteID := entity.NewBucketRoutingPolicyID(user, bucket)
		bucketRoutingResult = r.bucketRoutingStore.WithExecutor(tx).GetOp(ctx, bucketRouteID)
		bucketActiveSwitchResult = r.bucketReplicationSwitchStore.WithExecutor(tx).IsZeroDowntimeActiveOp(ctx, bucketReplID)
		bucketReplications = r.bucketReplicationPolicyStore.WithExecutor(tx).GetOp(ctx, bucketReplID)
		bucketGenerations = r.replicationGenerationStore.WithExecutor(tx).GetBucketOp(ctx, bucketReplID)
	}
	_ = tx.Exec(ctx) // ignore error - we will catch it on individual results

	// get active routing policy
	routeTo, err := determineActiveRouting(r.mainStorage, userRoutingResult, bucketRoutingResult)
	if errors.Is(err, dom.ErrRoutingBlock) {
		return &proxyData{routeErr: dom.ErrRoutingBlock}, nil
	}
	if err != nil {
		return nil, err
	}
	if bucket != "" {
		if _, bErr := bucketRoutingResult.Get(); errors.Is(bErr, dom.ErrNotFound) {
			if p := r.resolveGroupPrimary(ctx, user, bucket); p != "" {
				routeTo = p
			}
		}
	}
	data := &proxyData{routeTo: routeTo}

	// check in_progress zero downtime switch if present
	data.activeSwitch, err = r.getActiveZeroDowntimeSwitch(ctx, userActiveSwitchResult, bucketActiveSwitchResult, bucketReplID)
	if err != nil {
		return nil, err
	}

	// replication policies
	replications, err := getReplications(bucketReplications, userReplications)
	if err != nil {
		return nil, err
	}
	data.replications = keepReplicationsFromRoute(replications, routeTo)

	data.generations, err = replicationGenerations(userGenerations, bucketGenerations, data.replications, data.activeSwitch)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// keepReplicationsFromRoute keeps only replications whose source is the storage the request is
// routed to. Normally every replication of a scope starts at its routed storage, so nothing is
// dropped. After an automatic failover the scope is routed to the promoted storage while the
// replications from the failed storage still exist (the one to the promoted storage is paused,
// others such as main->f1 are not): events for them would be recorded against a storage the
// request did not write to, so they must not be part of the request. Replications to another
// bucket on the same storage (from == to storage) are kept.
func keepReplicationsFromRoute(repls []entity.UniversalReplicationID, routedTo string) []entity.UniversalReplicationID {
	n := 0
	for _, id := range repls {
		if id.FromStorage() == routedTo {
			repls[n] = id
			n++
		}
	}
	return repls[:n]
}

func replicationGenerations(
	userGenerations store.OperationResult[map[string]int64],
	bucketGenerations store.OperationResult[map[entity.BucketReplicationPolicyDestination]int64],
	replications []entity.UniversalReplicationID, activeSwitch *entity.ReplicationSwitchInfo) (map[entity.UniversalReplicationID]int64, error) {
	userGens, err := userGenerations.Get()
	if err != nil {
		return nil, err
	}
	bucketGens, err := bucketGenerations.Get()
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return nil, err
	}
	generationOf := func(id entity.UniversalReplicationID) int64 {
		if bucketID, ok := id.AsBucketID(); ok {
			return bucketGens[bucketID.Destination()]
		}
		userID, _ := id.AsUserID()
		return userGens[userID.ToStorage]
	}
	result := make(map[entity.UniversalReplicationID]int64, len(replications)+1)
	for _, id := range replications {
		result[id] = generationOf(id)
	}
	if activeSwitch != nil {
		id := activeSwitch.ReplicationID()
		result[id] = generationOf(id)
	}
	return result, nil
}

func setReplicationGenerations(ctx context.Context,
	userGenerations store.OperationResult[map[string]int64],
	bucketGenerations store.OperationResult[map[entity.BucketReplicationPolicyDestination]int64],
	replications []entity.UniversalReplicationID, activeSwitch *entity.ReplicationSwitchInfo) (context.Context, error) {
	result, err := replicationGenerations(userGenerations, bucketGenerations, replications, activeSwitch)
	if err != nil {
		return ctx, err
	}
	if len(result) == 0 {
		return ctx, nil
	}
	return xctx.SetReplicationGenerations(ctx, result), nil
}

func determineActiveRouting(mainStorage string, userRouting, bucketRouting store.OperationResult[string]) (string, error) {
	userRoute, userErr := userRouting.Get()
	bucketRoute, bucketErr := bucketRouting.Get()

	// check for routing blocks
	if errors.Is(userErr, dom.ErrRoutingBlock) || errors.Is(bucketErr, dom.ErrRoutingBlock) {
		return "", dom.ErrRoutingBlock
	}

	// fist check bucket routing
	switch {
	case bucketErr == nil:
		return bucketRoute, nil
	case errors.Is(bucketErr, dom.ErrNotFound):
		// bucket routing not found, try user routing
		break
	default:
		return "", fmt.Errorf("unable to get bucket routing policy: %w", bucketErr)
	}
	// try user routing
	switch {
	case userErr == nil:
		return userRoute, nil
	case errors.Is(userErr, dom.ErrNotFound):
		// user routing not found, use main mainStorage
		return mainStorage, nil
	default:
		return "", fmt.Errorf("unable to get user routing policy: %w", userErr)
	}
}

func (r *policySvc) getActiveZeroDowntimeSwitch(ctx context.Context, userActiveRes, bucketActiveRes store.OperationResult[bool], id entity.BucketReplicationPolicyID) (*entity.ReplicationSwitchInfo, error) {
	// check errors first
	userSwitchActive, err := userActiveRes.Get()
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return nil, fmt.Errorf("unable to get user zero downtime switch info: %w", err)
	}
	bucketSwitchActive, err := bucketActiveRes.Get()
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return nil, fmt.Errorf("unable to get user zero downtime switch info: %w", err)
	}
	// sanity check:
	if bucketSwitchActive && userSwitchActive {
		// should never happen - only one level of zero downtime switch can be active at a time
		return nil, fmt.Errorf("%w: both user and bucket zero downtime switch are active", dom.ErrInternal)
	}
	// lookup user switch if active
	if userSwitchActive {
		userSwitch, err := r.userReplicationSwitchStore.GetOp(ctx, id.User).Get()
		if err != nil {
			return nil, fmt.Errorf("unable to get user zero downtime switch info: %w", err)
		}
		if userSwitch.MultipartTTL == 0 {
			// should never happen
			return nil, fmt.Errorf("%w: user zero downtime switch info is invalid: %+v", dom.ErrInternal, userSwitch)
		}
		if userSwitch.LastStatus == entity.StatusInProgress {
			return &userSwitch, nil
		}
		// switch is not in progress anymore
		return nil, nil
	}
	//otherwise lookup bucket switch if active
	if bucketSwitchActive {
		bucketSwitch, err := r.bucketReplicationSwitchStore.GetOp(ctx, id).Get()
		if err != nil {
			return nil, fmt.Errorf("unable to get bucket zero downtime switch info: %w", err)
		}
		if bucketSwitch.MultipartTTL == 0 {
			// should never happen
			return nil, fmt.Errorf("%w: bucket zero downtime switch info is invalid: %+v", dom.ErrInternal, bucketSwitch)
		}
		if bucketSwitch.LastStatus == entity.StatusInProgress {
			return &bucketSwitch, nil
		}
	}
	// no active switch found
	return nil, nil
}

func getReplications(bucketPolicies store.OperationResult[[]entity.BucketReplicationPolicy], userPolicies store.OperationResult[[]entity.UserReplicationPolicy]) ([]entity.UniversalReplicationID, error) {
	bucketRepls, err := bucketPolicies.Get()
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return nil, fmt.Errorf("unable to get bucket replication policies: %w", err)
	}
	result := make([]entity.UniversalReplicationID, 0, len(bucketRepls))
	for _, br := range bucketRepls {
		uid := entity.UniversalFromBucketReplication(br)
		if err := uid.Validate(); err != nil {
			return nil, fmt.Errorf("invalid bucket replication policy from store: %w", err)
		}
		result = append(result, uid)
	}
	if len(result) != 0 {
		// bucket level replications take precedence over user level replications
		return result, nil
	}
	// try user level replications
	userRepls, err := userPolicies.Get()
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return nil, fmt.Errorf("unable to get user replication policies: %w", err)
	}
	for _, ur := range userRepls {
		uid := entity.UniversalFromUserReplication(ur)
		if err := uid.Validate(); err != nil {
			return nil, fmt.Errorf("invalid user replication policy from store: %w", err)
		}
		result = append(result, uid)
	}
	return result, nil
}

func (r *policySvc) BuildAgentContext(ctx context.Context, user, bucket string) (context.Context, error) {
	bucketReplID := entity.NewBucketReplicationPolicyID(user, bucket)
	// in a single batch:
	tx := r.userReplicationPolicyStore.TxExecutor()
	userReplications := r.userReplicationPolicyStore.WithExecutor(tx).GetOp(ctx, user)
	bucketReplications := r.bucketReplicationPolicyStore.WithExecutor(tx).GetOp(ctx, bucketReplID)
	userGenerations := r.replicationGenerationStore.WithExecutor(tx).GetUserOp(ctx, user)
	bucketGenerations := r.replicationGenerationStore.WithExecutor(tx).GetBucketOp(ctx, bucketReplID)
	_ = tx.Exec(ctx) // ignore error - we will catch it on individual results

	// set replication policies
	relications, err := getReplications(bucketReplications, userReplications)
	if err != nil {
		return ctx, err
	}
	if len(relications) != 0 {
		ctx = xctx.SetReplications(ctx, relications)
	}

	return setReplicationGenerations(ctx, userGenerations, bucketGenerations, relications, nil)
}

func (r *policySvc) resolveGroupPrimary(ctx context.Context, user, bucket string) string {
	if bucket == "" {
		return ""
	}
	if r.snap != nil {
		if idx, err := r.snap.protectionScope(ctx); err == nil {
			if groupID, ok := protectiongroups.LookupScope(idx, user, bucket); ok && groupID != "" {
				return protectiongroups.PrimaryOfGroup(idx, groupID)
			}
		}
		return ""
	}
	if r.client != nil {
		idx, err := protectiongroups.ScopeFields(ctx, r.client, user, bucket)
		if err == nil {
			if groupID, ok := protectiongroups.LookupScope(idx, user, bucket); ok && groupID != "" {
				p, _ := r.client.HGet(ctx, protectiongroups.ScopeIndexKey, "p:"+groupID).Result()
				return p
			}
		}
	}
	return ""
}
