// Copyright 2025 Clyso GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"golang.org/x/sync/singleflight"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
)

/*
Proxy policy snapshot.

Without it every S3 request reads routing, routing blocks, zero downtime switches, replication policies and
replication generations from Redis (one MULTI/EXEC, ~8 commands). With it the proxy keeps the resolved
data per (user, bucket) in memory, in a state object tagged with the config version:

  - Writers bump SnapshotVersionKey and PUBLISH on SnapshotChannel (change_notify.go).
  - The proxy refreshes the version on a notification and on a poll (default 1s). A different version swaps
    in an empty state (atomic pointer swap); entries are filled lazily, one Redis transaction per
    (user, bucket) after each change. The previous state is kept as last-known-good.
  - Read requests (GET/HEAD/LIST: ReadOnly(ctx)) use the state without touching Redis. They can be at most
    one notification latency (or the poll interval if pub/sub is down) behind a change.
  - All other requests, which may record replication events, first read the version key (1 command) and
    refresh when it changed, so a write never uses routing, generation or switch data older than the latest
    committed change that was announced. If Redis is unreachable they fail with dom.ErrUnavailable (503).

Staleness limits: a reader may use data that is not newer than the last bump it has seen. The bump is in
the same MULTI/EXEC as transactional changes (zero downtime switch creation, replication policy changes),
so a validated write never sees a half applied change; non-transactional single-command changes are
followed by the bump within the same millisecond. A request that is already in flight while a switch or
failover commits has always been able to use the old data; the snapshot does not widen that window for writes.
*/

// SnapshotConfig configures the proxy snapshot.
type SnapshotConfig struct {
	// PollInterval is how often the version key is read when no notification arrives. Default 1s.
	PollInterval time.Duration
	// FullReload drops all cached entries at least this often, even without a version change, as a safety
	// net for changes made without going through a hooked client (manual redis-cli edits, restores). Default 30s.
	FullReload time.Duration
	// Logger for refresh problems.
	Logger zerolog.Logger
}

func (c SnapshotConfig) withDefaults() SnapshotConfig {
	if c.PollInterval <= 0 {
		c.PollInterval = time.Second
	}
	if c.FullReload <= 0 {
		c.FullReload = 30 * time.Second
	}
	return c
}

// SnapshotStatus describes the freshness of the snapshot, used for readiness.
type SnapshotStatus struct {
	// Loaded is true once the version has been read from Redis at least once.
	Loaded  bool
	Version string
	// LastRefresh is the last successful contact with Redis (version read). Zero if never.
	LastRefresh time.Time
	// Age is the time since LastRefresh.
	Age time.Duration
	// RedisReachable is true if the last refresh attempt succeeded.
	RedisReachable bool
}

type readOnlyKey struct{}

// ReadOnly marks the context of a request that does not record replication events (GET/HEAD/LIST). Such
// requests are served from the snapshot without a Redis round trip, also when Redis is unreachable.
func ReadOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, readOnlyKey{}, true)
}

func isReadOnly(ctx context.Context) bool {
	v, _ := ctx.Value(readOnlyKey{}).(bool)
	return v
}

type snapState struct {
	version string
	created time.Time
	entries sync.Map // string -> *proxyData
	size    atomic.Int64
	// scope is the protection group scope index (protectiongroups.ScopeIndexKey), loaded on first use.
	// Group create/delete bump the version in the same transaction as the index, so a state never
	// holds an index older than its version.
	scope atomic.Pointer[map[string]string]
}

const maxSnapshotEntries = 200_000

type snapshot struct {
	rc  redis.UniversalClient
	cfg SnapshotConfig

	cur  atomic.Pointer[snapState]
	prev atomic.Pointer[snapState] // last-known-good
	mu   sync.Mutex                // serializes state swaps
	sf   singleflight.Group

	lastOK    atomic.Int64 // unix nanos of the last successful version read
	reachable atomic.Bool
}

