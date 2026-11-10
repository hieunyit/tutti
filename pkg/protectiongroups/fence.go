// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/util"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// ScopeIndexKey maps a protected scope to its group ID so admission reads O(1) keys instead of
// decoding every group. Fields: "u:<user>" (whole-user group), "b:<user>/<bucket>" (bucket group)
// and indexVersionField. It is rebuilt from the authoritative group documents in the same Redis
// transaction as every group create/delete (Store.Save/Store.Delete); scope is immutable otherwise.
const ScopeIndexKey = "tutti:protection:scope"
const indexVersionField = "_v"

// DefaultWriterLease is the writer lease TTL used when none is configured.
const DefaultWriterLease = 30 * time.Second

const (
	kindProxy  = "proxy"
	kindWorker = "worker"
)

// LeaseKey holds the instance nonce of a live proxy/worker process. It is refreshed by Lease.Run.
func LeaseKey(kind, owner string) string { return "tutti:protection:lease:" + kind + ":" + owner }

// Admission and freezing share the authoritative group document. Every admitted write of a
// protected scope registers a writer record "<owner>:<generation>:<instance>:<admittedUnixMs>"
// and refreshes the owner's process lease. Records are removed by Release. A record whose
// process lease expired is reaped ONLY when the owner is also proven gone (see Store.ReapWriters);
// a slow-but-alive proxy keeps its lease and keeps blocking promotion, which is the safe choice.
//
// KEYS[1]=groups KEYS[2]=scope index
// ARGV: 1 user, 2 bucket, 3 route, 4 token, 5 owner, 6 kind, 7 instance, 8 leaseMs, 9 nowMs
var admitScript = redis.NewScript(`
local ids=redis.call('hmget',KEYS[2],'_v','u:'..ARGV[1],'b:'..ARGV[1]..'/'..ARGV[2])
if not ids[1] then
 if redis.call('hlen',KEYS[1])==0 then return {'ok','','0'} end
 return {'noindex',''}
end
local id=ids[2]
if not id then id=ids[3] end
if not id then return {'ok','','0'} end
local raw=redis.call('hget',KEYS[1],id)
if not raw then return {'blocked',id} end
local g=cjson.decode(raw)
local worker=ARGV[6]=='worker'
if g.primary~=ARGV[3] then return {'stale',id} end
if (g.frozen and not worker) or (worker and g.state=='promoting') then return {'blocked',id} end
local suffix=worker and ':workers' or ''
local gen=tostring(g.generation)
redis.call('hset','tutti:protection:writers:'..id..suffix,ARGV[4],ARGV[5]..':'..gen..':'..ARGV[7]..':'..ARGV[9])
redis.call('set','tutti:protection:lease:'..ARGV[6]..':'..ARGV[5],ARGV[7],'PX',ARGV[8])
return {'ok',id,gen}
`)

type Admission struct {
	Group      string
	Generation int64
	Token      string
	Worker     bool
}

// Lease is the per-process writer lease of one proxy or worker. All writer records of the
// process reference its random instance nonce, so a restarted process with the same ID never
// revives records of its dead predecessor.
type Lease struct {
	store    *Store
	kind     string
	owner    string
	instance string
	ttl      time.Duration
	now      func() time.Time

	mu       sync.Mutex
	lastOK   time.Time
	inflight map[string]context.CancelFunc
	pending  map[string]Admission
}

// NewLease creates the lease of a proxy (worker=false) or worker process. ttl <= 0 uses DefaultWriterLease.
func (s *Store) NewLease(owner string, worker bool, ttl time.Duration) *Lease {
	if ttl <= 0 {
		ttl = DefaultWriterLease
	}
	var nonce [8]byte
	_, _ = rand.Read(nonce[:])
	kind := kindProxy
	if worker {
		kind = kindWorker
	}
	return &Lease{store: s, kind: kind, owner: owner, instance: hex.EncodeToString(nonce[:]), ttl: ttl, now: time.Now,
		lastOK: time.Now(), inflight: map[string]context.CancelFunc{}, pending: map[string]Admission{}}
}

func (l *Lease) Owner() string      { return l.owner }
func (l *Lease) TTL() time.Duration { return l.ttl }
func (l *Lease) heartbeat() time.Duration {
	return max(l.ttl/6, 50*time.Millisecond)
}

