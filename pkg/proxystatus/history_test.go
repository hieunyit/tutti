package proxystatus

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/trafficstats"
)

func TestHistory_AppendCapAndTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	ctx := t.Context()
	now := time.Now().Unix()
	for i := 0; i < HistoryMaxEntries+50; i++ {
		require.NoError(t, AppendSample(ctx, rc, "p1", Sample{Time: now - int64(HistoryMaxEntries+50-i)*10, Rates: trafficstats.Rates{RequestsPerSec: float64(i)}}))
	}
	n, err := rc.LLen(ctx, HistoryKey("p1")).Result()
	require.NoError(t, err)
	require.EqualValues(t, HistoryMaxEntries, n)
	require.Equal(t, HistoryTTL, mr.TTL(HistoryKey("p1")))

	// newest sample is kept, read returns oldest first and respects since
	res, err := ReadHistory(ctx, rc, time.Now().Add(-5*time.Minute))
	require.NoError(t, err)
	list := res["p1"]
	require.NotEmpty(t, list)
	require.LessOrEqual(t, len(list), 32)
	require.EqualValues(t, HistoryMaxEntries+49, list[len(list)-1].Rates.RequestsPerSec)
	for i := 1; i < len(list); i++ {
		require.Less(t, list[i-1].Time, list[i].Time)
	}
}

func TestAggregate(t *testing.T) {
	r := func(rps, lat, p95 float64) trafficstats.Rates {
		return trafficstats.Rates{RequestsPerSec: rps, LatencyAvgMs: lat, LatencyP95Ms: p95, ByStorage: map[string]float64{"s": rps}}
	}
	per := map[string][]Sample{
		"a": {{Time: 100, Rates: r(10, 10, 20)}, {Time: 110, Rates: r(20, 10, 40)}, {Time: 130, Rates: r(5, 5, 5)}},
		"b": {{Time: 100, Rates: r(30, 50, 60)}, {Time: 140, Rates: r(1, 1, 1)}},
	}
	// 10s buckets: summed across proxies
	pts := Aggregate(per, 10*time.Second)
	require.Len(t, pts, 4) // 100, 110, 130, 140
	require.Equal(t, int64(100), pts[0].Time.Unix())
	require.InDelta(t, 40, pts[0].Rates.RequestsPerSec, 1e-9)
	require.InDelta(t, 40, pts[0].Rates.LatencyAvgMs, 1e-9) // (10*10+30*50)/40
	require.InDelta(t, 60, pts[0].Rates.LatencyP95Ms, 1e-9)
	require.InDelta(t, 40, pts[0].Rates.ByStorage["s"], 1e-9)
	require.InDelta(t, 20, pts[1].Rates.RequestsPerSec, 1e-9)

	// 30s buckets (downsample): bucket 90..119 holds a:100,110 and b:100 -> a averaged (15) + b (30)
	pts = Aggregate(per, 30*time.Second)
	require.Len(t, pts, 2)
	require.Equal(t, int64(90), pts[0].Time.Unix())
	require.InDelta(t, 45, pts[0].Rates.RequestsPerSec, 1e-9)
	require.InDelta(t, 60, pts[0].Rates.LatencyP95Ms, 1e-9) // max(40 (a), 60 (b))
	require.Equal(t, int64(120), pts[1].Time.Unix())
	require.InDelta(t, 6, pts[1].Rates.RequestsPerSec, 1e-9) // a:130 -> 5, b:140 -> 1

	require.Empty(t, Aggregate(nil, time.Minute))
}

func TestBucketForRange(t *testing.T) {
	require.Equal(t, 10*time.Second, BucketForRange(5*time.Minute))
	require.Equal(t, 10*time.Second, BucketForRange(15*time.Minute))
	require.Equal(t, 30*time.Second, BucketForRange(time.Hour))
	require.Equal(t, 2*time.Minute, BucketForRange(6*time.Hour))
}
