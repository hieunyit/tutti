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

package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
)

const (
	taskTimeout = time.Hour * 8 // overrides asynq default task timeout (30m)
)

// encoder contains metadata for task payload.
type encoder[T TaskPayload] struct {
	// generates unique task ID based on payload. Used for deduplication.
	// If nil, no task ID is set and no deduplication is performed.
	taskID func(p T) string
	// calculates queue name based on payload.
	queue func(p T) string
	// Uniq
	taskType string
	// Identifies that all eligible child tasks should inherit queue
	inheritableQueue bool
}

func (e encoder[T]) Encode(ctx context.Context, payload T) (*asynq.Task, error) {
	// Ensure that replication ID is set for replication tasks.
	queue := e.queue(payload)
	if replTask, ok := any(&payload).(ReplicationTask); ok {
		id := replTask.GetReplicationID()
		if id.IsEmpty() {
			return nil, fmt.Errorf("%w: replicationID is not set to replication task: %#v. Use task.SetReplicationID() method", dom.ErrInternal, payload)
		}
		inheritableQueue := replTask.GetInheritableQueue()
		if inheritableQueue != "" {
			queue = inheritableQueue
		} else if e.inheritableQueue {
			replTask.SetInheritableQueue(queue)
		}
	}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if trTask, ok := any(&payload).(TraceableTask); ok {
		trTask.SetTraceContext(carrier.Get("traceparent"), carrier.Get("tracestate"))
	}
	bytes, err := json.Marshal(&payload)
	if err != nil {
		return nil, err
	}
	if tp := carrier.Get("traceparent"); tp != "" {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(bytes, &raw); err == nil {
			if _, exists := raw["traceparent"]; !exists {
				raw["traceparent"], _ = json.Marshal(tp)
				if ts := carrier.Get("tracestate"); ts != "" {
					raw["tracestate"], _ = json.Marshal(ts)
				}
				if newBytes, err := json.Marshal(raw); err == nil {
					bytes = newBytes
				}
			}
		}
	}
	optionList := []asynq.Option{asynq.Queue(queue), asynq.Timeout(taskTimeout)}
	if e.taskID != nil {
		id := e.taskID(payload)
		if id == "" {
			return nil, fmt.Errorf("%w: task ID generator returned empty task ID", dom.ErrInternal)
		}
		optionList = append(optionList, asynq.TaskID(id))
	}
	return asynq.NewTask(e.taskType, bytes, optionList...), nil
}

func (e encoder[T]) Enqueue(ctx context.Context, taskClient *asynq.Client, payload T) error {
	task, err := e.Encode(ctx, payload)
	if err != nil {
		return err
	}
	_, err = taskClient.EnqueueContext(ctx, task)
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) && !errors.Is(err, asynq.ErrTaskIDConflict) {
		return err
	}
	return nil
}

