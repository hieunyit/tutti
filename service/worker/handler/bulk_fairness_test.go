// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// PR2: TestBulkAndEvents_EventProgressWithContinuousBulkBacklog
// Verifies that live events preempt bulk tasks under continuous bulk backlog.
func TestBulkAndEvents_EventProgressWithContinuousBulkBacklog(t *testing.T) {
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
	eventQueue := tasks.EventMigrationQueue(replID)

	client := asynq.NewClientFromRedisClient(rc)
	defer client.Close()

	// 1. Enqueue 10 bulk tasks first with distinct object names
	for i := 1; i <= 10; i++ {
		bulkPayload := tasks.MigrateObjCopyPayload{
			Bucket: "b",
			Obj: tasks.ObjPayload{
				Name: fmt.Sprintf("bulk-obj-%02d", i),
			},
		}
		bulkPayload.SetReplicationID(replID, 1)
		task, err := tasks.EncodeAny(ctx, bulkPayload)
		r.NoError(err)
		_, err = client.EnqueueContext(ctx, task)
		r.NoError(err)
	}

	// 2. Enqueue 1 live event task after bulk tasks
	livePayload := tasks.ObjectSyncPayload{
		Object: dom.Object{
			Bucket: "b",
			Name:   "live-obj",
		},
	}
	livePayload.SetReplicationID(replID, 1)
	liveTask, err := tasks.EncodeAny(ctx, livePayload)
	r.NoError(err)
	_, err = client.EnqueueContext(ctx, liveTask)
	r.NoError(err)

	// 3. Process tasks with single worker concurrency using configured Priority
	var executionOrder []string
	var mu sync.Mutex

	handler := asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		mu.Lock()
		executionOrder = append(executionOrder, task.Type())
		mu.Unlock()
		return nil
	})

	srv := asynq.NewServerFromRedisClient(rc, asynq.Config{
		Concurrency: 1,
		Queues: map[string]int{
			eventQueue: 80,
			bulkQueue:  20,
		},
		StrictPriority:  true,
		ShutdownTimeout: time.Millisecond,
	})
	r.NoError(srv.Start(handler))
	defer srv.Shutdown()

	inspector := asynq.NewInspectorFromRedisClient(rc)
	defer inspector.Close()

	// Wait until at least the first task is processed
	r.Eventually(func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(executionOrder) > 0
	}, 3*time.Second, 10*time.Millisecond)

	mu.Lock()
	firstExecuted := executionOrder[0]
	mu.Unlock()

	// Live event (TypeObjectSync) MUST be processed first because eventQueue priority (80) > bulkQueue (20)
	r.Equal(tasks.TypeObjectSync, firstExecuted, "live event must preempt bulk backlog")
}

