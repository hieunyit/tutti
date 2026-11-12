// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// rateLimitingQueue rejects copy-task enqueues after `allow` successes, like a broker at the high watermark.
type rateLimitingQueue struct {
	*mockQueueRecorder
	allow        int
	copies       int
	continuation *tasks.MigrateBucketListObjectsPayload
	deferred     bool
}

func (q *rateLimitingQueue) EnqueueTask(ctx context.Context, task any, opts ...asynq.Option) error {
	switch p := task.(type) {
	case tasks.MigrateObjCopyPayload:
		if q.copies >= q.allow {
			return &dom.ErrRateLimitExceeded{RetryIn: time.Second}
		}
		q.copies++
	case tasks.MigrateBucketListObjectsPayload:
		q.continuation = &p
		q.deferred = len(opts) > 0
	}
	return q.mockQueueRecorder.EnqueueTask(ctx, task, opts...)
}

func TestMigrationList_EnqueueRateLimited_EndsPageAndSetsBackpressure(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()
	ResetQueueAdmissionState()
	t.Cleanup(ResetQueueAdmissionState)

	s, _, rec, bucket := setupTestProducer(t, 5)
	q := &rateLimitingQueue{mockQueueRecorder: rec, allow: 2}
	s.queueSvc = q

	replID := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User: "test-user", FromStorage: "from-storage", ToStorage: "to-storage",
		FromBucket: bucket, ToBucket: bucket,
	})
	payload := tasks.MigrateBucketListObjectsPayload{Bucket: bucket, Iteration: 1}
	payload.SetReplicationID(replID, 1)
	asynqTask, err := tasks.EncodeAny(ctx, payload)
	r.NoError(err)

	// Rate limit is not an error: the page ends and a deferred continuation is scheduled.
	r.NoError(s.HandleMigrationBucketListObj(ctx, asynqTask))
	r.Equal(2, q.copies)
	r.NotNil(q.continuation)
	r.True(q.continuation.Backpressured)
	r.Equal(2, q.continuation.Iteration)
	r.True(q.deferred, "continuation must be deferred")
	r.True(isQueueBackpressured(tasks.InitMigrationCopyQueue(payload.ID)))

	// Checkpoint advanced only past admitted objects: next run resumes at obj-003.
	q.allow = 100
	q.continuation = nil
	q.mockQueueRecorder.tasks = nil
	setQueueBackpressured(tasks.InitMigrationCopyQueue(payload.ID), false)
	r.NoError(s.HandleMigrationBucketListObj(ctx, asynqTask))
	r.Len(q.mockQueueRecorder.tasks, 3)
	first := q.mockQueueRecorder.tasks[0].(tasks.MigrateObjCopyPayload)
	r.Equal("obj-003", first.Obj.Name)
}
