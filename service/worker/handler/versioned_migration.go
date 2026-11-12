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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/log"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/service/worker/copy"
)

const (
	CMaxBufferedMoveObjectVersions = 100
)

func (s *svc) HandleObjectVersionList(ctx context.Context, t *asynq.Task) error {
	ctx = tasks.WithStrictWatermark(ctx)
	var listVersionsPayload tasks.ListObjectVersionsPayload
	if err := json.Unmarshal(t.Payload(), &listVersionsPayload); err != nil {
		return fmt.Errorf("unable to unmarshal payload: %w: %w", err, asynq.SkipRetry)
	}

	user := listVersionsPayload.ID.User()
	fromBucket, toBucket := listVersionsPayload.ID.FromToBuckets(listVersionsPayload.Bucket)

	objectVersionID := entity.NewVersionedObjectID(listVersionsPayload.ID.FromStorage(), fromBucket, listVersionsPayload.Prefix)
	replicationID := entity.NewBucketReplicationPolicy(user, listVersionsPayload.ID.FromStorage(), fromBucket, listVersionsPayload.ID.ToStorage(), toBucket)

	if err := s.versionedSvc.ListVersions(ctx, objectVersionID, replicationID, listVersionsPayload.Generation); err != nil {
		return fmt.Errorf("unable to list obejct versions: %w", err)
	}

	migratePayload := tasks.MigrateVersionedObjectPayload(listVersionsPayload)
	err := s.queueSvc.EnqueueTask(ctx, migratePayload)
	if err != nil {
		return fmt.Errorf("migration bucket list obj: unable to enqueue copy obj task: %w", err)
	}

	return nil
}

func (s *svc) HandleVersionedObjectMigration(ctx context.Context, t *asynq.Task) error {
	var migratePayload tasks.MigrateVersionedObjectPayload
	if err := json.Unmarshal(t.Payload(), &migratePayload); err != nil {
		return fmt.Errorf("unable to unmarshal payload: %w: %w", err, asynq.SkipRetry)
	}
	ctx = log.WithBucket(ctx, migratePayload.Bucket)
	ctx = log.WithUser(ctx, migratePayload.ID.User())
	logger := zerolog.Ctx(ctx)
	// check rate limit before proceeding the task to avoid rollback in the middle of the operation
	if err := s.limit.StorReq(ctx, migratePayload.ID.FromStorage()); err != nil {
		logger.Debug().Err(err).Str(log.Storage, migratePayload.ID.FromStorage()).Msg("rate limit error")
		return err
	}
	if err := s.limit.StorReq(ctx, migratePayload.ID.ToStorage()); err != nil {
		logger.Debug().Err(err).Str(log.Storage, migratePayload.ID.ToStorage()).Msg("rate limit error")
		return err
	}

	user := migratePayload.ID.User()
	fromBucket, toBucket := migratePayload.ID.FromToBuckets(migratePayload.Bucket)
	replicationID := entity.NewBucketReplicationPolicy(user, migratePayload.ID.FromStorage(), fromBucket, migratePayload.ID.ToStorage(), toBucket)

	if err := s.versionedSvc.MigrateVersions(ctx, replicationID, migratePayload.Prefix, migratePayload.Generation); err != nil {
		return fmt.Errorf("unable to migrate object version: %w", err)
	}

	return nil
}

type VersionedMigrationSvc struct {
	copySvc                copy.CopySvc
	objectVersionInfoStore *store.ObjectVersionInfoStore
	objectLocker           *store.ObjectLocker
}

func NewVersionedMigrationSvc(copySvc copy.CopySvc,
	objectVersionInfoStore *store.ObjectVersionInfoStore, objectLocker *store.ObjectLocker) *VersionedMigrationSvc {
	return &VersionedMigrationSvc{
		copySvc:                copySvc,
		objectVersionInfoStore: objectVersionInfoStore,
		objectLocker:           objectLocker,
	}
}

func workGeneration(generation []int64) int64 {
	if len(generation) > 0 {
		return generation[0]
	}
	return 0
}

func (r *VersionedMigrationSvc) withObjectLock(ctx context.Context, id entity.BucketReplicationPolicy, key string, work func(context.Context) error) error {
	locker := r.objectLocker
	if locker == nil {
		locker = store.NewObjectLocker(r.objectVersionInfoStore.Client(), 2*time.Second)
	}
	lock, err := locker.Lock(ctx, entity.NewObjectLockID(id.ToStorage, id.ToBucket, key), store.WithRetry(true))
	if err != nil {
		return err
	}
	defer lock.Release(ctx)
	return lock.DoCtx(ctx, 2*time.Second, work)
}

