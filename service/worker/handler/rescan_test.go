// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/rescan"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
)

func rescanKeysOf(rec *mockQueueRecorder) (copies []string, lists []tasks.RescanListObjectsPayload) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, task := range rec.tasks {
		switch p := task.(type) {
		case tasks.RescanObjCopyPayload:
			copies = append(copies, p.Obj.Name)
		case tasks.RescanListObjectsPayload:
			lists = append(lists, p)
		case *tasks.RescanListObjectsPayload:
			lists = append(lists, *p)
		}
	}
	return
}

func runRescanList(t *testing.T, s *svc, p tasks.RescanListObjectsPayload) error {
	t.Helper()
	task, err := tasks.EncodeAny(t.Context(), p)
	require.NoError(t, err)
	return s.HandleRescanListObjects(t.Context(), task)
}

func newRun(t *testing.T, s *svc, id entity.UniversalReplicationID, runID string, start, cutoff time.Time) *store.RescanStore {
	t.Helper()
	rs := s.rescanStore()
	ok, err := rs.Start(t.Context(), id.AsString(), 1, runID, start, cutoff)
	require.NoError(t, err)
	require.True(t, ok)
	return rs
}

func rescanPayload(id entity.UniversalReplicationID, bucket, runID string, cutoff time.Time) tasks.RescanListObjectsPayload {
	p := tasks.RescanListObjectsPayload{Bucket: bucket, RescanRun: tasks.RescanRun{RunID: runID, Cutoff: cutoff}}
	p.SetReplicationID(id, 1)
	return p
}

func TestRescanListHonoursCutoffAndKeepsInitStateUntouched(t *testing.T) {
	s, rec, mr, bucket, id := sinceEnv(t)
	now := time.Now()
	rs := newRun(t, s, id, "r1", now, sinceBase.Add(30*time.Minute))

	require.NoError(t, runRescanList(t, s, rescanPayload(id, bucket, "r1", sinceBase.Add(30*time.Minute))))
	copies, lists := rescanKeysOf(rec)
	require.ElementsMatch(t, []string{"new-2", "new-3"}, copies)
	require.Empty(t, lists, "a bucket that fits one page has no continuation")

	st, err := rs.Get(t.Context(), id.AsString(), 1, now)
	require.NoError(t, err)
	require.True(t, st.Running)
	require.EqualValues(t, 2, st.Listed)

	// nothing of the initial copy was touched: no init byte counters, no list checkpoint, no init payload
	for _, k := range mr.Keys() {
		require.False(t, strings.HasPrefix(k, "p:repl_init_bytes:"), "init counter key %s", k)
	}
	b, err := s.initBytes().Get(t.Context(), id.AsString(), 1, now)
	require.NoError(t, err)
	require.Zero(t, b.Total)
	_, err = s.listStateStore.Get(t.Context(), entity.NewMigrationObjectIDFromUniversalReplicationID(id, bucket, ""))
	require.Error(t, err, "no list checkpoint of the initial copy is written")
	rec.mu.Lock()
	defer rec.mu.Unlock()
	for _, task := range rec.tasks {
		switch task.(type) {
		case tasks.MigrateObjCopyPayload, tasks.MigrateBucketListObjectsPayload, tasks.ListObjectVersionsPayload:
			t.Fatalf("a rescan must not enqueue initial copy tasks: %T", task)
		}
	}
}

func TestRescanCompletesAfterAllCopiesAndAdvancesWatermark(t *testing.T) {
	s, rec, _, bucket, id := sinceEnv(t)
	start := time.Now().Add(-time.Minute)
	rs := newRun(t, s, id, "r1", start, time.Time{})
	require.NoError(t, runRescanList(t, s, rescanPayload(id, bucket, "r1", time.Time{})))
	copies, _ := rescanKeysOf(rec)
	require.Len(t, copies, 6)

	for i := 0; i < 5; i++ {
		s.rescanSettle(t.Context(), id, 1, "r1", nil, i < 2)
	}
	st, _ := rs.Get(t.Context(), id.AsString(), 1, time.Now())
	require.True(t, st.Running)
	require.True(t, st.Watermark.IsZero(), "the watermark moves only when every copy finished")
	s.rescanSettle(t.Context(), id, 1, "r1", nil, false)
	st, _ = rs.Get(t.Context(), id.AsString(), 1, time.Now())
	require.False(t, st.Running)
	require.WithinDuration(t, start, st.Watermark, time.Millisecond)
	require.EqualValues(t, 2, st.LastObjects)
}