// Run refreshes the lease, retries failed releases and self-fences: when the lease could not be
// refreshed for ttl/2, every in-flight protected write of this process is canceled, well before
// the lease can expire in Redis and a controller may reap its records.
func (l *Lease) Run(ctx context.Context) {
	t := time.NewTicker(l.heartbeat())
	defer t.Stop()
	for {
		l.tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (l *Lease) tick(ctx context.Context) {
	cmdCtx, cancel := context.WithTimeout(ctx, l.heartbeat())
	err := l.store.Redis.Set(cmdCtx, LeaseKey(l.kind, l.owner), l.instance, l.ttl).Err()
	cancel()
	l.mu.Lock()
	if err == nil {
		l.lastOK = l.now()
	}
	var cancels []context.CancelFunc
	if l.now().Sub(l.lastOK) >= l.ttl/2 {
		for token, c := range l.inflight {
			cancels = append(cancels, c)
			delete(l.inflight, token)
		}
	}
	pending := make([]Admission, 0, len(l.pending))
	for _, a := range l.pending {
		pending = append(pending, a)
	}
	l.mu.Unlock()
	if len(cancels) != 0 {
		zerolog.Ctx(ctx).Warn().Int("writes", len(cancels)).Msg("protection group writer lease lost: canceling in-flight protected writes")
	}
	for _, c := range cancels {
		c()
	}
	if err != nil {
		return
	}
	for _, a := range pending {
		if l.store.Release(ctx, a) == nil {
			l.mu.Lock()
			delete(l.pending, a.Token)
			l.mu.Unlock()
		}
	}
}

// track registers an in-flight write so a lost lease cancels it. It returns the write's context.
func (l *Lease) track(ctx context.Context, a Admission) (context.Context, func()) {
	if a.Group == "" {
		return ctx, func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	l.mu.Lock()
	l.lastOK = l.now() // admission refreshed the lease in the same script
	l.inflight[a.Token] = cancel
	l.mu.Unlock()
	return ctx, func() {
		l.mu.Lock()
		delete(l.inflight, a.Token)
		l.mu.Unlock()
		cancel()
	}
}

// release removes the writer record; on failure it is retried by Run so a Redis blip cannot leave
// a record of a live process behind forever (which would block promotion permanently).
func (l *Lease) release(ctx context.Context, a Admission) {
	if a.Group == "" {
		return
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := l.store.Release(cleanup, a); err != nil {
		l.mu.Lock()
		l.pending[a.Token] = a
		l.mu.Unlock()
	}
}

// InFlight returns the number of tracked protected writes and of releases awaiting retry.
func (l *Lease) InFlight() (writes, pendingReleases int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.inflight), len(l.pending)
}

// Admit registers a proxy write of this process. Callers must Release the admission.
func (l *Lease) Admit(ctx context.Context, user, bucket, route string) (Admission, error) {
	return l.store.admit(ctx, user, bucket, route, l)
}

// Admit registers a proxy write with a throw-away lease (tools and tests). Production code uses Middleware.
func (s *Store) Admit(ctx context.Context, user, bucket, route, proxy string) (Admission, error) {
	return s.admit(ctx, user, bucket, route, s.NewLease(proxy, false, 0))
}

var errStale = fmt.Errorf("%w: protection group routing is stale", dom.ErrUnavailable)

func (s *Store) admit(ctx context.Context, user, bucket, route string, l *Lease) (Admission, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return Admission{}, err
	}
	id := hex.EncodeToString(token[:])
	run := func() ([]string, error) {
		return admitScript.Run(ctx, s.Redis, []string{GroupsKey, ScopeIndexKey}, user, bucket, route, id, l.owner, l.kind,
			l.instance, l.ttl.Milliseconds(), l.now().UnixMilli()).StringSlice()
	}
	values, err := run()
	if err == nil && len(values) > 0 && values[0] == "noindex" {
		// Groups written before the scope index existed: rebuild it once from the documents.
		if err = s.EnsureIndex(ctx); err == nil {
			values, err = run()
		}
	}
	if err != nil {
		return Admission{}, fmt.Errorf("%w: write fence unavailable: %v", dom.ErrUnavailable, err)
	}
	if len(values) >= 1 && values[0] == "stale" {
		return Admission{}, errStale
	}
	if len(values) < 2 || values[0] != "ok" {
		return Admission{}, fmt.Errorf("%w: protection group is frozen or routing is stale", dom.ErrUnavailable)
	}
	var gen int64
	if len(values) > 2 {
		gen, _ = strconv.ParseInt(values[2], 10, 64)
	}
	return Admission{Group: values[1], Generation: gen, Token: id, Worker: l.kind == kindWorker}, nil
}

func (s *Store) Release(ctx context.Context, a Admission) error {
	if a.Group == "" {
		return nil
	}
	key := writerKey(a.Group)
	if a.Worker {
		key += ":workers"
	}
	return s.Redis.HDel(ctx, key, a.Token).Err()
}

// WorkerMiddleware also fences replication copies, including copies already running when
// a promotion is requested. Install outside the replication-generation check.
func (s *Store) WorkerMiddleware(l *Lease) asynq.MiddlewareFunc {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			var p struct{ ID entity.UniversalReplicationID }
			if err := json.Unmarshal(t.Payload(), &p); err != nil {
				return err
			}
			if p.ID.Validate() != nil {
				return next.ProcessTask(ctx, t)
			}
			bucket := ""
			if id, ok := p.ID.AsBucketID(); ok {
				bucket = id.FromBucket
			}
			a, err := l.Admit(ctx, p.ID.User(), bucket, p.ID.FromStorage())
			if errors.Is(err, errStale) {
				// Group replications only flow from the primary; a task from another source is a
				// leftover of a committed promotion (its generation was bumped). Retrying is useless.
				return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
			}
			if err != nil {
				return err
			}
			ctx, done := l.track(ctx, a)
			defer func() { done(); l.release(ctx, a) }()
			return next.ProcessTask(ctx, t)
		})
	}
}

