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
	"strings"
	"time"

	"github.com/hibiken/asynq"
	mclient "github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/filter"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/service/worker/copy"
)

var ErrDeleteDeferred = errors.New("delete propagation is deferred")

// DeleteDeferredError carries the earliest propagation time to the queue worker.
type DeleteDeferredError struct {
	Deadline time.Time
}

func (e *DeleteDeferredError) Error() string {
	return fmt.Sprintf("%s until %s", ErrDeleteDeferred, e.Deadline.Format(time.RFC3339Nano))
}

func (e *DeleteDeferredError) Unwrap() error { return ErrDeleteDeferred }

type PolicyResolver func(ctx context.Context, id entity.UniversalReplicationID) (entity.ReplicationStatus, error)
type DeleteAuditRecorder func(ctx context.Context, entry controlplane.DeleteAuditEntry) error

// ObjectSyncer brings a single object of one replication destination up to date.
type ObjectSyncer struct {
	versionSvc     meta.VersionService
	objectLocker   *store.ObjectLocker
	copySvc        copy.CopySvc
	clients        objstore.Clients
	policyResolver PolicyResolver
	auditRecorder  DeleteAuditRecorder
}

func New(versionSvc meta.VersionService, objectLocker *store.ObjectLocker, copySvc copy.CopySvc, clients objstore.Clients) *ObjectSyncer {
	return &ObjectSyncer{
		versionSvc:   versionSvc,
		objectLocker: objectLocker,
		copySvc:      copySvc,
		clients:      clients,
	}
}

func (s *ObjectSyncer) SetPolicyResolver(pr PolicyResolver) {
	s.policyResolver = pr
}

func (s *ObjectSyncer) SetDeleteAuditRecorder(ar DeleteAuditRecorder) {
	s.auditRecorder = ar
}