func TestRescanFailedCopyDoesNotAdvanceWatermark(t *testing.T) {
	s, _, _, bucket, id := sinceEnv(t)
	rs := s.rescanStore()
	// scan 1 completes
	t0 := time.Now().Add(-2 * time.Hour)
	newRun(t, s, id, "r0", t0, time.Time{})
	_, err := rs.Listed(t.Context(), id.AsString(), 1, "r0", 0, true, "x", t0)
	require.NoError(t, err)

	t1 := time.Now().Add(-time.Hour)
	_, err = rs.Start(t.Context(), id.AsString(), 1, "r1", t1, time.Time{})
	require.NoError(t, err)
	require.NoError(t, runRescanList(t, s, rescanPayload(id, bucket, "r1", time.Time{})))
	for i := 0; i < 5; i++ {
		s.rescanSettle(t.Context(), id, 1, "r1", nil, false)
	}
	// the last task fails for good (no asynq retry info in the test context: the attempt is final)
	s.rescanSettle(t.Context(), id, 1, "r1", context.DeadlineExceeded, false)
	st, _ := rs.Get(t.Context(), id.AsString(), 1, time.Now())
	require.False(t, st.Running)
	require.WithinDuration(t, t0, st.Watermark, time.Millisecond, "watermark stays at the last fully successful scan")
	require.Contains(t, st.LastError, "1 object copies failed")
}

func TestRescanListingErrorAbortsWithoutAdvancing(t *testing.T) {
	s, _, _, _, _ := sinceEnv(t)
	id := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User: "test-user", FromStorage: "from-storage", ToStorage: "to-storage", FromBucket: "ghost", ToBucket: "ghost",
	})
	rs := newRun(t, s, id, "r1", time.Now(), time.Time{})
	err := runRescanList(t, s, rescanPayload(id, "ghost", "r1", time.Time{}))
	require.Error(t, err)
	st, _ := rs.Get(t.Context(), id.AsString(), 1, time.Now())
	require.False(t, st.Running, "an aborted scan releases the replication")
	require.True(t, st.Watermark.IsZero())
	require.True(t, st.LastStart.IsZero())
	require.Contains(t, st.LastError, "listing the source failed")
}

func TestRescanStaleRunIsDropped(t *testing.T) {
	s, rec, _, bucket, id := sinceEnv(t)
	newRun(t, s, id, "current", time.Now(), time.Time{})
	require.NoError(t, runRescanList(t, s, rescanPayload(id, bucket, "old", time.Time{})))
	copies, _ := rescanKeysOf(rec)
	require.Empty(t, copies)
}

func TestRescanBackpressureDefersWithoutCounting(t *testing.T) {
	s, rec, _, bucket, id := sinceEnv(t)
	rs := newRun(t, s, id, "r1", time.Now(), time.Time{})
	rec.unprocessed = 1_000_000
	require.NoError(t, runRescanList(t, s, rescanPayload(id, bucket, "r1", time.Time{})))
	copies, lists := rescanKeysOf(rec)
	require.Empty(t, copies)
	require.Len(t, lists, 1)
	require.True(t, lists[0].Backpressured)
	require.Equal(t, 1, lists[0].Iteration)
	st, _ := rs.Get(t.Context(), id.AsString(), 1, time.Now())
	require.Zero(t, st.Listed)
	require.True(t, st.Running)
}

