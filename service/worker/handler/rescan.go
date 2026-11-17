// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	mclient "github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/features"
	"github.com/hieunyit/tutti/pkg/log"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/systemsettings"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/service/worker/copy"
)

// Periodic rescan of a replication source (docs/tutti/operations.md). A scan walks the source bucket and enqueues
// copy tasks for the objects last modified at or after the scan cutoff. It has its own task types, queues, counters
// and activity flow: it never touches the initial copy generation, list checkpoints, init byte counters or the
// queue statistics that define whether the initial copy is done. Objects the destination already holds are skipped by
// CopyObject (ETag / source marker), so a scan is idempotent and over-scanning is safe.

func (s *svc) rescanStore() *store.RescanStore {
	if rc := s.redisClient(); rc != nil {
		return store.NewRescanStore(rc)
	}
	return nil
}

func cutoffPtr(run tasks.RescanRun) *time.Time {
	if run.Cutoff.IsZero() {
		return nil
	}
	c := run.Cutoff
	return &c
}

// finalAttempt reports whether the failed attempt err was the last one of the task (no retry follows).
func finalAttempt(ctx context.Context, err error) bool {
	if errors.Is(err, asynq.SkipRetry) {
		return true
	}
	retried, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)
	return retried >= maxRetry
}

// rescanSettled folds the result of a store call into logs and metrics.
func rescanSettled(ctx context.Context, id entity.UniversalReplicationID, runID string, res store.ScanResult, rs *store.RescanStore, gen int64) {
	if res != store.ScanCompleted && res != store.ScanCompletedWithFailures {
		return
	}
	st, err := rs.Get(ctx, id.AsString(), gen, time.Now())
	if err != nil {
		return
	}
	metrics.RescanFinished(res == store.ScanCompleted, st.LastObjects, st.LastEnd.Sub(st.LastStart))
	ev := zerolog.Ctx(ctx).Info().Str("replication", id.AsString()).Str("run", runID).Int64("copied", st.LastObjects).
		Dur("duration", st.LastEnd.Sub(st.LastStart)).Time("cutoff", st.RunCutoff)
	if res == store.ScanCompletedWithFailures {
		ev.Str("error", st.LastError)
	}
	ev.Msg("rescan: finished")
}

// rescanAbort ends the scan after a listing failure that exhausted its retries.
func (s *svc) rescanAbort(ctx context.Context, rs *store.RescanStore, id entity.UniversalReplicationID, gen int64, runID string, err error) {
	zerolog.Ctx(ctx).Warn().Err(err).Str("replication", id.AsString()).Str("run", runID).Msg("rescan: listing failed, the watermark is not advanced")
	msg := "listing the source failed: " + err.Error()
	if len(msg) > 500 {
		msg = msg[:500]
	}
	if aerr := rs.Abort(context.WithoutCancel(ctx), id.AsString(), gen, runID, msg, time.Now()); aerr == nil {
		metrics.RescanAborted()
	}
}

// rescanCurrent reports whether the task belongs to the scan the replication currently runs. Tasks of a finished,
// superseded or deleted scan are dropped.
func (s *svc) rescanCurrent(ctx context.Context, rs *store.RescanStore, id entity.UniversalReplicationID, gen int64, runID string) (bool, error) {
	st, err := rs.Get(ctx, id.AsString(), gen, time.Now())
	if err != nil {
		return false, err
	}
	return st.RunID == runID, nil
}

// HandleRescanUserListBuckets starts one object listing per source bucket of a user-level replication.
func (s *svc) HandleRescanUserListBuckets(ctx context.Context, t *asynq.Task) (err error) {
	var p tasks.RescanUserListBucketsPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("RescanUserListBucketsPayload Unmarshal failed: %w: %w", err, asynq.SkipRetry)
	}
	rs := s.rescanStore()
	if rs == nil {
		return fmt.Errorf("rescan: redis is not available")
	}
	defer func() {
		if err != nil && finalAttempt(ctx, err) {
			s.rescanAbort(ctx, rs, p.ID, p.Generation, p.RunID, err)
		}
	}()
	if ok, cerr := s.rescanCurrent(ctx, rs, p.ID, p.Generation, p.RunID); cerr != nil || !ok {
		return cerr
	}
	if err = s.rateLimit(ctx, p.ID.FromStorage(), s3.ListBuckets); err != nil {
		return err
	}
	fromClient, err := s.clients.AsS3(ctx, p.ID.FromStorage(), p.ID.User())
	if err != nil {
		return fmt.Errorf("rescan: unable to get %q s3 client: %w", p.ID.FromStorage(), err)
	}
	buckets, err := fromClient.S3().ListBuckets(ctx)
	if err != nil {
		return fmt.Errorf("error listing buckets: %w", err)
	}
	names := make([]string, 0, len(buckets))
	for _, b := range buckets {
		names = append(names, b.Name)
	}
	names, err = s.scope.Filter(ctx, p.ID.FromStorage(), p.ID.User(), names)
	if err != nil {
		return fmt.Errorf("unable to apply access policy to bucket list: %w", err)
	}
	// register the listings before starting them, then close this task's own listing
	if _, err = rs.OpenLists(ctx, p.ID.AsString(), p.Generation, p.RunID, int64(len(names)), time.Now()); err != nil {
		return err
	}
	for _, bucket := range names {
		task := tasks.RescanListObjectsPayload{Bucket: bucket, RescanRun: p.RescanRun}
		task.SetReplicationID(p.ID, p.Generation)
		if err = s.queueSvc.EnqueueTask(ctx, task); err != nil {
			return err
		}
	}
	res, err := rs.Listed(ctx, p.ID.AsString(), p.Generation, p.RunID, 0, true, "user", time.Now())
	if err == nil {
		rescanSettled(ctx, p.ID, p.RunID, res, rs, p.Generation)
	}
	return err
}

