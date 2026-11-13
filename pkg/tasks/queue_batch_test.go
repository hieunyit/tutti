// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package tasks

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

// roundTripCounter counts Redis commands/round trips issued through a go-redis client.
// A pipeline counts as one round trip, an EVAL as one.
type roundTripCounter struct{ n atomic.Int64 }

func (c *roundTripCounter) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) { return next(ctx, network, addr) }
}
func (c *roundTripCounter) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error { c.n.Add(1); return next(ctx, cmd) }
}
func (c *roundTripCounter) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error { c.n.Add(1); return next(ctx, cmds) }
}

func TestEnqueueTasks_BatchEquivalentToSingleEnqueue(t *testing.T) {
	_, rc, client, insp, id, queue := atomicAdmissionSetup(t, 0)
	qs := NewQueueService(client, insp)
	var payloads []any
	for i := 0; i < 10; i++ {
		payloads = append(payloads, atomicAdmissionPayload(id, fmt.Sprintf("obj-%02d", i)))
	}
	n, err := qs.EnqueueTasks(t.Context(), payloads)
	require.NoError(t, err)
	require.Equal(t, 10, n)
	st, err := qs.Stats(t.Context(), queue)
	require.NoError(t, err)
	require.Equal(t, 10, st.Unprocessed)
	for i := 0; i < 10; i++ {
		_, err := insp.GetTaskInfo(queue, migrateObjCopy.taskID(atomicAdmissionPayload(id, fmt.Sprintf("obj-%02d", i))))
		require.NoError(t, err)
	}
	// re-enqueue: all duplicates are accepted idempotently, nothing added
	n, err = qs.EnqueueTasks(t.Context(), payloads)
	require.NoError(t, err)
	require.Equal(t, 10, n)
	st, _ = qs.Stats(t.Context(), queue)
	require.Equal(t, 10, st.Unprocessed)
	_ = rc
}

func TestEnqueueTasks_AdmissionStopsBatchAtWatermark(t *testing.T) {
	_, _, client, insp, id, queue := atomicAdmissionSetup(t, 4)
	qs := NewQueueService(client, insp)
	var payloads []any
	for i := 0; i < 10; i++ {
		payloads = append(payloads, atomicAdmissionPayload(id, fmt.Sprintf("obj-%02d", i)))
	}
	n, err := qs.EnqueueTasks(t.Context(), payloads)
	var rl *dom.ErrRateLimitExceeded
	require.ErrorAs(t, err, &rl)
	require.Equal(t, 4, n, "exactly the first 4 are accepted, in order")
	st, _ := qs.Stats(t.Context(), queue)
	require.Equal(t, 4, st.Unprocessed)
	for i := 4; i < 10; i++ {
		_, err := insp.GetTaskInfo(queue, migrateObjCopy.taskID(atomicAdmissionPayload(id, fmt.Sprintf("obj-%02d", i))))
		require.Error(t, err, "rejected task must not exist")
	}
	// a duplicate at full capacity stays idempotent
	n, err = qs.EnqueueTasks(t.Context(), payloads[:4])
	require.NoError(t, err)
	require.Equal(t, 4, n)
}

func TestEnqueueTasks_ConcurrentBatchesNeverExceedWatermark(t *testing.T) {
	_, rc, client, insp, id, queue := atomicAdmissionSetup(t, 50)
	other := asynq.NewClientFromRedisClient(rc)
	defer other.Close()
	qa, qb := NewQueueService(client, insp), NewQueueService(other, insp)
	done := make(chan struct{})
	for w, qs := range []*queueService{qa, qb} {
		go func() {
			defer func() { done <- struct{}{} }()
			var payloads []any
			for i := 0; i < 100; i++ {
				payloads = append(payloads, atomicAdmissionPayload(id, fmt.Sprintf("w%d-%03d", w, i)))
			}
			_, _ = qs.EnqueueTasks(context.Background(), payloads)
		}()
	}
	<-done
	<-done
	st, err := qa.Stats(t.Context(), queue)
	require.NoError(t, err)
	require.Equal(t, 50, st.Unprocessed)
}

func TestEnqueueTasks_ThreeRoundTripsFor600Tasks(t *testing.T) {
	_, rc, client, insp, id, _ := atomicAdmissionSetup(t, 0)
	_ = client
	counter := &roundTripCounter{}
	rc.AddHook(counter)
	qs := NewQueueService(asynq.NewClientFromRedisClient(rc), insp)
	var payloads []any
	for i := 0; i < 250; i++ {
		payloads = append(payloads, atomicAdmissionPayload(id, fmt.Sprintf("obj-%04d", i)))
	}
	counter.n.Store(0)
	_, err := qs.EnqueueTasks(t.Context(), payloads)
	require.NoError(t, err)
	require.LessOrEqual(t, counter.n.Load(), int64(2), "one EVAL (plus at most one script load) for 250 tasks")
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func TestUnprocessedCount_CachedForTTLAndOptimisticallyUpdated(t *testing.T) {
	_, rc, client, insp, id, queue := atomicAdmissionSetup(t, 0)
	counter := &roundTripCounter{}
	rc.AddHook(counter)
	qs := NewQueueService(client, insp)
	clk := &fakeClock{t: time.Unix(1000, 0)}
	qs.SetUnprocessedCountCache(300*time.Millisecond, clk.now)
	ctx := t.Context()

	require.NoError(t, qs.EnqueueTask(ctx, atomicAdmissionPayload(id, "seed")))
	c, err := qs.UnprocessedCount(ctx, false, queue)
	require.NoError(t, err)
	require.Equal(t, 1, c)

	// served from cache: no Redis traffic
	before := counter.n.Load()
	for i := 0; i < 5; i++ {
		c, err = qs.UnprocessedCount(ctx, false, queue)
		require.NoError(t, err)
		require.Equal(t, 1, c)
	}
	require.Equal(t, before, counter.n.Load(), "cache hits must not touch Redis")

	// optimistic update after enqueues (single and batch)
	require.NoError(t, qs.EnqueueTask(ctx, atomicAdmissionPayload(id, "a")))
	_, err = qs.EnqueueTasks(ctx, []any{atomicAdmissionPayload(id, "b"), atomicAdmissionPayload(id, "c")})
	require.NoError(t, err)
	c, _ = qs.UnprocessedCount(ctx, false, queue)
	require.Equal(t, 4, c)

	// an external change is invisible inside the TTL but visible right after it
	_, err = insp.DeleteAllPendingTasks(queue)
	require.NoError(t, err)
	clk.t = clk.t.Add(299 * time.Millisecond)
	c, _ = qs.UnprocessedCount(ctx, false, queue)
	require.Equal(t, 4, c)
	clk.t = clk.t.Add(2 * time.Millisecond)
	c, _ = qs.UnprocessedCount(ctx, false, queue)
	require.Equal(t, 0, c, "cache must never outlive its TTL")
}
