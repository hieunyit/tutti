// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	rescanKeyPrefix = "p:repl_rescan:"
	// RescanLease is how long a running scan stays owned without any progress signal. Every listed page, completed
	// copy and scheduler tick over a non empty rescan queue renews it. A scan whose lease expired (all of its tasks
	// were lost, for example a purged queue) is superseded by the next scheduled scan.
	RescanLease = 30 * time.Minute
	// RescanFirstDelay is how long after the scheduler first sees a finished initial copy the first scan runs.
	RescanFirstDelay = 2 * time.Minute
	// RescanRetryAfterFailure caps the wait for the next scan after a failed one.
	RescanRetryAfterFailure = 15 * time.Minute

	rescanStateTTL = 60 * 24 * time.Hour
	rescanPagesTTL = 7 * 24 * time.Hour
)

// RescanState is the runtime state of the periodic rescan of one replication.
type RescanState struct {
	// Running reports a scan whose lease has not expired.
	Running bool
	// RunID, RunStart and RunCutoff describe the current (or last unfinished) scan.
	RunID     string
	RunStart  time.Time
	RunCutoff time.Time
	// Listed and Done count source objects enqueued by the current scan and finished (or failed) ones.
	Listed, Done, Failed int64
	// LastStart and LastEnd are start and end of the last completed scan (zero when none completed yet).
	LastStart, LastEnd time.Time
	// LastObjects is the number of objects the last completed scan actually copied.
	LastObjects int64
	// LastError is empty when the last completed or attempted scan succeeded.
	LastError string
	// Watermark is the start of the last scan that listed everything and copied every object successfully.
	Watermark time.Time
	// LastAttempt is the start of the last scan attempt, completed or not.
	LastAttempt time.Time
	// Armed is when the scheduler first saw the finished initial copy of this generation.
	Armed time.Time
	// Full requests a catch-up scan of the whole source (set when rescan is enabled on an existing replication).
	Full bool
}

// RescanStore keeps the rescan state in Redis (the queue database, next to the initial copy byte counters).
//
// Keys are scoped by replication ID and generation, so a re-created replication starts clean.
//
//	p:repl_rescan:{<id>:<gen>}        hash, see the fields below
//	p:repl_rescan:{<id>:<gen>}:pages  set of listing pages already counted (makes a retried page idempotent)
type RescanStore struct {
	client redis.Cmdable
}

func NewRescanStore(client redis.Cmdable) *RescanStore { return &RescanStore{client: client} }

func rescanKeys(replID string, generation int64) (state, pages string) {
	base := fmt.Sprintf("%s{%s:%d}", rescanKeyPrefix, replID, generation)
	return base, base + ":pages"
}

func ms(t time.Time) int64 { return t.UnixMilli() }

func fromMs(v string) time.Time {
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(n).UTC()
}