func RecordDeleteAudit(ctx context.Context, record DeleteAuditRecorder, p tasks.ObjectSyncPayload, action string, metadata map[string]any) error {
	if record == nil {
		return nil
	}
	if err := p.ID.Validate(); err != nil {
		return err
	}
	bucket := p.Object.Bucket
	if id, ok := p.ID.AsBucketID(); ok {
		bucket = id.ToBucket
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	return record(ctx, controlplane.DeleteAuditEntry{
		DeleteEventID:         p.DeleteEventID,
		DeleteEventRetryCount: p.DeleteEventRetryCount,
		ReplicationID:         p.ID.AsString(),
		Bucket:                bucket,
		ObjectKey:             p.Object.Name,
		DestinationStorage:    p.ID.ToStorage(),
		Action:                action,
		Timestamp:             time.Now(),
		Metadata:              metadata,
	})
}

func (s *ObjectSyncer) recordDeleteAudit(ctx context.Context, p tasks.ObjectSyncPayload, action string, metadata map[string]any) {
	if err := RecordDeleteAudit(ctx, s.auditRecorder, p, action, metadata); err != nil {
		zerolog.Ctx(ctx).Error().Err(err).Int64("delete_event_id", p.DeleteEventID).Msg("record delete audit")
	}
}

// Sync takes the distributed destination-object lock, compares versions and either deletes the
// object in the destination, skips (destination is up to date) or copies it from the source.
func (s *ObjectSyncer) Sync(ctx context.Context, p tasks.ObjectSyncPayload) (syncErr error) {
	if _, queued := asynq.GetTaskID(ctx); p.Deleted && !queued {
		// Queued failures are recorded by the worker's terminal error handler.
		defer func() {
			if syncErr != nil && !errors.Is(syncErr, ErrDeleteDeferred) {
				s.recordDeleteAudit(ctx, p, "failed", map[string]any{"error": syncErr.Error()})
			}
		}()
	}
	logger := zerolog.Ctx(ctx)
	fromBucket, toBucket := p.ID.FromToBuckets(p.Object.Bucket)

	var status entity.ReplicationStatus
	if s.policyResolver != nil {
		st, err := s.policyResolver(ctx, p.ID)
		if err != nil {
			if !errors.Is(err, dom.ErrNotFound) {
				return fmt.Errorf("unable to resolve replication policy for %s: %w", p.ID.AsString(), err)
			}
		} else {
			status = st
			if len(st.IncludePatterns) > 0 || len(st.ExcludePatterns) > 0 {
				f := filter.New(st.IncludePatterns, st.ExcludePatterns)
				if !f.ShouldSync(p.Object.Name) {
					logger.Debug().Str("key", p.Object.Name).Msg("object sync: skipped by replication filter")
					if p.Deleted {
						s.recordDeleteAudit(ctx, p, "skipped", map[string]any{"reason": "replication filter mismatch"})
					}
					return nil
				}
			}
		}
	}

	objectLockID := entity.NewObjectLockID(p.ID.ToStorage(), toBucket, p.Object.Name)
	lock, err := s.objectLocker.Lock(ctx, objectLockID)
	if err != nil {
		return err
	}
	defer lock.Release(ctx)

	versions, err := s.versionSvc.GetObj(ctx, p.ID, p.Object)
	if err != nil {
		return err
	}

	if p.Deleted || versions.IsEmpty() {
		return lock.DoCtx(ctx, time.Second*2, func(ctx context.Context) error {
			return s.objectDelete(ctx, p, status)
		})
	}

	fromVer, toVer := versions.From, versions.To
	if fromVer <= toVer {
		logger.Info().Int("from_ver", fromVer).Int("to_ver", toVer).Msg("object sync: identical from/to obj version: skip copy")
		return nil
	}

	err = lock.DoCtx(ctx, time.Second*2, func(ctx context.Context) error {
		return s.copyObject(ctx, p, fromBucket, toBucket)
	})
	if err != nil {
		return err
	}

	if fromVer != 0 {
		destination := meta.Destination{Storage: p.ID.ToStorage(), Bucket: toBucket}
		return s.versionSvc.UpdateIfGreater(ctx, p.ID, p.Object, destination, fromVer)
	}
	return nil
}

func (s *ObjectSyncer) copyObject(ctx context.Context, p tasks.ObjectSyncPayload, fromBucket, toBucket string) error {
	ctx = copy.WithKeyLocked(ctx)
	err := s.copySvc.CopyObject(ctx, p.ID.User(), copy.File{
		Storage: p.ID.FromStorage(),
		Bucket:  fromBucket,
		Name:    p.Object.Name,
	}, copy.File{
		Storage: p.ID.ToStorage(),
		Bucket:  toBucket,
		Name:    p.Object.Name,
	})
	if err != nil {
		if errors.Is(err, dom.ErrNotFound) {
			zerolog.Ctx(ctx).Warn().Msg("object sync: skip object sync: object missing in source")
			return nil
		}
		return err
	}
	zerolog.Ctx(ctx).Info().Msg("object sync: done")
	return nil
}

func matchesDeletePrefixFilter(key string, includes, excludes []string) bool {
	for _, exc := range excludes {
		if exc != "" && strings.HasPrefix(key, exc) {
			return false
		}
	}
	hasInclude := false
	for _, inc := range includes {
		if inc != "" {
			hasInclude = true
			if strings.HasPrefix(key, inc) {
				return true
			}
		}
	}
	if !hasInclude {
		return true
	}
	return false
}

func (s *ObjectSyncer) objectDelete(ctx context.Context, p tasks.ObjectSyncPayload, status entity.ReplicationStatus) error {
	logger := zerolog.Ctx(ctx)
	_, toBucket := p.ID.FromToBuckets(p.Object.Bucket)

	mode := status.DeletePropagation
	if s.policyResolver == nil && mode == "" {
		mode = "sync"
	}
	if mode == "" {
		mode = "ignore"
	}
	var skipReason string
	switch {
	case mode == "ignore":
		skipReason = "propagation mode is ignore"
	case !matchesDeletePrefixFilter(p.Object.Name, status.DeleteIncludePrefixes, status.DeleteExcludePrefixes):
		skipReason = "prefix filter mismatch"
	}
	if p.Deleted && skipReason != "" {
		logger.Debug().Str("reason", skipReason).Msg("object delete: skipped by policy")
		s.recordDeleteAudit(ctx, p, "skipped", map[string]any{"reason": skipReason})
		return nil
	}

	fromClient, err := s.clients.AsS3(ctx, p.ID.FromStorage(), p.ID.User())
	if err != nil {
		return fmt.Errorf("unable to get %q s3 client: %w", p.ID.FromStorage(), err)
	}
	_, err = fromClient.S3().StatObject(ctx, p.Object.Bucket, p.Object.Name, mclient.StatObjectOptions{VersionID: p.Object.Version})
	if err == nil {
		if !p.Deleted {
			// Missing version metadata does not turn an existing object into a delete.
			return s.copyObject(ctx, p, p.Object.Bucket, toBucket)
		}
		zerolog.Ctx(ctx).Warn().Msg("skip obj delete: obj still exists in source storage")
		s.recordDeleteAudit(ctx, p, "skipped", map[string]any{"reason": "object still exists in source storage"})
		return nil
	}
	if skipReason != "" {
		return nil
	}
	// A failed existence check is not evidence of deletion. Only an explicit
	// NoSuchKey/NoSuchVersion response permits removing the replica.
	missing := mclient.ToErrorResponse(err)
	if missing.StatusCode != http.StatusNotFound || (missing.Code != "NoSuchKey" && missing.Code != "NoSuchVersion") {
		return fmt.Errorf("unable to confirm source object deletion: %w", err)
	}

	if mode == "marker-only" && p.Object.Version != "" {
		// A historical version can be absent while a newer head is still active.
		_, headErr := fromClient.S3().StatObject(ctx, p.Object.Bucket, p.Object.Name, mclient.StatObjectOptions{})
		if headErr == nil {
			zerolog.Ctx(ctx).Warn().Msg("skip marker creation: active head version already exists on source")
			s.recordDeleteAudit(ctx, p, "skipped", map[string]any{"reason": "active head version recreated on source"})
			return nil
		}
		headMissing := mclient.ToErrorResponse(headErr)
		if headMissing.StatusCode != http.StatusNotFound || (headMissing.Code != "NoSuchKey" && headMissing.Code != "NoSuchVersion") {
			return fmt.Errorf("unable to confirm source head deletion: %w", headErr)
		}
	}

	// A persisted deadline remains authoritative even if the policy is changed later.
	var fallbackDeadline time.Time
	if status.DeleteDelaySeconds > 0 {
		fallbackDeadline = time.Now().Add(time.Duration(status.DeleteDelaySeconds) * time.Second)
	}
	deadline, err := s.versionSvc.GetOrSetDeleteDeadline(ctx, p.ID, p.Object, fallbackDeadline)
	if err != nil {
		return err
	}
	if time.Now().Before(deadline) {
		return &DeleteDeferredError{Deadline: deadline}
	}
	toClient, err := s.clients.AsS3(ctx, p.ID.ToStorage(), p.ID.User())
	if err != nil {
		return fmt.Errorf("unable to get %q s3 client: %w", p.ID.ToStorage(), err)
	}

	if mode == "marker-only" {
		// A DeleteObject without a version ID is only safe here on an enabled bucket.
		vCfg, vErr := toClient.S3().GetBucketVersioning(ctx, toBucket)
		if vErr != nil {
			return fmt.Errorf("unable to confirm versioning on destination bucket %q for marker-only delete: %w", toBucket, vErr)
		}
		if vCfg.Status != "Enabled" {
			return fmt.Errorf("marker-only delete requires enabled versioning on destination bucket %q (status %q)", toBucket, vCfg.Status)
		}

		current, _ := toClient.S3().StatObject(ctx, toBucket, p.Object.Name, mclient.StatObjectOptions{})
		if current.IsDeleteMarker {
			s.recordDeleteAudit(ctx, p, "marker_created", map[string]any{"reason": "delete marker already present"})
			return nil
		}

		err = toClient.S3().RemoveObject(ctx, toBucket, p.Object.Name, mclient.RemoveObjectOptions{})
		if err != nil {
			return err
		}
		s.recordDeleteAudit(ctx, p, "marker_created", nil)
		return nil
	}

	// mode == "sync"
	err = toClient.S3().RemoveObject(ctx, toBucket, p.Object.Name, mclient.RemoveObjectOptions{VersionID: p.Object.Version})
	if err != nil {
		return err
	}
	s.recordDeleteAudit(ctx, p, "deleted", nil)
	return nil
}