func EncodeAny(ctx context.Context, payload any) (*asynq.Task, error) {
	if trTask, ok := payload.(TraceableTask); ok {
		carrier := propagation.MapCarrier{}
		otel.GetTextMapPropagator().Inject(ctx, carrier)
		trTask.SetTraceContext(carrier.Get("traceparent"), carrier.Get("tracestate"))
	}
	switch p := payload.(type) {
	case *BucketCreatePayload:
		return bucketCreate.Encode(ctx, *p)
	case BucketCreatePayload:
		return bucketCreate.Encode(ctx, p)
	case *BucketDeletePayload:
		return bucketDelete.Encode(ctx, *p)
	case BucketDeletePayload:
		return bucketDelete.Encode(ctx, p)
	case *BucketSyncTagsPayload:
		return bucketSyncTags.Encode(ctx, *p)
	case BucketSyncTagsPayload:
		return bucketSyncTags.Encode(ctx, p)
	case *BucketSyncACLPayload:
		return bucketSyncACL.Encode(ctx, *p)
	case BucketSyncACLPayload:
		return bucketSyncACL.Encode(ctx, p)
	case *ObjectSyncPayload:
		return objectSync.Encode(ctx, *p)
	case ObjectSyncPayload:
		return objectSync.Encode(ctx, p)
	case *ObjSyncTagsPayload:
		return objSyncTags.Encode(ctx, *p)
	case ObjSyncTagsPayload:
		return objSyncTags.Encode(ctx, p)
	case *ObjSyncACLPayload:
		return objSyncACL.Encode(ctx, *p)
	case ObjSyncACLPayload:
		return objSyncACL.Encode(ctx, p)
	case *MigrateS3UserPayload:
		return migrateS3User.Encode(ctx, *p)
	case MigrateS3UserPayload:
		return migrateS3User.Encode(ctx, p)
	case *MigrateBucketListObjectsPayload:
		return migrateBucketListObjects.Encode(ctx, *p)
	case MigrateBucketListObjectsPayload:
		return migrateBucketListObjects.Encode(ctx, p)
	case *MigrateObjCopyPayload:
		return migrateObjCopy.Encode(ctx, *p)
	case MigrateObjCopyPayload:
		return migrateObjCopy.Encode(ctx, p)
	case *ListObjectVersionsPayload:
		return listObjectVersions.Encode(ctx, *p)
	case ListObjectVersionsPayload:
		return listObjectVersions.Encode(ctx, p)
	case *MigrateVersionedObjectPayload:
		return migrateVersionedObject.Encode(ctx, *p)
	case MigrateVersionedObjectPayload:
		return migrateVersionedObject.Encode(ctx, p)
	case *ZeroDowntimeReplicationSwitchPayload:
		return zeroDowntimeReplicationSwitch.Encode(ctx, *p)
	case ZeroDowntimeReplicationSwitchPayload:
		return zeroDowntimeReplicationSwitch.Encode(ctx, p)
	case *SwitchWithDowntimePayload:
		return switchWithDowntime.Encode(ctx, *p)
	case SwitchWithDowntimePayload:
		return switchWithDowntime.Encode(ctx, p)
	case *DiffPayload:
		return diff.Encode(ctx, *p)
	case DiffPayload:
		return diff.Encode(ctx, p)
	case *DiffListObjectsPayload:
		return diffListObjects.Encode(ctx, *p)
	case DiffListObjectsPayload:
		return diffListObjects.Encode(ctx, p)
	case *DiffListVersionsPayload:
		return diffListVersions.Encode(ctx, *p)
	case DiffListVersionsPayload:
		return diffListVersions.Encode(ctx, p)
	case *DiffFixCollectObjectsPayload:
		return diffFixCollectObjectsPayload.Encode(ctx, *p)
	case DiffFixCollectObjectsPayload:
		return diffFixCollectObjectsPayload.Encode(ctx, p)
	case *DiffFixRemoveObjectsPayload:
		return diffFixRemoveObjects.Encode(ctx, *p)
	case DiffFixRemoveObjectsPayload:
		return diffFixRemoveObjects.Encode(ctx, p)
	case *DiffFixEnsureObjectsRemovedPayload:
		return diffFixEnsureObjectsRemoved.Encode(ctx, *p)
	case DiffFixEnsureObjectsRemovedPayload:
		return diffFixEnsureObjectsRemoved.Encode(ctx, p)
	case *DiffFixS3CopyPayload:
		return diffFixS3Copy.Encode(ctx, *p)
	case DiffFixS3CopyPayload:
		return diffFixS3Copy.Encode(ctx, p)
	case *DiffFixSwiftCopyPayload:
		return diffFixSwiftCopy.Encode(ctx, *p)
	case DiffFixSwiftCopyPayload:
		return diffFixSwiftCopy.Encode(ctx, p)
	case *DiffFixS3ListVersionsPayload:
		return diffFixS3ListVersions.Encode(ctx, *p)
	case DiffFixS3ListVersionsPayload:
		return diffFixS3ListVersions.Encode(ctx, p)
	case *SwiftAccountUpdatePayload:
		return swiftAccountUpdate.Encode(ctx, *p)
	case SwiftAccountUpdatePayload:
		return swiftAccountUpdate.Encode(ctx, p)
	case *SwiftContainerUpdatePayload:
		return swiftContainerUpdate.Encode(ctx, *p)
	case SwiftContainerUpdatePayload:
		return swiftContainerUpdate.Encode(ctx, p)
	case *SwiftObjectMetaUpdatePayload:
		return swiftObjectMetaUpdate.Encode(ctx, *p)
	case SwiftObjectMetaUpdatePayload:
		return swiftObjectMetaUpdate.Encode(ctx, p)
	case *SwiftObjectUpdatePayload:
		return swiftObjectUpdate.Encode(ctx, *p)
	case SwiftObjectUpdatePayload:
		return swiftObjectUpdate.Encode(ctx, p)
	case *SwiftObjectDeletePayload:
		return swiftObjectDelete.Encode(ctx, *p)
	case SwiftObjectDeletePayload:
		return swiftObjectDelete.Encode(ctx, p)
	case *SwiftAccountMigrationPayload:
		return swiftAccountMigration.Encode(ctx, *p)
	case SwiftAccountMigrationPayload:
		return swiftAccountMigration.Encode(ctx, p)
	case *SwiftContainerMigrationPayload:
		return swiftContainerMigration.Encode(ctx, *p)
	case SwiftContainerMigrationPayload:
		return swiftContainerMigration.Encode(ctx, p)
	case *SwiftObjectMigrationPayload:
		return swiftObjectMigration.Encode(ctx, *p)
	case SwiftObjectMigrationPayload:
		return swiftObjectMigration.Encode(ctx, p)
	default:
		return nil, fmt.Errorf("%w: unsupported payload type %T. Define encoder[%T] instance in pkg/tasks/encoder.go and add it to EncodeAny switch statement", dom.ErrNotImplemented, payload, payload)
	}
}