func atoi(v string) int64 {
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

// KEYS: state, pages. ARGV: runID, now, cutoff, lease, ttl.
var rescanStartScript = redis.NewScript(`
local rid = redis.call('HGET', KEYS[1], 'run_id')
local lease = tonumber(redis.call('HGET', KEYS[1], 'lease') or '0')
if rid and rid ~= '' and lease > tonumber(ARGV[2]) then return 0 end
redis.call('HSET', KEYS[1], 'run_id', ARGV[1], 'run_start', ARGV[2], 'run_cutoff', ARGV[3],
  'lease', tonumber(ARGV[2]) + tonumber(ARGV[4]), 'run_listed', 0, 'run_done', 0, 'run_failed', 0,
  'run_copied', 0, 'lists_open', 1, 'last_attempt', ARGV[2])
redis.call('DEL', KEYS[2])
redis.call('EXPIRE', KEYS[1], ARGV[5])
return 1
`)

// KEYS: state, pages. ARGV: runID, op, a, b, now, lease, pagesTTL, token, message.
// Returns -1 for a stale run, 0 for a no-op (page already counted), 1 applied, 2 applied and the scan completed
// successfully, 3 applied and the scan completed with failed copies.
var rescanApplyScript = redis.NewScript(`
local rid = redis.call('HGET', KEYS[1], 'run_id')
if not rid or rid == '' or rid ~= ARGV[1] then return -1 end
local op = ARGV[2]
local now = tonumber(ARGV[5])
redis.call('HSET', KEYS[1], 'lease', now + tonumber(ARGV[6]))
if (op == 'listed' or op == 'open') and ARGV[8] ~= '' then
  -- a retried page or fan-out is counted once
  if redis.call('SADD', KEYS[2], ARGV[8]) == 0 then return 0 end
  redis.call('EXPIRE', KEYS[2], ARGV[7])
end
if op == 'listed' then
  -- a = objects enqueued by the page, b = 1 when it was the last page of its listing chain
  redis.call('HINCRBY', KEYS[1], 'run_listed', ARGV[3])
  if ARGV[4] == '1' then redis.call('HINCRBY', KEYS[1], 'lists_open', -1) end
elseif op == 'open' then
  redis.call('HINCRBY', KEYS[1], 'lists_open', ARGV[3])
elseif op == 'done' then
  -- a = 1 when the task transferred data
  redis.call('HINCRBY', KEYS[1], 'run_done', 1)
  if ARGV[3] == '1' then redis.call('HINCRBY', KEYS[1], 'run_copied', 1) end
elseif op == 'failed' then
  redis.call('HINCRBY', KEYS[1], 'run_failed', 1)
elseif op == 'abort' then
  redis.call('HSET', KEYS[1], 'last_error', ARGV[9], 'run_id', '', 'lease', 0)
  return 1
end
local open = tonumber(redis.call('HGET', KEYS[1], 'lists_open') or '0')
if open > 0 then return 1 end
local listed = tonumber(redis.call('HGET', KEYS[1], 'run_listed') or '0')
local done = tonumber(redis.call('HGET', KEYS[1], 'run_done') or '0')
local failed = tonumber(redis.call('HGET', KEYS[1], 'run_failed') or '0')
if done + failed < listed then return 1 end
local start = redis.call('HGET', KEYS[1], 'run_start')
redis.call('HSET', KEYS[1], 'last_start', start, 'last_end', ARGV[5], 'last_objects',
  redis.call('HGET', KEYS[1], 'run_copied') or '0', 'run_id', '', 'lease', 0)
if failed == 0 then
  redis.call('HSET', KEYS[1], 'wm', start, 'last_error', '')
  redis.call('HDEL', KEYS[1], 'full')
  return 2
end
redis.call('HSET', KEYS[1], 'last_error', failed .. ' object copies failed after all retries, the next scan retries them')
return 3
`)

// Start begins a scan unless one is running. started is false when another scan owns the replication.
func (s *RescanStore) Start(ctx context.Context, replID string, generation int64, runID string, now, cutoff time.Time) (started bool, err error) {
	state, pages := rescanKeys(replID, generation)
	var cut int64
	if !cutoff.IsZero() {
		cut = ms(cutoff)
	}
	n, err := rescanStartScript.Run(ctx, s.client, []string{state, pages}, runID, ms(now), cut, RescanLease.Milliseconds(), int64(rescanStateTTL.Seconds())).Int()
	return n == 1, err
}

// ScanResult is what Apply reports about the scan.
type ScanResult int

const (
	ScanStale ScanResult = -1 // the task belongs to a superseded or finished scan
	ScanNoop  ScanResult = 0
	ScanOK    ScanResult = 1
	// ScanCompleted and ScanCompletedWithFailures mean this call finished the scan.
	ScanCompleted             ScanResult = 2
	ScanCompletedWithFailures ScanResult = 3
)

func (s *RescanStore) apply(ctx context.Context, replID string, generation int64, runID, op string, a, b int64, token, msg string, now time.Time) (ScanResult, error) {
	state, pages := rescanKeys(replID, generation)
	n, err := rescanApplyScript.Run(ctx, s.client, []string{state, pages}, runID, op, a, b, ms(now), RescanLease.Milliseconds(), int64(rescanPagesTTL.Seconds()), token, msg).Int()
	return ScanResult(n), err
}

// Listed registers a listing page that enqueued n copy-stage tasks. last marks the end of its listing chain.
// token identifies the page: a retried page is counted once.
func (s *RescanStore) Listed(ctx context.Context, replID string, generation int64, runID string, n int64, last bool, token string, now time.Time) (ScanResult, error) {
	b := int64(0)
	if last {
		b = 1
	}
	return s.apply(ctx, replID, generation, runID, "listed", n, b, token, "", now)
}

// OpenLists registers n more listing chains (user-level replications start one per bucket).
func (s *RescanStore) OpenLists(ctx context.Context, replID string, generation int64, runID string, n int64, now time.Time) (ScanResult, error) {
	return s.apply(ctx, replID, generation, runID, "open", n, 0, "open", "", now)
}

// ObjectDone registers a copy-stage task that finished successfully. copied tells whether data was transferred.
func (s *RescanStore) ObjectDone(ctx context.Context, replID string, generation int64, runID string, copied bool, now time.Time) (ScanResult, error) {
	a := int64(0)
	if copied {
		a = 1
	}
	return s.apply(ctx, replID, generation, runID, "done", a, 0, "", "", now)
}

// ObjectFailed registers a task that failed for good (retries exhausted).
func (s *RescanStore) ObjectFailed(ctx context.Context, replID string, generation int64, runID string, now time.Time) (ScanResult, error) {
	return s.apply(ctx, replID, generation, runID, "failed", 0, 0, "", "", now)
}

// Renew extends the lease of the running scan.
func (s *RescanStore) Renew(ctx context.Context, replID string, generation int64, runID string, now time.Time) (ScanResult, error) {
	return s.apply(ctx, replID, generation, runID, "renew", 0, 0, "", "", now)
}

// Abort ends the scan with an error without advancing the watermark.
func (s *RescanStore) Abort(ctx context.Context, replID string, generation int64, runID, message string, now time.Time) error {
	_, err := s.apply(ctx, replID, generation, runID, "abort", 0, 0, "", message, now)
	return err
}

// Arm records when the scheduler first saw the finished initial copy (first writer wins).
func (s *RescanStore) Arm(ctx context.Context, replID string, generation int64, now time.Time) error {
	state, _ := rescanKeys(replID, generation)
	pipe := s.client.Pipeline()
	pipe.HSetNX(ctx, state, "armed_at", ms(now))
	pipe.Expire(ctx, state, rescanStateTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// RequestFull asks for a catch-up scan of the whole source (rescan was just enabled on an existing replication).
func (s *RescanStore) RequestFull(ctx context.Context, replID string, generation int64) error {
	state, _ := rescanKeys(replID, generation)
	pipe := s.client.Pipeline()
	pipe.HSet(ctx, state, "full", 1)
	pipe.HDel(ctx, state, "armed_at", "wm")
	pipe.Expire(ctx, state, rescanStateTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// Delete removes all rescan state of the replication generation.
func (s *RescanStore) Delete(ctx context.Context, replID string, generation int64) error {
	state, pages := rescanKeys(replID, generation)
	return s.client.Del(ctx, state, pages).Err()
}

// Get reads the state. A replication that never scanned yields the zero state.
func (s *RescanStore) Get(ctx context.Context, replID string, generation int64, now time.Time) (RescanState, error) {
	state, _ := rescanKeys(replID, generation)
	h, err := s.client.HGetAll(ctx, state).Result()
	if err != nil {
		return RescanState{}, err
	}
	st := RescanState{
		RunID:       h["run_id"],
		RunStart:    fromMs(h["run_start"]),
		RunCutoff:   fromMs(h["run_cutoff"]),
		Listed:      atoi(h["run_listed"]),
		Done:        atoi(h["run_done"]),
		Failed:      atoi(h["run_failed"]),
		LastStart:   fromMs(h["last_start"]),
		LastEnd:     fromMs(h["last_end"]),
		LastObjects: atoi(h["last_objects"]),
		LastError:   h["last_error"],
		Watermark:   fromMs(h["wm"]),
		LastAttempt: fromMs(h["last_attempt"]),
		Armed:       fromMs(h["armed_at"]),
		Full:        h["full"] == "1",
	}
	st.Running = st.RunID != "" && atoi(h["lease"]) > ms(now)
	return st, nil
}

// RescanDue returns when the next scan of a replication with the given interval is due. now is used for a replication
// the scheduler has not seen yet (first scan: a short delay after it is first seen).
func RescanDue(interval time.Duration, st RescanState, now time.Time) time.Time {
	if !st.LastAttempt.IsZero() {
		wait := interval
		if st.LastError != "" && wait > RescanRetryAfterFailure {
			wait = RescanRetryAfterFailure
		}
		return st.LastAttempt.Add(wait)
	}
	if st.Armed.IsZero() {
		return now.Add(RescanFirstDelay)
	}
	return st.Armed.Add(RescanFirstDelay)
}

// RescanCutoff returns the cutoff of the next scan: source objects last modified before it are skipped. Zero means
// the whole source. since is the replication cutoff of the initial copy, createdAt its creation time and overlap the
// safety margin before the previous successful scan.
//
// The first scan of a replication that had rescan from the start only looks back to its creation (the initial copy
// covers everything older). A catch-up scan (rescan enabled later) starts from since or the beginning.
func RescanCutoff(st RescanState, since *time.Time, createdAt time.Time, overlap time.Duration) time.Time {
	var base time.Time
	switch {
	case st.Full:
		// whole source
	case !st.Watermark.IsZero():
		base = st.Watermark.Add(-overlap)
	case !createdAt.IsZero():
		base = createdAt.Add(-overlap)
	}
	if since != nil && since.After(base) {
		base = *since
	}
	return base.UTC()
}
