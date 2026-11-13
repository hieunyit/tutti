// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

type tripCounter struct{ n atomic.Int64 }

func (c *tripCounter) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) { return next(ctx, network, addr) }
}
func (c *tripCounter) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error { c.n.Add(1); return next(ctx, cmd) }
}
func (c *tripCounter) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error { c.n.Add(1); return next(ctx, cmds) }
}

func newSvc(t *testing.T, addr string, conf map[string]RateLimit) (*Svc, *tripCounter) {
	rc := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = rc.Close() })
	c := &tripCounter{}
	rc.AddHook(c)
	return New(rc, conf), c
}

func isLimited(err error) (*dom.ErrRateLimitExceeded, bool) {
	rl, ok := err.(*dom.ErrRateLimitExceeded)
	return rl, ok
}

func TestStorReqMulti_AllOrNothingAndSameSemantics(t *testing.T) {
	mr := miniredis.RunT(t)
	s, _ := newSvc(t, mr.Addr(), map[string]RateLimit{
		"a": {Enabled: true, IncludeMetadataAPI: true, RPM: 100},
		"b": {Enabled: true, IncludeMetadataAPI: true, RPM: 2},
		"c": {Enabled: false, RPM: 1},
	})
	ctx := t.Context()
	pair := []StorageReq{{Storage: "a", N: 1}, {Storage: "b", N: 1}, {Storage: "c", N: 5}}
	require.NoError(t, s.StorReqMulti(ctx, pair...))
	require.NoError(t, s.StorReqMulti(ctx, pair...))
	err := s.StorReqMulti(ctx, pair...)
	rl, ok := isLimited(err)
	require.True(t, ok, "third pair must be rejected with ErrRateLimitExceeded, got %v", err)
	require.Greater(t, rl.RetryIn, time.Duration(0))
	// rejected request consumed nothing on "a": exactly 98 tokens remain
	require.NoError(t, s.StorReqN(ctx, "a", 98))
	_, ok = isLimited(s.StorReq(ctx, "a"))
	require.True(t, ok)
	// same storage on both sides is merged: needs 2 tokens of "b"
	s2, _ := newSvc(t, miniredis.RunT(t).Addr(), map[string]RateLimit{"b": {Enabled: true, IncludeMetadataAPI: true, RPM: 2}})
	require.NoError(t, s2.StorReqMulti(ctx, StorageReq{Storage: "b", N: 1}, StorageReq{Storage: "b", N: 1}))
	_, ok = isLimited(s2.StorReq(ctx, "b"))
	require.True(t, ok)
}

func TestStorReqMulti_UsesOneRoundTripPerPair(t *testing.T) {
	mr := miniredis.RunT(t)
	conf := map[string]RateLimit{
		"src": {Enabled: true, IncludeMetadataAPI: true, RPM: 1000000},
		"dst": {Enabled: true, IncludeMetadataAPI: true, RPM: 1000000},
	}
	s, c := newSvc(t, mr.Addr(), conf)
	ctx := t.Context()
	// warm up script cache
	require.NoError(t, s.StorReqMulti(ctx, StorageReq{Storage: "src", N: 1}, StorageReq{Storage: "dst", N: 1}))
	require.NoError(t, s.StorReq(ctx, "src"))
	c.n.Store(0)
	for i := 0; i < 1000; i++ {
		require.NoError(t, s.StorReqMulti(ctx, StorageReq{Storage: "src", N: 1}, StorageReq{Storage: "dst", N: 1}))
	}
	multi := c.n.Load()
	c.n.Store(0)
	for i := 0; i < 1000; i++ {
		require.NoError(t, s.StorReq(ctx, "src"))
		require.NoError(t, s.StorReq(ctx, "dst"))
	}
	sequential := c.n.Load()
	t.Logf("redis round trips per 1000 objects (src+dst limit): sequential=%d multi-key=%d", sequential, multi)
	require.Equal(t, int64(1000), multi)
	require.Equal(t, int64(2000), sequential)
}