func (r *VersionedMigrationSvc) snapshot(ctx context.Context, objectID entity.VersionedObjectID, id entity.BucketReplicationPolicy, work *store.ObjectVersionInfoStore) error {
	versions, err := r.copySvc.GetVersionInfo(ctx, id.User, copy.NewFile(objectID.Storage, objectID.Bucket, objectID.Name))
	if err != nil {
		return fmt.Errorf("unable to list object versions: %w", err)
	}
	// Copy the slice: a client or test may retain its listing result.
	chronological := append([]entity.ObjectVersionInfo(nil), versions...)
	slices.Reverse(chronological)
	if err := ctx.Err(); err != nil {
		return err
	}
	return work.ReplaceSnapshot(ctx, objectID, chronological)
}

func (r *VersionedMigrationSvc) ListVersions(ctx context.Context, objectID entity.VersionedObjectID, id entity.BucketReplicationPolicy, generation ...int64) error {
	work := r.objectVersionInfoStore.ForReplication(id, workGeneration(generation))
	return r.withObjectLock(ctx, id, objectID.Name, func(ctx context.Context) error {
		return r.snapshot(ctx, objectID, id, work)
	})
}

func (r *VersionedMigrationSvc) MigrateVersions(ctx context.Context, id entity.BucketReplicationPolicy, prefix string, generation ...int64) error {
	work := r.objectVersionInfoStore.ForReplication(id, workGeneration(generation))
	return r.withObjectLock(ctx, id, prefix, func(ctx context.Context) error {
		return r.migrateVersions(ctx, id, prefix, work)
	})
}

func (r *VersionedMigrationSvc) migrateVersions(ctx context.Context, replicationID entity.BucketReplicationPolicy, prefix string, work *store.ObjectVersionInfoStore) error {
	toFile := copy.File{
		Storage: replicationID.ToStorage,
		Bucket:  replicationID.ToBucket,
		Name:    prefix,
	}

	objectID := entity.NewVersionedObjectID(replicationID.FromStorage, replicationID.FromBucket, prefix)
	firstPage, err := work.GetPage(ctx, objectID, store.NewPager(0, 1))
	if err != nil {
		return err
	}
	if len(firstPage) == 0 {
		if err := r.snapshot(ctx, objectID, replicationID, work); err != nil {
			return err
		}
	}

	var destinationIsEmpty bool
	var shouldClearDestination bool
	migratedObjectVersionInfo, err := r.copySvc.GetLastMigratedVersionInfo(ctx, replicationID.User, toFile)
	switch {
	case errors.Is(err, dom.ErrNotFound):
		destinationIsEmpty = true
	case errors.Is(err, dom.ErrInvalidArg):
		shouldClearDestination = true
	case err != nil:
		return fmt.Errorf("unable to get migrated object version info: %w", err)
	}

	firstVersionToMigrateIdx := uint64(0)
	versionedObjectID := entity.NewVersionedObjectID(replicationID.FromStorage, replicationID.FromBucket, prefix)
	if !destinationIsEmpty {
		if migratedObjectVersionInfo.Version == "" {
			shouldClearDestination = true
		} else {
			versionIdx, err := work.Find(ctx, versionedObjectID, migratedObjectVersionInfo)
			if errors.Is(err, dom.ErrNotFound) {
				shouldClearDestination = true
			} else if err != nil {
				return fmt.Errorf("unable to get object version index: %w", err)
			}

			firstVersionToMigrateIdx = uint64(versionIdx) + 1
		}
	}

	if shouldClearDestination {
		if err := r.copySvc.ClearDestination(ctx, replicationID.User, toFile, true); err != nil {
			return fmt.Errorf("unable to delete destination object: %w", err)
		}
	}

	pager := store.NewPager(firstVersionToMigrateIdx, CMaxBufferedMoveObjectVersions)
	for {
		objectVersionInfoList, err := work.GetPage(ctx, versionedObjectID, pager)
		if err != nil {
			return fmt.Errorf("unable to get object version info list: %w", err)
		}

		for _, info := range objectVersionInfoList {
			fromFile := copy.NewVersionedFile(replicationID.FromStorage, replicationID.FromBucket, prefix, info.Version)
			toFile := copy.NewVersionedFile(replicationID.ToStorage, replicationID.ToBucket, prefix, info.Version)

			if err = r.copySvc.CopyObject(ctx, replicationID.User, fromFile, toFile); err != nil {
				if errors.Is(err, dom.ErrNotFound) {
					zerolog.Ctx(ctx).Warn().Str(log.ObjectVer, info.Version).Msg("versioned migration: skip object version copy: version missing in source")
					continue
				}
				return fmt.Errorf("unable to copy object: %w", err)
			}
		}

		if len(objectVersionInfoList) < CMaxBufferedMoveObjectVersions {
			break
		}

		pager.From += CMaxBufferedMoveObjectVersions
	}

	if _, err := work.Drop(ctx, versionedObjectID); err != nil {
		return fmt.Errorf("unable to clean up version work: %w", err)
	}

	return nil
}