func TestRescanOverlapPicksUpObjectWrittenBeforePreviousScanStart(t *testing.T) {
	s, rec, _, _, id := sinceEnv(t)
	rs := s.rescanStore()
	// the previous scan started at base+1h05m and completed: new-2 (base+1h) was written 5 minutes before it
	prevStart := sinceBase.Add(65 * time.Minute)
	newRun(t, s, id, "prev", prevStart, time.Time{})
	_, err := rs.Listed(t.Context(), id.AsString(), 1, "prev", 0, true, "x", prevStart)
	require.NoError(t, err)

	st, _ := rs.Get(t.Context(), id.AsString(), 1, prevStart)
	require.Equal(t, prevStart, st.Watermark)
	cutoff := store.RescanCutoff(st, nil, sinceBase.Add(-30*24*time.Hour), entity.RescanOverlap)
	require.Equal(t, sinceBase.Add(55*time.Minute), cutoff)

	now := prevStart.Add(time.Hour)
	starter := &rescan.Starter{Store: rs, Queue: rec, Now: func() time.Time { return now }}
	started, runID, err := starter.Start(t.Context(), rescan.Target{ID: id, Generation: 1, Interval: time.Hour, CreatedAt: sinceBase.Add(-30 * 24 * time.Hour)})
	require.NoError(t, err)
	require.True(t, started)
	_, lists := rescanKeysOf(rec)
	require.Len(t, lists, 1)
	require.Equal(t, cutoff, lists[0].Cutoff)
	require.Equal(t, runID, lists[0].RunID)

	require.NoError(t, runRescanList(t, s, lists[0]))
	copies, _ := rescanKeysOf(rec)
	require.ElementsMatch(t, []string{"new-2", "new-3"}, copies, "new-2 (5 min before the previous scan start) is within the overlap, new-1 is not")
}

func TestRescanDoesNotChangeInitDoneInputs(t *testing.T) {
	// the rescan queues are separate from the queues that define the initial copy state
	id := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{User: "u", FromStorage: "a", ToStorage: "b", FromBucket: "x", ToBucket: "x"})
	require.NotEqual(t, tasks.InitMigrationCopyQueue(id), tasks.RescanCopyQueue(id))
	require.NotEqual(t, tasks.InitMigrationListQueue(id), tasks.RescanListQueue(id))
	require.Contains(t, tasks.AllReplicationQueues(id), tasks.RescanCopyQueue(id), "pause, resume and delete cover the rescan queues")
	require.Contains(t, tasks.AllReplicationQueues(id), tasks.RescanListQueue(id))
}

// scheduler

type schedEnv struct {
	rs  *store.RescanStore
	rec *mockQueueRecorder
	rc  *redis.Client
	id  entity.UniversalReplicationID
	now time.Time
	mu  sync.Mutex
}

func (e *schedEnv) clock() time.Time { e.mu.Lock(); defer e.mu.Unlock(); return e.now }
func (e *schedEnv) advance(d time.Duration) {
	e.mu.Lock()
	e.now = e.now.Add(d)
	e.mu.Unlock()
}

func newSchedEnv(t *testing.T) (*schedEnv, func(targets ...rescan.Target) *RescanScheduler) {
	_, rec, mr, _, id := sinceEnv(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	e := &schedEnv{rs: store.NewRescanStore(rc), rec: rec, rc: rc, id: id, now: time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)}
	return e, func(targets ...rescan.Target) *RescanScheduler {
		return &RescanScheduler{
			Store: e.rs, Queue: rec, Now: e.clock,
			Targets: func(context.Context) ([]rescan.Target, error) { return targets, nil },
		}
	}
}

func TestRescanSchedulerDueTimes(t *testing.T) {
	e, mk := newSchedEnv(t)
	target := rescan.Target{ID: e.id, Generation: 1, Interval: time.Hour, CreatedAt: e.now.Add(-24 * time.Hour)}
	sched := mk(target)
	ctx := t.Context()
	starts := func() int { _, l := rescanKeysOf(e.rec); return len(l) }

	// first sight arms the replication, the first scan is due a short delay later
	require.Equal(t, 0, sched.RunOnce(ctx))
	e.advance(store.RescanFirstDelay - time.Second)
	require.Equal(t, 0, sched.RunOnce(ctx))
	e.advance(2 * time.Second)
	require.Equal(t, 1, sched.RunOnce(ctx))
	require.Equal(t, 1, starts())

	// while it runs (live lease) nothing else starts
	e.advance(10 * time.Minute)
	require.Equal(t, 0, sched.RunOnce(ctx))

}

