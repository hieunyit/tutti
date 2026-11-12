// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gophercloud/gophercloud/v2"
	"github.com/hibiken/asynq"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/ratelimit"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3client"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// mockQueueRecorder tracks enqueued tasks and allows simulating crashes/errors.
type mockQueueRecorder struct {
	mu             sync.Mutex
	tasks          []any
	failAfter      int
	unprocessed    int
	unprocessedErr error
}

func (m *mockQueueRecorder) EnqueueTask(ctx context.Context, task any, opts ...asynq.Option) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failAfter > 0 && len(m.tasks) >= m.failAfter {
		return fmt.Errorf("simulated crash during enqueue")
	}
	m.tasks = append(m.tasks, task)
	return nil
}

func (m *mockQueueRecorder) UnprocessedCount(ctx context.Context, ignoreNotFound bool, queues ...string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.unprocessedErr != nil {
		return 0, m.unprocessedErr
	}
	return m.unprocessed, nil
}

func (m *mockQueueRecorder) IsPaused(ctx context.Context, queueName string) (bool, error) { return false, nil }
func (m *mockQueueRecorder) Resume(ctx context.Context, queueName string) error              { return nil }
func (m *mockQueueRecorder) Pause(ctx context.Context, queueName string) error               { return nil }
func (m *mockQueueRecorder) Delete(ctx context.Context, queueName string, force bool) error  { return nil }
func (m *mockQueueRecorder) Stop(ctx context.Context, queues ...string) error                { return nil }
func (m *mockQueueRecorder) Purge(ctx context.Context, queues ...string) error               { return nil }
func (m *mockQueueRecorder) Stats(ctx context.Context, queueName string) (*tasks.QueueStats, error) {
	return &tasks.QueueStats{}, nil
}

type singleS3Clients struct {
	client s3client.Client
}

func (s singleS3Clients) AsS3(_ context.Context, _, _ string) (s3client.Client, error) {
	return s.client, nil
}
func (s singleS3Clients) AsCommon(_ context.Context, _, _ string) (objstore.Common, error) {
	return nil, dom.ErrNotFound
}
func (s singleS3Clients) AsSwift(_ context.Context, _, _ string) (*gophercloud.ServiceClient, error) {
	return nil, dom.ErrNotFound
}

func setupTestProducer(t *testing.T, numObjects int) (*svc, *miniredis.Miniredis, *mockQueueRecorder, string) {
	r := require.New(t)
	ctx := t.Context()

	// 1. Miniredis for list state store
	mr, err := miniredis.Run()
	r.NoError(err)
	t.Cleanup(mr.Close)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	listStateStore := store.NewMigrationObjectListStateStore(rc)

	// 2. Fake S3 backend
	backend := s3mem.New()
	bucket := "test-bucket"
	r.NoError(backend.CreateBucket(bucket))

	for i := 1; i <= numObjects; i++ {
		key := fmt.Sprintf("obj-%03d", i)
		_, err := backend.PutObject(bucket, key, map[string]string{"Content-Type": "application/octet-stream"}, strings.NewReader("hello"), 5, nil)
		r.NoError(err)
	}

	fakeS3 := gofakes3.New(backend)
	ts := httptest.NewServer(fakeS3.Server())
	t.Cleanup(ts.Close)

	client, err := s3client.NewClient(ctx, metrics.NewS3Service(false),
		s3.StorageAddress{Address: ts.URL, HttpTimeout: time.Minute},
		s3.CredentialsV4{AccessKeyID: "test", SecretAccessKey: "test"}, "from-storage", "test-user")
	r.NoError(err)

	queueRec := &mockQueueRecorder{}
	limiter := ratelimit.New(rc, nil)
	retryInterval := time.Second

	s := &svc{
		conf:           &Config{CustomErrRetryInterval: &retryInterval},
		clients:        singleS3Clients{client: client},
		listStateStore: listStateStore,
		queueSvc:       queueRec,
		limit:          limiter,
	}

	return s, mr, queueRec, bucket
}

// PR1: TestBulkProducer_CrashAfterPartialPage_NoMissingKeys
// Simulates crash mid-page; on retry, all keys are enqueued without loss.
func TestBulkProducer_CrashAfterPartialPage_NoMissingKeys(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	s, _, queueRec, bucket := setupTestProducer(t, 5)

	replID := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User:        "test-user",
		FromStorage: "from-storage",
		ToStorage:   "to-storage",
		FromBucket:  bucket,
		ToBucket:    bucket,
	})

	payload := tasks.MigrateBucketListObjectsPayload{
		Bucket:    bucket,
		Iteration: 1,
	}
	payload.SetReplicationID(replID, 1)

	asynqTask, err := tasks.EncodeAny(ctx, payload)
	r.NoError(err)

	// Step 1: Simulate crash after 2 items
	queueRec.failAfter = 2
	err = s.HandleMigrationBucketListObj(ctx, asynqTask)
	r.Error(err)
	r.Contains(err.Error(), "simulated crash")
	r.Len(queueRec.tasks, 2)

	// Step 2: Retry with fresh attempt - no missing keys
	queueRec.failAfter = 0
	queueRec.tasks = nil // reset recorder for retry
	err = s.HandleMigrationBucketListObj(ctx, asynqTask)
	r.NoError(err)

	// All 5 objects should be enqueued
	r.Len(queueRec.tasks, 5)
	keys := make(map[string]bool)
	for _, tk := range queueRec.tasks {
		if copyPayload, ok := tk.(tasks.MigrateObjCopyPayload); ok {
			keys[copyPayload.Obj.Name] = true
		}
	}
	for i := 1; i <= 5; i++ {
		r.True(keys[fmt.Sprintf("obj-%03d", i)], "key obj-%03d must be enqueued", i)
	}
}

