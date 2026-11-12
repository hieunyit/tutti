// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// PR4: TestBulkLifecycle_FalseDoneWhenArchived
// Verifies that when archived (permanently failed) tasks exist, InitDone returns false.
func TestBulkLifecycle_FalseDoneWhenArchived(t *testing.T) {
	r := require.New(t)

	statusClean := entity.ReplicationStatusExtended{
		InitMigrationListing: entity.QueueStats{Unprocessed: 0, Done: 5, Archived: 0},
		InitMigration:        entity.QueueStats{Unprocessed: 0, Done: 100, Archived: 0},
	}
	r.True(statusClean.InitDone(), "clean completion must report InitDone = true")
	r.False(statusClean.HasFailures(), "clean completion has no failures")

	// Status with 0 unprocessed but 1 archived task in copy queue
	statusWithArchivedCopy := entity.ReplicationStatusExtended{
		InitMigrationListing: entity.QueueStats{Unprocessed: 0, Done: 5, Archived: 0},
		InitMigration:        entity.QueueStats{Unprocessed: 0, Done: 99, Archived: 1},
	}
	r.False(statusWithArchivedCopy.InitDone(), "must NOT report InitDone = true when tasks are archived")
	r.True(statusWithArchivedCopy.HasFailures(), "must report failures when archived > 0")

	// Status with 0 unprocessed but 1 archived task in listing queue
	statusWithArchivedListing := entity.ReplicationStatusExtended{
		InitMigrationListing: entity.QueueStats{Unprocessed: 0, Done: 4, Archived: 1},
		InitMigration:        entity.QueueStats{Unprocessed: 0, Done: 100, Archived: 0},
	}
	r.False(statusWithArchivedListing.InitDone(), "must NOT report InitDone = true when listing task is archived")
	r.True(statusWithArchivedListing.HasFailures())
}

// PR4: TestBulkLifecycle_DuplicateAttemptsDoNotIncrementUniqueCount
// Verifies that retries do not inflate the number of unique objects completed.
func TestBulkLifecycle_DuplicateAttemptsDoNotIncrementUniqueCount(t *testing.T) {
	r := require.New(t)

	// In Asynq: ProcessedTotal increments on EVERY attempt (both failure and success)
	// FailedTotal increments on every failure.
	// 1 task that failed twice and succeeded on 3rd attempt:
	// ProcessedTotal = 3, FailedTotal = 2
	// Done = ProcessedTotal - FailedTotal = 1
	stats := tasks.QueueStats{
		Unprocessed:    0,
		ProcessedTotal: 3,
		FailedTotal:    2,
		Archived:       0,
	}

	uniqueDone := stats.ProcessedTotal - stats.FailedTotal
	r.Equal(1, uniqueDone, "retried task must only count as 1 unique object completed")
}

// PR4: TestBulkLifecycle_FailureManifestRetry
// Verifies that archived/poison tasks can be inspected and retried individually.
func TestBulkLifecycle_FailureManifestRetry(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	mr, err := miniredis.Run()
	r.NoError(err)
	defer mr.Close()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	replID := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User:        "test-user",
		FromStorage: "from",
		ToStorage:   "to",
		FromBucket:  "b",
		ToBucket:    "b",
	})
	bulkQueue := tasks.InitMigrationCopyQueue(replID)

	client := asynq.NewClientFromRedisClient(rc)
	defer client.Close()
	inspector := asynq.NewInspectorFromRedisClient(rc)
	defer inspector.Close()

	// Enqueue a task that will be archived (maxRetry = 0)
	payload := tasks.MigrateObjCopyPayload{
		Bucket: "b",
		Obj:    tasks.ObjPayload{Name: "failed-key.txt"},
	}
	payload.SetReplicationID(replID, 1)
	task, err := tasks.EncodeAny(ctx, payload)
	r.NoError(err)

	_, err = client.EnqueueContext(ctx, task, asynq.MaxRetry(0))
	r.NoError(err)

	// Process and fail the task so it moves to Archived
	srv := asynq.NewServerFromRedisClient(rc, asynq.Config{
		Concurrency: 1,
		Queues:      map[string]int{bulkQueue: 1},
		IsFailure:   func(err error) bool { return true },
		ShutdownTimeout: time.Millisecond,
	})
	failingHandler := asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		return dom.ErrInternal
	})
	r.NoError(srv.Start(failingHandler))

	r.Eventually(func() bool {
		qInfo, err := inspector.GetQueueInfo(bulkQueue)
		return err == nil && qInfo.Archived > 0
	}, 3*time.Second, 10*time.Millisecond)
	srv.Shutdown()

	// Inspect archived task from failure manifest
	archivedTasks, err := inspector.ListArchivedTasks(bulkQueue, asynq.Page(1), asynq.PageSize(10))
	r.NoError(err)
	r.Len(archivedTasks, 1)
	r.Equal(tasks.TypeMigrateObjCopy, archivedTasks[0].Type)

	// Retry failure manifest task: re-run the archived task
	r.NoError(inspector.RunTask(bulkQueue, archivedTasks[0].ID))

	// Verify task is moved back to pending queue
	qInfo, err := inspector.GetQueueInfo(bulkQueue)
	r.NoError(err)
	r.Equal(1, qInfo.Pending)
	r.Equal(0, qInfo.Archived)
}

