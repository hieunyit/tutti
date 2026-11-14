package handler

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/service/worker/copy"
)

func TestLiveActivityLimiter(t *testing.T) {
	r := require.New(t)
	now := time.Unix(1_000_000, 0)
	allowed := 0
	for i := 0; i < 100; i++ {
		if allowLiveActivity("limiter-test", now) {
			allowed++
		}
	}
	r.Equal(liveActivityPerSecond, allowed)
	r.True(allowLiveActivity("limiter-test", now.Add(time.Second)))
}

func TestActivityTrackerFinish(t *testing.T) {
	r := require.New(t)
	mr := miniredis.RunT(t)
	st := store.NewActivityStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	mk := func(flow string, sampled bool) *activityTracker {
		return &activityTracker{st: st, id: "u:a:b", gen: 1, key: "k", size: 100, flow: flow, attempt: 1, start: time.Now().Add(-time.Second), sampled: sampled}
	}
	types := func() []string {
		got, err := st.Get(ctx, "u:a:b", 1, "", 100, time.Now())
		r.NoError(err)
		var out []string
		for _, e := range got.Events {
			out = append(out, e.Type)
		}
		return out
	}

	mk(store.ActivityFlowInit, true).finish(ctx, nil, &copy.Outcome{Called: true, Bytes: 100})
	mk(store.ActivityFlowInit, true).finish(ctx, nil, &copy.Outcome{Called: true, Skipped: true})
	mk(store.ActivityFlowInit, true).finish(ctx, errors.New("boom"), &copy.Outcome{})
	mk(store.ActivityFlowInit, true).finish(ctx, fmt.Errorf("boom: %w", asynq.SkipRetry), nil) // nil outcome is fine
	r.Equal([]string{"done", "skipped", "failed", "failed"}, types())

	// live flow: unsampled success dropped, unsampled failure kept, no-op sync produces no event
	mk(store.ActivityFlowLive, false).finish(ctx, nil, &copy.Outcome{Called: true, Bytes: 1})
	mk(store.ActivityFlowLive, true).finish(ctx, nil, &copy.Outcome{})
	r.Len(types(), 4)
	mk(store.ActivityFlowLive, false).finish(ctx, errors.New("x"), &copy.Outcome{})
	r.Len(types(), 5)

	var nilTracker *activityTracker
	nilTracker.finish(ctx, nil, nil) // must not panic
}
