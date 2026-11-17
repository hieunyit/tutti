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

package replication

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/controlplane"
	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/filter"
	"github.com/hieunyit/tutti/pkg/meta"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/tasks"
)

type Service interface {
	Replicate(ctx context.Context, routedTo string, task tasks.ReplicationTask) error
}

func NewS3(queueSvc tasks.QueueService, versionSvc meta.VersionService, policySvc policy.Service) Service {
	return &s3Svc{
		queueSvc:   queueSvc,
		versionSvc: versionSvc,
		policySvc:  policySvc,
	}
}

type s3Svc struct {
	queueSvc   tasks.QueueService
	versionSvc meta.VersionService
	policySvc  policy.Service
}

func (s *s3Svc) Replicate(ctx context.Context, routedTo string, task tasks.ReplicationTask) error {
	if task == nil {
		zerolog.Ctx(ctx).Info().Msg("replication task is nil")
		return nil
	}
	zerolog.Ctx(ctx).Debug().Msg("creating replication task")

	// 1. find repl rule(-s)
	replications, skipTasks, err := s.getDestinations(ctx, routedTo)
	if err != nil {
		if errors.Is(err, dom.ErrPolicy) {
			zerolog.Ctx(ctx).Info().Err(err).Msg("skip Replicate: replication is not configured")
			return nil
		}
		return err
	}
	if len(replications) == 0 {
		zerolog.Ctx(ctx).Info().Msg("skip Replicate: replication is not configured")
		return nil
	}
	destination := meta.Destination{
		Storage: routedTo,
		Bucket:  xctx.GetBucket(ctx),
	}
	for _, replID := range replications {
		if objTask, ok := task.(*tasks.ObjectSyncPayload); ok {
			if err := s.replicateObject(ctx, replID, destination, *objTask, skipTasks); err != nil {
				return err
			}
			continue
		}

		// 2. increment versions in source storage
		switch t := task.(type) {
		case *tasks.BucketCreatePayload:
		// no version increment needed
		case *tasks.BucketDeletePayload:
			// During zero-downtime switch use Increment instead of DeleteAll
			// to keep version non-empty so old events see From <= To and skip.
			if skipTasks {
				_, err = s.versionSvc.IncrementBucket(ctx, replID, t.Bucket, destination)
			} else {
				err = s.versionSvc.DeleteBucketAll(ctx, replID, t.Bucket)
			}
			if err != nil {
				return err
			}
		case *tasks.BucketSyncACLPayload:
			_, err = s.versionSvc.IncrementBucketACL(ctx, replID, t.Bucket, destination)
			if err != nil {
				return err
			}
		case *tasks.BucketSyncTagsPayload:
			_, err = s.versionSvc.IncrementBucketTags(ctx, replID, t.Bucket, destination)
			if err != nil {
				return err
			}
		case *tasks.ObjSyncACLPayload:
			_, err = s.versionSvc.IncrementACL(ctx, replID, t.Object, destination)
			if err != nil {
				return err
			}
		case *tasks.ObjSyncTagsPayload:
			_, err = s.versionSvc.IncrementTags(ctx, replID, t.Object, destination)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: unsupported replication task type %T", dom.ErrInternal, task)
		}
		// 3. fan out tasks for each destination
		if !skipTasks {
			task.SetReplicationID(replID, xctx.GetReplicationGeneration(ctx, replID))
			err := s.queueSvc.EnqueueTask(ctx, task)
			if err != nil {
				return fmt.Errorf("unable to fan out replication task to %+v: %w", replID, err)
			}
		}
	}

	return nil
}

