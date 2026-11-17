// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/tasks"
)

func TestDeleteTaskAuditWaitsUntilRetriesExhausted(t *testing.T) {
	ctx := t.Context()
	rc := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { require.NoError(t, rc.Close()) })
	client := asynq.NewClientFromRedisClient(rc)
	records := make(chan controlplane.DeleteAuditEntry, 2)
	auditErrors := make(chan error, 2)
	recorder := func(_ context.Context, entry controlplane.DeleteAuditEntry) error {
		records <- entry
		return nil
	}
	id := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{User: "u", FromStorage: "main", ToStorage: "dst", FromBucket: "src", ToBucket: "replica"})
	p := tasks.ObjectSyncPayload{
		Object: dom.Object{Bucket: "src", Name: "file"}, Deleted: true, DeleteEventID: 10, DeleteEventRetryCount: 4,
	}
	p.SetReplicationID(id, 7)
	task, err := tasks.EncodeAny(ctx, p)
	require.NoError(t, err)
	srv := asynq.NewServerFromRedisClient(rc, asynq.Config{
		Concurrency:              1,
		Queues:                   map[string]int{task.QueueName(): 1},
		TaskCheckInterval:        10 * time.Millisecond,
		DelayedTaskCheckInterval: 10 * time.Millisecond,
		RetryDelayFunc:           func(int, error, *asynq.Task) time.Duration { return 0 },
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			auditErrors <- recordFailedDeleteTask(ctx, recorder, task, err)
		}),
	})
	// The first attempt fails before the syncer. Only the final failure should be recorded.
	failure := errors.New("client lookup failed")
	require.NoError(t, srv.Start(asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
		retried, _ := asynq.GetRetryCount(ctx)
		if retried == 0 {
			return errors.New("initial lookup failed")
		}
		return failure
	})))
	t.Cleanup(srv.Shutdown)
	_, err = client.EnqueueContext(ctx, task, asynq.MaxRetry(1))
	require.NoError(t, err)
	for range 2 {
		select {
		case err := <-auditErrors:
			require.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Fatal("delete task attempt did not reach the error handler")
		}
	}
	require.Len(t, records, 1)
	entry := <-records
	require.Equal(t, p.DeleteEventID, entry.DeleteEventID)
	require.Equal(t, p.DeleteEventRetryCount, entry.DeleteEventRetryCount)
	require.Equal(t, "failed", entry.Action)
	require.Equal(t, "replica", entry.Bucket)
	require.Equal(t, failure.Error(), entry.Metadata["error"])
}
