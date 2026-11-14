package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestInitBytesStore(t *testing.T) {
	r := require.New(t)
	mr := miniredis.RunT(t)
	s := NewInitBytesStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}))
	ctx := context.Background()
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	got, err := s.Get(ctx, "u:a:b", 1, t0)
	r.NoError(err)
	r.Equal(InitBytes{}, got)

	r.NoError(s.AddTotal(ctx, "u:a:b", 1, 1000))
	r.NoError(s.AddTotal(ctx, "u:a:b", 1, 500))
	r.NoError(s.AddDone(ctx, "u:a:b", 1, 600, "a\x00", t0))
	r.NoError(s.AddDone(ctx, "u:a:b", 1, 600, "b\x00", t0.Add(30*time.Second)))

	got, err = s.Get(ctx, "u:a:b", 1, t0.Add(60*time.Second))
	r.NoError(err)
	r.EqualValues(1500, got.Total)
	r.EqualValues(1200, got.Done)
	r.EqualValues(20, got.BytesPerSecond) // 1200 bytes over 60s since the first completion

	// other generation is isolated
	other, _ := s.Get(ctx, "u:a:b", 2, t0)
	r.Equal(InitBytes{}, other)

	// completions older than the window no longer count
	got, _ = s.Get(ctx, "u:a:b", 1, t0.Add(20*time.Minute))
	r.EqualValues(0, got.BytesPerSecond)
	r.EqualValues(1200, got.Done)
}

func TestRollingRate(t *testing.T) {
	now := time.Date(2026, 1, 1, 10, 7, 0, 0, time.UTC)
	r := require.New(t)
	r.EqualValues(0, rollingRate(nil, 0, now))
	// first completion 5s ago: elapsed floors to 10s
	r.EqualValues(100, rollingRate(map[string]string{"1767262020": "1000"}, now.Unix()-5, now))
}

func newProgressStore(t *testing.T) (*InitBytesStore, *redis.Client, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewInitBytesStore(c), c, mr
}

func TestInitBytesProgress(t *testing.T) {
	r := require.New(t)
	s, _, mr := newProgressStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	const id = "u:a:b"
	big, small := ProgressField("big", ""), ProgressField("small", "v1")
	r.NoError(s.AddTotal(ctx, id, 1, 1000+50))
	get := func(at time.Time) InitBytes {
		got, err := s.Get(ctx, id, 1, at)
		r.NoError(err)
		return got
	}

	// absolute updates: repeating a value is idempotent, bytes count while the copy runs
	r.NoError(s.SetProgress(ctx, id, 1, big, 0, 1000, t0))
	r.NoError(s.SetProgress(ctx, id, 1, big, 200, 1000, t0.Add(10*time.Second)))
	r.NoError(s.SetProgress(ctx, id, 1, big, 200, 1000, t0.Add(11*time.Second)))
	r.NoError(s.SetProgress(ctx, id, 1, small, 20, 50, t0.Add(12*time.Second)))
	got := get(t0.Add(20 * time.Second))
	r.EqualValues(220, got.Done)
	r.EqualValues(22, got.BytesPerSecond) // 220 bytes in 10 s since the first transferred byte

	// never above the object size
	r.NoError(s.SetProgress(ctx, id, 1, big, 5000, 1000, t0.Add(21*time.Second)))
	r.EqualValues(1020, get(t0.Add(22*time.Second)).Done)

	// a retry restarts from 0: done drops, the speed only counts growth (no negative delta)
	before := get(t0.Add(30 * time.Second)).BytesPerSecond
	r.NoError(s.SetProgress(ctx, id, 1, big, 0, 1000, t0.Add(30*time.Second)))
	r.EqualValues(20, get(t0.Add(30*time.Second)).Done)
	r.GreaterOrEqual(get(t0.Add(30*time.Second)).BytesPerSecond, int64(0))
	r.NotZero(before)
	r.NoError(s.SetProgress(ctx, id, 1, big, 100, 1000, t0.Add(31*time.Second)))
	r.EqualValues(120, get(t0.Add(31*time.Second)).Done)

	// finish: progress is removed in the same step and the size is added to done exactly once
	r.NoError(s.AddDone(ctx, id, 1, 1000, big, t0.Add(40*time.Second)))
	got = get(t0.Add(40 * time.Second))
	r.EqualValues(1020, got.Done)
	r.NoError(s.AddDone(ctx, id, 1, 50, small, t0.Add(41*time.Second)))
	got = get(t0.Add(41 * time.Second))
	r.EqualValues(1050, got.Done)
	r.EqualValues(1050, got.Total)
	_, _, prog := initBytesKeys(id, 1)
	r.False(mr.Exists(prog), "the progress hash is removed with its last object")

	// an object finished without progress (small single PUT) counts fully for the speed
	r.NoError(s.AddDone(ctx, id, 2, 600, ProgressField("x", ""), t0))
	r.EqualValues(10, mustGet(t, s, id, 2, t0.Add(60*time.Second)).BytesPerSecond)
}