var (
	snapReloads = promauto.NewCounter(prometheus.CounterOpts{
		Name: "policy_snapshot_reloads_total", Help: "Number of times the proxy policy snapshot was replaced because the config version changed."})
	snapHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "policy_snapshot_hits_total", Help: "Proxy requests served from the policy snapshot."})
	snapMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "policy_snapshot_misses_total", Help: "Proxy requests that had to load routing/replication data from Redis."})
	snapStale = promauto.NewCounter(prometheus.CounterOpts{
		Name: "policy_snapshot_stale_served_total", Help: "Read requests served from a stale policy snapshot because Redis was unreachable."})
	snapErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "policy_snapshot_refresh_errors_total", Help: "Failed attempts to read the config version from Redis."})
	snapAge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "policy_snapshot_age_seconds", Help: "Seconds since the proxy last successfully contacted Redis to validate its policy snapshot."})
)

// EnableSnapshot turns on the in-memory snapshot for proxy requests. It reads the version once (the error is
// returned but the snapshot keeps retrying), subscribes to change notifications and polls the version until
// ctx is done. Not safe to call concurrently with requests; call it before serving.
func (r *policySvc) EnableSnapshot(ctx context.Context, cfg SnapshotConfig) error {
	s := &snapshot{rc: r.client, cfg: cfg.withDefaults()}
	r.snap = s
	err := s.refresh(ctx, false)
	go s.run(ctx)
	return err
}

// SnapshotStatus reports the snapshot freshness; Loaded is false if the snapshot is disabled.
func (r *policySvc) SnapshotStatus() SnapshotStatus {
	if r.snap == nil {
		return SnapshotStatus{}
	}
	return r.snap.status()
}

func (s *snapshot) status() SnapshotStatus {
	st := SnapshotStatus{RedisReachable: s.reachable.Load()}
	if cur := s.cur.Load(); cur != nil {
		st.Loaded = true
		st.Version = cur.version
	}
	if ns := s.lastOK.Load(); ns != 0 {
		st.LastRefresh = time.Unix(0, ns)
		st.Age = time.Since(st.LastRefresh)
	}
	return st
}

// resubscribe/backoff bounds for the change-notification subscription after its channel closed.
const (
	resubscribeMin = 500 * time.Millisecond
	resubscribeMax = 30 * time.Second
)

// nextBackoff doubles d up to resubscribeMax.
func nextBackoff(d time.Duration) time.Duration {
	d *= 2
	if d > resubscribeMax {
		return resubscribeMax
	}
	return d
}

// resubscribe opens a new change-notification subscription and waits until Redis confirms it.
func (s *snapshot) resubscribe(ctx context.Context) (*redis.PubSub, error) {
	ps := s.rc.Subscribe(ctx, SnapshotChannel)
	if _, err := ps.ReceiveTimeout(ctx, 2*time.Second); err != nil {
		_ = ps.Close()
		return nil, err
	}
	return ps, nil
}

func (s *snapshot) run(ctx context.Context) {
	ps := s.rc.Subscribe(ctx, SnapshotChannel)
	defer func() { _ = ps.Close() }()
	msgs := ps.Channel()
	// resubC fires when a new subscription should be attempted after the channel was closed
	var resubC <-chan time.Time
	backoff := resubscribeMin
	t := time.NewTicker(s.cfg.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-msgs:
			if !ok {
				// closed: the poll keeps the snapshot fresh meanwhile; re-subscribe with backoff
				_ = ps.Close()
				msgs = nil
				s.cfg.Logger.Warn().Dur("retry_in", backoff).Msg("policy snapshot: change subscription closed, resubscribing")
				resubC = time.After(backoff)
				continue
			}
			_ = s.refresh(ctx, false)
		case <-resubC:
			resubC = nil
			nps, err := s.resubscribe(ctx)
			if err != nil {
				backoff = nextBackoff(backoff)
				s.cfg.Logger.Warn().Err(err).Dur("retry_in", backoff).Msg("policy snapshot: resubscribe failed")
				resubC = time.After(backoff)
				continue
			}
			ps, msgs = nps, nps.Channel()
			backoff = resubscribeMin
			_ = s.refresh(ctx, false) // catch up on changes missed while unsubscribed
		case <-t.C:
			force := false
			if cur := s.cur.Load(); cur != nil && time.Since(cur.created) >= s.cfg.FullReload {
				force = true
			}
			_ = s.refresh(ctx, force)
			if ns := s.lastOK.Load(); ns != 0 {
				snapAge.Set(time.Since(time.Unix(0, ns)).Seconds())
			}
		}
	}
}