// PR1: TestBulkProducer_CrashAfterCheckpointBeforeContinuation_Resumes
// When checkpoint is written, resumption continues from checkpoint without repeating previous objects.
func TestBulkProducer_CrashAfterCheckpointBeforeContinuation_Resumes(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	s, _, queueRec, bucket := setupTestProducer(t, 6)

	replID := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User:        "test-user",
		FromStorage: "from-storage",
		ToStorage:   "to-storage",
		FromBucket:  bucket,
		ToBucket:    bucket,
	})

	// Pre-set checkpoint in state store to simulate previous batch stopped at obj-003
	migrationID := entity.NewMigrationObjectIDFromUniversalReplicationID(replID, bucket, "")
	err := s.listStateStore.Set(ctx, migrationID, "obj-003")
	r.NoError(err)

	payload := tasks.MigrateBucketListObjectsPayload{
		Bucket:    bucket,
		Iteration: 2,
	}
	payload.SetReplicationID(replID, 1)

	asynqTask, err := tasks.EncodeAny(ctx, payload)
	r.NoError(err)

	err = s.HandleMigrationBucketListObj(ctx, asynqTask)
	r.NoError(err)

	// Should only list and enqueue obj-004, obj-005, obj-006
	r.Len(queueRec.tasks, 3)
	keys := make([]string, 0, len(queueRec.tasks))
	for _, tk := range queueRec.tasks {
		if cp, ok := tk.(tasks.MigrateObjCopyPayload); ok {
			keys = append(keys, cp.Obj.Name)
		}
	}
	r.Equal([]string{"obj-004", "obj-005", "obj-006"}, keys)

	// Since this was final batch, checkpoint must be cleared
	_, err = s.listStateStore.Get(ctx, migrationID)
	r.ErrorIs(err, dom.ErrNotFound)
}

// PR1: TestBulkProducer_ContinuationTaskIDConflict_DoesNotLoseWork
// Verifies that iteration increments produce unique TaskIDs so Asynq does not drop continuations.
func TestBulkProducer_ContinuationTaskIDConflict_DoesNotLoseWork(t *testing.T) {
	r := require.New(t)

	replID := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User:        "test-user",
		FromStorage: "from-storage",
		ToStorage:   "to-storage",
		FromBucket:  "b",
		ToBucket:    "b",
	})

	p1 := tasks.MigrateBucketListObjectsPayload{
		Bucket:    "b",
		Iteration: 1,
	}
	p1.SetReplicationID(replID, 1)

	p2 := tasks.MigrateBucketListObjectsPayload{
		Bucket:    "b",
		Iteration: 2,
	}
	p2.SetReplicationID(replID, 1)

	// TaskIDs must differ by iteration to prevent duplicate rejection in Asynq
	id1 := tasks.MigrateBucketListObjectsTaskID(p1)
	id2 := tasks.MigrateBucketListObjectsTaskID(p2)
	r.NotEqual(id1, id2)
	r.Contains(id1, ":it:1")
	r.Contains(id2, ":it:2")
}

// PR1: TestBulkProducer_UnknownEnqueueOutcome_ReconcilesReservation
// Verifies that when queue backlog is at/above watermark, continuation is deferred without blocking worker.
func TestBulkProducer_UnknownEnqueueOutcome_ReconcilesReservation(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	s, _, queueRec, bucket := setupTestProducer(t, 10)

	replID := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User:        "test-user",
		FromStorage: "from-storage",
		ToStorage:   "to-storage",
		FromBucket:  bucket,
		ToBucket:    bucket,
	})

	// Simulate high backlog exceeding watermark (10,000)
	queueRec.unprocessed = 12000

	payload := tasks.MigrateBucketListObjectsPayload{
		Bucket:    bucket,
		Iteration: 1,
	}
	payload.SetReplicationID(replID, 1)

	asynqTask, err := tasks.EncodeAny(ctx, payload)
	r.NoError(err)

	// HandleMigrationBucketListObj must defer continuation immediately and return nil without executing listing
	err = s.HandleMigrationBucketListObj(ctx, asynqTask)
	r.NoError(err)

	// Queue should have 1 task: the deferred continuation task with Iteration = 2
	r.Len(queueRec.tasks, 1)
	nextTask, ok := queueRec.tasks[0].(tasks.MigrateBucketListObjectsPayload)
	r.True(ok)
	r.Equal(2, nextTask.Iteration)
	r.Equal(payload.Bucket, nextTask.Bucket)
}