func enqueueAny(ctx context.Context, taskClient *asynq.Client, payload any, opts ...asynq.Option) error {
	task, err := EncodeAny(ctx, payload)
	if err != nil {
		return err
	}
	_, err = taskClient.EnqueueContext(ctx, task, opts...)
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) && !errors.Is(err, asynq.ErrTaskIDConflict) {
		return err
	}
	return nil
}

var (
	bucketCreate = encoder[BucketCreatePayload]{
		taskID: func(p BucketCreatePayload) string {
			return toTaskID("cb", p.ID.AsString(), p.Bucket)
		},
		queue: func(p BucketCreatePayload) string {
			return initMigrationListQueue(&p)
		},
		taskType: TypeBucketCreate,
	}
	bucketDelete = encoder[BucketDeletePayload]{
		taskID: nil,
		queue: func(p BucketDeletePayload) string {
			return eventQueue(&p)
		},
		taskType: TypeBucketDelete,
	}
	bucketSyncTags = encoder[BucketSyncTagsPayload]{
		taskID: nil,
		queue: func(p BucketSyncTagsPayload) string {
			return eventQueue(&p)

		},
		taskType: TypeBucketSyncTags,
	}
	bucketSyncACL = encoder[BucketSyncACLPayload]{
		taskID: nil,
		queue: func(p BucketSyncACLPayload) string {
			return eventQueue(&p)
		},
		taskType: TypeBucketSyncACL,
	}
	objectSync = encoder[ObjectSyncPayload]{
		taskID: nil,
		queue: func(p ObjectSyncPayload) string {
			return eventQueue(&p)
		},
		taskType: TypeObjectSync,
	}
	objSyncTags = encoder[ObjSyncTagsPayload]{
		taskID: nil,
		queue: func(p ObjSyncTagsPayload) string {
			return eventQueue(&p)
		},
		taskType: TypeObjectSyncTags,
	}
	objSyncACL = encoder[ObjSyncACLPayload]{
		taskID: nil,
		queue: func(p ObjSyncACLPayload) string {
			return eventQueue(&p)
		},
		taskType: TypeObjectSyncACL,
	}
	migrateS3User = encoder[MigrateS3UserPayload]{
		taskID: func(p MigrateS3UserPayload) string {
			return toTaskID("mgr:s3u", p.ID.AsString())
		},
		queue: func(p MigrateS3UserPayload) string {
			return initMigrationListQueue(&p)
		},
		taskType: TypeMigrateS3User,
	}
	migrateBucketListObjects = encoder[MigrateBucketListObjectsPayload]{
		taskID: func(p MigrateBucketListObjectsPayload) string {
			if p.Iteration > 0 {
				return toTaskID("mgr:lo", p.ID.AsString(), p.Bucket, p.Prefix, fmt.Sprintf("it:%d", p.Iteration))
			}
			return toTaskID("mgr:lo", p.ID.AsString(), p.Bucket, p.Prefix)
		},
		queue: func(p MigrateBucketListObjectsPayload) string {
			return initMigrationListQueue(&p)
		},
		taskType: TypeMigrateBucketListObjects,
	}
	migrateObjCopy = encoder[MigrateObjCopyPayload]{
		taskID: func(p MigrateObjCopyPayload) string {
			return toTaskID("mgr:co", p.ID.AsString(), p.Bucket, p.Obj.Name, p.Obj.VersionID)
		},
		queue: func(p MigrateObjCopyPayload) string {
			return initMigrationCopyQueue(&p)
		},
		taskType: TypeMigrateObjCopy,
	}
	listObjectVersions = encoder[ListObjectVersionsPayload]{
		taskID: func(p ListObjectVersionsPayload) string {
			return toTaskID("mgr:lov", p.ID.AsString(), p.Bucket, p.Prefix)
		},
		queue: func(p ListObjectVersionsPayload) string {
			return initMigrationListQueue(&p)
		},
		taskType: TypeMigrateObjectListVersions,
	}
	migrateVersionedObject = encoder[MigrateVersionedObjectPayload]{
		taskID: func(p MigrateVersionedObjectPayload) string {
			return toTaskID("mgr:cov", p.ID.AsString(), p.Bucket, p.Prefix)
		},
		queue: func(p MigrateVersionedObjectPayload) string {
			return initMigrationCopyQueue(&p)
		},
		taskType: TypeMigrateVersionedObject,
	}
	zeroDowntimeReplicationSwitch = encoder[ZeroDowntimeReplicationSwitchPayload]{
		taskID: func(p ZeroDowntimeReplicationSwitchPayload) string {
			return toTaskID("api:zdrs", p.ID.AsString())
		},
		queue: func(p ZeroDowntimeReplicationSwitchPayload) string {
			return string(QueueAPI)
		},
		taskType: TypeApiZeroDowntimeSwitch,
	}
	switchWithDowntime = encoder[SwitchWithDowntimePayload]{
		taskID: func(p SwitchWithDowntimePayload) string {
			return toTaskID("api:sd", p.ID.AsString())
		},
		queue: func(p SwitchWithDowntimePayload) string {
			return string(QueueAPI)
		},
		taskType: TypeApiSwitchWithDowntime,
	}
	diff = encoder[DiffPayload]{
		taskID: func(p DiffPayload) string {
			locationTokens := make([]string, 0, len(p.Locations)*2)
			for _, location := range p.Locations {
				locationTokens = append(locationTokens, location.Storage, location.Bucket)
			}
			return toTaskID(append([]string{"diff:check"}, locationTokens...)...)
		},
		queue: func(p DiffPayload) string {
			locations := make([]entity.DiffLocation, 0, len(p.Locations))
			for _, payloadLocation := range p.Locations {
				locations = append(locations, entity.NewDiffLocation(payloadLocation.Storage, payloadLocation.Bucket))
			}
			diffID := entity.NewDiffID(locations...)
			return DiffQueue(diffID)
		},
		taskType: TypeDiff,
	}
	diffListObjects = encoder[DiffListObjectsPayload]{
		taskID: func(p DiffListObjectsPayload) string {
			locationTokens := make([]string, 0, len(p.Locations)*2)
			for _, location := range p.Locations {
				locationTokens = append(locationTokens, location.Storage, location.Bucket)
			}
			return toTaskID(append(append([]string{"diff:lo"}, locationTokens...), strconv.Itoa(p.Index), p.Prefix)...)
		},
		queue: func(p DiffListObjectsPayload) string {
			locations := make([]entity.DiffLocation, 0, len(p.Locations))
			for _, payloadLocation := range p.Locations {
				locations = append(locations, entity.NewDiffLocation(payloadLocation.Storage, payloadLocation.Bucket))
			}
			diffID := entity.NewDiffID(locations...)
			return DiffQueue(diffID)
		},
		taskType: TypeDiffListObjects,
	}
	diffListVersions = encoder[DiffListVersionsPayload]{
		taskID: func(p DiffListVersionsPayload) string {
			locationTokens := make([]string, 0, len(p.Locations)*2)
			for _, location := range p.Locations {
				locationTokens = append(locationTokens, location.Storage, location.Bucket)
			}
			return toTaskID(append(append([]string{"diff:lv"}, locationTokens...), strconv.Itoa(p.Index), p.Prefix)...)
		},
		queue: func(p DiffListVersionsPayload) string {
			locations := make([]entity.DiffLocation, 0, len(p.Locations))
			for _, payloadLocation := range p.Locations {
				locations = append(locations, entity.NewDiffLocation(payloadLocation.Storage, payloadLocation.Bucket))
			}
			diffID := entity.NewDiffID(locations...)
			return DiffQueue(diffID)
		},
		taskType: TypeDiffListVersions,
	}
	diffFixCollectObjectsPayload = encoder[DiffFixCollectObjectsPayload]{
		taskID: func(p DiffFixCollectObjectsPayload) string {
			locationTokens := make([]string, 0, len(p.Locations)*2)
			for _, location := range p.Locations {
				locationTokens = append(locationTokens, location.Storage, location.Bucket)
			}
			return toTaskID(append([]string{"diff:fixcollect"}, locationTokens...)...)
		},
		queue: func(p DiffFixCollectObjectsPayload) string {
			locations := make([]entity.DiffLocation, 0, len(p.Locations))
			for _, payloadLocation := range p.Locations {
				locations = append(locations, entity.NewDiffLocation(payloadLocation.Storage, payloadLocation.Bucket))
			}
			diffID := entity.NewDiffID(locations...)
			return DiffFixQueue(diffID)
		},
		taskType: TypeDiffFixCollectObjects,
	}
	diffFixRemoveObjects = encoder[DiffFixRemoveObjectsPayload]{
		taskID: func(p DiffFixRemoveObjectsPayload) string {
			locationTokens := make([]string, 0, len(p.Locations)*2)
			for _, location := range p.Locations {
				locationTokens = append(locationTokens, location.Storage, location.Bucket)
			}
			return toTaskID(append([]string{"diff:fixremove"}, locationTokens...)...)
		},
		queue: func(p DiffFixRemoveObjectsPayload) string {
			locations := make([]entity.DiffLocation, 0, len(p.Locations))
			for _, payloadLocation := range p.Locations {
				locations = append(locations, entity.NewDiffLocation(payloadLocation.Storage, payloadLocation.Bucket))
			}
			diffID := entity.NewDiffID(locations...)
			return DiffFixQueue(diffID)
		},
		taskType: TypeDiffFixRemoveObjects,
	}
	diffFixEnsureObjectsRemoved = encoder[DiffFixEnsureObjectsRemovedPayload]{
		taskID: func(p DiffFixEnsureObjectsRemovedPayload) string {
			locationTokens := make([]string, 0, len(p.Locations)*2)
			for _, location := range p.Locations {
				locationTokens = append(locationTokens, location.Storage, location.Bucket)
			}
			return toTaskID(append([]string{"diff:fixensure"}, locationTokens...)...)
		},
		queue: func(p DiffFixEnsureObjectsRemovedPayload) string {
			locations := make([]entity.DiffLocation, 0, len(p.Locations))
			for _, payloadLocation := range p.Locations {
				locations = append(locations, entity.NewDiffLocation(payloadLocation.Storage, payloadLocation.Bucket))
			}
			diffID := entity.NewDiffID(locations...)
			return DiffFixQueue(diffID)
		},
		taskType: TypeDiffFixEnsureRemove,
	}
	diffFixS3Copy = encoder[DiffFixS3CopyPayload]{
		taskID: func(p DiffFixS3CopyPayload) string {
			return toTaskID("diff:fixcopys3", p.ID.AsString(), p.Bucket, p.Obj.Name)
		},
		queue: func(p DiffFixS3CopyPayload) string {
			locations := make([]entity.DiffLocation, 0, len(p.Locations))
			for _, payloadLocation := range p.Locations {
				locations = append(locations, entity.NewDiffLocation(payloadLocation.Storage, payloadLocation.Bucket))
			}
			diffID := entity.NewDiffID(locations...)
			return DiffFixQueue(diffID)
		},
		taskType: TypeDiffFixCopyS3,
	}
	diffFixSwiftCopy = encoder[DiffFixSwiftCopyPayload]{
		taskID: func(p DiffFixSwiftCopyPayload) string {
			return toTaskID("diff:fixcopyswift", p.ID.AsString(), p.Bucket, p.ObjName, p.ObjVersion)
		},
		queue: func(p DiffFixSwiftCopyPayload) string {
			locations := make([]entity.DiffLocation, 0, len(p.Locations))
			for _, payloadLocation := range p.Locations {
				locations = append(locations, entity.NewDiffLocation(payloadLocation.Storage, payloadLocation.Bucket))
			}
			diffID := entity.NewDiffID(locations...)
			return DiffFixQueue(diffID)
		},
		taskType: TypeDiffFixCopySwift,
	}
	diffFixS3ListVersions = encoder[DiffFixS3ListVersionsPayload]{
		taskID: func(p DiffFixS3ListVersionsPayload) string {
			return toTaskID("diff:fixlistversionss3", p.ID.AsString(), p.Bucket, p.Prefix)
		},
		queue: func(p DiffFixS3ListVersionsPayload) string {
			locations := make([]entity.DiffLocation, 0, len(p.Locations))
			for _, payloadLocation := range p.Locations {
				locations = append(locations, entity.NewDiffLocation(payloadLocation.Storage, payloadLocation.Bucket))
			}
			diffID := entity.NewDiffID(locations...)
			return DiffFixQueue(diffID)
		},
		inheritableQueue: true,
		taskType:         TypeDiffFixS3ListVersions,
	}
	swiftAccountUpdate = encoder[SwiftAccountUpdatePayload]{
		taskID: nil,
		queue: func(p SwiftAccountUpdatePayload) string {
			return eventQueue(&p)
		},
		taskType: TypeSwiftAccountUpdate,
	}
	swiftContainerUpdate = encoder[SwiftContainerUpdatePayload]{
		taskID: nil,
		queue: func(p SwiftContainerUpdatePayload) string {
			return eventQueue(&p)
		},
		taskType: TypeSwiftContainerUpdate,
	}
	swiftObjectMetaUpdate = encoder[SwiftObjectMetaUpdatePayload]{
		taskID: nil,
		queue: func(p SwiftObjectMetaUpdatePayload) string {
			return eventQueue(&p)
		},
		taskType: TypeSwiftObjMetaUpdate,
	}
	swiftObjectUpdate = encoder[SwiftObjectUpdatePayload]{
		taskID: nil,
		queue: func(p SwiftObjectUpdatePayload) string {
			return eventQueue(&p)
		},
		taskType: TypeSwiftObjUpdate,
	}
	swiftObjectDelete = encoder[SwiftObjectDeletePayload]{
		taskID: nil,
		queue: func(p SwiftObjectDeletePayload) string {
			return eventQueue(&p)
		},
		taskType: TypeSwiftObjDelete,
	}
	swiftAccountMigration = encoder[SwiftAccountMigrationPayload]{
		taskID: func(p SwiftAccountMigrationPayload) string {
			return toTaskID("mgr:sw:a", p.ID.AsString())
		},
		queue: func(p SwiftAccountMigrationPayload) string {
			return initMigrationListQueue(&p)
		},
		taskType: TypeSwiftAccountMigration,
	}
	swiftContainerMigration = encoder[SwiftContainerMigrationPayload]{
		taskID: func(p SwiftContainerMigrationPayload) string {
			return toTaskID("mgr:sw:c", p.ID.AsString(), p.Bucket)
		},
		queue: func(p SwiftContainerMigrationPayload) string {
			return initMigrationListQueue(&p)
		},
		taskType: TypeSwiftContainerMigration,
	}
	swiftObjectMigration = encoder[SwiftObjectMigrationPayload]{
		taskID: func(p SwiftObjectMigrationPayload) string {
			return toTaskID("mgr:sw:o", p.ID.AsString(), p.Bucket, p.ObjName)
		},
		queue: func(p SwiftObjectMigrationPayload) string {
			return initMigrationCopyQueue(&p)
		},
		taskType: TypeSwiftObjectMigration,
	}
)

func toTaskID(args ...string) string {
	if len(args) == 0 {
		panic("toTaskID: no args provided")
	}
	return strings.Join(args, ":")
}

// MigrateBucketListObjectsTaskID exposes the task ID generator for MigrateBucketListObjectsPayload.
func MigrateBucketListObjectsTaskID(p MigrateBucketListObjectsPayload) string {
	return migrateBucketListObjects.taskID(p)
}
