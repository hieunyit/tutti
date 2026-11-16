// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package store

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func rescanTestStore(t *testing.T) *RescanStore {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	return NewRescanStore(rc)
}

var rescanT0 = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)

func TestRescanSingleFlight(t *testing.T) {
	ctx := t.Context()
	s := rescanTestStore(t)
	ok, err := s.Start(ctx, "r", 1, "a", rescanT0, time.Time{})
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = s.Start(ctx, "r", 1, "b", rescanT0.Add(time.Minute), time.Time{})
	require.NoError(t, err)
	require.False(t, ok, "a second start while the first runs must be refused")

	st, err := s.Get(ctx, "r", 1, rescanT0.Add(time.Minute))
	require.NoError(t, err)
	require.True(t, st.Running)
	require.Equal(t, "a", st.RunID)

	// the lease expires without any progress: a later scan supersedes it
	ok, err = s.Start(ctx, "r", 1, "c", rescanT0.Add(RescanLease+time.Second), time.Time{})
	require.NoError(t, err)
	require.True(t, ok)
	res, err := s.ObjectDone(ctx, "r", 1, "a", true, rescanT0.Add(RescanLease+2*time.Second))
	require.NoError(t, err)
	require.Equal(t, ScanStale, res, "tasks of the superseded scan are ignored")

	// another generation is independent
	ok, err = s.Start(ctx, "r", 2, "x", rescanT0, time.Time{})
	require.NoError(t, err)
	require.True(t, ok)
}

func TestRescanWatermarkAdvancesOnlyAfterCompleteListingAndCopies(t *testing.T) {
	ctx := t.Context()
	s := rescanTestStore(t)
	_, err := s.Start(ctx, "r", 1, "a", rescanT0, time.Time{})
	require.NoError(t, err)

	// first page: 2 objects, listing continues
	res, err := s.Listed(ctx, "r", 1, "a", 2, false, "b:0", rescanT0)
	require.NoError(t, err)
	require.Equal(t, ScanOK, res)
	// a retried page is counted once
	res, _ = s.Listed(ctx, "r", 1, "a", 2, false, "b:0", rescanT0)
	require.Equal(t, ScanNoop, res)
	// both copies finish before the listing ends: the scan must not complete yet
	res, _ = s.ObjectDone(ctx, "r", 1, "a", true, rescanT0)
	require.Equal(t, ScanOK, res)
	res, _ = s.ObjectDone(ctx, "r", 1, "a", false, rescanT0)
	require.Equal(t, ScanOK, res)
	st, _ := s.Get(ctx, "r", 1, rescanT0)
	require.True(t, st.Watermark.IsZero())
	require.True(t, st.LastStart.IsZero())

	// last page, one more object
	res, _ = s.Listed(ctx, "r", 1, "a", 1, true, "b:1", rescanT0)
	require.Equal(t, ScanOK, res)
	st, _ = s.Get(ctx, "r", 1, rescanT0)
	require.True(t, st.Watermark.IsZero(), "the last copy is still open")
	end := rescanT0.Add(90 * time.Second)
	res, _ = s.ObjectDone(ctx, "r", 1, "a", true, end)
	require.Equal(t, ScanCompleted, res)

	st, _ = s.Get(ctx, "r", 1, end)
	require.False(t, st.Running)
	require.Equal(t, rescanT0, st.Watermark)
	require.Equal(t, rescanT0, st.LastStart)
	require.Equal(t, end, st.LastEnd)
	require.EqualValues(t, 2, st.LastObjects, "only objects that transferred data count")
	require.Empty(t, st.LastError)
}

func TestRescanFailedCopyKeepsWatermark(t *testing.T) {
	ctx := t.Context()
	s := rescanTestStore(t)
	// scan 1 succeeds
	_, _ = s.Start(ctx, "r", 1, "a", rescanT0, time.Time{})
	res, _ := s.Listed(ctx, "r", 1, "a", 0, true, "b:0", rescanT0)
	require.Equal(t, ScanCompleted, res, "an empty listing completes at once")

	// scan 2: one object fails for good
	t1 := rescanT0.Add(time.Hour)
	_, _ = s.Start(ctx, "r", 1, "b", t1, time.Time{})
	_, _ = s.Listed(ctx, "r", 1, "b", 2, true, "b:0", t1)
	_, _ = s.ObjectDone(ctx, "r", 1, "b", true, t1)
	res, _ = s.ObjectFailed(ctx, "r", 1, "b", t1)
	require.Equal(t, ScanCompletedWithFailures, res)
	st, _ := s.Get(ctx, "r", 1, t1)
	require.Equal(t, rescanT0, st.Watermark, "the watermark stays at the last fully successful scan")
	require.Equal(t, t1, st.LastStart)
	require.Contains(t, st.LastError, "1 object copies failed")
	// the failed scan is retried sooner than the interval
	require.Equal(t, t1.Add(RescanRetryAfterFailure), RescanDue(time.Hour, st, t1))
	// the next scan looks back to the old watermark minus the overlap
	require.Equal(t, rescanT0.Add(-10*time.Minute), RescanCutoff(st, nil, rescanT0.Add(-24*time.Hour), 10*time.Minute))
}