func (s *s3Svc) replicateObject(ctx context.Context, id entity.UniversalReplicationID, dest meta.Destination, task tasks.ObjectSyncPayload, skipTasks bool) error {
	if skipTasks {
		// Keep versions non-empty during a zero-downtime switch, even for a delete.
		_, err := s.versionSvc.IncrementObj(ctx, id, task.Object, dest)
		return err
	}
	var delay time.Duration
	if s.policySvc != nil {
		ext, err := s.policySvc.GetReplicationPolicyInfoExtended(ctx, id)
		if err != nil {
			return fmt.Errorf("resolve replication policy: %w", err)
		}
		if st := ext.ReplicationStatus; st != nil {
			delay = time.Duration(st.DeleteDelaySeconds) * time.Second
			f := filter.New(st.IncludePatterns, st.ExcludePatterns)
			if !f.ShouldSync(task.Object.Name) {
				zerolog.Ctx(ctx).Debug().Str("key", task.Object.Name).Msg("skip replication: filtered out by pattern")
				return nil
			}
		}
	}
	generation := xctx.GetReplicationGeneration(ctx, id)
	task.SetReplicationID(id, generation)
	if !task.Deleted {
		if _, err := s.versionSvc.IncrementObj(ctx, id, task.Object, dest); err != nil {
			return err
		}
		return s.queueSvc.EnqueueTask(ctx, &task)
	}

	sourceTime := time.Now()
	deadline, err := s.versionSvc.DeleteObjWithDeadline(ctx, id, task.Object, sourceTime.Add(delay))
	if err != nil {
		return err
	}
	var events controlplane.DeleteEvents
	if s.policySvc != nil {
		if cp := s.policySvc.ControlPlane(); cp != nil {
			events = cp.Repos().DeleteEvents
		}
	}
	if events != nil {
		task.DeleteEventID, err = events.Create(ctx, controlplane.DeleteEvent{
			ReplicationID:   id.AsString(),
			Generation:      generation,
			Bucket:          task.Object.Bucket,
			ObjectKey:       task.Object.Name,
			VersionID:       task.Object.Version,
			DeletionType:    "hard",
			SourceTimestamp: sourceTime,
			Status:          "pending",
		})
		if err != nil {
			return fmt.Errorf("create delete event: %w", err)
		}
	}
	var opts []asynq.Option
	if time.Now().Before(deadline) {
		opts = append(opts, asynq.ProcessAt(deadline.Truncate(time.Second).Add(time.Second)))
	}
	if err := s.queueSvc.EnqueueTask(ctx, &task, opts...); err != nil {
		if events != nil {
			auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_, bucket := id.FromToBuckets(task.Object.Bucket)
			auditErr := events.RecordAudit(auditCtx, controlplane.DeleteAuditEntry{
				DeleteEventID:      task.DeleteEventID,
				ReplicationID:      id.AsString(),
				Bucket:             bucket,
				ObjectKey:          task.Object.Name,
				DestinationStorage: id.ToStorage(),
				Action:             "failed",
				Metadata:           map[string]any{"error": err.Error()},
			})
			if auditErr != nil {
				zerolog.Ctx(ctx).Error().Err(auditErr).Int64("delete_event_id", task.DeleteEventID).Msg("record delete enqueue failure")
			}
		}
		return fmt.Errorf("unable to fan out replication task to %+v: %w", id, err)
	}
	return nil
}

// getDestinations returns replication targets for the current request.
// skipTasks=true means: increment versions but do not enqueue replication tasks.
// This is used during zero-downtime switch to prevent old pre-switch events from
// overwriting post-switch data.
func (s *s3Svc) getDestinations(ctx context.Context, routedTo string) (replicateTo []entity.UniversalReplicationID, skipTasks bool, err error) {
	destinations := xctx.GetReplications(ctx)
	if len(destinations) != 0 {
		return destinations, false, nil
	}

	inProgressZeroDowntimeSwitch := xctx.GetInProgressZeroDowntime(ctx)
	if inProgressZeroDowntimeSwitch == nil {
		return nil, false, nil
	}
	originalReplID := inProgressZeroDowntimeSwitch.ReplicationID()

	// zero-downtime switch is in progress
	// check if we need to replicate dangling multipart upload to old main storage
	wasRoutedToOldStorage := routedTo == originalReplID.FromStorage()

	isCompleteMutlipart := xctx.GetMethod(ctx) == s3.CompleteMultipartUpload
	if wasRoutedToOldStorage && isCompleteMutlipart {
		// BUGFIX:
		// 1. A is main, B is follower
		// 2. User initiates multipart upload to A
		// 3. User uploads some parts to A
		// 4. User initiates zero-downtime switch from A to B
		//     - reads goes to B or to A if object is not yet replicated
		//     - non-multipart writes now go to B
		//     - replication policy is archived: no copy from A to B
		//
		// 5. All writes except multipart now correctly routed to B
		// 5. We dont create replication tasks for new writes
		// 7. We handle UploadPart correctly and route to A
		//    For UploadPart tutti never creates replication tasks because user can call AbortMultipartUpload
		// 8. User calls CompleteMultipartUpload
		//    -- we are here on this line of code --
		// PROBLEM:
		// - because replication policy is archived we dont create replication tasks
		// - but we need to finish old multipart upload on A and replicate completed object to B
		return []entity.UniversalReplicationID{originalReplID}, false, nil
	}

	// zero-downtime switch in progress.
	// increment version in routed storage and skip creating replication tasks
	return []entity.UniversalReplicationID{originalReplID}, true, nil
}