func mustGet(t *testing.T, s *InitBytesStore, id string, gen int64, at time.Time) InitBytes {
	got, err := s.Get(context.Background(), id, gen, at)
	require.NoError(t, err)
	return got
}

func TestInitBytesClearProgress(t *testing.T) {
	r := require.New(t)
	s, _, mr := newProgressStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	f := ProgressField("k", "")
	r.NoError(s.AddTotal(ctx, "u:a:b", 1, 100))
	r.NoError(s.SetProgress(ctx, "u:a:b", 1, f, 40, 100, t0))
	r.EqualValues(40, mustGet(t, s, "u:a:b", 1, t0).Done)
	r.NoError(s.ClearProgress(ctx, "u:a:b", 1, f))
	r.NoError(s.ClearProgress(ctx, "u:a:b", 1, f)) // idempotent
	r.EqualValues(0, mustGet(t, s, "u:a:b", 1, t0).Done)
	_, _, prog := initBytesKeys("u:a:b", 1)
	r.False(mr.Exists(prog))
	r.NoError(s.SetProgress(ctx, "u:a:b", 1, f, 10, 100, t0))
	r.True(mr.TTL(prog) > 0)
}

func TestInitBytesProgressCap(t *testing.T) {
	r := require.New(t)
	s, _, _ := newProgressStore(t)
	ctx := context.Background()
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	for i := 0; i < InitBytesMaxProgress+50; i++ {
		r.NoError(s.SetProgress(ctx, "u:a:b", 1, ProgressField(fmt.Sprintf("k%d", i), ""), 1, 10, t0))
	}
	// the sum only covers tracked objects; the extra ones are ignored, existing ones can still be updated
	r.EqualValues(InitBytesMaxProgress, mustGet(t, s, "u:a:b", 1, t0).Done)
	r.NoError(s.SetProgress(ctx, "u:a:b", 1, ProgressField("k0", ""), 5, 10, t0))
	r.EqualValues(InitBytesMaxProgress+4, mustGet(t, s, "u:a:b", 1, t0).Done)
}

func TestActivityInFlightBytesDone(t *testing.T) {
	r := require.New(t)
	s, c, _ := newProgressStore(t)
	ctx := context.Background()
	act := NewActivityStore(c)
	r.NoError(act.Begin(ctx, "u:a:b", 1, ActivityInFlight{Key: "big", Size: 1000, Flow: ActivityFlowInit, Attempt: 1}, nil))
	r.NoError(s.SetProgress(ctx, "u:a:b", 1, ProgressField("big", ""), 612, 1000, time.Now()))
	got, err := act.Get(ctx, "u:a:b", 1, "", 10, time.Now())
	r.NoError(err)
	r.Len(got.InFlight, 1)
	r.EqualValues(612, got.InFlight[0].BytesDone)
}
