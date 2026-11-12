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

package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"golang.org/x/sync/singleflight"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/log"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
)

type DiffCtrl struct {
	diffSvc  *DiffSvc
	queueSvc tasks.QueueService
}

func NewDiffCtrl(svc *DiffSvc, queueSvc tasks.QueueService) *DiffCtrl {
	return &DiffCtrl{
		diffSvc:  svc,
		queueSvc: queueSvc,
	}
}

func (r *DiffCtrl) HandleDiff(ctx context.Context, t *asynq.Task) error {
	var payload tasks.DiffPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unable to unmarshal payload: %w: %w", err, asynq.SkipRetry)
	}

	for idx := range payload.Locations {
		if err := r.queueSvc.EnqueueTask(ctx, tasks.DiffListObjectsPayload{
			Locations:   payload.Locations,
			User:        payload.User,
			Index:       idx,
			Versioned:   payload.Versioned,
			IgnoreEtags: payload.IgnoreEtags,
			IgnoreSizes: payload.IgnoreSizes,
		}); err != nil {
			return fmt.Errorf("unable to enqueue diff list task: %w", err)
		}
	}

	return nil
}

func (r *DiffCtrl) HandleDiffList(ctx context.Context, t *asynq.Task) error {
	var payload tasks.DiffListObjectsPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unable to unmarshal payload: %w: %w", err, asynq.SkipRetry)
	}

	diffID := r.diffIDFromMigrateLocations(payload.Locations)
	settings := entity.NewDiffSettings(payload.User, payload.Versioned, payload.IgnoreSizes, payload.IgnoreEtags)
	location := entity.NewDiffLocation(payload.Locations[payload.Index].Storage, payload.Locations[payload.Index].Bucket)

	ctx = log.WithStorage(ctx, location.Storage)
	ctx = log.WithBucket(ctx, location.Bucket)

	payloadMaker := func(prefix string) tasks.DiffListVersionsPayload {
		return tasks.DiffListVersionsPayload{
			Locations:   payload.Locations,
			User:        payload.User,
			Index:       payload.Index,
			Prefix:      prefix,
			IgnoreEtags: payload.IgnoreEtags,
			IgnoreSizes: payload.IgnoreSizes,
		}
	}

	if err := r.diffSvc.ProduceObjectTasks(ctx, diffID, settings, location, payload.Prefix, payloadMaker); err != nil {
		return fmt.Errorf("unable to produce object tasks: %w", err)
	}

	return nil
}

func (r *DiffCtrl) HandleDiffListVersions(ctx context.Context, t *asynq.Task) error {
	var payload tasks.DiffListVersionsPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unable to unmarshal payload: %w: %w", err, asynq.SkipRetry)
	}

	diffID := r.diffIDFromMigrateLocations(payload.Locations)
	storage := payload.Locations[payload.Index].Storage
	bucket := payload.Locations[payload.Index].Bucket

	if err := r.diffSvc.AccountObjectVersions(ctx, diffID, payload.User,
		entity.NewDiffLocation(storage, bucket), payload.Prefix, payload.IgnoreEtags, payload.IgnoreSizes); err != nil {
		return fmt.Errorf("unable to account object versions: %w", err)
	}

	return nil
}

func (r *DiffCtrl) HandleDiffCollectObjects(ctx context.Context, t *asynq.Task) error {
	var payload tasks.DiffFixCollectObjectsPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unable to unmarshal payload: %w: %w", err, asynq.SkipRetry)
	}

	diffID := r.diffIDFromMigrateLocations(payload.Locations)
	sourceStorage := payload.Locations[payload.SourceIndex].Storage
	for idx := range payload.Locations {
		if idx == payload.SourceIndex {
			continue
		}

		destinationStorage := payload.Locations[idx].Storage
		diffFixID := entity.NewDiffFixID(diffID, destinationStorage)
		if err := r.diffSvc.CollectObjectsToFix(ctx, diffFixID, sourceStorage, payload.Versioned); err != nil {
			return fmt.Errorf("unable to collect objects to fix: %w", err)
		}

		if err := r.queueSvc.EnqueueTask(ctx, tasks.DiffFixRemoveObjectsPayload{
			Locations:   payload.Locations,
			SourceIndex: payload.SourceIndex,
			RemoveIndex: idx,
			StorageType: payload.StorageType,
			Versioned:   payload.Versioned,
			User:        payload.User,
		}); err != nil {
			return fmt.Errorf("unable to enqueue remove diff objects task: %w", err)
		}
	}

	return nil
}

func (r *DiffCtrl) HandleDiffRemoveObjects(ctx context.Context, t *asynq.Task) error {
	var payload tasks.DiffFixRemoveObjectsPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unable to unmarshal payload: %w: %w", err, asynq.SkipRetry)
	}

	diffID := r.diffIDFromMigrateLocations(payload.Locations)
	toLocation := entity.NewDiffLocation(payload.Locations[payload.RemoveIndex].Storage, payload.Locations[payload.RemoveIndex].Bucket)
	diffFixID := entity.NewDiffFixID(diffID, toLocation.Storage)
	if err := r.diffSvc.RemoveDiffObjects(ctx, diffFixID, toLocation, payload.User, payload.Versioned); err != nil {
		return fmt.Errorf("unable to remove diff objects: %w", err)
	}

	if err := r.queueSvc.EnqueueTask(ctx, tasks.DiffFixEnsureObjectsRemovedPayload(payload)); err != nil {
		return fmt.Errorf("unable to enqueue remove diff objects task: %w", err)
	}

	return nil
}

