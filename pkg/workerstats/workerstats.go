// Package workerstats keeps in-memory replication throughput counters of a worker process, publishes a JSON
// snapshot to Redis every few seconds and reads/aggregates the snapshots of all live workers.
//
// The counters are updated in memory only (no per-object Redis writes); the publisher writes one key per
// worker, tutti:worker:stats:<appID>, with a short TTL so a stopped worker disappears by itself.
package workerstats

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/systemsettings"
)

const (
	// KeyPrefix is the Redis key prefix of worker snapshots.
	KeyPrefix = "tutti:worker:stats:"
	// PublishInterval is the snapshot publish period.
	PublishInterval = 5 * time.Second
	// TTL is the lifetime of a snapshot key; older snapshots are also ignored when reading.
	TTL = 20 * time.Second

	shortWindow = time.Minute
	longWindow  = 5 * time.Minute
)

type bucket struct {
	sec      int64
	objects  int64
	bytes    int64
	durNanos int64
}

// Counters holds the throughput counters of one worker process.
type Counters struct {
	now      func() time.Time
	started  time.Time
	inFlight atomic.Int64

	mu       sync.Mutex
	objects  int64
	bytes    int64
	failed   int64
	buckets  [300]bucket // one per second, covers the longest (5m) window
	workerID string

	concurrency, bulk, live int
}

// New creates counters. now may be nil (time.Now).
func New(workerID string, now func() time.Time) *Counters {
	if now == nil {
		now = time.Now
	}
	return &Counters{now: now, started: now().UTC(), workerID: workerID}
}

// Default is the process-wide instance fed by the copy service.
var Default = New("", nil)

// SetWorker sets the worker id and the startup concurrency split reported in snapshots.
func (c *Counters) SetWorker(id string, concurrency, bulk, live int) {
	c.mu.Lock()
	c.workerID, c.concurrency, c.bulk, c.live = id, concurrency, bulk, live
	c.mu.Unlock()
}

// Start marks a copy as running; call Finish* when it ends.
func (c *Counters) Start() { c.inFlight.Add(1) }

// FinishSuccess records a successfully copied object.
func (c *Counters) FinishSuccess(bytes int64, d time.Duration) {
	c.inFlight.Add(-1)
	sec := c.now().Unix()
	c.mu.Lock()
	c.objects++
	c.bytes += bytes
	b := c.slot(sec)
	b.objects++
	b.bytes += bytes
	b.durNanos += int64(d)
	c.mu.Unlock()
}

// Skip ends a copy that transferred nothing (the destination was already up to date).
func (c *Counters) Skip() { c.inFlight.Add(-1) }

// FinishFailure records a failed copy.
func (c *Counters) FinishFailure() {
	c.inFlight.Add(-1)
	c.mu.Lock()
	c.failed++
	c.mu.Unlock()
}

func (c *Counters) slot(sec int64) *bucket {
	b := &c.buckets[sec%int64(len(c.buckets))]
	if b.sec != sec {
		*b = bucket{sec: sec}
	}
	return b
}

// window sums the buckets of the last w seconds (including the current one).
func (c *Counters) window(now time.Time, w time.Duration) (objects, bytes, durNanos int64) {
	sec := now.Unix()
	from := sec - int64(w/time.Second)
	for i := range c.buckets {
		b := c.buckets[i]
		if b.sec > from && b.sec <= sec {
			objects += b.objects
			bytes += b.bytes
			durNanos += b.durNanos
		}
	}
	return
}

// Snapshot is the document published to Redis; field names are the REST camelCase names.
type Snapshot struct {
	WorkerID               string  `json:"workerId"`
	StartedAt              string  `json:"startedAt"`
	UpdatedAt              string  `json:"updatedAt"`
	Concurrency            int     `json:"concurrency"`
	BulkConcurrency        int     `json:"bulkConcurrency"`
	LiveConcurrency        int     `json:"liveConcurrency"`
	InFlight               int64   `json:"inFlight"`
	CopiedObjectsTotal     int64   `json:"copiedObjectsTotal"`
	CopiedBytesTotal       int64   `json:"copiedBytesTotal"`
	FailedTotal            int64   `json:"failedTotal"`
	ObjectsPerSec1m        float64 `json:"objectsPerSec1m"`
	BytesPerSec1m          float64 `json:"bytesPerSec1m"`
	ObjectsPerSec5m        float64 `json:"objectsPerSec5m"`
	BytesPerSec5m          float64 `json:"bytesPerSec5m"`
	AvgCopySeconds1m       float64 `json:"avgCopySeconds1m"`
	BufferTotalBytes       int64   `json:"bufferTotalBytes"`
	BufferUsedBytes        int64   `json:"bufferUsedBytes"`
	BufferWaiters          int64   `json:"bufferWaiters"`
	BufferWaitSecondsTotal float64 `json:"bufferWaitSecondsTotal"`
}