// HandleRescanListObjects lists one page of the source bucket and enqueues the copy of every object at or after the
// scan cutoff. The next page is a new task carrying the continuation key.
func (s *svc) HandleRescanListObjects(ctx context.Context, t *asynq.Task) (err error) {
	var p tasks.RescanListObjectsPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("RescanListObjectsPayload Unmarshal failed: %w: %w", err, asynq.SkipRetry)
	}
	ctx = log.WithBucket(ctx, p.Bucket)
	logger := zerolog.Ctx(ctx)
	rs := s.rescanStore()
	if rs == nil {
		return fmt.Errorf("rescan: redis is not available")
	}
	defer func() {
		if err != nil && finalAttempt(ctx, err) {
			s.rescanAbort(ctx, rs, p.ID, p.Generation, p.RunID, err)
		}
	}()
	if ok, cerr := s.rescanCurrent(ctx, rs, p.ID, p.Generation, p.RunID); cerr != nil || !ok {
		return cerr
	}
	if err = s.rateLimit(ctx, p.ID.FromStorage(), s3.ListObjects, s3.GetBucketVersioning); err != nil {
		return err
	}
	if err = s.limit.StorReq(ctx, p.ID.FromStorage()); err != nil {
		return err
	}

	settings := systemsettings.Current()
	highWatermark := settings.Worker.BacklogHighWatermark
	if highWatermark <= 0 {
		highWatermark = 10000
	}
	copyQueue := tasks.RescanCopyQueue(p.ID)
	unprocessed, err := s.queueSvc.UnprocessedCount(ctx, true, copyQueue)
	if err != nil {
		return fmt.Errorf("rescan: queue stats unavailable: %w", err)
	}
	if rc := s.redisClient(); rc != nil {
		if err = rc.Set(ctx, fmt.Sprintf("asynq:{%s}:high_watermark", copyQueue), highWatermark, 0).Err(); err != nil {
			return fmt.Errorf("rescan: unable to publish queue high watermark: %w", err)
		}
	}
	maxItems := min(MaxListingPageSize, highWatermark-unprocessed)
	if maxItems <= 0 {
		// the copy queue is full: yield this worker slot and come back later
		next := p
		next.Iteration++
		next.Backpressured = true
		return s.queueSvc.EnqueueTask(ctx, next, asynq.ProcessIn(BackpressureDeferInterval))
	}

	fromClient, err := s.clients.AsS3(ctx, p.ID.FromStorage(), p.ID.User())
	if err != nil {
		return fmt.Errorf("rescan: unable to get %q s3 client: %w", p.ID.FromStorage(), err)
	}
	fromBucket, _ := p.ID.FromToBuckets(p.Bucket)
	versioning, err := fromClient.S3().GetBucketVersioning(ctx, fromBucket)
	if err != nil {
		return fmt.Errorf("rescan: unable to get source bucket versioning config: %w", err)
	}
	versioned := versioning.Enabled() && features.Versioning(ctx)
	cutoff := cutoffPtr(p.RescanRun)
	replFilter, err := s.replicationFilter(ctx, p.ID)
	if err != nil {
		return err
	}

	listCtx, cancelList := context.WithCancel(ctx)
	defer cancelList()
	objects := fromClient.S3().ListObjects(listCtx, p.Bucket, mclient.ListObjectsOptions{
		StartAfter: p.StartAfter,
		Recursive:  true,
	})

	var (
		batch       []any
		batchKeys   []string
		count       int
		scanned     int
		lastKey     = p.StartAfter
		lastScanned = p.StartAfter
		hitLimit    bool
	)
	flush := func() (stop bool, err error) {
		if len(batch) == 0 {
			return false, nil
		}
		defer func() { batch, batchKeys = batch[:0], batchKeys[:0] }()
		n, enqueueErr := tasks.EnqueueMany(ctx, s.queueSvc, batch)
		if n > 0 {
			lastKey = batchKeys[n-1]
			if n == len(batch) && lastScanned > lastKey {
				lastKey = lastScanned
			}
			count += n
		}
		if enqueueErr != nil {
			var rl *dom.ErrRateLimitExceeded
			if errors.As(enqueueErr, &rl) {
				hitLimit = true
				return true, nil
			}
			return false, fmt.Errorf("rescan: unable to enqueue task: %w", enqueueErr)
		}
		if count >= maxItems {
			hitLimit = true
			return true, nil
		}
		return false, nil
	}

	stopped := false
	for object := range objects {
		if object.Err != nil {
			return fmt.Errorf("rescan: list objects error: %w", object.Err)
		}
		if scanned >= maxScannedPerPage {
			// a long stretch of skipped keys: end the slice here and continue from the last scanned key
			if stopped, err = flush(); err != nil {
				return err
			}
			if !stopped {
				lastKey = lastScanned
				hitLimit = true
				stopped = true
			}
			break
		}
		scanned++
		lastScanned = object.Key
		if replFilter != nil && !replFilter.ShouldSync(object.Key) {
			continue
		}
		if !keepSince(object.LastModified, cutoff) {
			continue
		}
		isDir := object.Size == 0 && strings.HasSuffix(object.Key, "/")
		if versioned && !isDir {
			task := tasks.RescanListVersionsPayload{
				ListObjectVersionsPayload: tasks.ListObjectVersionsPayload{Bucket: p.Bucket, Prefix: object.Key},
				RescanRun:                 p.RescanRun,
			}
			task.SetReplicationID(p.ID, p.Generation)
			batch = append(batch, task)
		} else {
			task := tasks.RescanObjCopyPayload{
				MigrateObjCopyPayload: tasks.MigrateObjCopyPayload{
					Bucket: p.Bucket,
					Obj: tasks.ObjPayload{
						Name: object.Key, VersionID: object.VersionID, ETag: object.ETag,
						Size: object.Size, ContentType: object.ContentType,
					},
				},
				RescanRun: p.RescanRun,
			}
			task.SetReplicationID(p.ID, p.Generation)
			batch = append(batch, task)
		}
		batchKeys = append(batchKeys, object.Key)
		if len(batch) >= listEnqueueBatchSize || count+len(batch) >= maxItems {
			if stopped, err = flush(); err != nil {
				return err
			}
			if stopped {
				break
			}
		}
	}
	cancelList()
	if !stopped {
		if _, err = flush(); err != nil {
			return err
		}
	}

	last := !hitLimit
	if hitLimit {
		next := p
		next.Iteration++
		next.StartAfter = lastKey
		next.Backpressured = false
		var opts []asynq.Option
		if after, uerr := s.queueSvc.UnprocessedCount(ctx, true, copyQueue); uerr != nil || after >= highWatermark {
			opts = append(opts, asynq.ProcessIn(BackpressureDeferInterval))
			next.Backpressured = true
		}
		if err = s.queueSvc.EnqueueTask(ctx, next, opts...); err != nil {
			return fmt.Errorf("rescan: unable to enqueue next page: %w", err)
		}
	}
	// counted last and once per page (token), so a retry of a finished page does not count twice
	res, err := rs.Listed(ctx, p.ID.AsString(), p.Generation, p.RunID, int64(count), last, fmt.Sprintf("%s:%d", p.Bucket, p.Iteration), time.Now())
	if err != nil {
		return err
	}
	logger.Debug().Str("run", p.RunID).Int("page_objects", count).Bool("last_page", last).Msg("rescan: page listed")
	rescanSettled(ctx, p.ID, p.RunID, res, rs, p.Generation)
	return nil
}

