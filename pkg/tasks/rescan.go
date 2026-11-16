// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package tasks

import (
	"fmt"
	"time"
)

// Rescan tasks implement the periodic re-listing of a replication source (see docs/tutti/operations.md). They have
// their own types and queues, so they never show up in the queue statistics that define the initial copy state
// (InitDone, objects listed/done) of the replication.
const (
	TypeRescanBucketListObjects  = "rescan:bucket:list_objects"
	TypeRescanObjCopy            = "rescan:object:copy"
	TypeRescanObjectListVersions = "rescan:object:list_versions"
	TypeRescanVersionedObject    = "rescan:object:copy_versioned"
	TypeRescanUserListBuckets    = "rescan:user:list_buckets"
	QueueRescanListObjectsPrefix = Queue("rescan_list_obj")
	QueueRescanCopyObjectPrefix  = Queue("rescan_copy_obj")
)

// RescanRun identifies one scan of a replication and carries its cutoff.
type RescanRun struct {
	// RunID is unique per scan (start time in unix milliseconds).
	RunID string
	// Cutoff: source objects last modified before it are skipped. Zero copies everything.
	Cutoff time.Time `json:",omitzero"`
}

// RescanUserListBucketsPayload lists the buckets of a user-level replication and starts one object listing per bucket.
type RescanUserListBucketsPayload struct {
	replicationID
	RescanRun
}

// RescanListObjectsPayload is one page of the object listing of a source bucket.
type RescanListObjectsPayload struct {
	Bucket string
	replicationID
	RescanRun
	// Versioned reports a source with versioning enabled: objects fan out into per-version copies.
	Versioned bool
	// StartAfter is the continuation key (the last key of the previous page).
	StartAfter string
	// Iteration is the page index; it makes the task ID of every page unique.
	Iteration int
	// Backpressured: the producer yielded because the copy queue was full.
	Backpressured bool
}

// RescanObjCopyPayload copies one source object (same body as MigrateObjCopyPayload).
type RescanObjCopyPayload struct {
	MigrateObjCopyPayload
	RescanRun
}

// RescanListVersionsPayload snapshots the versions of one object of a versioned source.
type RescanListVersionsPayload struct {
	ListObjectVersionsPayload
	RescanRun
}

// RescanVersionedObjectPayload copies the versions of one object of a versioned source.
type RescanVersionedObjectPayload struct {
	MigrateVersionedObjectPayload
	RescanRun
}

var (
	rescanUserListBuckets = encoder[RescanUserListBucketsPayload]{
		taskID: func(p RescanUserListBucketsPayload) string {
			return toTaskID("rs:ub", p.ID.AsString(), p.RunID)
		},
		queue:    func(p RescanUserListBucketsPayload) string { return rescanListQueue(&p) },
		taskType: TypeRescanUserListBuckets,
	}
	rescanListObjects = encoder[RescanListObjectsPayload]{
		taskID: func(p RescanListObjectsPayload) string {
			return toTaskID("rs:lo", p.ID.AsString(), p.RunID, p.Bucket, fmt.Sprintf("it:%d", p.Iteration))
		},
		queue:    func(p RescanListObjectsPayload) string { return rescanListQueue(&p) },
		taskType: TypeRescanBucketListObjects,
	}
	rescanObjCopy = encoder[RescanObjCopyPayload]{
		taskID: func(p RescanObjCopyPayload) string {
			return toTaskID("rs:co", p.ID.AsString(), p.RunID, p.Bucket, p.Obj.Name, p.Obj.VersionID)
		},
		queue:    func(p RescanObjCopyPayload) string { return rescanCopyQueue(&p) },
		taskType: TypeRescanObjCopy,
	}
	rescanListVersions = encoder[RescanListVersionsPayload]{
		taskID: func(p RescanListVersionsPayload) string {
			return toTaskID("rs:lov", p.ID.AsString(), p.RunID, p.Bucket, p.Prefix)
		},
		queue:    func(p RescanListVersionsPayload) string { return rescanListQueue(&p) },
		taskType: TypeRescanObjectListVersions,
	}
	rescanVersionedObject = encoder[RescanVersionedObjectPayload]{
		taskID: func(p RescanVersionedObjectPayload) string {
			return toTaskID("rs:cov", p.ID.AsString(), p.RunID, p.Bucket, p.Prefix)
		},
		queue:    func(p RescanVersionedObjectPayload) string { return rescanCopyQueue(&p) },
		taskType: TypeRescanVersionedObject,
	}
)

func rescanListQueue(p ReplicationTask) string {
	return replicationQueueName(QueueRescanListObjectsPrefix, p.GetReplicationID())
}

func rescanCopyQueue(p ReplicationTask) string {
	return replicationQueueName(QueueRescanCopyObjectPrefix, p.GetReplicationID())
}