func TestReservation_DisabledOrOneBehavesLikeToday(t *testing.T) {
	for _, batch := range []int{0, 1} {
		mr := miniredis.RunT(t)
		s, c := newSvc(t, mr.Addr(), map[string]RateLimit{"a": {Enabled: true, IncludeMetadataAPI: true, RPM: 1000}})
		s.SetReserveBatch(batch)
		require.NoError(t, s.StorReq(t.Context(), "a"))
		c.n.Store(0)
		for i := 0; i < 100; i++ {
			require.NoError(t, s.StorReq(t.Context(), "a"))
		}
		require.Equal(t, int64(100), c.n.Load(), "one EVAL per request when reservation is off (batch=%d)", batch)
	}
}

func TestReservation_CutsRoundTripsAndLeaseExpires(t *testing.T) {
	mr := miniredis.RunT(t)
	s, c := newSvc(t, mr.Addr(), map[string]RateLimit{"a": {Enabled: true, IncludeMetadataAPI: true, RPM: 100000}})
	now := time.Now()
	s.SetClock(func() time.Time { return now })
	s.SetReserveBatch(10)
	ctx := t.Context()
	require.NoError(t, s.StorReq(ctx, "a")) // warm script
	now = now.Add(2 * reserveLeaseTTL)      // drop warm-up lease
	c.n.Store(0)
	for i := 0; i < 100; i++ {
		require.NoError(t, s.StorReq(ctx, "a"))
	}
	t.Logf("redis round trips per 100 requests: batch=10 -> %d (off -> 100)", c.n.Load())
	require.LessOrEqual(t, c.n.Load(), int64(11))
	// leftover tokens are not spendable after the lease TTL
	c.n.Store(0)
	require.NoError(t, s.StorReq(ctx, "a")) // new lease, 9 left
	require.Equal(t, int64(1), c.n.Load())
	now = now.Add(reserveLeaseTTL + time.Millisecond)
	require.NoError(t, s.StorReq(ctx, "a"))
	require.Equal(t, int64(2), c.n.Load(), "expired lease must trigger a new reservation")
}

// Two simulated worker processes with local reservation must never consume more than the shared bucket allows.
func TestReservation_TwoWorkersNeverExceedLimit(t *testing.T) {
	for _, batch := range []int{0, 1, 7, 50} {
		mr := miniredis.RunT(t)
		conf := map[string]RateLimit{"src": {Enabled: true, IncludeMetadataAPI: true, RPM: 100}, "dst": {Enabled: true, IncludeMetadataAPI: true, RPM: 60}}
		w1, _ := newSvc(t, mr.Addr(), conf)
		w2, _ := newSvc(t, mr.Addr(), conf)
		w1.SetReserveBatch(batch)
		w2.SetReserveBatch(batch)
		var srcOnly, pairs atomic.Int64
		start := time.Now()
		var wg sync.WaitGroup
		for _, w := range []*Svc{w1, w2} {
			wg.Add(2)
			go func() {
				defer wg.Done()
				for i := 0; i < 200; i++ {
					if err := w.StorReq(context.Background(), "src"); err == nil {
						srcOnly.Add(1)
					} else if _, ok := isLimited(err); !ok {
						t.Error(err)
					}
				}
			}()
			go func() {
				defer wg.Done()
				for i := 0; i < 200; i++ {
					err := w.StorReqMulti(context.Background(), StorageReq{Storage: "src", N: 1}, StorageReq{Storage: "dst", N: 1})
					if err == nil {
						pairs.Add(1)
					} else if _, ok := isLimited(err); !ok {
						t.Error(err)
					}
				}
			}()
		}
		wg.Wait()
		elapsed := time.Since(start)
		// GCRA: burst + refill during the test
		srcMax := int64(100 + 1 + int(elapsed.Seconds()/0.6))
		dstMax := int64(60 + 1 + int(elapsed.Seconds()/1.0))
		require.LessOrEqual(t, srcOnly.Load()+pairs.Load(), srcMax, "src over limit, batch=%d", batch)
		require.LessOrEqual(t, pairs.Load(), dstMax, "dst over limit, batch=%d", batch)
		require.Greater(t, srcOnly.Load()+pairs.Load(), int64(0))
		t.Logf("batch=%d src used=%d (max %d) dst used=%d (max %d)", batch, srcOnly.Load()+pairs.Load(), srcMax, pairs.Load(), dstMax)
	}
}