// settle records the outcome of a copy-stage task (object copy, or the versions copy of one object).
func (s *svc) rescanSettle(ctx context.Context, id entity.UniversalReplicationID, gen int64, runID string, err error, copied bool) {
	rs := s.rescanStore()
	if rs == nil {
		return
	}
	var res store.ScanResult
	var serr error
	switch {
	case err == nil:
		res, serr = rs.ObjectDone(ctx, id.AsString(), gen, runID, copied, time.Now())
		if copied {
			metrics.RescanObjectCopied()
		}
	case finalAttempt(ctx, err):
		res, serr = rs.ObjectFailed(ctx, id.AsString(), gen, runID, time.Now())
	default:
		return
	}
	if serr != nil {
		zerolog.Ctx(ctx).Debug().Err(serr).Msg("rescan: unable to record task result")
		return
	}
	rescanSettled(ctx, id, runID, res, rs, gen)
}

// HandleRescanObjCopy copies one object of a scan. The copy itself is the initial copy handler (same locks, rate
// limits, large object cap, bandwidth limit and skip of objects the destination holds).
func (s *svc) HandleRescanObjCopy(ctx context.Context, t *asynq.Task) error {
	var p tasks.RescanObjCopyPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("RescanObjCopyPayload Unmarshal failed: %w: %w", err, asynq.SkipRetry)
	}
	var out copy.Outcome
	err := s.handleMigrationObjCopy(copy.WithOutcome(ctx, &out), t)
	s.rescanSettle(ctx, p.ID, p.Generation, p.RunID, err, out.Copied > 0)
	return err
}