// Snapshot computes the current snapshot.
func (c *Counters) Snapshot() Snapshot {
	now := c.now()
	buf := systemsettings.CurrentBufferStats()
	c.mu.Lock()
	o1, b1, d1 := c.window(now, shortWindow)
	o5, b5, _ := c.window(now, longWindow)
	s := Snapshot{
		WorkerID: c.workerID, StartedAt: c.started.Format(time.RFC3339), UpdatedAt: now.UTC().Format(time.RFC3339),
		Concurrency: c.concurrency, BulkConcurrency: c.bulk, LiveConcurrency: c.live,
		CopiedObjectsTotal: c.objects, CopiedBytesTotal: c.bytes, FailedTotal: c.failed,
		ObjectsPerSec1m: float64(o1) / shortWindow.Seconds(), BytesPerSec1m: float64(b1) / shortWindow.Seconds(),
		ObjectsPerSec5m: float64(o5) / longWindow.Seconds(), BytesPerSec5m: float64(b5) / longWindow.Seconds(),
	}
	c.mu.Unlock()
	if o1 > 0 {
		s.AvgCopySeconds1m = time.Duration(d1).Seconds() / float64(o1)
	}
	s.InFlight = c.inFlight.Load()
	s.BufferTotalBytes, s.BufferUsedBytes, s.BufferWaiters, s.BufferWaitSecondsTotal = buf.TotalBytes, buf.UsedBytes, buf.Waiters, buf.WaitSeconds
	return s
}

// Publish writes the snapshot to Redis with the TTL.
func (c *Counters) Publish(ctx context.Context, rdb redis.Cmdable, appID string) error {
	raw, err := json.Marshal(c.Snapshot())
	if err != nil {
		return err
	}
	return rdb.Set(ctx, KeyPrefix+appID, raw, TTL).Err()
}

// Run publishes every PublishInterval until ctx is done.
func (c *Counters) Run(ctx context.Context, rdb redis.Cmdable, appID string, logger zerolog.Logger) {
	t := time.NewTicker(PublishInterval)
	defer t.Stop()
	for {
		if err := c.Publish(ctx, rdb, appID); err != nil && ctx.Err() == nil {
			logger.Warn().Err(err).Msg("worker stats publish failed")
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Totals are the sums across live workers.
type Totals struct {
	InFlight           int64
	Concurrency        int64
	ObjectsPerSec1m    float64
	BytesPerSec1m      float64
	ObjectsPerSec5m    float64
	BytesPerSec5m      float64
	BufferTotalBytes   int64
	BufferUsedBytes    int64
	BufferWaiters      int64
	CopiedObjectsTotal int64
	CopiedBytesTotal   int64
	FailedTotal        int64
}

// Read returns the live snapshots (updatedAt not older than TTL, relative to now) sorted by worker id.
func Read(ctx context.Context, rdb redis.Cmdable, now time.Time) ([]Snapshot, error) {
	var out []Snapshot
	var cursor uint64
	for {
		keys, next, err := rdb.Scan(ctx, cursor, KeyPrefix+"*", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			raw, err := rdb.Get(ctx, k).Bytes()
			if err != nil {
				continue // expired between scan and get
			}
			var s Snapshot
			if json.Unmarshal(raw, &s) != nil {
				continue
			}
			out = append(out, s)
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	live := Filter(out, now)
	sort.Slice(live, func(i, j int) bool { return live[i].WorkerID < live[j].WorkerID })
	return live, nil
}

// Filter drops snapshots older than TTL.
func Filter(all []Snapshot, now time.Time) []Snapshot {
	var live []Snapshot
	for _, s := range all {
		t, err := time.Parse(time.RFC3339, s.UpdatedAt)
		if err != nil || now.Sub(t) > TTL {
			continue
		}
		live = append(live, s)
	}
	return live
}

// Aggregate sums the snapshots.
func Aggregate(snaps []Snapshot) Totals {
	var t Totals
	for _, s := range snaps {
		t.InFlight += s.InFlight
		t.Concurrency += int64(s.Concurrency)
		t.ObjectsPerSec1m += s.ObjectsPerSec1m
		t.BytesPerSec1m += s.BytesPerSec1m
		t.ObjectsPerSec5m += s.ObjectsPerSec5m
		t.BytesPerSec5m += s.BytesPerSec5m
		t.BufferTotalBytes += s.BufferTotalBytes
		t.BufferUsedBytes += s.BufferUsedBytes
		t.BufferWaiters += s.BufferWaiters
		t.CopiedObjectsTotal += s.CopiedObjectsTotal
		t.CopiedBytesTotal += s.CopiedBytesTotal
		t.FailedTotal += s.FailedTotal
	}
	return t
}
