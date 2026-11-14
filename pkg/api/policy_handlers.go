/*
 * Copyright © 2024 Clyso GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"sort"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/hieunyit/tutti/pkg/controlplane"
	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/hieunyit/tutti/pkg/notifications"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/replscope"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

const (
	// TODO: move to config
	defaultZeroDowntimeMultipartTTL = time.Hour
)

func PolicyHandlers(
	credsSvc objstore.CredsService,
	clients objstore.Clients,
	queueSvc tasks.QueueService,
	policySvc policy.Service,
	versionSvc meta.VersionService,
	objectListStateStore *store.MigrationObjectListStateStore,
	bucketListStateStore *store.MigrationBucketListStateStore,
	notificationSvc *notifications.Service,
	replicationStatusLocker *store.ReplicationStatusLocker,
	userLocker *store.UserLocker,
	webhookConf *WebhookConfig,
	opts ...PolicyOption,
) pb.PolicyServer {
	h := &policyHandlers{
		credsSvc:                credsSvc,
		clients:                 clients,
		queueSvc:                queueSvc,
		policySvc:               policySvc,
		versionSvc:              versionSvc,
		objectListStateStore:    objectListStateStore,
		bucketListStateStore:    bucketListStateStore,
		notificationSvc:         notificationSvc,
		replicationStatusLocker: replicationStatusLocker,
		userLocker:              userLocker,
		webhookConf:             webhookConf,
	}
	for _, o := range opts {
		o(h)
	}
	return h
}

// PolicyOption customizes PolicyHandlers.
type PolicyOption func(*policyHandlers)

// WithPolicyWebhook resolves the current effective settings for every request.
// Token and listener configuration remain controlled by worker startup config.
func WithPolicyWebhook(current func() WebhookConfig) PolicyOption {
	return func(h *policyHandlers) { h.currentWebhook = current }
}

// WithPolicyScope restricts user-level replications and bucket listings of users on a shared connection account to
// the buckets of their access policy.
func WithPolicyScope(scope *accesspolicy.Scope) PolicyOption {
	return func(h *policyHandlers) { h.scope = scope }
}

// WithPolicyStorageLocker makes replication and routing creation take the per-storage lock shared with runtime
// storage update/delete (WithStorageLocker), so a reference cannot appear while a storage is being deleted.
func WithPolicyStorageLocker(l *StorageLocker) PolicyOption {
	return func(h *policyHandlers) { h.storageLocker = l }
}

// WithPolicyControlPlaneStore provides controlplane store for recording audit trail of runtime operations.
func WithPolicyControlPlaneStore(cp controlplane.Store) PolicyOption {
	return func(h *policyHandlers) { h.cpStore = cp }
}

// WithPolicyScopeLocker serializes the replication scope conflict check with the write, shared with every
// other creation path (protection groups). Without it creations are not serialized.
func WithPolicyScopeLocker(l replscope.Locker) PolicyOption {
	return func(h *policyHandlers) { h.scopeLocker = l }
}

var _ pb.PolicyServer = &policyHandlers{}

type policyHandlers struct {
	credsSvc                objstore.CredsService
	clients                 objstore.Clients
	queueSvc                tasks.QueueService
	policySvc               policy.Service
	versionSvc              meta.VersionService
	objectListStateStore    *store.MigrationObjectListStateStore
	bucketListStateStore    *store.MigrationBucketListStateStore
	notificationSvc         *notifications.Service
	replicationStatusLocker *store.ReplicationStatusLocker
	userLocker              *store.UserLocker
	webhookConf             *WebhookConfig
	currentWebhook          func() WebhookConfig
	storageLocker           *StorageLocker
	scope                   *accesspolicy.Scope
	cpStore                 controlplane.Store
	scopeLocker             replscope.Locker
}

func (h *policyHandlers) AddReplication(ctx context.Context, req *pb.AddReplicationRequest) (*emptypb.Empty, error) {
	// validate request:
	uid, err := pbToReplicationID(req.Id)
	if err != nil {
		return nil, err
	}
	if uid.User() == "" {
		// storage-level replication runs with the connection account of both storages
		for _, st := range []string{uid.FromStorage(), uid.ToStorage()} {
			if err := h.credsSvc.HasUser(st, ""); err != nil {
				return nil, err
			}
		}
	}
	if err := h.credsSvc.ValidateReplicationID(uid); err != nil {
		if errors.Is(err, dom.ErrNotFound) {
			return nil, fmt.Errorf("%w: %v", dom.ErrInvalidArg, err)
		}
		return nil, err
	}
	// acquire user lock (not for storage-level), then the lock of both storages (the replication references them)
	err = h.inUserLock(ctx, uid.User(), func(ctx context.Context) error {
		return h.storageLocker.With(ctx, []string{uid.FromStorage(), uid.ToStorage()}, func(ctx context.Context) error {
			// the storage may have been deleted while waiting for the lock
			if err := h.credsSvc.ValidateReplicationID(uid); err != nil {
				return err
			}
			// every network validation runs before the scope lock (its TTL is short and not renewed)
			opts, err := h.validateAddReplication(ctx, uid, req)
			if err != nil {
				return err
			}
			// the scope lock covers only the conflict check and the creation
			unlock, err := h.lockScope(ctx, uid)
			if err != nil {
				return err
			}
			defer unlock()
			return h.addReplicationLocked(ctx, uid, req, opts)
		})
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// proposedScope is the scope of a replication: user-level covers all buckets, bucket-level its source and
// destination bucket names (same representation as policy.ListScopeOwners).
func proposedScope(uid entity.UniversalReplicationID) replscope.Scope {
	var buckets []string
	if b, ok := uid.AsBucketID(); ok {
		buckets = []string{b.FromBucket}
		if b.ToBucket != b.FromBucket {
			buckets = append(buckets, b.ToBucket)
		}
	}
	return replscope.ReplicationScope(uid.FromStorage(), uid.ToStorage(), buckets).WithUser(uid.User())
}

// lockScope takes the global scope lock and refuses a replication whose scope conflicts with a group or another
// replication (gRPC AlreadyExists via replscope.ConflictError). On success the caller creates the replication and
// calls unlock; keep that critical section short (the lock is not renewed).
func (h *policyHandlers) lockScope(ctx context.Context, uid entity.UniversalReplicationID) (func(), error) {
	unlock := func() {}
	if h.scopeLocker != nil {
		u, err := h.scopeLocker.Lock(ctx)
		if err != nil {
			return nil, err
		}
		unlock = u
	}
	owners, err := h.policySvc.ListScopeOwners(ctx)
	if err != nil {
		unlock()
		return nil, err
	}
	// an existing replication with the same id is reported by the creation validation, not as a scope conflict
	if cerr := replscope.Check(proposedScope(uid), owners, uid.AsString()); cerr != nil {
		unlock()
		return nil, cerr
	}
	return unlock, nil
}

// validateAddReplication runs the validations that may call a storage or the credentials service and resolves the
// replication options. It must run before the scope lock is taken.
func (h *policyHandlers) validateAddReplication(ctx context.Context, uid entity.UniversalReplicationID, req *pb.AddReplicationRequest) (entity.ReplicationOptions, error) {
	var eventSource pb.EventSource
	if req.Opts != nil {
		eventSource = req.Opts.EventSource
	}
	opts := entity.ReplicationOptions{EventSource: pbToEventSource(eventSource)}
	if userRepl, ok := uid.AsUserID(); ok {
		if eventSource == pb.EventSource_EVENT_SOURCE_S3_NOTIFICATION {
			return opts, fmt.Errorf("%w: s3_notification event source is not supported for user-level replication", dom.ErrInvalidArg)
		}
		if err := h.validateWebhookEventSource(eventSource); err != nil {
			return opts, err
		}
		if err := h.scope.CheckUserReplication(ctx, userRepl.FromStorage, userRepl.User); err != nil {
			return opts, err
		}
		return opts, nil
	}
	b, ok := uid.AsBucketID()
	if !ok {
		return opts, fmt.Errorf("%w: invalid replication ID", dom.ErrInvalidArg)
	}
	storageType := h.credsSvc.Storages()[b.FromStorage]
	switch eventSource {
	case pb.EventSource_EVENT_SOURCE_S3_NOTIFICATION:
		if storageType != dom.S3 {
			return opts, fmt.Errorf("%w: s3_notification event source is only supported for S3 storage", dom.ErrInvalidArg)
		}
		webhook := h.effectiveWebhook()
		if webhook == nil || !webhook.Enabled {
			return opts, fmt.Errorf("%w: webhook must be enabled for s3_notification event source", dom.ErrInvalidArg)
		}
		agentURL, err := webhook.S3NotificationURL(b.FromStorage)
		if err != nil {
			return opts, err
		}
		opts.AgentURL = agentURL
	case pb.EventSource_EVENT_SOURCE_WEBHOOK:
		if err := h.validateWebhookEventSource(eventSource); err != nil {
			return opts, err
		}
		webhookURL, err := h.webhookURLForStorage(b.FromStorage, storageType)
		if err != nil {
			return opts, err
		}
		opts.AgentURL = webhookURL
	}
	fromClient, err := h.clients.AsCommon(ctx, b.FromStorage, b.User)
	if err != nil {
		return opts, err
	}
	// check if bucket exists in source (network call)
	exists, err := fromClient.BucketExists(ctx, b.FromBucket)
	if err != nil {
		return opts, fmt.Errorf("%w: unable to validate source bucket", err)
	}
	if !exists {
		return opts, fmt.Errorf("%w: bucket %s not exists in source storage %s", dom.ErrInvalidArg, b.FromBucket, b.FromStorage)
	}
	return opts, nil
}

func (h *policyHandlers) addReplicationLocked(ctx context.Context, uid entity.UniversalReplicationID, req *pb.AddReplicationRequest, opts entity.ReplicationOptions) error {
	if userRepl, ok := uid.AsUserID(); ok {
		// create user replication
		return h.addUserReplication(ctx, userRepl, opts)
	} else if bucketRepl, ok := uid.AsBucketID(); ok {
		// create bucket replication
		return h.addBucketReplication(ctx, bucketRepl, opts)
	}
	return fmt.Errorf("%w: invalid replication ID", dom.ErrInvalidArg)
}

func (h *policyHandlers) addUserReplication(ctx context.Context, replicationID entity.UserReplicationPolicy, opts entity.ReplicationOptions) (err error) {
	if err := h.policySvc.ValidateUserReplicationCreation(ctx, replicationID); err != nil {
		return err
	}
	if err := h.removeLeftoversOfDeletedReplication(ctx, entity.UniversalFromUserReplication(replicationID)); err != nil {
		return err
	}
	generation, err := h.policySvc.AddUserReplicationPolicy(ctx, replicationID, opts)
	if err != nil {
		return err
	}
	defer func() {
		// rollback policy on error
		if err == nil {
			return
		}
		cleanupErr := h.policySvc.DeleteUserReplication(ctx, replicationID)
		if cleanupErr != nil {
			zerolog.Ctx(ctx).Err(cleanupErr).Msgf("unable to cleanup user replication policy for user %s", replicationID.User)
		}
	}()

	// create s3/swift task:
	var task tasks.ReplicationTask
	storageType := h.credsSvc.Storages()[replicationID.FromStorage]
	switch storageType {
	case dom.S3:
		task = &tasks.MigrateS3UserPayload{}
	case dom.Swift:
		task = &tasks.SwiftAccountMigrationPayload{}
	default:
		return fmt.Errorf("%w: unsupported storage type %s for user replication", dom.ErrInvalidArg, storageType)
	}
	// enqueue
	uid := entity.UniversalFromUserReplication(replicationID)
	task.SetReplicationID(uid, generation)
	return h.queueSvc.EnqueueTask(ctx, task)
}

func (h *policyHandlers) addBucketReplication(ctx context.Context, replicationID entity.BucketReplicationPolicy, opts entity.ReplicationOptions) (err error) {
	storageType := h.credsSvc.Storages()[replicationID.FromStorage]
	if err := h.policySvc.ValidateBucketReplicationCreation(ctx, replicationID); err != nil {
		return err
	}
	if err := h.removeLeftoversOfDeletedReplication(ctx, entity.UniversalFromBucketReplication(replicationID)); err != nil {
		return err
	}
	// create policy:
	generation, err := h.policySvc.AddBucketReplicationPolicy(ctx, replicationID, opts)
	if err != nil {
		return fmt.Errorf("%w: unable to add bucket replication policy", err)
	}
	defer func() {
		if err == nil {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if cleanupErr := h.policySvc.DeleteBucketReplication(cleanupCtx, replicationID); cleanupErr != nil {
			zerolog.Ctx(ctx).Error().Err(cleanupErr).Msg("bucket replication cleanup failed")
		}
	}()
	// create bucket notification for agent:
	err = h.createAgentBucketNotification(ctx, replicationID, opts.AgentURL)
	if err != nil {
		return fmt.Errorf("%w: unable to create agent bucket notification", err)
	}
	// create task
	var task tasks.ReplicationTask
	switch storageType {
	case dom.S3:
		task = &tasks.BucketCreatePayload{
			Bucket: replicationID.FromBucket,
		}
	case dom.Swift:
		task = &tasks.SwiftContainerMigrationPayload{
			Bucket: replicationID.FromBucket,
		}
	default:
		return fmt.Errorf("%w: unsupported storage type %s for bucket replication", dom.ErrInvalidArg, storageType)
	}
	task.SetReplicationID(entity.UniversalFromBucketReplication(replicationID), generation)
	return h.queueSvc.EnqueueTask(ctx, task)
}

func (h *policyHandlers) AvailableBuckets(ctx context.Context, req *pb.AvailableBucketsRequest) (*pb.AvailableBucketsResponse, error) {
	// validate request: an empty user means storage-level (connection account of both storages)
	if err := h.credsSvc.HasUser(req.FromStorage, req.User); err != nil {
		return nil, err
	}
	if err := h.credsSvc.HasUser(req.ToStorage, req.User); err != nil {
		return nil, err
	}

	if req.User != "" {
		userReplExists, err := h.policySvc.IsUserReplicationExists(ctx, req.User)
		if err != nil {
			return nil, err
		}
		if userReplExists {
			// user replication exists - no need to create bucket replications
			return &pb.AvailableBucketsResponse{}, nil
		}
	}

	bucketRepl, err := h.policySvc.ListBucketReplicationsInfo(ctx, req.User)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return nil, err
	}

	usedBuckets := make(map[string]struct{})
	for k := range bucketRepl {
		// create set of buckets already used either as source or destination
		if k.FromStorage == req.FromStorage && k.ToStorage == req.ToStorage && k.User == req.User {
			usedBuckets[k.FromBucket] = struct{}{}
			usedBuckets[k.ToBucket] = struct{}{}
		}
	}

	client, err := h.clients.AsCommon(ctx, req.FromStorage, req.User)
	if err != nil {
		return nil, err
	}
	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	buckets, err = h.scope.Filter(ctx, req.FromStorage, req.User, buckets)
	if err != nil {
		return nil, err
	}
	// buckets covered for this edge by any group or replication of any user
	owners, err := h.policySvc.ListScopeOwners(ctx)
	if err != nil {
		return nil, err
	}
	res := &pb.AvailableBucketsResponse{}
	for _, bucket := range buckets {
		if _, used := usedBuckets[bucket]; used {
			// already used by this user on the same edge
			if req.ShowReplicated {
				res.ReplicatedBuckets = append(res.ReplicatedBuckets, bucket)
			}
			continue
		}
		if cerr := replscope.Check(replscope.ReplicationScope(req.FromStorage, req.ToStorage, []string{bucket}).WithUser(req.User), owners, ""); cerr != nil {
			res.CoveredBuckets = append(res.CoveredBuckets, &pb.CoveredBucket{Bucket: bucket, OwnerKind: cerr.OwnerKind, OwnerId: cerr.OwnerID})
			continue
		}
		res.Buckets = append(res.Buckets, bucket)
	}
	return res, nil
}

func (h *policyHandlers) DeleteReplication(ctx context.Context, req *pb.ReplicationID) (*emptypb.Empty, error) {
	uid, err := pbToReplicationID(req)
	if err != nil {
		return nil, err
	}
	// lock order: user, then replication
	err = h.inUserLock(ctx, req.User, func(ctx context.Context) error {
		return h.inReplicationLock(ctx, uid, func(ctx context.Context) error {
			return h.deleteReplication(ctx, uid)
		})
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *policyHandlers) deleteReplication(ctx context.Context, uid entity.UniversalReplicationID) error {
	existing, err := h.policySvc.GetReplicationPolicyInfoExtended(ctx, uid)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return err
	}
	// missing replication may have leftover queues, so policy delete still runs; metadata is kept because user replication cleanup also matches bucket replications
	exists := err == nil
	if userRepl, ok := uid.AsUserID(); ok {
		if err := h.policySvc.DeleteUserReplication(ctx, userRepl); err != nil {
			return err
		}
	} else if bucketRepl, ok := uid.AsBucketID(); ok {
		if err := h.policySvc.DeleteBucketReplication(ctx, bucketRepl); err != nil {
			return fmt.Errorf("%w: unable to delete replication policy", err)
		}
		if exists && existing.AgentURL != "" {
			err := h.notificationSvc.DeleteBucketNotification(ctx, bucketRepl.FromStorage, bucketRepl.User, bucketRepl.FromBucket)
			if err != nil {
				zerolog.Ctx(ctx).Err(err).Msg("unable to delete agent bucket notification")
			}
		}
	} else {
		return fmt.Errorf("%w: invalid replication ID", dom.ErrInvalidArg)
	}
	if !exists {
		return nil
	}
	return h.deleteReplicationMetadata(ctx, uid)
}

// removeLeftoversOfDeletedReplication must be called only after creation is validated:
// metadata cleanup of a user replication also matches its bucket replications.
// Queues are stopped first so canceled tasks cannot start new metadata writes (go-redis rejects commands on canceled ctx).
// A write already sent can still land.
func (h *policyHandlers) removeLeftoversOfDeletedReplication(ctx context.Context, uid entity.UniversalReplicationID) error {
	if err := tasks.StopAndPurgeReplicationQueues(ctx, h.queueSvc, uid); err != nil {
		return fmt.Errorf("unable to remove queues of deleted replication: %w", err)
	}
	if err := h.deleteReplicationMetadata(ctx, uid); err != nil {
		return fmt.Errorf("unable to clean up leftovers of deleted replication: %w", err)
	}
	return nil
}

func (h *policyHandlers) deleteReplicationMetadata(ctx context.Context, uid entity.UniversalReplicationID) error {
	if err := h.versionSvc.Cleanup(ctx, uid); err != nil {
		return fmt.Errorf("%w: unable to delete version metadata", err)
	}
	if err := h.objectListStateStore.DeleteForReplication(ctx, uid); err != nil {
		return fmt.Errorf("%w: unable to delete list obj metadata", err)
	}
	if err := h.bucketListStateStore.DeleteForReplication(ctx, uid); err != nil {
		return fmt.Errorf("%w: unable to delete list bucket metadata", err)
	}
	return nil
}

func (h *policyHandlers) DeleteSwitch(ctx context.Context, req *pb.ReplicationID) (*emptypb.Empty, error) {
	uid, err := pbToReplicationID(req)
	if err != nil {
		return nil, err
	}
	err = h.inReplicationLock(ctx, uid, func(ctx context.Context) error {
		return h.policySvc.DeleteReplicationSwitch(ctx, uid)
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *policyHandlers) GetReplication(ctx context.Context, req *pb.ReplicationID) (*pb.Replication, error) {
	uid, err := pbToReplicationID(req)
	if err != nil {
		return nil, err
	}
	status, err := h.policySvc.GetReplicationPolicyInfoExtended(ctx, uid)
	if err != nil {
		return nil, err
	}
	return replicationToPb(uid, status), nil
}

func (h *policyHandlers) GetSwitchStatus(ctx context.Context, req *pb.ReplicationID) (*pb.ReplicationSwitch, error) {
	uid, err := pbToReplicationID(req)
	if err != nil {
		return nil, err
	}
	res, err := h.policySvc.GetReplicationSwitchInfo(ctx, uid)
	if err != nil {
		return nil, err
	}
	return toPbSwitchStatus(&res), nil
}

func (h *policyHandlers) ListReplications(ctx context.Context, req *pb.ListReplicationsRequest) (*pb.ListReplicationsResponse, error) {
	var res []*pb.Replication
	if !req.HideBucketReplications {
		bucketRepls, err := h.listBucketReplications(ctx, req.Filter)
		if err != nil {
			return nil, err
		}
		res = bucketRepls
	}
	if !req.HideUserReplications {
		userRepls, err := h.listUserReplications(ctx, req.Filter)
		if err != nil {
			return nil, err
		}
		res = append(res, userRepls...)
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].CreatedAt.AsTime().After(res[j].CreatedAt.AsTime())
	})
	if req.HideSwitchInfo {
		for _, r := range res {
			r.SwitchInfo = nil
		}
	}
	return &pb.ListReplicationsResponse{Replications: res}, nil
}

func (h *policyHandlers) listBucketReplications(ctx context.Context, filter *pb.ListReplicationsRequest_Filter) ([]*pb.Replication, error) {
	users := h.credsSvc.ListUsers(h.credsSvc.MainStorage())
	if filter != nil && filter.User != nil && *filter.User != "" {
		// filter by user if set
		users = []string{*filter.User}
	} else {
		// storage-level bucket replications have an empty user
		users = append(append([]string(nil), users...), "")
	}
	usersRes := make([]map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended, len(users))
	// list bucket replications per user in parallel
	g, gCtx := errgroup.WithContext(ctx)
	for i, user := range users {
		g.Go(func() error {
			replications, err := h.policySvc.ListBucketReplicationsInfo(gCtx, user)
			if err != nil && !errors.Is(err, dom.ErrNotFound) {
				return fmt.Errorf("unable to list replications for user %s: %w", user, err)
			}
			usersRes[i] = replications
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	totalNum := 0
	for _, userReplications := range usersRes {
		for k, v := range userReplications {
			if h.skipBucketReplication(k, v, filter) {
				delete(userReplications, k)
			}
		}
		totalNum += len(userReplications)
	}
	res := make([]*pb.Replication, 0, totalNum)
	for _, replications := range usersRes {
		if replications == nil {
			continue
		}
		for k, v := range replications {
			res = append(res, replicationToPb(entity.UniversalFromBucketReplication(k), v))
		}
	}
	return res, nil
}

func (h *policyHandlers) listUserReplications(ctx context.Context, filter *pb.ListReplicationsRequest_Filter) ([]*pb.Replication, error) {
	replications, err := h.policySvc.ListUserReplicationsInfo(ctx)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return nil, err
	}
	for k, v := range replications {
		if h.skipUserReplication(k, v, filter) {
			delete(replications, k)
		}
	}
	res := make([]*pb.Replication, 0, len(replications))
	for k, v := range replications {
		res = append(res, replicationToPb(entity.UniversalFromUserReplication(k), v))
	}
	return res, nil
}

func (h *policyHandlers) skipBucketReplication(k entity.BucketReplicationPolicy, v entity.ReplicationStatusExtended, filter *pb.ListReplicationsRequest_Filter) bool {
	if filter == nil {
		return false
	}
	if filter.User != nil && *filter.User != "" && k.User != *filter.User {
		return true
	}
	if filter.FromStorage != nil && *filter.FromStorage != "" && k.FromStorage != *filter.FromStorage {
		return true
	}
	if filter.ToStorage != nil && *filter.ToStorage != "" && k.ToStorage != *filter.ToStorage {
		return true
	}
	if filter.FromBucket != nil && *filter.FromBucket != "" && k.FromBucket != *filter.FromBucket {
		return true
	}
	if filter.ToBucket != nil && *filter.ToBucket != "" && k.ToBucket != *filter.ToBucket {
		return true
	}
	if filter.IsPaused != nil && v.IsPaused != *filter.IsPaused {
		return true
	}
	if filter.EventSource != nil && eventSourceFromStatus(v.ReplicationStatus) != *filter.EventSource {
		return true
	}
	if filter.IsArchived != nil && v.IsArchived != *filter.IsArchived {
		return true
	}
	if filter.HasSwitch != nil && v.Switch != nil != *filter.HasSwitch {
		return true
	}
	return false
}

func (h *policyHandlers) skipUserReplication(k entity.UserReplicationPolicy, v entity.ReplicationStatusExtended, filter *pb.ListReplicationsRequest_Filter) bool {
	if filter == nil {
		return false
	}
	if filter.User != nil && *filter.User != "" && k.User != *filter.User {
		return true
	}
	if filter.FromStorage != nil && *filter.FromStorage != "" && k.FromStorage != *filter.FromStorage {
		return true
	}
	if filter.ToStorage != nil && *filter.ToStorage != "" && k.ToStorage != *filter.ToStorage {
		return true
	}
	if filter.IsPaused != nil && v.IsPaused != *filter.IsPaused {
		return true
	}
	if filter.EventSource != nil && eventSourceFromStatus(v.ReplicationStatus) != *filter.EventSource {
		return true
	}
	if filter.IsArchived != nil && v.IsArchived != *filter.IsArchived {
		return true
	}
	if filter.HasSwitch != nil && v.Switch != nil != *filter.HasSwitch {
		return true
	}
	return false
}

func (h *policyHandlers) PauseReplication(ctx context.Context, req *pb.ReplicationID) (*emptypb.Empty, error) {
	uid, err := pbToReplicationID(req)
	if err != nil {
		return nil, err
	}
	if h.cpStore != nil {
		if err := h.cpStore.Repos().Audit.Record(ctx, "PAUSE_INTENT", "replication", uid.String(), req); err != nil {
			return nil, fmt.Errorf("audit: unable to record pause intent: %w", err)
		}
	}
	err = h.inUserLock(ctx, req.User, func(ctx context.Context) error {
		return h.policySvc.PauseReplication(ctx, uid)
	})
	if err != nil {
		return nil, err
	}
	if h.cpStore != nil {
		_ = h.cpStore.Repos().Audit.Record(ctx, "PAUSE", "replication", uid.String(), req)
	}
	return &emptypb.Empty{}, nil
}

func (h *policyHandlers) ResumeReplication(ctx context.Context, req *pb.ReplicationID) (*emptypb.Empty, error) {
	uid, err := pbToReplicationID(req)
	if err != nil {
		return nil, err
	}
	if h.cpStore != nil {
		if err := h.cpStore.Repos().Audit.Record(ctx, "RESUME_INTENT", "replication", uid.String(), req); err != nil {
			return nil, fmt.Errorf("audit: unable to record resume intent: %w", err)
		}
	}
	err = h.inUserLock(ctx, req.User, func(ctx context.Context) error {
		return h.policySvc.ResumeReplication(ctx, uid)
	})
	if err != nil {
		return nil, err
	}
	if h.cpStore != nil {
		_ = h.cpStore.Repos().Audit.Record(ctx, "RESUME", "replication", uid.String(), req)
	}
	return &emptypb.Empty{}, nil
}

func (h *policyHandlers) StreamReplication(req *pb.ReplicationID, server grpc.ServerStreamingServer[pb.Replication]) error {
	uid, err := pbToReplicationID(req)
	if err != nil {
		return err
	}
	const pollInterval = time.Millisecond * 500
	timer := time.NewTimer(pollInterval)
	defer timer.Stop()

	ctx := server.Context()
	var prev *pb.Replication
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			res, err := h.policySvc.GetReplicationPolicyInfoExtended(ctx, uid)
			if err != nil {
				return err
			}
			curr := replicationToPb(uid, res)
			if prev == nil || !proto.Equal(prev, curr) {
				prev = curr
				err = server.Send(curr)
				if err != nil {
					return err
				}
			}
			timer.Reset(pollInterval)
		}
	}
}

var errSwitchStorageLevel = fmt.Errorf("%w: switch is not supported for storage-level replication; use a protection group switchover", dom.ErrFailedPrecondition)

func (h *policyHandlers) SwitchWithDowntime(ctx context.Context, req *pb.SwitchDowntimeRequest) (*emptypb.Empty, error) {
	if req.DowntimeOpts != nil {
		if err := validateSwitchDonwtimeOpts(req.DowntimeOpts); err != nil {
			return nil, err
		}
	}
	uid, err := pbToReplicationID(req.ReplicationId)
	if err != nil {
		return nil, err
	}
	if uid.IsStorageLevel() {
		return nil, errSwitchStorageLevel
	}
	err = h.inReplicationLock(ctx, uid, func(ctx context.Context) error {
		// persist switch metadata
		err = h.policySvc.SetDowntimeReplicationSwitch(ctx, uid, pbToDowntimeOpts(req.DowntimeOpts))
		if err != nil {
			return fmt.Errorf("unable to store switch metadata: %w", err)
		}
		// create switch task
		err = h.queueSvc.EnqueueTask(ctx, tasks.SwitchWithDowntimePayload{
			ID: uid,
		})
		if err != nil {
			return fmt.Errorf("unable to enqueue switch task: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *policyHandlers) SwitchWithZeroDowntime(ctx context.Context, req *pb.SwitchZeroDowntimeRequest) (*emptypb.Empty, error) {
	uid, err := pbToReplicationID(req.ReplicationId)
	if err != nil {
		return nil, err
	}
	if uid.IsStorageLevel() {
		return nil, errSwitchStorageLevel
	}
	storageType := h.credsSvc.Storages()[uid.FromStorage()]
	if storageType != dom.S3 {
		return nil, fmt.Errorf("%w: zero-downtime switch is only supported for S3 storage type", dom.ErrInvalidArg)
	}
	err = h.inReplicationLock(ctx, uid, func(ctx context.Context) error {
		// persist switch metadata
		opts := &entity.ReplicationSwitchZeroDowntimeOpts{
			MultipartTTL: defaultZeroDowntimeMultipartTTL,
		}
		if req.MultipartTtl != nil && req.MultipartTtl.AsDuration() > 0 {
			opts.MultipartTTL = req.MultipartTtl.AsDuration()
		}

		err = h.policySvc.AddZeroDowntimeReplicationSwitch(ctx, uid, opts)
		if err != nil {
			return fmt.Errorf("unable to store switch metadata: %w", err)
		}
		// create switch task
		err = h.queueSvc.EnqueueTask(ctx, tasks.ZeroDowntimeReplicationSwitchPayload{
			ID: uid,
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func pbToEventSource(es pb.EventSource) dom.EventSource {
	switch es {
	case pb.EventSource_EVENT_SOURCE_S3_NOTIFICATION:
		return dom.EventSourceS3Notification
	case pb.EventSource_EVENT_SOURCE_WEBHOOK:
		return dom.EventSourceWebhook
	default:
		return dom.EventSourceProxy
	}
}

func (h *policyHandlers) webhookURLForStorage(storage string, storageType dom.StorageType) (string, error) {
	webhook := h.effectiveWebhook()
	if webhook == nil || !webhook.Enabled {
		return "", fmt.Errorf("%w: webhook must be enabled", dom.ErrInvalidArg)
	}
	switch storageType {
	case dom.S3:
		return webhook.S3NotificationURL(storage)
	case dom.Swift:
		return webhook.SwiftWebhookURL(storage)
	default:
		return "", fmt.Errorf("%w: unsupported storage type %s for webhook", dom.ErrInvalidArg, storageType)
	}
}

func (h *policyHandlers) validateWebhookEventSource(eventSource pb.EventSource) error {
	webhook := h.effectiveWebhook()
	if eventSource == pb.EventSource_EVENT_SOURCE_WEBHOOK && (webhook == nil || !webhook.Enabled) {
		return fmt.Errorf("%w: webhook must be enabled for webhook event source", dom.ErrInvalidArg)
	}
	return nil
}

func (h *policyHandlers) effectiveWebhook() *WebhookConfig {
	if h.currentWebhook != nil {
		conf := h.currentWebhook()
		return &conf
	}
	return h.webhookConf
}

func (h *policyHandlers) createAgentBucketNotification(ctx context.Context, replicationID entity.BucketReplicationPolicy, agentURL string) error {
	if agentURL == "" {
		// agent is not set
		return nil
	}
	// create a topic and bucket notification for agent event source.
	err := h.notificationSvc.SubscribeToBucketNotifications(ctx, replicationID.FromStorage, replicationID.User, replicationID.FromBucket, agentURL)
	if err != nil {
		cleanupErr := h.policySvc.DeleteBucketReplication(context.Background(), replicationID)
		if cleanupErr != nil {
			zerolog.Ctx(ctx).Err(cleanupErr).Msgf("unable to cleanup replication policy for bucket %s", replicationID.FromBucket)
		}
		return err
	}
	return nil
}

func (h *policyHandlers) inUserLock(ctx context.Context, user string, fn func(ctx context.Context) error) error {
	if user == "" {
		// storage-level replication has no user to serialize on
		return fn(ctx)
	}
	lock, err := h.userLocker.Lock(ctx, user, store.WithDuration(time.Second), store.WithRetry(true))
	if err != nil {
		return err
	}
	defer lock.Release(ctx)
	return lock.DoCtx(ctx, time.Second, fn)
}

func (h *policyHandlers) inReplicationLock(ctx context.Context, id entity.UniversalReplicationID, fn func(ctx context.Context) error) error {
	lock, err := h.replicationStatusLocker.Lock(ctx, id, store.WithDuration(time.Second), store.WithRetry(true))
	if err != nil {
		return err
	}
	defer lock.Release(ctx)
	return lock.DoCtx(ctx, time.Second, fn)
}

func (h *policyHandlers) AddRouting(ctx context.Context, req *pb.AddRoutingRequest) (*emptypb.Empty, error) {
	if req.User == "" {
		return nil, fmt.Errorf("%w: user is required", dom.ErrInvalidArg)
	}
	if err := h.credsSvc.HasUser(req.ToStorage, req.User); err != nil {
		return nil, err
	}
	err := h.inUserLock(ctx, req.User, func(ctx context.Context) error {
		return h.storageLocker.With(ctx, []string{req.ToStorage}, func(ctx context.Context) error {
			// the storage may have been deleted while waiting for the lock
			if err := h.credsSvc.HasUser(req.ToStorage, req.User); err != nil {
				return err
			}
			if req.Bucket != nil && *req.Bucket != "" {
				return h.policySvc.SetBucketRouting(ctx, entity.BucketRoutingPolicyID{
					User:   req.User,
					Bucket: *req.Bucket,
				}, req.ToStorage)
			}
			return h.policySvc.SetUserRouting(ctx, req.User, req.ToStorage)
		})
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// UpdateRouting changes the target storage of an existing user or bucket routing atomically.
func (h *policyHandlers) UpdateRouting(ctx context.Context, req *pb.UpdateRoutingRequest) (*emptypb.Empty, error) {
	if req.User == "" {
		return nil, fmt.Errorf("%w: user is required", dom.ErrInvalidArg)
	}
	if req.ToStorage == "" {
		return nil, fmt.Errorf("%w: to_storage is required", dom.ErrInvalidArg)
	}
	if err := h.credsSvc.HasUser(req.ToStorage, req.User); err != nil {
		return nil, err
	}
	err := h.inUserLock(ctx, req.User, func(ctx context.Context) error {
		return h.storageLocker.With(ctx, []string{req.ToStorage}, func(ctx context.Context) error {
			if err := h.credsSvc.HasUser(req.ToStorage, req.User); err != nil {
				return err
			}
			if req.Bucket != nil && *req.Bucket != "" {
				return h.policySvc.UpdateBucketRouting(ctx, entity.BucketRoutingPolicyID{
					User:   req.User,
					Bucket: *req.Bucket,
				}, req.ToStorage)
			}
			return h.policySvc.UpdateUserRouting(ctx, req.User, req.ToStorage)
		})
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *policyHandlers) DeleteRouting(ctx context.Context, req *pb.RoutingID) (*emptypb.Empty, error) {
	if req.User == "" {
		return nil, fmt.Errorf("%w: user is required", dom.ErrInvalidArg)
	}
	err := h.inUserLock(ctx, req.User, func(ctx context.Context) error {
		if req.Bucket != nil && *req.Bucket != "" {
			return h.policySvc.DeleteBucketRouting(ctx, entity.BucketRoutingPolicyID{
				User:   req.User,
				Bucket: *req.Bucket,
			})
		}
		return h.policySvc.DeleteUserRouting(ctx, req.User)
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *policyHandlers) BlockRouting(ctx context.Context, req *pb.RoutingID) (*emptypb.Empty, error) {
	if req.User == "" {
		return nil, fmt.Errorf("%w: user is required", dom.ErrInvalidArg)
	}
	err := h.inUserLock(ctx, req.User, func(ctx context.Context) error {
		if req.Bucket != nil && *req.Bucket != "" {
			return h.policySvc.BlockBucketRouting(ctx, entity.BucketRoutingPolicyID{
				User:   req.User,
				Bucket: *req.Bucket,
			})
		}
		return h.policySvc.BlockUserRouting(ctx, req.User)
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *policyHandlers) UnblockRouting(ctx context.Context, req *pb.RoutingID) (*emptypb.Empty, error) {
	if req.User == "" {
		return nil, fmt.Errorf("%w: user is required", dom.ErrInvalidArg)
	}
	err := h.inUserLock(ctx, req.User, func(ctx context.Context) error {
		if req.Bucket != nil && *req.Bucket != "" {
			return h.policySvc.UnblockBucketRouting(ctx, entity.BucketRoutingPolicyID{
				User:   req.User,
				Bucket: *req.Bucket,
			})
		}
		return h.policySvc.UnblockUserRouting(ctx, req.User)
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *policyHandlers) ListRoutings(ctx context.Context, req *pb.RoutingsRequest) (*pb.RoutingsResponse, error) {
	res := &pb.RoutingsResponse{
		Main: h.credsSvc.MainStorage(),
	}

	users := h.credsSvc.ListUsers(h.credsSvc.MainStorage())
	if req.Filter != nil && req.Filter.User != nil && *req.Filter.User != "" {
		// filter by user if set
		users = []string{*req.Filter.User}
	}
	if !req.HideBucketRoutings {
		for _, user := range users {
			br, err := h.policySvc.GetBucketRoutings(ctx, user)
			if err != nil {
				return nil, err
			}
			bb, err := h.policySvc.GetBucketRoutingBlocks(ctx, user)
			if err != nil {
				return nil, err
			}
			res.BucketRoutings = append(res.BucketRoutings, toBucketRoutingsPb(req.Filter, user, br, bb)...)
		}
	}
	if !req.HideUserRoutings {
		ur, err := h.policySvc.GetUserRoutings(ctx)
		if err != nil {
			return nil, err
		}
		ub, err := h.policySvc.GetUserRoutingBlocks(ctx)
		if err != nil {
			return nil, err
		}
		res.UserRoutings = toUserRoutingsPb(req.Filter, ur, ub)
	}
	return res, nil
}

func (h *policyHandlers) TestProxy(ctx context.Context, req *pb.TestProxyRequest) (*pb.TestProxyResponse, error) {
	var err error
	ctx, err = h.policySvc.BuildProxyContext(ctx, req.User, req.Bucket)
	if err != nil {
		if errors.Is(err, dom.ErrRoutingBlock) {
			return &pb.TestProxyResponse{
				IsBlocked: true,
			}, nil
		}
		return nil, err
	}
	result := &pb.TestProxyResponse{
		RouteToStorage: xctx.GetRoutingPolicy(ctx),
	}
	replications := xctx.GetReplications(ctx)
	for _, repl := range replications {
		result.Replications = append(result.Replications, replicationIDToPb(repl))
	}
	return result, nil
}