func (r *DiffCtrl) HandleDiffEnsureObjectsRemoved(ctx context.Context, t *asynq.Task) error {
	var payload tasks.DiffFixEnsureObjectsRemovedPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unable to unmarshal payload: %w: %w", err, asynq.SkipRetry)
	}

	diffID := r.diffIDFromMigrateLocations(payload.Locations)
	fromLocation := entity.NewDiffLocation(payload.Locations[payload.SourceIndex].Storage, payload.Locations[payload.SourceIndex].Bucket)
	toLocation := entity.NewDiffLocation(payload.Locations[payload.RemoveIndex].Storage, payload.Locations[payload.RemoveIndex].Bucket)
	bucketReplID := entity.NewBucketReplicationPolicy(payload.User, fromLocation.Storage, fromLocation.Bucket, toLocation.Storage, toLocation.Bucket)
	universalReplID := entity.UniversalFromBucketReplication(bucketReplID)
	diffFixID := entity.NewDiffFixID(diffID, toLocation.Storage)

	// diff fix queue is not generation-checked, so generation is 0
	payloadMaker := func(object string, isDir bool) (any, error) {
		switch payload.StorageType {
		case dom.Swift:
			if payload.Versioned {
				return nil, fmt.Errorf("swift versioned objects are not supported: %w", asynq.SkipRetry)
			}
			task := tasks.DiffFixSwiftCopyPayload{
				Locations: payload.Locations,
				SwiftObjectMigrationPayload: tasks.SwiftObjectMigrationPayload{
					Bucket:  fromLocation.Bucket,
					ObjName: object,
				},
			}
			task.SetReplicationID(universalReplID, 0)
			return task, nil
		case dom.S3:
			if payload.Versioned && !isDir {
				task := tasks.DiffFixS3ListVersionsPayload{
					Locations: payload.Locations,
					ListObjectVersionsPayload: tasks.ListObjectVersionsPayload{
						Bucket: fromLocation.Bucket,
						Prefix: object,
					},
				}
				task.SetReplicationID(universalReplID, 0)
				return task, nil
			}
			task := tasks.DiffFixS3CopyPayload{
				Locations: payload.Locations,
				MigrateObjCopyPayload: tasks.MigrateObjCopyPayload{
					Bucket: fromLocation.Bucket,
					Obj: tasks.ObjPayload{
						Name: object,
					},
				},
			}
			task.SetReplicationID(universalReplID, 0)
			return task, nil
		default:
			return nil, fmt.Errorf("unknown storage type %s: %w", payload.StorageType, asynq.SkipRetry)
		}
	}

	if err := r.diffSvc.EnsureObjectsDeleted(ctx, diffFixID, toLocation, payload.User, payload.Versioned, payloadMaker); err != nil {
		return fmt.Errorf("unable to ensure that objects are deleted: %w", err)
	}

	return nil
}

func (r *DiffCtrl) diffIDFromMigrateLocations(migrateLocations []tasks.MigrateLocation) entity.DiffID {
	locationCount := len(migrateLocations)
	locations := make([]entity.DiffLocation, 0, locationCount)
	for _, payloadLocation := range migrateLocations {
		locations = append(locations, entity.NewDiffLocation(payloadLocation.Storage, payloadLocation.Bucket))
	}

	return entity.NewDiffID(locations...)
}

type DiffSvc struct {
	client            redis.Cmdable
	idStore           *store.DiffIDStore
	settingsStore     *store.DiffSettingsStore
	listStateStore    *store.DiffListStateStore
	setStore          *store.DiffSetStore
	fixCopySetStore   *store.DiffFixCopySetStore
	fixRemoveSetStore *store.DiffFixRemoveSetStore
	clients           objstore.Clients
	queueSvc          tasks.QueueService
	reconcileSF       singleflight.Group
}

func NewDiffSvc(redisClient redis.Cmdable, clients objstore.Clients, queueSvc tasks.QueueService) *DiffSvc {
	return &DiffSvc{
		client:            redisClient,
		idStore:           store.NewDiffIDStore(redisClient),
		settingsStore:     store.NewDiffSettingsStore(redisClient),
		listStateStore:    store.NewDiffListStateStore(redisClient),
		setStore:          store.NewDiffSetStore(redisClient),
		fixCopySetStore:   store.NewDiffFixCopySetStore(redisClient),
		fixRemoveSetStore: store.NewDiffFixRemoveSetStore(redisClient),
		clients:           clients,
		queueSvc:          queueSvc,
	}
}

