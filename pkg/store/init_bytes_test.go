package store

import (
	"context"
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
	r.NoError(s.AddDone(ctx, "u:a:b", 1, 600, t0))
	r.NoError(s.AddDone(ctx, "u:a:b", 1, 600, t0.Add(30*time.Second)))

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
