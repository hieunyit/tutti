package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestActivityStore(t *testing.T) {
	r := require.New(t)
	mr := miniredis.RunT(t)
	s := NewActivityStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	id := "u:a:b"

	got, err := s.Get(ctx, id, 1, "", 100, time.Now())
	r.NoError(err)
	r.Empty(got.InFlight)
	r.Empty(got.Events)
	r.Equal("", got.NextCursor)

	r.NoError(s.Begin(ctx, id, 1, ActivityInFlight{Key: "a.bin", Size: 10, Flow: ActivityFlowInit, Attempt: 1},
		&ActivityEvent{Level: ActivityInfo, Type: ActivityStarted, Key: "a.bin", Size: 10, Flow: ActivityFlowInit, Attempt: 1}))
	r.NoError(s.Begin(ctx, id, 1, ActivityInFlight{Key: "b.bin", Size: 20, Flow: ActivityFlowInit, Attempt: 1}, nil))
	got, err = s.Get(ctx, id, 1, "", 100, time.Now())
	r.NoError(err)
	r.Len(got.InFlight, 2)
	r.Len(got.Events, 1)
	first := got.NextCursor
	r.NotEmpty(first)

	r.NoError(s.End(ctx, id, 1, "a.bin", "", true,
		&ActivityEvent{Level: ActivityInfo, Type: ActivityDone, Key: "a.bin", Size: 10, DurationMs: 500, BytesPerSecond: 20, Flow: ActivityFlowInit, Attempt: 1}))
	r.NoError(s.End(ctx, id, 1, "b.bin", "", true,
		&ActivityEvent{Level: ActivityError, Type: ActivityFailed, Key: "b.bin", Error: strings.Repeat("x", 1000)}))

	got, err = s.Get(ctx, id, 1, first, 100, time.Now())
	r.NoError(err)
	r.Empty(got.InFlight)
	r.Len(got.Events, 2)
	r.Equal("done", got.Events[0].Type)
	r.EqualValues(500, got.Events[0].DurationMs)
	r.Equal("failed", got.Events[1].Type)
	r.LessOrEqual(len(got.Events[1].Error), ActivityMaxErrorLen+3)
	last := got.NextCursor

	// nothing newer: cursor is stable
	got, _ = s.Get(ctx, id, 1, last, 100, time.Now())
	r.Empty(got.Events)
	r.Equal(last, got.NextCursor)

	// limit with an empty cursor returns the newest events in order
	got, _ = s.Get(ctx, id, 1, "", 2, time.Now())
	r.Len(got.Events, 2)
	r.Equal("done", got.Events[0].Type)

	// other generation is isolated
	other, _ := s.Get(ctx, id, 2, "", 100, time.Now())
	r.Empty(other.Events)
}

func TestActivityInFlightCapAndExpiry(t *testing.T) {
	r := require.New(t)
	mr := miniredis.RunT(t)
	s := NewActivityStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	for i := 0; i < ActivityMaxInFlight+20; i++ {
		r.NoError(s.Begin(ctx, "x", 1, ActivityInFlight{Key: "k" + time.Duration(i).String(), Flow: ActivityFlowLive}, nil))
	}
	got, err := s.Get(ctx, "x", 1, "", 10, time.Now())
	r.NoError(err)
	r.Len(got.InFlight, ActivityMaxInFlight)

	// entries older than the safety expiry are not reported
	got, _ = s.Get(ctx, "x", 1, "", 10, time.Now().Add(ActivityInFlightMaxAge+time.Minute))
	r.Empty(got.InFlight)
}

func TestNextStreamID(t *testing.T) {
	require.Equal(t, "5-3", nextStreamID("5-2"))
	require.Equal(t, "5-1", nextStreamID("5"))
}

func TestTruncateActivityError(t *testing.T) {
	r := require.New(t)
	r.Equal("short", TruncateActivityError("short"))
	long := strings.Repeat("é", 400)
	out := TruncateActivityError(long)
	r.True(strings.HasSuffix(out, "..."))
	r.LessOrEqual(len(out), ActivityMaxErrorLen+3)
}
