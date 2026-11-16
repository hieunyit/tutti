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

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/log"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/systemsettings"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/util"
	"github.com/hieunyit/tutti/service/worker/copy"
)

// retry delay of a large copy that found no free stream slot
const (
	largeObjectRetryMin = 3 * time.Second
	largeObjectRetryMax = 10 * time.Second
)

func (s *svc) HandleMigrationObjCopy(ctx context.Context, t *asynq.Task) error {
	err := s.handleMigrationObjCopy(ctx, t)
	if err == nil && t.Type() == tasks.TypeMigrateObjCopy {
		var p tasks.MigrateObjCopyPayload
		if json.Unmarshal(t.Payload(), &p) == nil {
			s.addInitDoneBytes(ctx, p.ID, p.Generation, p.Obj.Name, p.Obj.VersionID, p.Obj.Size)
		}
	}
	return err
}

func (s *svc) handleMigrationObjCopy(ctx context.Context, t *asynq.Task) (err error) {
	var p tasks.MigrateObjCopyPayload
	if err = json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("MigrateObjCopyPayload Unmarshal failed: %w: %w", err, asynq.SkipRetry)
	}
	ctx = log.WithBucket(ctx, p.Bucket)
	ctx = log.WithObjName(ctx, p.Obj.Name)
	ctx = log.WithUser(ctx, p.ID.User())
	logger := zerolog.Ctx(ctx)
	fromBucket, toBucket := p.ID.FromToBuckets(p.Bucket)

	// Large objects share a per-destination-storage stream cap. The slot is taken before anything else (no lock
	// or rate-limit token is held while waiting) and never blocks: when all slots are busy the task goes back to
	// the retry set with a short delay (not counted as a failure), so no worker slot is parked and live events
	// keep their share. Small objects are never delayed. See docs/tutti/configuration.md.
	if p.Obj.Size >= systemsettings.LargeObjectThreshold {
		release, ok := systemsettings.TryAcquireLargeObject(p.ID.ToStorage(), p.Obj.Size)
		if !ok {
			logger.Debug().Str(log.Storage, p.ID.ToStorage()).Int64("size", p.Obj.Size).Msg("large object stream cap reached: retry later")
			return &dom.ErrRateLimitExceeded{RetryIn: util.DurationJitter(largeObjectRetryMin, largeObjectRetryMax)}
		}
		defer release()
	}

	// acquire rate limits for source and destination storage before proceeding
	if storage, err := s.rateLimitSides(ctx,
		p.ID.FromStorage(), []s3.Method{s3.HeadObject, s3.GetObject, s3.GetObjectAcl},
		p.ID.ToStorage(), []s3.Method{s3.HeadObject, s3.PutObject, s3.PutObjectAcl}); err != nil {
		logger.Debug().Err(err).Str(log.Storage, storage).Msg("rate limit error")
		return err
	}

	isRescan := t.Type() == tasks.TypeRescanObjCopy
	flow := store.ActivityFlowInit
	if isRescan {
		flow = store.ActivityFlowRescan
	}
	act := s.activityBegin(ctx, p.ID, p.Generation, p.Obj.Name, p.Obj.VersionID, p.Obj.Size, flow)
	var outcome copy.Outcome
	if act != nil {
		if pre := copy.OutcomeFrom(ctx); pre != nil {
			// the caller (rescan accounting) already watches the outcome: share it
			ctx = copy.WithOutcome(ctx, pre)
			defer func() { act.finish(ctx, err, pre) }()
		} else {
			ctx = copy.WithOutcome(ctx, &outcome)
			defer func() { act.finish(ctx, err, &outcome) }()
		}
	}

	if t.Type() == tasks.TypeMigrateObjCopy {
		// bytes of the running copy feed the initial replication progress; HandleMigrationObjCopy settles them
		var prog *initProgress
		ctx, prog = s.startInitProgress(ctx, p.ID, p.Generation, p.Obj.Name, p.Obj.VersionID, p.Obj.Size)
		defer func() {
			prog.stop()
			if err != nil {
				prog.clear(ctx)
			}
		}()
	}

	domObj := dom.Object{
		Bucket:  p.Bucket,
		Name:    p.Obj.Name,
		Version: p.Obj.VersionID,
	}

	objectLockID := entity.NewVersionedObjectLockID(p.ID.ToStorage(), toBucket, p.Obj.Name, p.Obj.VersionID)
	lock, err := s.objectLocker.Lock(ctx, objectLockID)
	if err != nil {
		return err
	}
	defer lock.Release(ctx)
	versions, err := s.versionSvc.GetObj(ctx, p.ID, domObj)
	if err != nil {
		return fmt.Errorf("migration obj copy: unable to get obj meta: %w", err)
	}
	fromVer, toVer := versions.From, versions.To

	// diff fix and rescan copies bypass the version check: objects written directly to the source never bump the
	// proxy version, and CopyObject itself skips an object the destination already holds (ETag / source marker).
	isDiffFix := t.Type() == tasks.TypeDiffFixCopyS3
	if !isDiffFix && !isRescan && fromVer != 0 && fromVer <= toVer {
		logger.Info().Int("from_ver", fromVer).Int("to_ver", toVer).Msg("migration obj copy: identical from/to obj version: skip copy")
		return nil
	}
	if isDiffFix {
		logger.Info().Int("from_ver", fromVer).Int("to_ver", toVer).Msg("diff fix copy: proceeding with copy despite version check")
	}
	// 1. sync obj meta and content
	err = lock.DoCtx(ctx, time.Second*2, func(ctx context.Context) error {
		ctx = copy.WithKeyLocked(ctx)
		if systemsettings.EffectiveResumeMultipart() {
			if rc := s.redisClient(); rc != nil {
				ctx = copy.WithResume(ctx, store.NewMultipartResumeStore(rc), copy.ResumeKey(p.ID.AsString(), p.Generation, p.Obj.Name, p.Obj.VersionID))
			}
		}
		return s.copySvc.CopyObject(ctx, p.ID.User(), copy.File{
			Storage: p.ID.FromStorage(),
			Bucket:  fromBucket,
			Name:    p.Obj.Name,
		}, copy.File{
			Storage: p.ID.ToStorage(),
			Bucket:  toBucket,
			Name:    p.Obj.Name,
		})
	})
	if err != nil {
		if errors.Is(err, dom.ErrNotFound) {
			logger.Warn().Msg("migration obj copy: skip object sync: object missing in source")
			return nil
		}
		return fmt.Errorf("migration obj copy: unable to copy object: %w", err)
	}

	if fromVer != 0 {
		destination := meta.Destination{Storage: p.ID.ToStorage(), Bucket: toBucket}
		err = s.versionSvc.UpdateIfGreater(ctx, p.ID, domObj, destination, fromVer)
		if err != nil {
			return fmt.Errorf("migration obj copy: unable to update obj meta: %w", err)
		}
	}
	if isDiffFix {
		logger.Info().Msg("diff fix copy: object copied successfully")
	} else {
		logger.Info().Msg("migration obj copy: done")
	}

	return nil
}
