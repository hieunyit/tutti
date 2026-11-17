// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/controlplane/pgtest"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/service/worker/syncer"
)

type retryQueue struct {
	tasks.QueueService
	enqueue func(context.Context, any, ...asynq.Option) error
}

func (q *retryQueue) EnqueueTask(ctx context.Context, value any, opts ...asynq.Option) error {
	return q.enqueue(ctx, value, opts...)
}

func TestDeleteRetryEnqueuesPersistedEvent(t *testing.T) {
	ctx := t.Context()
	cp, err := pg.Open(ctx, controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: pgtest.NewDB(t), AutoMigrate: true}})
	require.NoError(t, err)
	t.Cleanup(cp.Close)
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	client := asynq.NewClientFromRedisClient(rc)
	inspector := asynq.NewInspectorFromRedisClient(rc)
	queue := tasks.NewQueueService(client, inspector)
	q := &retryQueue{QueueService: queue}
	svc := NewService(rc, q, "main").WithControlPlane(cp)
	uid := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{User: "u", FromStorage: "main", ToStorage: "dst", FromBucket: "src", ToBucket: "replica"})
	repos := cp.Repos()
	require.NoError(t, repos.Replications.SetGeneration(ctx, toCPReplicationID(uid), 7))
	eventID, err := repos.DeleteEvents.Create(ctx, controlplane.DeleteEvent{
		ReplicationID: uid.AsString(), Bucket: "src", ObjectKey: "file", VersionID: "v1", Status: "failed", Generation: 7, RetryCount: 2, LastError: "original failure",
	})
	require.NoError(t, err)

	// A lost response to Redis leaves the task in the queue while Postgres rolls back.
	unavailable := errors.New("queue unavailable")
	q.enqueue = func(ctx context.Context, payload any, opts ...asynq.Option) error {
		if err := queue.EnqueueTask(ctx, payload, opts...); err != nil {
			return err
		}
		return unavailable
	}
	n, err := svc.RetryFailedDeletes(ctx, uid.AsString())
	require.ErrorIs(t, err, unavailable)
	require.Zero(t, n)
	events, _, err := repos.DeleteEvents.List(ctx, controlplane.DeleteEventFilter{Status: "failed"})
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "original failure", events[0].LastError)

	// The same task ID reconciles the pending claim without creating a second task.
	q.enqueue = func(ctx context.Context, payload any, opts ...asynq.Option) error {
		task, err := tasks.EncodeAny(ctx, payload)
		if err != nil {
			return err
		}
		_, err = client.EnqueueContext(ctx, task, opts...)
		return err
	}
	n, err = svc.RetryFailedDeletes(ctx, uid.AsString())
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	n, err = svc.RetryFailedDeletes(ctx, uid.AsString())
	require.NoError(t, err)
	require.Zero(t, n)
	queueName := "event:" + uid.AsString()
	pending, err := inspector.ListPendingTasks(queueName)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.Equal(t, fmt.Sprintf("delete-retry-%d-2", eventID), pending[0].ID)

	var calls atomic.Int32
	srv := asynq.NewServerFromRedisClient(rc, asynq.Config{
		Concurrency: 1, Queues: map[string]int{queueName: 1}, TaskCheckInterval: 10 * time.Millisecond,
	})
	worker := tasks.DropStaleReplicationTasks(svc.GetReplicationGeneration)(asynq.HandlerFunc(func(ctx context.Context, task *asynq.Task) error {
		calls.Add(1)
		var p tasks.ObjectSyncPayload
		if err := json.Unmarshal(task.Payload(), &p); err != nil {
			return err
		}
		want := dom.Object{Bucket: "src", Name: "file", Version: "v1"}
		if p.Object != want || p.ID != uid || p.Generation != 7 || p.DeleteEventID != eventID || p.DeleteEventRetryCount != 2 || !p.Deleted {
			return fmt.Errorf("wrong delete retry payload: %+v", p)
		}
		return syncer.RecordDeleteAudit(ctx, repos.DeleteEvents.RecordAudit, p, "deleted", nil)
	}))
	require.NoError(t, srv.Start(worker))
	t.Cleanup(srv.Shutdown)
	require.Eventually(t, func() bool {
		events, _, err := repos.DeleteEvents.List(ctx, controlplane.DeleteEventFilter{Status: "success"})
		return err == nil && len(events) == 1 && events[0].ID == eventID
	}, 3*time.Second, 10*time.Millisecond)
	require.EqualValues(t, 1, calls.Load())
}

func TestDeleteRetryRejectsUnknownGeneration(t *testing.T) {
	ctx := t.Context()
	cp, err := pg.Open(ctx, controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: pgtest.NewDB(t), AutoMigrate: true}})
	require.NoError(t, err)
	t.Cleanup(cp.Close)
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	q := &retryQueue{enqueue: func(context.Context, any, ...asynq.Option) error {
		t.Fatal("delete without the original generation was queued")
		return nil
	}}
	svc := NewService(rc, q, "main").WithControlPlane(cp)
	const id = "u:main:dst"
	_, err = cp.Repos().DeleteEvents.Create(ctx, controlplane.DeleteEvent{ReplicationID: id, Bucket: "src", ObjectKey: "file", Status: "failed", Generation: -1})
	require.NoError(t, err)
	n, err := svc.RetryFailedDeletes(ctx, id)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.ErrorContains(t, err, "original replication generation is unavailable")
	require.Zero(t, n)
}