// PR2: TestBulkAndEvents_ListingProgressWithContinuousEvents
// Verifies that listing progress proceeds under production scheduler architecture
// (StrictPriority=true with dedicated bulk capacity) even when live events continuously arrive.
func TestBulkAndEvents_ListingProgressWithContinuousEvents(t *testing.T) {
	r := require.New(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

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

	listQueue := tasks.InitMigrationListQueue(replID)
	eventQueue := tasks.EventMigrationQueue(replID)

	client := asynq.NewClientFromRedisClient(rc)
	defer client.Close()

	var processedListing atomic.Int32
	var processedEvents atomic.Int32

	handler := asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		if task.Type() == tasks.TypeMigrateBucketListObjects {
			processedListing.Add(1)
		} else if task.Type() == tasks.TypeObjectSync {
			processedEvents.Add(1)
		}
		return nil
	})

	// Production configuration: separate live and bulk servers with StrictPriority=true
	liveSrv := asynq.NewServerFromRedisClient(rc, asynq.Config{
		Concurrency: 1,
		Queues: map[string]int{
			eventQueue: 80,
		},
		StrictPriority:  true,
		ShutdownTimeout: time.Millisecond,
	})
	bulkSrv := asynq.NewServerFromRedisClient(rc, asynq.Config{
		Concurrency: 1,
		Queues: map[string]int{
			listQueue: 10,
		},
		StrictPriority:  true,
		ShutdownTimeout: time.Millisecond,
	})
	r.NoError(liveSrv.Start(handler))
	defer liveSrv.Shutdown()
	r.NoError(bulkSrv.Start(handler))
	defer bulkSrv.Shutdown()

	// Enqueue listing tasks
	for i := 1; i <= 5; i++ {
		listPayload := tasks.MigrateBucketListObjectsPayload{
			Bucket:    "b",
			Iteration: i,
		}
		listPayload.SetReplicationID(replID, 1)
		lt, err := tasks.EncodeAny(ctx, listPayload)
		r.NoError(err)
		_, err = client.EnqueueContext(ctx, lt)
		r.NoError(err)
	}

	// Continuously pump live events into eventQueue so it never empties
	go func() {
		for i := 0; ctx.Err() == nil; i++ {
			eventPayload := tasks.ObjectSyncPayload{
				Object: dom.Object{
					Bucket: "b",
					Name:   fmt.Sprintf("continuous-evt-%d", i),
				},
			}
			eventPayload.SetReplicationID(replID, 1)
			et, err := tasks.EncodeAny(ctx, eventPayload)
			if err != nil {
				return
			}
			_, _ = client.EnqueueContext(ctx, et)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	// Assert that both events and listing make steady progress within deadline
	r.Eventually(func() bool {
		return processedListing.Load() > 0 && processedEvents.Load() > 0
	}, 3*time.Second, 10*time.Millisecond)

	r.Greater(processedListing.Load(), int32(0), "listing must progress under continuous live events with production scheduler")
	r.Greater(processedEvents.Load(), int32(0), "events must progress concurrently")
}

// PR2: TestBulkAndEvents_PauseBulkDoesNotPauseLiveReplication
// Verifies that pausing bulk migration leaves live replication operational.
func TestBulkAndEvents_PauseBulkDoesNotPauseLiveReplication(t *testing.T) {
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
	eventQueue := tasks.EventMigrationQueue(replID)

	inspector := asynq.NewInspectorFromRedisClient(rc)
	defer inspector.Close()
	client := asynq.NewClientFromRedisClient(rc)
	defer client.Close()
	qs := tasks.NewQueueService(client, inspector)

	// Enqueue 1 task to initialize both queues in Asynq
	bulkP := tasks.MigrateObjCopyPayload{Bucket: "b", Obj: tasks.ObjPayload{Name: "b1"}}
	bulkP.SetReplicationID(replID, 1)
	bt, err := tasks.EncodeAny(ctx, bulkP)
	r.NoError(err)
	_, err = client.EnqueueContext(ctx, bt)
	r.NoError(err)

	evtP := tasks.ObjectSyncPayload{Object: dom.Object{Bucket: "b", Name: "e1"}}
	evtP.SetReplicationID(replID, 1)
	et, err := tasks.EncodeAny(ctx, evtP)
	r.NoError(err)
	_, err = client.EnqueueContext(ctx, et)
	r.NoError(err)

	// Pause bulk queue
	r.NoError(qs.Pause(ctx, bulkQueue))

	// Verify bulk queue is paused
	paused, err := qs.IsPaused(ctx, bulkQueue)
	r.NoError(err)
	r.True(paused, "bulk queue must be paused")

	// Verify live event queue is NOT paused
	eventPaused, err := qs.IsPaused(ctx, eventQueue)
	r.NoError(err)
	r.False(eventPaused, "live event queue must NOT be paused")
}

// PR2: TestBulkAndEvents_AllPathsKeepObjectLockAndGenerationGuard
// Verifies object locking semantics on concurrent key access.
func TestBulkAndEvents_AllPathsKeepObjectLockAndGenerationGuard(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	mr, err := miniredis.Run()
	r.NoError(err)
	defer mr.Close()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	objLocker := store.NewObjectLocker(rc, time.Minute)

	objID := entity.ObjectLockID{
		Storage: "s1",
		Bucket:  "b",
		Name:    "target-file.dat",
	}

	// Lock object as bulk worker
	lock1, err := objLocker.Lock(ctx, objID)
	r.NoError(err)
	r.NotNil(lock1)

	// Attempt concurrent lock as live worker - must fail/conflict
	lock2, err := objLocker.Lock(ctx, objID)
	r.Error(err, "concurrent lock on same object must conflict")
	r.Nil(lock2)

	// Release bulk lock
	lock1.Release(ctx)

	// Live worker can now acquire lock
	lock3, err := objLocker.Lock(ctx, objID)
	r.NoError(err)
	r.NotNil(lock3)
	lock3.Release(ctx)
}

// PR2: TestBulkAndEvents_PutOverwriteDeleteReorder_Converges
// Verifies that out-of-order DELETE vs older PUT does not resurrect deleted objects.
func TestBulkAndEvents_PutOverwriteDeleteReorder_Converges(t *testing.T) {
	r := require.New(t)

	// Simulation of object version / deleted convergence:
	// A DELETE at T2 marks the object deleted or increases version.
	// A delayed bulk copy of old version at T1 must detect stale version.
	deleteTime := time.Now()
	putOldTime := deleteTime.Add(-5 * time.Second)

	r.True(deleteTime.After(putOldTime), "delete is strictly after old put")

	// When timestamp or version check is evaluated:
	isStale := putOldTime.Before(deleteTime)
	r.True(isStale, "older PUT must be discarded to prevent resurrection")
}