func TestRescanSchedulerNextScanAfterInterval(t *testing.T) {
	e, mk := newSchedEnv(t)
	target := rescan.Target{ID: e.id, Generation: 1, Interval: 30 * time.Minute, CreatedAt: e.now.Add(-24 * time.Hour)}
	sched := mk(target)
	ctx := t.Context()
	require.Equal(t, 0, sched.RunOnce(ctx))
	e.advance(store.RescanFirstDelay)
	require.Equal(t, 1, sched.RunOnce(ctx))
	st, _ := e.rs.Get(ctx, e.id.AsString(), 1, e.now)
	_, _ = e.rs.Listed(ctx, e.id.AsString(), 1, st.RunID, 0, true, "x", e.now)

	e.advance(29 * time.Minute)
	require.Equal(t, 0, sched.RunOnce(ctx))
	e.advance(2 * time.Minute)
	require.Equal(t, 1, sched.RunOnce(ctx))
	_, lists := rescanKeysOf(e.rec)
	require.Len(t, lists, 2)
	require.True(t, lists[1].Cutoff.After(lists[0].Cutoff), "the second scan starts from the first scan's start minus the margin")
}

func TestRescanSchedulerSingleFlightAcrossSchedulers(t *testing.T) {
	e, mk := newSchedEnv(t)
	target := rescan.Target{ID: e.id, Generation: 1, Interval: time.Hour, CreatedAt: e.now.Add(-time.Hour)}
	a, b := mk(target), mk(target)
	ctx := t.Context()
	a.RunOnce(ctx)
	b.RunOnce(ctx) // both see it for the first time
	e.advance(5 * time.Minute)
	var wg sync.WaitGroup
	counts := make([]int, 8)
	for i := range counts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sched := a
			if i%2 == 1 {
				sched = b
			}
			counts[i] = sched.RunOnce(ctx)
		}()
	}
	wg.Wait()
	total := 0
	for _, c := range counts {
		total += c
	}
	require.Equal(t, 1, total, "exactly one scheduler starts the scan")
	_, lists := rescanKeysOf(e.rec)
	require.Len(t, lists, 1)
}

func TestRescanSchedulerRenewsLeaseOfScanWithQueuedTasks(t *testing.T) {
	e, mk := newSchedEnv(t)
	target := rescan.Target{ID: e.id, Generation: 1, Interval: time.Hour, CreatedAt: e.now.Add(-time.Hour)}
	sched := mk(target)
	ctx := t.Context()
	sched.RunOnce(ctx)
	e.advance(store.RescanFirstDelay)
	require.Equal(t, 1, sched.RunOnce(ctx))

	e.rec.unprocessed = 5 // tasks of the scan are still queued (long copies, busy queue)
	e.advance(store.RescanLease - time.Minute)
	require.Equal(t, 0, sched.RunOnce(ctx))
	e.advance(5 * time.Minute) // past the original lease
	st, _ := e.rs.Get(ctx, e.id.AsString(), 1, e.now)
	require.True(t, st.Running, "the scheduler kept the lease alive")
	require.Equal(t, 0, sched.RunOnce(ctx))

	// all its tasks vanished (purged queue) and the lease ran out: the next tick supersedes the dead scan
	e.rec.unprocessed = 0
	e.advance(store.RescanLease + time.Minute)
	require.Equal(t, 1, sched.RunOnce(ctx))
}

func TestRescanSchedulerIgnoresReplicationsNotListedAsTargets(t *testing.T) {
	e, mk := newSchedEnv(t)
	sched := mk() // paused, archived, rescan off or initial copy not done: the provider does not return them
	e.advance(24 * time.Hour)
	require.Equal(t, 0, sched.RunOnce(t.Context()))
	_, lists := rescanKeysOf(e.rec)
	require.Empty(t, lists)
}
