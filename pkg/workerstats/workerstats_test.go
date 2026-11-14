package workerstats

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestRollingWindows(t *testing.T) {
	clk := &clock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	c := New("w1", clk.now)
	c.SetWorker("w1", 10, 2, 8)

	// 60 objects of 1000 bytes (2s each) in the current second
	for i := 0; i < 60; i++ {
		c.Start()
		c.FinishSuccess(1000, 2*time.Second)
	}
	s := c.Snapshot()
	require.Equal(t, 1.0, s.ObjectsPerSec1m)
	require.Equal(t, 1000.0, s.BytesPerSec1m)
	require.Equal(t, 0.2, s.ObjectsPerSec5m)
	require.Equal(t, 2.0, s.AvgCopySeconds1m)

	// 90s later the 1m window is empty while the 5m window still holds them
	clk.t = clk.t.Add(90 * time.Second)
	s = c.Snapshot()
	require.Zero(t, s.ObjectsPerSec1m)
	require.Zero(t, s.AvgCopySeconds1m)
	require.Equal(t, 0.2, s.ObjectsPerSec5m)
	require.EqualValues(t, 60, s.CopiedObjectsTotal)

	// after 5m everything left the windows, totals stay
	clk.t = clk.t.Add(5 * time.Minute)
	s = c.Snapshot()
	require.Zero(t, s.ObjectsPerSec5m)
	require.EqualValues(t, 60000, s.CopiedBytesTotal)

	c.Start()
	c.FinishFailure()
	c.Start()
	s = c.Snapshot()
	require.EqualValues(t, 1, s.FailedTotal)
	require.EqualValues(t, 1, s.InFlight)
}

func TestPublishWritesSnapshotWithTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	clk := &clock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	c := New("w1", clk.now)
	c.SetWorker("w1", 10, 2, 8)
	c.Start()
	c.FinishSuccess(500, time.Second)

	require.NoError(t, c.Publish(t.Context(), rdb, "app1"))
	raw, err := mr.Get(KeyPrefix + "app1")
	require.NoError(t, err)
	require.Equal(t, TTL, mr.TTL(KeyPrefix+"app1"))

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &m))
	for _, k := range []string{"workerId", "startedAt", "updatedAt", "concurrency", "bulkConcurrency", "liveConcurrency", "inFlight",
		"copiedObjectsTotal", "copiedBytesTotal", "failedTotal", "objectsPerSec1m", "bytesPerSec1m", "objectsPerSec5m", "bytesPerSec5m",
		"avgCopySeconds1m", "bufferTotalBytes", "bufferUsedBytes", "bufferWaiters", "bufferWaitSecondsTotal"} {
		require.Contains(t, m, k)
	}
	require.Equal(t, "w1", m["workerId"])
	require.Equal(t, "2026-01-01T12:00:00Z", m["updatedAt"])
	require.EqualValues(t, 500, m["copiedBytesTotal"])
	require.EqualValues(t, 8, m["liveConcurrency"])
}

func TestReadIgnoresStaleAndAggregates(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	put := func(id string, age time.Duration, s Snapshot) {
		s.WorkerID = id
		s.UpdatedAt = now.Add(-age).Format(time.RFC3339)
		raw, _ := json.Marshal(s)
		require.NoError(t, rdb.Set(t.Context(), KeyPrefix+id, raw, time.Minute).Err())
	}
	put("a", 2*time.Second, Snapshot{Concurrency: 10, InFlight: 3, ObjectsPerSec1m: 1.5, BytesPerSec1m: 100, CopiedObjectsTotal: 10, BufferWaiters: 1})
	put("b", 10*time.Second, Snapshot{Concurrency: 5, InFlight: 2, ObjectsPerSec1m: 0.5, BytesPerSec1m: 50, CopiedObjectsTotal: 5, FailedTotal: 2})
	put("stale", 30*time.Second, Snapshot{Concurrency: 99, InFlight: 99})

	snaps, err := Read(t.Context(), rdb, now)
	require.NoError(t, err)
	require.Len(t, snaps, 2)
	require.Equal(t, "a", snaps[0].WorkerID)
	tot := Aggregate(snaps)
	require.EqualValues(t, 15, tot.Concurrency)
	require.EqualValues(t, 5, tot.InFlight)
	require.Equal(t, 2.0, tot.ObjectsPerSec1m)
	require.Equal(t, 150.0, tot.BytesPerSec1m)
	require.EqualValues(t, 15, tot.CopiedObjectsTotal)
	require.EqualValues(t, 2, tot.FailedTotal)
	require.EqualValues(t, 1, tot.BufferWaiters)
}