// refresh reads the version and replaces the state if it changed (or force is set).
func (s *snapshot) refresh(ctx context.Context, force bool) error {
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	v, err := s.rc.Get(rctx, SnapshotVersionKey).Result()
	if errors.Is(err, redis.Nil) {
		v, err = "0", nil
	}
	if err != nil {
		snapErrors.Inc()
		if s.reachable.Swap(false) {
			s.cfg.Logger.Warn().Err(err).Msg("policy snapshot: redis unreachable, serving reads from the last known snapshot")
		}
		return fmt.Errorf("%w: unable to read config version: %w", dom.ErrUnavailable, err)
	}
	if !s.reachable.Swap(true) && s.lastOK.Load() != 0 {
		s.cfg.Logger.Info().Msg("policy snapshot: redis reachable again")
	}
	s.lastOK.Store(time.Now().UnixNano())
	if cur := s.cur.Load(); cur != nil && cur.version == v && !force {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.cur.Load()
	if cur != nil && cur.version == v && !force {
		return nil
	}
	if cur != nil {
		s.prev.Store(cur)
		snapReloads.Inc()
	}
	s.cur.Store(&snapState{version: v, created: time.Now()})
	return nil
}

// lookup returns the data for the request. See the package comment for the staleness rules.
func (s *snapshot) lookup(ctx context.Context, r *policySvc, user, bucket string) (*proxyData, error) {
	read := isReadOnly(ctx)
	if !read || s.cur.Load() == nil {
		if err := s.refresh(ctx, false); err != nil {
			if !read {
				return nil, err
			}
			if s.cur.Load() == nil {
				return nil, err
			}
		}
	}
	st := s.cur.Load()
	key := user + "\x00" + bucket
	if e, ok := st.entries.Load(key); ok {
		snapHits.Inc()
		return e.(*proxyData), nil
	}
	snapMisses.Inc()
	res, err, _ := s.sf.Do(st.version+"\x00"+key, func() (any, error) {
		lctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		d, err := r.loadProxyData(lctx, user, bucket)
		if err != nil {
			return nil, err
		}
		if st.size.Load() >= maxSnapshotEntries {
			st.entries.Clear()
			st.size.Store(0)
		}
		if _, loaded := st.entries.LoadOrStore(key, d); !loaded {
			st.size.Add(1)
		}
		return d, nil
	})
	if err != nil {
		if read && isRedisDown(err) {
			// last-known-good: the entry of the previous state, if any
			if p := s.prev.Load(); p != nil {
				if e, ok := p.entries.Load(key); ok {
					snapStale.Inc()
					return e.(*proxyData), nil
				}
			}
		}
		if isRedisDown(err) {
			return nil, fmt.Errorf("%w: policy data: %w", dom.ErrUnavailable, err)
		}
		return nil, err
	}
	return res.(*proxyData), nil
}

// protectionScope returns the protection group scope index of the current state (one HGETALL per version).
func (s *snapshot) protectionScope(ctx context.Context) (map[string]string, error) {
	st := s.cur.Load()
	if st == nil {
		return nil, fmt.Errorf("%w: policy snapshot not loaded", dom.ErrUnavailable)
	}
	if m := st.scope.Load(); m != nil {
		return *m, nil
	}
	res, err, _ := s.sf.Do(st.version+"\x00\x00protection-scope", func() (any, error) {
		lctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		m, err := s.rc.HGetAll(lctx, protectiongroups.ScopeIndexKey).Result()
		if err != nil {
			return nil, err
		}
		st.scope.Store(&m)
		return m, nil
	})
	if err != nil {
		return nil, err
	}
	return res.(map[string]string), nil
}

// ProtectionScope resolves the protection group of a write from the snapshot (see protectiongroups.ScopeCache).
// Callers must have validated the config version for this request first (BuildProxyContext on the write path
// does). ok is false when the snapshot is disabled or cannot answer; the caller then asks Redis.
func (r *policySvc) ProtectionScope(ctx context.Context, user, bucket string) (string, bool) {
	if r.snap == nil {
		return "", false
	}
	idx, err := r.snap.protectionScope(ctx)
	if err != nil {
		return "", false
	}
	return protectiongroups.LookupScope(idx, user, bucket)
}

// isRedisDown tells connectivity failures from semantic ones.
func isRedisDown(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, dom.ErrUnavailable) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var nerr interface{ Timeout() bool }
	if errors.As(err, &nerr) {
		return true
	}
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, redis.ErrClosed) || errors.Is(err, redis.ErrPoolTimeout)
}
