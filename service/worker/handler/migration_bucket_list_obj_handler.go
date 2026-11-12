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
	"strings"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	mclient "github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/log"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/systemsettings"
	"github.com/hieunyit/tutti/pkg/tasks"
)

var (
	queueBackpressureState sync.Map // string -> bool
)

// redisClient returns the Redis client used only to publish the queue high watermark that the broker enforces.
func (s *svc) redisClient() redis.Cmdable {
	if c, ok := s.queueSvc.(interface{ RedisClient() redis.Cmdable }); ok && c != nil {
		if rc := c.RedisClient(); rc != nil {
			return rc
		}
	}
	if s.listStateStore != nil {
		return s.listStateStore.Client()
	}
	return nil
}

func isQueueBackpressured(queue string) bool {
	v, ok := queueBackpressureState.Load(queue)
	return ok && v.(bool)
}

func setQueueBackpressured(queue string, bp bool) {
	if bp {
		queueBackpressureState.Store(queue, true)
	} else {
		queueBackpressureState.Delete(queue)
	}
}

// ResetQueueAdmissionState clears global admission locks and backpressure state (primarily for tests)
func ResetQueueAdmissionState() {
	queueBackpressureState = sync.Map{}
}

const (
	// MaxListingPageSize is the maximum number of objects discovered and enqueued in one listing execution.
	// This bounds the memory, execution time, and Redis task injection per slice.
	MaxListingPageSize = 1000

	// BackpressureDeferInterval is the delay before the next continuation task runs when backlog is at or above high watermark.
	// By yielding the worker slot, copy workers can drain the queue without being blocked by sleeping listing workers.
	BackpressureDeferInterval = 3 * time.Second
)