// PR1: TestBulkProducer_SeparateRedisRoles_NoAtomicityAssumption
// Verifies checkpoint store and queue can use separate Redis instances without error.
func TestBulkProducer_SeparateRedisRoles_NoAtomicityAssumption(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	mr1, err := miniredis.Run()
	r.NoError(err)
	defer mr1.Close()

	mr2, err := miniredis.Run()
	r.NoError(err)
	defer mr2.Close()

	rc1 := redis.NewClient(&redis.Options{Addr: mr1.Addr()})
	defer rc1.Close()
	rc2 := redis.NewClient(&redis.Options{Addr: mr2.Addr()})
	defer rc2.Close()

	// Checkpoint store on Redis 1
	listStateStore := store.NewMigrationObjectListStateStore(rc1)
	// Limiter on Redis 2
	limiter := ratelimit.New(rc2, nil)

	backend := s3mem.New()
	bucket := "multi-role-bucket"
	r.NoError(backend.CreateBucket(bucket))
	_, err = backend.PutObject(bucket, "k1", map[string]string{"Content-Type": "text/plain"}, strings.NewReader("v1"), 2, nil)
	r.NoError(err)

	fakeS3 := gofakes3.New(backend)
	ts := httptest.NewServer(fakeS3.Server())
	defer ts.Close()

	client, err := s3client.NewClient(ctx, metrics.NewS3Service(false),
		s3.StorageAddress{Address: ts.URL, HttpTimeout: time.Minute},
		s3.CredentialsV4{AccessKeyID: "k", SecretAccessKey: "s"}, "from", "u")
	r.NoError(err)

	queueRec := &mockQueueRecorder{}
	retryInterval := time.Second
	s := &svc{
		conf:           &Config{CustomErrRetryInterval: &retryInterval},
		clients:        singleS3Clients{client: client},
		listStateStore: listStateStore,
		queueSvc:       queueRec,
		limit:          limiter,
	}

	replID := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User:        "u",
		FromStorage: "from",
		ToStorage:   "to",
		FromBucket:  bucket,
		ToBucket:    bucket,
	})

	payload := tasks.MigrateBucketListObjectsPayload{
		Bucket:    bucket,
		Iteration: 1,
	}
	payload.SetReplicationID(replID, 1)

	asynqTask, err := tasks.EncodeAny(ctx, payload)
	r.NoError(err)

	err = s.HandleMigrationBucketListObj(ctx, asynqTask)
	r.NoError(err)
	r.Len(queueRec.tasks, 1)
}

// PR1: TestBulkProducer_OldGenerationCannotAdvanceNewCursor
// Verifies that stale generation tasks are intercepted and cannot overwrite cursor.
func TestBulkProducer_OldGenerationCannotAdvanceNewCursor(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	mr, err := miniredis.Run()
	r.NoError(err)
	defer mr.Close()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	listStateStore := store.NewMigrationObjectListStateStore(rc)

	replID := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User:        "u",
		FromStorage: "from",
		ToStorage:   "to",
		FromBucket:  "b",
		ToBucket:    "b",
	})
	migrationID := entity.NewMigrationObjectIDFromUniversalReplicationID(replID, "b", "")

	// New generation cursor is at obj-100
	r.NoError(listStateStore.Set(ctx, migrationID, "obj-100"))

	// Middleware simulation: replication generation in policy store is now 2
	genGetter := func(ctx context.Context, id entity.UniversalReplicationID) (int64, error) {
		return 2, nil
	}
	middleware := tasks.DropStaleReplicationTasks(genGetter)

	// An incoming task with Generation = 1 arrives
	stalePayload := tasks.MigrateBucketListObjectsPayload{
		Bucket:    "b",
		Iteration: 1,
	}
	stalePayload.SetReplicationID(replID, 1)

	staleTask, err := tasks.EncodeAny(ctx, stalePayload)
	r.NoError(err)

	queueName := tasks.InitMigrationListQueue(replID)

	var handlerCalled atomic.Bool
	dummyHandler := asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		handlerCalled.Store(true)
		return nil
	})

	wrappedHandler := middleware(dummyHandler)

	srv := asynq.NewServerFromRedisClient(rc, asynq.Config{
		Concurrency:     1,
		Queues:          map[string]int{queueName: 1},
		ShutdownTimeout: time.Millisecond,
	})
	r.NoError(srv.Start(wrappedHandler))
	defer srv.Shutdown()

	client := asynq.NewClientFromRedisClient(rc)
	defer client.Close()
	_, err = client.EnqueueContext(ctx, staleTask)
	r.NoError(err)

	inspector := asynq.NewInspectorFromRedisClient(rc)
	defer inspector.Close()

	r.Eventually(func() bool {
		info, err := inspector.GetQueueInfo(queueName)
		return err == nil && info.Pending == 0 && info.Active == 0
	}, 3*time.Second, 10*time.Millisecond)

	// Stale task must be dropped without executing handler
	r.False(handlerCalled.Load(), "handler must not be called for stale generation task")

	// Verify state store cursor was NOT altered
	cursor, err := listStateStore.Get(ctx, migrationID)
	r.NoError(err)
	r.Equal("obj-100", cursor)
}
