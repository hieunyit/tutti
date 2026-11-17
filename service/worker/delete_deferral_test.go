package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/testutil"
	"github.com/hieunyit/tutti/service/worker/syncer"
)

func TestDeferredDeleteDoesNotExhaustRetries(t *testing.T) {
	redis := testutil.SetupRedis(t)
	client := asynq.NewClientFromRedisClient(redis)
	server := asynq.NewServerFromRedisClient(redis, asynq.Config{
		Concurrency:              1,
		IsFailure:                isTaskFailure,
		RetryDelayFunc:           retryDelayFunc(asynq.DefaultRetryDelayFunc),
		TaskCheckInterval:        10 * time.Millisecond,
		DelayedTaskCheckInterval: 10 * time.Millisecond,
	})
	var attempts atomic.Int32
	type resumed struct {
		at      time.Time
		retried int
	}
	done := make(chan resumed, 1)
	deadline := time.Now().Add(250 * time.Millisecond)
	require.NoError(t, server.Start(asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
		if attempts.Add(1) == 1 {
			return &syncer.DeleteDeferredError{Deadline: deadline}
		}
		if retried, _ := asynq.GetRetryCount(ctx); attempts.Load() == 2 {
			done <- resumed{at: time.Now(), retried: retried}
		}
		return nil
	})))
	t.Cleanup(server.Shutdown)
	_, err := client.EnqueueContext(t.Context(), asynq.NewTask("test:delete", nil), asynq.MaxRetry(0))
	require.NoError(t, err)
	select {
	case retry := <-done:
		require.False(t, retry.at.Before(deadline))
		require.Zero(t, retry.retried)
	case <-time.After(5 * time.Second):
		t.Fatal("deferred delete was not resumed")
	}
}