func (r *DiffSvc) ShouldCheckVersions(ctx context.Context, user string, locations []entity.DiffLocation) (bool, error) {
	versionedLocations := 0

	for _, location := range locations {
		client, err := r.clients.AsCommon(ctx, location.Storage, user)
		if err != nil {
			return false, fmt.Errorf("unable to obtain client: %w", err)
		}
		versioned, err := client.IsBucketVersioned(ctx, location.Bucket)
		if err != nil {
			return false, fmt.Errorf("unable to check if bucket is versioned: %w", err)
		}
		if versioned {
			versionedLocations++
		}
	}

	return versionedLocations == len(locations), nil
}

func (r *DiffSvc) ProduceObjectTasks(ctx context.Context, diffID entity.DiffID, settings entity.DiffSettings, location entity.DiffLocation, prefix string, makePayload func(prefix string) tasks.DiffListVersionsPayload) error {
	locationCount := len(diffID.Locations)
	objectID := entity.NewDiffObjectID(diffID, location.Storage, location.Bucket, prefix)

	lastObject, err := r.listStateStore.Get(ctx, objectID)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return fmt.Errorf("unable to get last listed object: %w", err)
	}

	client, err := r.clients.AsCommon(ctx, location.Storage, settings.User)
	if err != nil {
		return fmt.Errorf("unable to obtain client: %w", err)
	}

	objects := client.ListObjects(ctx, location.Bucket, objstore.WithPrefix(prefix), objstore.WithAfter(lastObject))
	for object, err := range objects {
		if err != nil {
			if errors.Is(err, objstore.ErrBucketDoesntExist) {
				return nil
			}

			return fmt.Errorf("unable to list objects: %w", err)
		}

		isDir := object.Size == 0 && object.VersionID == "" && strings.HasSuffix(object.Key, "/")

		if settings.Versioned && !isDir {
			listVersionsTask := makePayload(object.Key)
			if err := r.queueSvc.EnqueueTask(ctx, listVersionsTask); err != nil {
				return fmt.Errorf("unable to enqueue diff list task: %w", err)
			}

			continue
		}

		var id entity.DiffSetID
		switch {
		case settings.IgnoreSizes:
			id = entity.NewNameDiffSetID(diffID, object.Key)
		case settings.IgnoreEtags:
			id = entity.NewSizeDiffSetID(diffID, object.Key, object.Size)
		default:
			id = entity.NewEtagDiffSetID(diffID, object.Key, object.Size, object.Etag)
		}

		entry := entity.NewDiffSetEntry(location, isDir)
		if err := r.setStore.Add(ctx, id, entry, uint8(locationCount)); err != nil {
			return fmt.Errorf("unable to add storage to diff set: %w", err)
		}

		if err := r.listStateStore.Set(ctx, objectID, object.Key); err != nil {
			return fmt.Errorf("unable to set last listed object: %w", err)
		}
	}

	if _, err := r.listStateStore.Drop(ctx, objectID); err != nil {
		return fmt.Errorf("unable to delete last listed object: %w", err)
	}

	return nil
}

func (r *DiffSvc) AccountObjectVersions(ctx context.Context, diffID entity.DiffID,
	user string, location entity.DiffLocation, object string, ignoreEtags bool, ignoreSizes bool) error {
	locationCount := len(diffID.Locations)

	client, err := r.clients.AsCommon(ctx, location.Storage, user)
	if err != nil {
		return fmt.Errorf("unable to obtain client: %w", err)
	}

	versionIdx := uint64(0)
	objects := client.ObjectVersions(ctx, location.Bucket, object)
	for object, err := range objects {
		if err != nil {
			return fmt.Errorf("unable to list object versions: %w", err)
		}

		var id entity.DiffSetID
		switch {
		case ignoreSizes:
			id = entity.NewVersionedNameDiffSetID(diffID, object.Key, versionIdx)
		case ignoreEtags:
			id = entity.NewVersionedSizeDiffSetID(diffID, object.Key, versionIdx, object.Size)
		default:
			id = entity.NewVersionedEtagDiffSetID(diffID, object.Key, versionIdx, object.Size, object.Etag)
		}

		entry := entity.NewVersionedDiffSetEntry(location, object.VersionID)
		if err := r.setStore.Add(ctx, id, entry, uint8(locationCount)); err != nil {
			return fmt.Errorf("unable to add to check set: %w", err)
		}
		versionIdx++
	}

	return nil
}