func TestRescanAbortKeepsWatermarkAndSetsError(t *testing.T) {
	ctx := t.Context()
	s := rescanTestStore(t)
	_, _ = s.Start(ctx, "r", 1, "a", rescanT0, time.Time{})
	_, _ = s.Listed(ctx, "r", 1, "a", 0, true, "b:0", rescanT0)
	t1 := rescanT0.Add(time.Hour)
	_, _ = s.Start(ctx, "r", 1, "b", t1, time.Time{})
	require.NoError(t, s.Abort(ctx, "r", 1, "b", "listing the source failed: boom", t1))
	st, _ := s.Get(ctx, "r", 1, t1)
	require.False(t, st.Running)
	require.Equal(t, rescanT0, st.Watermark)
	require.Equal(t, rescanT0, st.LastStart, "last_rescan_at is the start of the last COMPLETED scan")
	require.Equal(t, t1, st.LastAttempt)
	require.Equal(t, "listing the source failed: boom", st.LastError)
}

func TestRescanUserLevelFanOut(t *testing.T) {
	ctx := t.Context()
	s := rescanTestStore(t)
	_, _ = s.Start(ctx, "r", 1, "a", rescanT0, time.Time{})
	// the user task opens 2 bucket listings, then closes its own
	_, _ = s.OpenLists(ctx, "r", 1, "a", 2, rescanT0)
	res, _ := s.OpenLists(ctx, "r", 1, "a", 2, rescanT0)
	require.Equal(t, ScanNoop, res, "a retried fan-out is counted once")
	_, _ = s.Listed(ctx, "r", 1, "a", 0, true, "user", rescanT0)
	_, _ = s.Listed(ctx, "r", 1, "a", 0, true, "b1:0", rescanT0)
	st, _ := s.Get(ctx, "r", 1, rescanT0)
	require.True(t, st.Running, "bucket 2 is still being listed")
	res, _ = s.Listed(ctx, "r", 1, "a", 0, true, "b2:0", rescanT0)
	require.Equal(t, ScanCompleted, res)
}

func TestRescanDeleteAndFull(t *testing.T) {
	ctx := t.Context()
	s := rescanTestStore(t)
	_, _ = s.Start(ctx, "r", 1, "a", rescanT0, time.Time{})
	_, _ = s.Listed(ctx, "r", 1, "a", 0, true, "b:0", rescanT0)
	require.NoError(t, s.RequestFull(ctx, "r", 1))
	st, _ := s.Get(ctx, "r", 1, rescanT0)
	require.True(t, st.Full)
	require.True(t, st.Watermark.IsZero(), "a catch-up scan forgets the old watermark")
	require.True(t, RescanCutoff(st, nil, rescanT0, 10*time.Minute).Equal(time.Time{}.UTC()))
	since := rescanT0.Add(-48 * time.Hour)
	require.Equal(t, since, RescanCutoff(st, &since, rescanT0, 10*time.Minute))
	// a successful scan clears the flag
	_, _ = s.Start(ctx, "r", 1, "b", rescanT0.Add(time.Hour), time.Time{})
	_, _ = s.Listed(ctx, "r", 1, "b", 0, true, "b:0", rescanT0)
	st, _ = s.Get(ctx, "r", 1, rescanT0.Add(time.Hour))
	require.False(t, st.Full)

	require.NoError(t, s.Delete(ctx, "r", 1))
	st, _ = s.Get(ctx, "r", 1, rescanT0)
	require.Equal(t, RescanState{}, st)
}

func TestRescanDue(t *testing.T) {
	now := rescanT0
	iv := time.Hour
	for _, tc := range []struct {
		name string
		st   RescanState
		want time.Time
	}{
		{"never seen: a short delay from now", RescanState{}, now.Add(RescanFirstDelay)},
		{"armed: first scan a short delay after the initial copy was seen", RescanState{Armed: now.Add(-time.Hour)}, now.Add(-time.Hour).Add(RescanFirstDelay)},
		{"after a scan: last start plus interval", RescanState{LastAttempt: now.Add(-20 * time.Minute), Armed: now.Add(-3 * time.Hour)}, now.Add(40 * time.Minute)},
		{"failed scan: retry within 15 minutes", RescanState{LastAttempt: now.Add(-5 * time.Minute), LastError: "x"}, now.Add(10 * time.Minute)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, RescanDue(iv, tc.st, now))
		})
	}
	require.Equal(t, now.Add(10*time.Minute), RescanDue(10*time.Minute, RescanState{LastAttempt: now, LastError: "x"}, now), "a short interval is not stretched")
}

func TestRescanCutoff(t *testing.T) {
	ov := 10 * time.Minute
	created := rescanT0.Add(-100 * time.Hour)
	wm := rescanT0.Add(-2 * time.Hour)
	since := func(d time.Duration) *time.Time { v := rescanT0.Add(d); return &v }
	for _, tc := range []struct {
		name  string
		st    RescanState
		since *time.Time
		want  time.Time
	}{
		{"first scan looks back to the creation minus the margin", RescanState{}, nil, created.Add(-ov)},
		{"first scan never goes before since", RescanState{}, since(-10 * time.Hour), rescanT0.Add(-10 * time.Hour)},
		{"later scan: watermark minus margin", RescanState{Watermark: wm}, nil, wm.Add(-ov)},
		{"later scan never before since", RescanState{Watermark: wm}, since(-time.Hour), rescanT0.Add(-time.Hour)},
		{"since older than the watermark does not widen it", RescanState{Watermark: wm}, since(-99 * time.Hour), wm.Add(-ov)},
		{"catch-up: whole source", RescanState{Full: true, Watermark: wm}, nil, time.Time{}},
		{"catch-up honours since", RescanState{Full: true}, since(-5 * time.Hour), rescanT0.Add(-5 * time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, tc.want.Equal(RescanCutoff(tc.st, tc.since, created, ov)), "got %v want %v", RescanCutoff(tc.st, tc.since, created, ov), tc.want)
		})
	}
}