// HandleRescanListVersions snapshots the versions of one object (cutoff applied per version) and enqueues their copy.
func (s *svc) HandleRescanListVersions(ctx context.Context, t *asynq.Task) (err error) {
	var p tasks.RescanListVersionsPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("RescanListVersionsPayload Unmarshal failed: %w: %w", err, asynq.SkipRetry)
	}
	defer func() {
		if err != nil {
			s.rescanSettle(ctx, p.ID, p.Generation, p.RunID, err, false)
		}
	}()
	replFilter, err := s.replicationFilter(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("rescan list versions: unable to get replication filter: %w", err)
	}
	if !replFilter.ShouldSync(p.Prefix) {
		zerolog.Ctx(ctx).Debug().Str("key", p.Prefix).Msg("rescan list versions: skipped by filter")
		return nil
	}
	fromBucket, toBucket := p.ID.FromToBuckets(p.Bucket)
	objectVersionID := entity.NewVersionedObjectID(p.ID.FromStorage(), fromBucket, p.Prefix)
	replicationID := entity.NewBucketReplicationPolicy(p.ID.User(), p.ID.FromStorage(), fromBucket, p.ID.ToStorage(), toBucket)
	if err = s.versionedSvc.ListVersionsSince(ctx, objectVersionID, replicationID, cutoffPtr(p.RescanRun), p.Generation); err != nil {
		return fmt.Errorf("unable to list object versions: %w", err)
	}
	next := tasks.RescanVersionedObjectPayload{
		MigrateVersionedObjectPayload: tasks.MigrateVersionedObjectPayload(p.ListObjectVersionsPayload),
		RescanRun:                     p.RescanRun,
	}
	if err = s.queueSvc.EnqueueTask(ctx, next); err != nil {
		return fmt.Errorf("rescan: unable to enqueue versioned copy task: %w", err)
	}
	return nil
}

// HandleRescanVersionedObject copies the versions of one object that the destination does not hold yet.
func (s *svc) HandleRescanVersionedObject(ctx context.Context, t *asynq.Task) (err error) {
	var p tasks.RescanVersionedObjectPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("RescanVersionedObjectPayload Unmarshal failed: %w: %w", err, asynq.SkipRetry)
	}
	var out copy.Outcome
	defer func() { s.rescanSettle(ctx, p.ID, p.Generation, p.RunID, err, out.Copied > 0) }()
	replFilter, err := s.replicationFilter(ctx, p.ID)
	if err != nil {
		return fmt.Errorf("rescan versioned object: unable to get replication filter: %w", err)
	}
	if !replFilter.ShouldSync(p.Prefix) {
		zerolog.Ctx(ctx).Debug().Str("key", p.Prefix).Msg("rescan versioned object: skipped by filter")
		return nil
	}
	ctx = log.WithBucket(ctx, p.Bucket)
	ctx = log.WithUser(ctx, p.ID.User())
	logger := zerolog.Ctx(ctx)
	if err = s.limit.StorReq(ctx, p.ID.FromStorage()); err != nil {
		logger.Debug().Err(err).Str(log.Storage, p.ID.FromStorage()).Msg("rate limit error")
		return err
	}
	if err = s.limit.StorReq(ctx, p.ID.ToStorage()); err != nil {
		logger.Debug().Err(err).Str(log.Storage, p.ID.ToStorage()).Msg("rate limit error")
		return err
	}
	act := s.activityBegin(ctx, p.ID, p.Generation, p.Prefix, "", 0, store.ActivityFlowRescan)
	ctx = copy.WithOutcome(ctx, &out)
	if act != nil {
		defer func() { act.finish(ctx, err, &out) }()
	}
	fromBucket, toBucket := p.ID.FromToBuckets(p.Bucket)
	replicationID := entity.NewBucketReplicationPolicy(p.ID.User(), p.ID.FromStorage(), fromBucket, p.ID.ToStorage(), toBucket)
	if err = s.versionedSvc.MigrateVersionsSince(ctx, replicationID, p.Prefix, cutoffPtr(p.RescanRun), p.Generation); err != nil {
		err = fmt.Errorf("unable to migrate object version: %w", err)
		return err
	}
	return nil
}