func (r *DiffSvc) DeleteDiff(ctx context.Context, id entity.DiffID) error {
	tokens, err := store.DiffIDToTokensConverter(id)
	if err != nil {
		return fmt.Errorf("unable to make tokens out of diff id: %w", err)
	}

	if r.client != nil && len(tokens) > 0 {
		_ = r.client.Del(ctx, "cc:reconciled:"+tokens[0], "cc:fix:error:"+tokens[0]).Err()
		_ = r.client.Incr(ctx, "cc:diff:gen:"+tokens[0]).Err()
	}

	queues := []string{tasks.DiffQueue(id), tasks.DiffFixQueue(id)}
	if err := r.queueSvc.Stop(ctx, queues...); err != nil {
		return fmt.Errorf("unable to stop diff queues: %w", err)
	}
	if err := r.queueSvc.Purge(ctx, queues...); err != nil {
		return fmt.Errorf("unable to purge diff queues: %w", err)
	}

	exec := r.idStore.GroupExecutor()
	_ = r.idStore.WithExecutor(exec).RemoveOp(ctx, struct{}{}, id)
	_ = r.settingsStore.WithExecutor(exec).DropOp(ctx, id)
	_ = r.setStore.WithExecutor(exec).DropIDsOp(ctx, tokens...)
	_ = r.listStateStore.WithExecutor(exec).DropIDsOp(ctx, tokens...)
	_ = r.fixCopySetStore.WithExecutor(exec).DropIDsOp(ctx, tokens...)
	_ = r.fixRemoveSetStore.WithExecutor(exec).DropIDsOp(ctx, tokens...)

	if err := exec.Exec(ctx); err != nil {
		return fmt.Errorf("unable to clean up diff data: %w", err)
	}

	return nil
}

func (r *DiffSvc) RegisterDiff(ctx context.Context, id entity.DiffID, settings entity.DiffSettings) error {
	affected, err := r.idStore.Add(ctx, struct{}{}, id)
	if err != nil {
		return fmt.Errorf("unable to check if diff id exists: %w", err)
	}
	if affected != 1 {
		return errors.New("diff for this set of storage locations already exists")
	}
	if err := r.settingsStore.Set(ctx, id, settings); err != nil {
		return fmt.Errorf("unable to save diff settings: %w", err)
	}
	return nil
}