// ScopeCache resolves a scope from an in-memory copy of the scope index that is at least as new as the
// config version the current request validated (the proxy policy snapshot). ok=false means "unknown":
// the caller must use the Redis admission path.
type ScopeCache func(ctx context.Context, user, bucket string) (group string, ok bool)

// Middleware must execute after routing resolution and surround both forwarding and event
// recording. With a ScopeCache, writes of unprotected scopes are admitted from memory (no Redis
// command, no dependency on Lua); protected scopes, and every write when the cache cannot answer,
// take the fail-closed Redis path.
func (s *Store) Middleware(l *Lease, cached ScopeCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}
			ctx := r.Context()
			user, bucket := xctx.GetUser(ctx), xctx.GetBucket(ctx)
			if cached != nil {
				if group, ok := cached(ctx, user, bucket); ok && group == "" {
					next.ServeHTTP(w, r)
					return
				}
			}
			a, err := l.Admit(ctx, user, bucket, xctx.GetRoutingPolicy(ctx))
			if err != nil {
				util.WriteError(ctx, w, err)
				return
			}
			ctx, done := l.track(ctx, a)
			defer func() { done(); l.release(ctx, a) }()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (s *Store) Writers(ctx context.Context, id string) (map[string]string, error) {
	return s.Redis.HGetAll(ctx, writerKey(id)).Result()
}

// ClearWriter is an explicit recovery operation, never called by the controller. The caller
// must attest that the process and its outstanding backend writes have been externally fenced.
func (s *Store) ClearWriter(ctx context.Context, id, token string, externallyFenced bool) error {
	return s.ClearWriterKind(ctx, id, token, externallyFenced, false)
}
func (s *Store) ClearWriterKind(ctx context.Context, id, token string, externallyFenced, worker bool) error {
	if !externallyFenced || token == "" {
		return fmt.Errorf("%w: explicit external fencing attestation and writer token required", dom.ErrFailedPrecondition)
	}
	g, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if !g.Frozen {
		return fmt.Errorf("%w: freeze the group before clearing a writer", dom.ErrFailedPrecondition)
	}
	key := writerKey(id)
	if worker {
		key += ":workers"
	}
	return s.Redis.HDel(ctx, key, token).Err()
}

type writerRecord struct {
	owner    string
	instance string
	admitted time.Time
}

// parseWriter parses "<owner>:<generation>:<instance>:<admittedUnixMs>". Owner IDs may contain ':'.
// Records in the legacy "<owner>:<generation>" format return ok=false and are never reaped.
func parseWriter(v string) (writerRecord, bool) {
	parts := strings.Split(v, ":")
	n := len(parts)
	if n < 4 {
		return writerRecord{}, false
	}
	ms, err := strconv.ParseInt(parts[n-1], 10, 64)
	if err != nil || parts[n-2] == "" {
		return writerRecord{}, false
	}
	if _, err = strconv.ParseInt(parts[n-3], 10, 64); err != nil {
		return writerRecord{}, false
	}
	owner := strings.Join(parts[:n-3], ":")
	if owner == "" {
		return writerRecord{}, false
	}
	return writerRecord{owner: owner, instance: parts[n-2], admitted: time.UnixMilli(ms)}, true
}
