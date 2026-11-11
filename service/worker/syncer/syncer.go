/*
 * Copyright © 2025 Clyso GmbH
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

// Package syncer implements the single-object synchronization shared by the replication worker
// (asynchronous mode) and the proxy (synchronous mode).
package syncer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	mclient "github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/service/worker/copy"
)

// ObjectSyncer brings a single object of one replication destination up to date.
type ObjectSyncer struct {
	versionSvc   meta.VersionService
	objectLocker *store.ObjectLocker
	copySvc      copy.CopySvc
	clients      objstore.Clients
}

func New(versionSvc meta.VersionService, objectLocker *store.ObjectLocker, copySvc copy.CopySvc, clients objstore.Clients) *ObjectSyncer {
	return &ObjectSyncer{
		versionSvc:   versionSvc,
		objectLocker: objectLocker,
		copySvc:      copySvc,
		clients:      clients,
	}
}

// Sync takes the distributed destination-object lock, compares versions and either deletes the
// object in the destination, skips (destination is up to date) or copies it from the source.
func (s *ObjectSyncer) Sync(ctx context.Context, p tasks.ObjectSyncPayload) error {
	logger := zerolog.Ctx(ctx)
	fromBucket, toBucket := p.ID.FromToBuckets(p.Object.Bucket)

	objectLockID := entity.NewVersionedObjectLockID(p.ID.ToStorage(), toBucket, p.Object.Name, p.Object.Version)
	lock, err := s.objectLocker.Lock(ctx, objectLockID)
	if err != nil {
		return err
	}
	defer lock.Release(ctx)
	versions, err := s.versionSvc.GetObj(ctx, p.ID, p.Object)
	if err != nil {
		return err
	}
	if versions.IsEmpty() {
		return lock.DoCtx(ctx, time.Second*2, func(ctx context.Context) error {
			return s.objectDelete(ctx, p)
		})
	}

	fromVer, toVer := versions.From, versions.To
	if fromVer <= toVer {
		logger.Info().Int("from_ver", fromVer).Int("to_ver", toVer).Msg("object sync: identical from/to obj version: skip copy")
		return nil
	}

	err = lock.DoCtx(ctx, time.Second*2, func(ctx context.Context) error {
		return s.copySvc.CopyObject(ctx, p.ID.User(), copy.File{
			Storage: p.ID.FromStorage(),
			Bucket:  fromBucket,
			Name:    p.Object.Name,
		}, copy.File{
			Storage: p.ID.ToStorage(),
			Bucket:  toBucket,
			Name:    p.Object.Name,
		})
	})
	if err != nil {
		if errors.Is(err, dom.ErrNotFound) {
			logger.Warn().Msg("object sync: skip object sync: object missing in source")
			return nil
		}
		return err
	}
	logger.Info().Msg("object sync: done")

	if fromVer != 0 {
		destination := meta.Destination{Storage: p.ID.ToStorage(), Bucket: toBucket}
		return s.versionSvc.UpdateIfGreater(ctx, p.ID, p.Object, destination, fromVer)
	}
	return nil
}

func (s *ObjectSyncer) objectDelete(ctx context.Context, p tasks.ObjectSyncPayload) error {
	fromClient, err := s.clients.AsS3(ctx, p.ID.FromStorage(), p.ID.User())
	if err != nil {
		return fmt.Errorf("unable to get %q s3 client: %w", p.ID.FromStorage(), err)
	}
	toClient, err := s.clients.AsS3(ctx, p.ID.ToStorage(), p.ID.User())
	if err != nil {
		return fmt.Errorf("unable to get %q s3 client: %w", p.ID.ToStorage(), err)
	}
	_, err = fromClient.S3().StatObject(ctx, p.Object.Bucket, p.Object.Name, mclient.StatObjectOptions{})
	if err == nil {
		zerolog.Ctx(ctx).Warn().Msg("skip obj delete: obj still exists in source storage")
		return nil
	}
	// A failed existence check is not evidence of deletion. Only an explicit
	// NoSuchKey/NoSuchVersion response permits removing the replica.
	missing := mclient.ToErrorResponse(err)
	if missing.StatusCode != http.StatusNotFound || (missing.Code != "NoSuchKey" && missing.Code != "NoSuchVersion") {
		return fmt.Errorf("unable to confirm source object deletion: %w", err)
	}

	_, toBucket := p.ID.FromToBuckets(p.Object.Bucket)
	return toClient.S3().RemoveObject(ctx, toBucket, p.Object.Name, mclient.RemoveObjectOptions{VersionID: p.Object.Version})
}