func (r *DiffSvc) GetDiffList(ctx context.Context) ([]entity.DiffStatus, error) {
	ids, err := r.idStore.Get(ctx, struct{}{})
	if errors.Is(err, dom.ErrNotFound) {
		return []entity.DiffStatus{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("unable to get diff ids: %w", err)
	}

	statuses := make([]entity.DiffStatus, 0, len(ids))
	for _, id := range ids {
		status, err := r.GetDiffStatus(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("unable to get diff status: %w", err)
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

func (r *DiffSvc) GetDiffStatus(ctx context.Context, id entity.DiffID) (entity.DiffStatus, error) {
	diffQueue := tasks.DiffQueue(id)

	settings, err := r.settingsStore.Get(ctx, id)
	if err != nil {
		return entity.DiffStatus{}, fmt.Errorf("unable to get diff settings: %w", err)
	}

	diffQueueStats, err := r.queueSvc.Stats(ctx, diffQueue)
	if err != nil {
		return entity.DiffStatus{}, fmt.Errorf("unable to get diff queue stats: %w", err)
	}

	ready := diffQueueStats.Unprocessed == 0

	status := entity.DiffStatus{
		Locations: id.Locations,
		Check: entity.DiffCheckStatus{
			Queue: entity.DiffQueueStatus{
				Queued:    uint64(diffQueueStats.Unprocessed + diffQueueStats.ProcessedTotal),
				Completed: uint64(diffQueueStats.ProcessedTotal),
				Ready:     ready,
			},
			Settings: settings,
		},
	}

	if !ready {
		return status, nil
	}

	tokens, err := store.DiffIDToTokensConverter(id)
	if err != nil {
		return entity.DiffStatus{}, fmt.Errorf("unable to make tokens out of diff id: %w", err)
	}

	notConsistent, err := r.setStore.HasIDs(ctx, tokens...)
	if err != nil {
		return entity.DiffStatus{}, fmt.Errorf("unable to check if there are inconsistencies: %w", err)
	}

	if notConsistent && !settings.IgnoreEtags {
		_ = r.ReconcileInconclusiveMultipart(ctx, id, settings)
		notConsistent, _ = r.setStore.HasIDs(ctx, tokens...)
	}

	status.Check.Consistent = !notConsistent

	fixQueue := tasks.DiffFixQueue(id)
	fixQueueStats, err := r.queueSvc.Stats(ctx, fixQueue)
	if errors.Is(err, dom.ErrNotFound) {
		// if fix queue is not found, then returning collected diff stats
		return status, nil
	}
	if err != nil {
		return entity.DiffStatus{}, fmt.Errorf("unable to get diff fix queue stats: %w", err)
	}

	status.FixQueue = &entity.DiffQueueStatus{
		Queued:    uint64(fixQueueStats.Unprocessed + fixQueueStats.ProcessedTotal),
		Completed: uint64(fixQueueStats.ProcessedTotal),
		Ready:     fixQueueStats.Unprocessed == 0,
	}

	return status, nil
}

func (r *DiffSvc) GetDiffReportEntries(ctx context.Context, id entity.DiffID, cursor uint64, pageSize uint64) (entity.DiffReportEntryPage, error) {
	settings, _ := r.settingsStore.Get(ctx, id)
	if !settings.IgnoreEtags {
		diffQueue := tasks.DiffQueue(id)
		diffQueueStats, err := r.queueSvc.Stats(ctx, diffQueue)
		if err == nil && diffQueueStats.Unprocessed == 0 {
			_ = r.ReconcileInconclusiveMultipart(ctx, id, settings)
		}
	}

	tokens, err := store.DiffIDToTokensConverter(id)
	if err != nil {
		return entity.DiffReportEntryPage{}, fmt.Errorf("unable to make tokens out of diff id: %w", err)
	}

	pager := store.NewPager(cursor, pageSize)
	idPage, err := r.setStore.GetIDs(ctx, pager, tokens...)
	if err != nil {
		return entity.DiffReportEntryPage{}, fmt.Errorf("unable to get id page: %w", err)
	}

	exec := r.setStore.GroupExecutor()
	execSetStore := r.setStore.WithExecutor(exec)

	ops := make([]store.OperationResult[[]entity.DiffSetEntry], 0, len(idPage.Entries))
	for _, id := range idPage.Entries {
		op := execSetStore.GetOp(ctx, id)
		ops = append(ops, op)
	}

	if err := exec.Exec(ctx); err != nil {
		return entity.DiffReportEntryPage{}, fmt.Errorf("unable to get diff sets: %w", err)
	}

	entries := make([]entity.DiffReportEntry, 0, len(idPage.Entries))
	for idx, op := range ops {
		storageEntries, _ := op.Get()
		id := idPage.Entries[idx]
		entry := entity.NewDiffReportEntry(id.Object, id.VersionIndex, id.Size, id.Etag, storageEntries)
		entries = append(entries, entry)
	}

	return entity.NewDiffReportEntryPage(entries, idPage.Next), nil
}

func isMultipartETag(etag string) bool {
	e := strings.ToLower(strings.Trim(strings.TrimSpace(etag), `"`))
	if strings.Contains(e, "-") {
		return true
	}
	if len(e) != 32 && len(e) > 0 {
		return true
	}
	for _, c := range e {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return true
		}
	}
	return false
}

// ReconcileInconclusiveMultipart checks remaining diff entries where ETags cannot be definitively
// compared (e.g. multipart upload with '-' part suffix vs singlepart upload). If all locations have
// matching sizes and content SHA-256 digests, the entries represent identical content and are pruned
// from the diff set so false inconsistencies are not reported.
// Singleflight deduplicates concurrent reconciliation runs, generation keys ensure fresh runs on restart,
// and filtering restricts in-memory buffers to multipart discrepancies (O(1) memory for standard diffs).
func (r *DiffSvc) ReconcileInconclusiveMultipart(ctx context.Context, id entity.DiffID, settings entity.DiffSettings) error {
	if r.clients == nil || r.client == nil || len(id.Locations) < 2 {
		return nil
	}

	tokens, err := store.DiffIDToTokensConverter(id)
	if err != nil {
		return err
	}

	_, err, _ = r.reconcileSF.Do(tokens[0], func() (any, error) {
		return nil, r.reconcileInconclusiveMultipartInner(ctx, id, settings, tokens)
	})
	return err
}

func (r *DiffSvc) reconcileInconclusiveMultipartInner(ctx context.Context, id entity.DiffID, settings entity.DiffSettings, tokens []string) error {
	gen, _ := r.client.Get(ctx, "cc:diff:gen:"+tokens[0]).Int64()
	reconciledKey := fmt.Sprintf("cc:reconciled:%s:%d", tokens[0], gen)
	if val, _ := r.client.Get(ctx, reconciledKey).Result(); val == "1" {
		return nil
	}

	// First pass: scan IDs to see if any multipart ETags exist across discrepancies.
	// Only objects that have at least one multipart ETag need to be considered.
	// This avoids allocating candidate maps and querying object sets for buckets where
	// discrepancies are all standard single-PUT objects (O(1) memory for standard diffs).
	multipartObjects := make(map[string]struct{})
	pager := store.NewPager(0, 1000)
	for {
		idPage, err := r.setStore.GetIDs(ctx, pager, tokens...)
		if err != nil {
			return err
		}
		for _, sid := range idPage.Entries {
			if isMultipartETag(sid.Etag) {
				multipartObjects[sid.Object] = struct{}{}
			}
		}
		if idPage.Next == 0 {
			break
		}
		pager.From = idPage.Next
	}

	if len(multipartObjects) == 0 {
		currentGen, _ := r.client.Get(ctx, "cc:diff:gen:"+tokens[0]).Int64()
		if currentGen == gen {
			_ = r.client.Set(ctx, reconciledKey, "1", 24*time.Hour).Err()
		}
		return nil
	}

	type candInfo struct {
		size         uint64
		hasMultipart bool
		ids          []entity.DiffSetID
		locations    map[string]string // storage -> versionID
	}

	candidates := map[string]*candInfo{}
	pager = store.NewPager(0, 500)
	for {
		idPage, err := r.setStore.GetIDs(ctx, pager, tokens...)
		if err != nil {
			return err
		}

		var filteredEntries []entity.DiffSetID
		for _, sid := range idPage.Entries {
			if _, ok := multipartObjects[sid.Object]; ok {
				filteredEntries = append(filteredEntries, sid)
			}
		}

		if len(filteredEntries) > 0 {
			exec := r.setStore.GroupExecutor()
			execSetStore := r.setStore.WithExecutor(exec)
			opResults := make([]store.OperationResult[[]entity.DiffSetEntry], 0, len(filteredEntries))
			for _, sid := range filteredEntries {
				opResults = append(opResults, execSetStore.GetOp(ctx, sid))
			}
			if err := exec.Exec(ctx); err != nil {
				return err
			}

			for idx, sid := range filteredEntries {
				objKey := sid.Object + "\x00" + strconv.FormatUint(sid.VersionIndex, 10)
				cand := candidates[objKey]
				if cand == nil {
					cand = &candInfo{
						size:      sid.Size,
						locations: make(map[string]string),
					}
					candidates[objKey] = cand
				}
				cand.ids = append(cand.ids, sid)
				if isMultipartETag(sid.Etag) {
					cand.hasMultipart = true
				}
				entrySet, _ := opResults[idx].Get()
				for _, entry := range entrySet {
					cand.locations[entry.Location.Storage] = entry.VersionID
				}
			}
		}

		if idPage.Next == 0 {
			break
		}
		pager.From = idPage.Next
	}

	requiredStorageCount := len(id.Locations)
	var spuriousIDs []entity.DiffSetID

	for _, cand := range candidates {
		if !cand.hasMultipart || len(cand.locations) < requiredStorageCount {
			continue
		}

		var firstHash []byte
		allMatch := true

		for _, loc := range id.Locations {
			c, err := r.clients.AsCommon(ctx, loc.Storage, settings.User)
			if err != nil {
				allMatch = false
				break
			}
			var reader io.Reader
			if verID := cand.locations[loc.Storage]; verID != "" {
				reader, err = c.GetObject(ctx, loc.Bucket, cand.ids[0].Object, objstore.WithVersionID(verID))
			} else {
				reader, err = c.GetObject(ctx, loc.Bucket, cand.ids[0].Object)
			}
			if err != nil {
				allMatch = false
				break
			}
			h := sha256.New()
			n, err := io.Copy(h, reader)
			if closer, ok := reader.(io.Closer); ok {
				_ = closer.Close()
			}
			if err != nil || uint64(n) != cand.size {
				allMatch = false
				break
			}
			sum := h.Sum(nil)
			if firstHash == nil {
				firstHash = sum
			} else if !bytes.Equal(firstHash, sum) {
				allMatch = false
				break
			}
		}

		if allMatch && firstHash != nil {
			spuriousIDs = append(spuriousIDs, cand.ids...)
		}
	}

	if len(spuriousIDs) > 0 {
		exec := r.setStore.GroupExecutor()
		for _, sid := range spuriousIDs {
			_ = r.setStore.WithExecutor(exec).DropOp(ctx, sid)
		}
		_ = exec.Exec(ctx)
	}

	currentGen, _ := r.client.Get(ctx, "cc:diff:gen:"+tokens[0]).Int64()
	if currentGen == gen {
		_ = r.client.Set(ctx, reconciledKey, "1", 24*time.Hour).Err()
	}
	return nil
}

func (r *DiffSvc) StartDiff(ctx context.Context, id entity.DiffID, settings entity.DiffSettings) error {
	if err := r.RegisterDiff(ctx, id, settings); err != nil {
		return fmt.Errorf("unable to register diff: %w", err)
	}

	tokens, err := store.DiffIDToTokensConverter(id)
	if err == nil && len(tokens) > 0 && r.client != nil {
		_ = r.client.Incr(ctx, "cc:diff:gen:"+tokens[0]).Err()
		_ = r.client.Del(ctx, "cc:reconciled:"+tokens[0], "cc:fix:error:"+tokens[0]).Err()
	}

	taskLocations := make([]tasks.MigrateLocation, 0, len(id.Locations))
	for _, location := range id.Locations {
		taskLocations = append(taskLocations, tasks.MigrateLocation{
			Storage: location.Storage,
			Bucket:  location.Bucket,
		})
	}

	diffTask := tasks.DiffPayload{
		Locations:   taskLocations,
		User:        settings.User,
		Versioned:   settings.Versioned,
		IgnoreEtags: settings.IgnoreEtags,
		IgnoreSizes: settings.IgnoreSizes,
	}
	if err := r.queueSvc.EnqueueTask(ctx, diffTask); err != nil {
		return fmt.Errorf("unable to enqueue diff check task: %w", err)
	}

	return nil
}

func (r *DiffSvc) RestartDiff(ctx context.Context, id entity.DiffID) error {
	status, err := r.GetDiffStatus(ctx, id)
	if err != nil {
		return fmt.Errorf("unable to get diff status: %w", err)
	}

	if err := r.DeleteDiff(ctx, id); err != nil {
		return fmt.Errorf("unable to delete diff: %w", err)
	}

	if err := r.StartDiff(ctx, id, status.Check.Settings); err != nil {
		return fmt.Errorf("unable to start diff: %w", err)
	}

	return nil
}

func (r *DiffSvc) FixDiff(ctx context.Context, id entity.DiffID, source entity.DiffLocation, storageType dom.StorageType) error {
	status, err := r.GetDiffStatus(ctx, id)
	if err != nil {
		return fmt.Errorf("unable to get diff status: %w", err)
	}
	if !status.Check.Queue.Ready {
		return fmt.Errorf("%w: diff is not ready", dom.ErrFailedPrecondition)
	}
	if status.Check.Consistent {
		return fmt.Errorf("%w: storages are consistent, nothing to do", dom.ErrFailedPrecondition)
	}
	if status.FixQueue != nil {
		if !status.FixQueue.Ready {
			return fmt.Errorf("%w: diff fix already running", dom.ErrFailedPrecondition)
		}
		_ = r.queueSvc.Purge(ctx, tasks.DiffFixQueue(id))
		tokens, err := store.DiffIDToTokensConverter(id)
		if err == nil && len(tokens) > 0 && r.client != nil {
			exec := r.idStore.GroupExecutor()
			_ = r.fixCopySetStore.WithExecutor(exec).DropIDsOp(ctx, tokens...)
			_ = r.fixRemoveSetStore.WithExecutor(exec).DropIDsOp(ctx, tokens...)
			_ = exec.Exec(ctx)
			_ = r.client.Del(ctx, "cc:fix:error:"+tokens[0]).Err()
		}
	}

	sourceIdx := -1
	taskLocations := make([]tasks.MigrateLocation, 0, len(id.Locations))
	for idx, location := range id.Locations {
		if location == source {
			sourceIdx = idx
		}
		taskLocations = append(taskLocations, tasks.MigrateLocation{
			Storage: location.Storage,
			Bucket:  location.Bucket,
		})
	}

	if sourceIdx < 0 {
		return fmt.Errorf("%w: unable to determine source index", dom.ErrFailedPrecondition)
	}

	task := tasks.DiffFixCollectObjectsPayload{
		Locations:   taskLocations,
		SourceIndex: sourceIdx,
		StorageType: storageType,
		Versioned:   status.Check.Settings.Versioned,
		User:        status.Check.Settings.User,
	}

	if err := r.queueSvc.EnqueueTask(ctx, task); err != nil {
		return fmt.Errorf("unable to enqueue diff fix purge task: %w", err)
	}

	return nil
}

var luaRecordPresence = redis.NewScript(`
for i = 1, #ARGV, 2 do
	local obj = ARGV[i]
	local mask = tonumber(ARGV[i+1])
	local old = tonumber(redis.call('HGET', KEYS[1], obj) or '0')
	local s = (old % 2 >= 1 or mask % 2 >= 1) and 1 or 0
	local d = (math.floor(old / 2) % 2 >= 1 or math.floor(mask / 2) % 2 >= 1) and 2 or 0
	local dir = (math.floor(old / 4) % 2 >= 1 or math.floor(mask / 4) % 2 >= 1) and 4 or 0
	redis.call('HSET', KEYS[1], obj, tostring(s + d + dir))
end
redis.call('EXPIRE', KEYS[1], 3600)
return 0
`)

func (r *DiffSvc) CollectObjectsToFix(ctx context.Context, diffFixID entity.DiffFixID, source string, versioned bool) error {
	settings, _ := r.settingsStore.Get(ctx, diffFixID.DiffID)
	if !settings.IgnoreEtags {
		_ = r.ReconcileInconclusiveMultipart(ctx, diffFixID.DiffID, settings)
	}

	tokens, err := store.DiffIDToTokensConverter(diffFixID.DiffID)
	if err != nil {
		return fmt.Errorf("unable to make tokens out of diff id: %w", err)
	}

	destination := diffFixID.Destination
	presenceKey := "cc:fix:presence:" + tokens[0] + ":" + destination
	defer func() {
		if r.client != nil {
			_ = r.client.Del(ctx, presenceKey).Err()
		}
	}()

	pager := store.NewPager(0, 1000)
	hasAny := false
	for {
		idPage, err := r.setStore.GetIDs(ctx, pager, tokens...)
		if err != nil {
			return fmt.Errorf("unable to get id page: %w", err)
		}

		if len(idPage.Entries) > 0 {
			exec := r.setStore.GroupExecutor()
			execSetStore := r.setStore.WithExecutor(exec)

			opResults := make([]store.OperationResult[[]entity.DiffSetEntry], 0, len(idPage.Entries))
			for _, id := range idPage.Entries {
				opResults = append(opResults, execSetStore.GetOp(ctx, id))
			}

			if err := exec.Exec(ctx); err != nil {
				return fmt.Errorf("unable to get diff sets: %w", err)
			}

			args := make([]any, 0, len(idPage.Entries)*2)
			for idx, id := range idPage.Entries {
				mask := 0
				entrySet, _ := opResults[idx].Get()
				for _, entry := range entrySet {
					switch entry.Location.Storage {
					case source:
						mask |= 1
						if entry.IsDir {
							mask |= 4
						}
					case destination:
						mask |= 2
					}
				}
				if mask > 0 {
					args = append(args, id.Object, mask)
					hasAny = true
				}
			}

			if len(args) > 0 {
				if err := luaRecordPresence.Eval(ctx, r.client, []string{presenceKey}, args...).Err(); err != nil {
					return fmt.Errorf("unable to record presence in redis: %w", err)
				}
			}
		}

		if idPage.Next == 0 {
			break
		}

		pager.From = idPage.Next
	}

	if !hasAny {
		return nil
	}

	var cursor uint64
	for {
		entries, nextCur, err := r.client.HScan(ctx, presenceKey, cursor, "*", 1000).Result()
		if err != nil {
			return fmt.Errorf("unable to scan presence: %w", err)
		}

		var copyBatch []entity.DiffFixCopyObject
		var hasRemove bool

		for i := 0; i < len(entries); i += 2 {
			object := entries[i]
			val, _ := strconv.Atoi(entries[i+1])
			inSource := (val & 1) != 0
			inDest := (val & 2) != 0
			isDir := (val & 4) != 0

			switch {
			case inSource && !inDest:
				copyBatch = append(copyBatch, entity.NewDiffFixCopyObject(object, isDir))
			case !inSource && inDest:
				hasRemove = true
			case inSource && inDest:
				if !versioned {
					copyBatch = append(copyBatch, entity.NewDiffFixCopyObject(object, isDir))
				} else {
					copyBatch = append(copyBatch, entity.NewDiffFixCopyObject(object, isDir))
					hasRemove = true
				}
			}
		}

		if hasRemove {
			if r.client != nil && len(tokens) > 0 {
				_ = r.client.Set(ctx, "cc:fix:error:"+tokens[0], "destructive diff fix requires fenced writes", 24*time.Hour).Err()
			}
			return fmt.Errorf("%w: destructive diff fix requires fenced writes: %w",
				dom.ErrFailedPrecondition, asynq.SkipRetry)
		}

		if len(copyBatch) > 0 {
			exec := r.fixCopySetStore.GroupExecutor()
			r.fixCopySetStore.WithExecutor(exec).AddOp(ctx, diffFixID, copyBatch...)
			if err := exec.Exec(ctx); err != nil {
				return fmt.Errorf("unable to add objects to fix sets: %w", err)
			}
		}

		if nextCur == 0 {
			break
		}
		cursor = nextCur
	}

	return nil
}

// TODO add option to move conflicting objects rather than removing them?
func (r *DiffSvc) RemoveDiffObjects(ctx context.Context, id entity.DiffFixID, location entity.DiffLocation, user string, versioned bool) error {
	objectNames, err := r.fixRemoveSetStore.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("unable to get objects to remove: %w", err)
	}

	if len(objectNames) == 0 {
		return nil
	}

	tokens, err := store.DiffIDToTokensConverter(id.DiffID)
	if err == nil && len(tokens) > 0 && r.client != nil {
		_ = r.client.Set(ctx, "cc:fix:error:"+tokens[0], "destructive diff fix requires fenced writes", 24*time.Hour).Err()
	}
	return fmt.Errorf("%w: destructive diff fix requires fenced writes: %w", dom.ErrFailedPrecondition, asynq.SkipRetry)
}

func (r *DiffSvc) EnsureObjectsDeleted(ctx context.Context, id entity.DiffFixID, location entity.DiffLocation, user string, versioned bool, makePayload func(object string, isDir bool) (any, error)) error {
	objectsToRemove, err := r.fixRemoveSetStore.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("unable to get objects to remove: %w", err)
	}

	zerolog.Ctx(ctx).Info().Int("count", len(objectsToRemove)).Msg("EnsureObjectsDeleted: checking objects to remove")
	client, err := r.clients.AsCommon(ctx, location.Storage, user)
	if err != nil {
		return fmt.Errorf("unable to obtain client: %w", err)
	}

	for _, object := range objectsToRemove {
		exists, err := client.ObjectExists(ctx, location.Bucket, object)
		if err != nil {
			return fmt.Errorf("unable to check if object exists: %w", err)
		}
		if exists {
			return fmt.Errorf("object %s is still present", object)
		}

		if _, err := r.fixRemoveSetStore.Remove(ctx, id, object); err != nil {
			return fmt.Errorf("unable to remove object from fix set: %w", err)
		}
	}

	// Stream objectsToCopy in bounded batches to prevent unbounded memory consumption
	err = r.fixCopySetStore.ForEach(ctx, id, 500, func(batch []entity.DiffFixCopyObject) error {
		for _, object := range batch {
			payload, err := makePayload(object.Name, object.IsDir)
			if err != nil {
				return fmt.Errorf("unable to make payload: %w", err)
			}
			if err := r.queueSvc.EnqueueTask(ctx, payload); err != nil {
				return fmt.Errorf("unable to enqueue diff copy task: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to process objects to copy: %w", err)
	}

	if _, err := r.fixRemoveSetStore.Drop(ctx, id); err != nil {
		return fmt.Errorf("unable to clean fix set store: %w", err)
	}

	return nil
}