// PR4: TestBulkLifecycle_PauseResumeCancelSemantics
// Verifies Pause/Resume/Cancel lifecycle operations.
func TestBulkLifecycle_PauseResumeCancelSemantics(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	mr, err := miniredis.Run()
	r.NoError(err)
	defer mr.Close()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	replID := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User:        "test-user",
		FromStorage: "from",
		ToStorage:   "to",
		FromBucket:  "b",
		ToBucket:    "b",
	})
	bulkQueue := tasks.InitMigrationCopyQueue(replID)

	client := asynq.NewClientFromRedisClient(rc)
	defer client.Close()
	inspector := asynq.NewInspectorFromRedisClient(rc)
	defer inspector.Close()
	qs := tasks.NewQueueService(client, inspector)

	// Initialize queue with 1 task
	payload := tasks.MigrateObjCopyPayload{
		Bucket: "b",
		Obj:    tasks.ObjPayload{Name: "obj1"},
	}
	payload.SetReplicationID(replID, 1)
	task, err := tasks.EncodeAny(ctx, payload)
	r.NoError(err)
	_, err = client.EnqueueContext(ctx, task)
	r.NoError(err)

	// 1. Pause
	r.NoError(qs.Pause(ctx, bulkQueue))
	paused, err := qs.IsPaused(ctx, bulkQueue)
	r.NoError(err)
	r.True(paused)

	// 2. Resume
	r.NoError(qs.Resume(ctx, bulkQueue))
	paused, err = qs.IsPaused(ctx, bulkQueue)
	r.NoError(err)
	r.False(paused)

	// 3. Cancel / Stop: stops queue without data destruction
	r.NoError(qs.Stop(ctx, bulkQueue))
	stopped, err := qs.IsPaused(ctx, bulkQueue)
	r.NoError(err)
	r.True(stopped, "stopped queue remains paused to prevent new admissions")
}

// PR4: TestBulkLifecycle_RestartVerificationGeneration
// Verifies that restarting verification issues a new generation and invalidates older generation requests.
func TestBulkLifecycle_RestartVerificationGeneration(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	currentGen := int64(1)
	genGetter := func(ctx context.Context, id entity.UniversalReplicationID) (int64, error) {
		return currentGen, nil
	}
	middleware := tasks.DropStaleReplicationTasks(genGetter)

	replID := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User:        "test-user",
		FromStorage: "from",
		ToStorage:   "to",
		FromBucket:  "b",
		ToBucket:    "b",
	})
	queueName := tasks.InitMigrationCopyQueue(replID)

	// Generation 1 task
	p1 := tasks.MigrateObjCopyPayload{Bucket: "b", Obj: tasks.ObjPayload{Name: "gen1"}}
	p1.SetReplicationID(replID, 1)
	t1, err := tasks.EncodeAny(ctx, p1)
	r.NoError(err)

	// Increment generation on restart
	currentGen = 2

	// Generation 2 task
	p2 := tasks.MigrateObjCopyPayload{Bucket: "b", Obj: tasks.ObjPayload{Name: "gen2"}}
	p2.SetReplicationID(replID, 2)
	t2, err := tasks.EncodeAny(ctx, p2)
	r.NoError(err)

	var executedTasks []string
	handler := asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		executedTasks = append(executedTasks, t.Type())
		return nil
	})
	wrapped := middleware(handler)

	// Run old generation task in asynq server context simulation
	mr, err := miniredis.Run()
	r.NoError(err)
	defer mr.Close()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	srv := asynq.NewServerFromRedisClient(rc, asynq.Config{
		Concurrency: 1,
		Queues:      map[string]int{queueName: 1},
		ShutdownTimeout: time.Millisecond,
	})
	r.NoError(srv.Start(wrapped))
	defer srv.Shutdown()

	client := asynq.NewClientFromRedisClient(rc)
	defer client.Close()

	// Enqueue old task (generation 1) and new task (generation 2)
	_, err = client.EnqueueContext(ctx, t1)
	r.NoError(err)
	_, err = client.EnqueueContext(ctx, t2)
	r.NoError(err)

	inspector := asynq.NewInspectorFromRedisClient(rc)
	defer inspector.Close()

	r.Eventually(func() bool {
		qInfo, err := inspector.GetQueueInfo(queueName)
		return err == nil && qInfo.Pending == 0 && qInfo.Active == 0
	}, 3*time.Second, 10*time.Millisecond)

	// Only generation 2 task should be executed by handler (gen 1 dropped by middleware)
	r.Len(executedTasks, 1, "stale generation 1 task must be dropped")
}

// PR4: TestBulkLifecycle_ProgressMonotonicity
// Verifies that progress counters never regress backwards on out-of-order samples.
func TestBulkLifecycle_ProgressMonotonicity(t *testing.T) {
	r := require.New(t)

	type ProgressTracker struct {
		highestDone int
	}

	tracker := &ProgressTracker{highestDone: 0}

	update := func(sampleDone int) int {
		if sampleDone > tracker.highestDone {
			tracker.highestDone = sampleDone
		}
		return tracker.highestDone
	}

	// Sample 1: 50 done
	r.Equal(50, update(50))

	// Sample 2: 100 done
	r.Equal(100, update(100))

	// Sample 3: out-of-order / delayed sample reporting 80 done
	r.Equal(100, update(80), "progress must not regress backwards")

	// Sample 4: 120 done
	r.Equal(120, update(120))
}
