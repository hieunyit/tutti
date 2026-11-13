// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/tasks"
)

type cmdCounter struct{ cmds, trips atomic.Int64 }

func (c *cmdCounter) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) { return next(ctx, network, addr) }
}
func (c *cmdCounter) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error { c.cmds.Add(1); c.trips.Add(1); return next(ctx, cmd) }
}
func (c *cmdCounter) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		c.cmds.Add(int64(len(cmds)))
		c.trips.Add(1)
		return next(ctx, cmds)
	}
}

// perObjectQueue hides the batch capability, reproducing the pre-batching behavior.
type perObjectQueue struct{ tasks.QueueService }

// listOnePage lists a 1000-object bucket against a real broker and returns Redis round trips used.
func listOnePage(t *testing.T, legacy bool, versioned bool) (trips int64, enqueued int) {
	t.Helper()
	const numObjects = 1000
	s, mr, _, bucket := setupTestProducer(t, numObjects)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	counter := &cmdCounter{}
	rc.AddHook(counter)
	client := asynq.NewClientFromRedisClient(rc)
	insp := asynq.NewInspectorFromRedisClient(rc)
	qs := tasks.NewQueueService(client, insp)
	if legacy {
		qs.SetUnprocessedCountCache(0, nil)
		s.queueSvc = perObjectQueue{qs}
	} else {
		s.queueSvc = qs
	}
	ResetQueueAdmissionState()
	t.Cleanup(ResetQueueAdmissionState)

	id := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User: "test-user", FromStorage: "from-storage", ToStorage: "to-storage", FromBucket: bucket, ToBucket: bucket,
	})
	payload := tasks.MigrateBucketListObjectsPayload{Bucket: bucket, Iteration: 1, Versioned: versioned}
	payload.SetReplicationID(id, 1)
	task, err := tasks.EncodeAny(t.Context(), payload)
	require.NoError(t, err)

	counter.trips.Store(0)
	require.NoError(t, s.HandleMigrationBucketListObj(t.Context(), task))
	trips = counter.trips.Load()
	queue := tasks.InitMigrationCopyQueue(id)
	if versioned {
		queue = tasks.InitMigrationListQueue(id)
	}
	info, err := insp.GetQueueInfo(queue)
	require.NoError(t, err)
	return trips, info.Pending
}

func TestMigrationList_BatchedEnqueueCutsRedisRoundTrips(t *testing.T) {
	for _, versioned := range []bool{false, true} {
		before, nBefore := listOnePage(t, true, versioned)
		after, nAfter := listOnePage(t, false, versioned)
		t.Logf("versioned=%v redis round trips per 1000 listed objects: before=%d after=%d (enqueued %d / %d)", versioned, before, after, nBefore, nAfter)
		// versioned pages also enqueue one continuation task into the list queue
		require.Equal(t, nBefore, nAfter, "batched listing must enqueue exactly the same tasks")
		require.GreaterOrEqual(t, nAfter, 1000)
		require.Less(t, after, before/20)
	}
}