func (s *svc) HandleMigrationBucketListObj(ctx context.Context, t *asynq.Task) error {
	var p tasks.MigrateBucketListObjectsPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("HandleMigrationBucketListObj Unmarshal failed: %w: %w", err, asynq.SkipRetry)
	}
	ctx = log.WithBucket(ctx, p.Bucket)
	ctx = tasks.WithStrictWatermark(ctx)
	logger := zerolog.Ctx(ctx)

	// Acquire rate limits for source storage before proceeding
	if err := s.rateLimit(ctx, p.ID.FromStorage(), s3.ListObjects); err != nil {
		logger.Debug().Err(err).Str(log.Storage, p.ID.FromStorage()).Msg("rate limit error")
		return err
	}

	replicationID := p.GetReplicationID()

	if err := s.limit.StorReq(ctx, p.ID.FromStorage()); err != nil {
		logger.Debug().Err(err).Str(log.Storage, p.ID.FromStorage()).Msg("rate limit error")
		return err
	}

	// Dynamic Backpressure check:
	// Verify total unprocessed tasks (pending + active + retry + scheduled) in the copy queue.
	currSettings := systemsettings.Current()
	highWatermark := currSettings.Worker.BacklogHighWatermark
	if highWatermark <= 0 {
		highWatermark = 10000
	}
	lowWatermark := currSettings.Worker.BacklogLowWatermark
	if lowWatermark <= 0 || lowWatermark >= highWatermark {
		lowWatermark = highWatermark / 2
	}

	copyQueue := tasks.InitMigrationCopyQueue(p.ID)
	unprocessed, err := s.queueSvc.UnprocessedCount(ctx, true, copyQueue)
	if err != nil {
		logger.Warn().Err(err).Msg("migration bucket list obj: unable to fetch queue stats, failing closed")
		return fmt.Errorf("queue stats unavailable: %w", err)
	}

	// Publish the watermark so the broker enforces admission atomically inside EnqueueTask (asynq.ErrQueueFull).
	if rc := s.redisClient(); rc != nil {
		if err := rc.Set(ctx, fmt.Sprintf("asynq:{%s}:high_watermark", copyQueue), highWatermark, 0).Err(); err != nil {
			return fmt.Errorf("migration bucket list obj: unable to publish queue high watermark: %w", err)
		}
	}

	backpressured := p.Backpressured || isQueueBackpressured(copyQueue)

	if backpressured {
		if unprocessed > lowWatermark {
			setQueueBackpressured(copyQueue, true)
			logger.Info().
				Int("unprocessed_tasks", unprocessed).
				Int("low_watermark", lowWatermark).
				Dur("defer_interval", BackpressureDeferInterval).
				Msg("migration bucket list obj: backlog still above low watermark after backpressure, yielding worker slot")

			nextTask := p
			nextTask.Iteration = p.Iteration + 1
			nextTask.Backpressured = true
			if enqueueErr := s.queueSvc.EnqueueTask(ctx, nextTask, asynq.ProcessIn(BackpressureDeferInterval)); enqueueErr != nil {
				return fmt.Errorf("unable to defer continuation listing task: %w", enqueueErr)
			}
			return nil
		}
		// Drained to or below low watermark: clear backpressure
		setQueueBackpressured(copyQueue, false)
		p.Backpressured = false
	} else if unprocessed >= highWatermark {
		// Backlog is high: yield this worker slot immediately and schedule continuation task with a delay.
		setQueueBackpressured(copyQueue, true)
		logger.Info().
			Int("unprocessed_tasks", unprocessed).
			Int("high_watermark", highWatermark).
			Dur("defer_interval", BackpressureDeferInterval).
			Msg("migration bucket list obj: backlog high watermark reached, yielding worker slot and deferring continuation")

		nextTask := p
		nextTask.Iteration = p.Iteration + 1
		nextTask.Backpressured = true
		if enqueueErr := s.queueSvc.EnqueueTask(ctx, nextTask, asynq.ProcessIn(BackpressureDeferInterval)); enqueueErr != nil {
			return fmt.Errorf("unable to defer continuation listing task: %w", enqueueErr)
		}
		return nil
	}

	maxItems := MaxListingPageSize
	remainingCapacity := highWatermark - unprocessed
	if remainingCapacity < maxItems {
		maxItems = remainingCapacity
	}
	if maxItems <= 0 {
		setQueueBackpressured(copyQueue, true)
		nextTask := p
		nextTask.Iteration = p.Iteration + 1
		nextTask.Backpressured = true
		if enqueueErr := s.queueSvc.EnqueueTask(ctx, nextTask, asynq.ProcessIn(BackpressureDeferInterval)); enqueueErr != nil {
			return fmt.Errorf("unable to defer continuation listing task: %w", enqueueErr)
		}
		return nil
	}

	fromClient, err := s.clients.AsS3(ctx, p.ID.FromStorage(), p.ID.User())
	if err != nil {
		return fmt.Errorf("migration bucket list obj: unable to get %q s3 client: %w: %w", p.ID.FromStorage(), err, asynq.SkipRetry)
	}

	migrationID := entity.NewMigrationObjectIDFromUniversalReplicationID(p.ID, p.Bucket, p.Prefix)
	lastObjName, err := s.listStateStore.Get(ctx, migrationID)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return fmt.Errorf("unable to get last listed object: %w", err)
	}

	// Create cancellable context for listing so breaking early stops MinIO SDK background stream immediately
	listCtx, cancelList := context.WithCancel(ctx)
	defer cancelList()

	objects := fromClient.S3().ListObjects(listCtx, p.Bucket, mclient.ListObjectsOptions{
		StartAfter: lastObjName,
		Prefix:     p.Prefix,
		Recursive:  true,
	})

	count := 0
	hitLimit := false

	for object := range objects {
		if object.Err != nil {
			return fmt.Errorf("migration bucket list obj: list objects error %w", object.Err)
		}
		isDir := object.Size == 0 && strings.HasSuffix(object.Key, "/")
		logger.Debug().Str(log.Object, object.Key).Str("obj_version_id", object.VersionID).Bool("is_dir", isDir).Msg("migration bucket list obj: start processing object from the list")

		var enqueueErr error
		if p.Versioned && !isDir {
			// The list-versions task lands on the list queue but fans out into copy tasks: admit it against the copy queue.
			if unprocessed, uErr := s.queueSvc.UnprocessedCount(ctx, true, copyQueue); uErr != nil {
				cancelList()
				return fmt.Errorf("migration bucket list obj: unable to fetch queue stats: %w", uErr)
			} else if unprocessed >= highWatermark {
				setQueueBackpressured(copyQueue, true)
				hitLimit = true
				cancelList()
				break
			}
			task := tasks.ListObjectVersionsPayload{
				Bucket: p.Bucket,
				Prefix: object.Key,
			}
			task.SetReplicationID(replicationID, p.Generation)
			enqueueErr = s.queueSvc.EnqueueTask(ctx, task)
		} else {
			task := tasks.MigrateObjCopyPayload{
				Bucket: p.Bucket,
				Obj: tasks.ObjPayload{
					Name:        object.Key,
					VersionID:   object.VersionID,
					ETag:        object.ETag,
					Size:        object.Size,
					ContentType: object.ContentType,
				},
			}
			task.SetReplicationID(replicationID, p.Generation)
			enqueueErr = s.queueSvc.EnqueueTask(ctx, task)
		}
		if enqueueErr != nil {
			var rl *dom.ErrRateLimitExceeded
			if errors.As(enqueueErr, &rl) {
				// broker admission rejected the task: queue is full, stop this page and defer
				setQueueBackpressured(copyQueue, true)
				hitLimit = true
				cancelList()
				break
			}
			return fmt.Errorf("migration bucket list obj: unable to enqueue task for %q: %w", object.Key, enqueueErr)
		}

		lastObjName = object.Key
		count++

		// If page or capacity limit reached, stop listing and break
		if count >= maxItems {
			hitLimit = true
			cancelList()
			break
		}
	}

	// Update checkpoint in state store if any objects were processed
	if lastObjName != "" {
		if err = s.listStateStore.Set(ctx, migrationID, lastObjName); err != nil {
			return fmt.Errorf("migration bucket list obj: unable to update last obj meta: %w", err)
		}
	}

	// If page or capacity limit reached, there are more objects to list: schedule next page continuation
	if hitLimit {
		nextTask := p
		nextTask.Iteration = p.Iteration + 1

		// Check queue backlog again to decide immediate enqueue or deferred backpressure enqueue
		unprocessedAfter, err := s.queueSvc.UnprocessedCount(ctx, true, copyQueue)
		if err != nil || unprocessedAfter >= highWatermark || isQueueBackpressured(copyQueue) {
			setQueueBackpressured(copyQueue, true)
			nextTask.Backpressured = true
			logger.Info().
				Int("page_items", count).
				Int("unprocessed_tasks", unprocessedAfter).
				Int("high_watermark", highWatermark).
				Msg("migration bucket list obj: limit reached with queue backlog, deferring next page")
			if enqueueErr := s.queueSvc.EnqueueTask(ctx, nextTask, asynq.ProcessIn(BackpressureDeferInterval)); enqueueErr != nil {
				return fmt.Errorf("unable to enqueue deferred continuation task: %w", enqueueErr)
			}
		} else {
			setQueueBackpressured(copyQueue, false)
			nextTask.Backpressured = false
			logger.Debug().
				Int("page_items", count).
				Int("unprocessed_tasks", unprocessedAfter).
				Msg("migration bucket list obj: page limit reached, enqueuing next page continuation immediately")
			if enqueueErr := s.queueSvc.EnqueueTask(ctx, nextTask); enqueueErr != nil {
				return fmt.Errorf("unable to enqueue continuation task: %w", enqueueErr)
			}
		}
		return nil
	}

	// Listing fully finished (fewer than MaxListingPageSize objects in final batch)
	_, _ = s.listStateStore.Drop(ctx, migrationID)
	logger.Info().Int("final_page_items", count).Msg("migration bucket list obj: done (all objects listed)")
	return nil
}
