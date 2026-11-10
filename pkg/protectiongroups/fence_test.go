// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/alicebob/miniredis/v2/server"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/proxystatus"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type commandCounter struct {
	mu    sync.Mutex
	count map[string]int
}

func (c *commandCounter) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, v := range c.count {
		n += v
	}
	return n
}
func (c *commandCounter) reset() {
	c.mu.Lock()
	c.count = map[string]int{}
	c.mu.Unlock()
}

func newFenceStore(t testing.TB) (*Store, *miniredis.Miniredis, *commandCounter) {
	mr := miniredis.RunT(t)
	counter := &commandCounter{count: map[string]int{}}
	mr.Server().SetPreHook(func(_ *server.Peer, cmd string, _ ...string) bool {
		counter.mu.Lock()
		counter.count[cmd]++
		counter.mu.Unlock()
		return false
	})
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	return NewStore(rc), mr, counter
}

func saveGroup(t testing.TB, s *Store, g Group) Group {
	t.Helper()
	if g.Members == nil {
		g.Members = []string{"a", "b"}
	}
	if g.Copies == 0 {
		g.Copies = 2
	}
	if g.State == "" {
		g.State = "ready"
	}
	if g.Generation == 0 {
		g.Generation = 1
	}
	require.NoError(t, g.Validate())
	require.NoError(t, s.Save(context.Background(), &g))
	return g
}

// NEW-01: admission reads a constant number of keys, independent of the number of groups.
func TestAdmitRedisCommandsDoNotGrowWithGroups(t *testing.T) {
	perAdmit := map[int]int{}
	for _, groups := range []int{1, 50, 500} {
		s, _, counter := newFenceStore(t)
		ctx := t.Context()
		for i := 0; i < groups; i++ {
			saveGroup(t, s, Group{ID: fmt.Sprintf("g%d", i), User: fmt.Sprintf("u%d", i), Members: []string{fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", i)}})
		}
		lease := s.NewLease("p1", false, time.Minute)
		// warm the script cache (EVALSHA miss + EVAL on the first call)
		a, err := lease.Admit(ctx, "u0", "bucket", "a0")
		require.NoError(t, err)
		require.NoError(t, s.Release(ctx, a))

		counter.reset()
		const n = 20
		for i := 0; i < n; i++ {
			a, err := lease.Admit(ctx, "u0", "bucket", "a0")
			require.NoError(t, err)
			require.Equal(t, "g0", a.Group)
		}
		counter.mu.Lock()
		require.Zero(t, counter.count["HVALS"]+counter.count["HGETALL"], "admission must not scan groups")
		counter.mu.Unlock()
		perAdmit[groups] = counter.total() / n
	}
	require.Equal(t, perAdmit[1], perAdmit[500], "commands per admit must not grow with the number of groups: %v", perAdmit)
	// EVALSHA + HMGET + HGET + HSET + SET
	require.LessOrEqual(t, perAdmit[500], 5, "%v", perAdmit)

	// unprotected scope: EVALSHA + HMGET
	s, _, counter := newFenceStore(t)
	saveGroup(t, s, Group{ID: "g", User: "tenant"})
	lease := s.NewLease("p1", false, time.Minute)
	_, err := lease.Admit(t.Context(), "someone-else", "b", "a")
	require.NoError(t, err)
	counter.reset()
	a, err := lease.Admit(t.Context(), "someone-else", "b", "a")
	require.NoError(t, err)
	require.Empty(t, a.Group)
	require.Equal(t, 2, counter.total())
}

func BenchmarkAdmit(b *testing.B) {
	for _, groups := range []int{1, 1000} {
		b.Run(fmt.Sprintf("groups=%d", groups), func(b *testing.B) {
			s, _, _ := newFenceStore(b)
			for i := 0; i < groups; i++ {
				saveGroup(b, s, Group{ID: fmt.Sprintf("g%d", i), User: fmt.Sprintf("u%d", i), Members: []string{fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", i)}})
			}
			lease := s.NewLease("p1", false, time.Minute)
			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				a, err := lease.Admit(ctx, "u0", "bucket", "a0")
				if err != nil {
					b.Fatal(err)
				}
				_ = s.Release(ctx, a)
			}
		})
	}
}

func TestAdmitScopeIndex(t *testing.T) {
	s, _, _ := newFenceStore(t)
	ctx := t.Context()
	lease := s.NewLease("p1", false, time.Minute)

	// no groups at all: unprotected
	a, err := lease.Admit(ctx, "tenant", "x", "a")
	require.NoError(t, err)
	require.Empty(t, a.Group)

	whole := saveGroup(t, s, Group{ID: "whole", User: "tenant"})
	buckets := saveGroup(t, s, Group{ID: "buckets", User: "other", Buckets: []string{"b1", "b2"}, Members: []string{"c", "d"}})

	a, err = lease.Admit(ctx, "tenant", "anything", "a")
	require.NoError(t, err)
	require.Equal(t, "whole", a.Group)
	require.EqualValues(t, 1, a.Generation)
	a, err = lease.Admit(ctx, "other", "b2", "c")
	require.NoError(t, err)
	require.Equal(t, "buckets", a.Group)
	a, err = lease.Admit(ctx, "other", "b3", "zzz")
	require.NoError(t, err)
	require.Empty(t, a.Group, "bucket outside the group is unprotected")

	// stale routing and frozen groups are refused
	_, err = lease.Admit(ctx, "tenant", "x", "b")
	require.ErrorIs(t, err, dom.ErrUnavailable)
	require.ErrorIs(t, err, errStale)
	whole.Frozen = true
	require.NoError(t, s.Save(ctx, &whole))
	_, err = lease.Admit(ctx, "tenant", "x", "a")
	require.ErrorIs(t, err, dom.ErrUnavailable)
	require.NotErrorIs(t, err, errStale)
	// workers are admitted while frozen (draining), but not while promoting
	wl := s.NewLease("w1", true, time.Minute)
	a, err = wl.Admit(ctx, "tenant", "x", "a")
	require.NoError(t, err)
	require.True(t, a.Worker)
	require.NoError(t, s.Release(ctx, a))

	// an index lost or written before it existed is rebuilt from the documents, never ignored
	require.NoError(t, s.Redis.Del(ctx, ScopeIndexKey).Err())
	_, err = lease.Admit(ctx, "tenant", "x", "a")
	require.ErrorIs(t, err, dom.ErrUnavailable, "frozen group must still be enforced after index loss")
	ok, err := s.Redis.HExists(ctx, ScopeIndexKey, "b:other/b1").Result()
	require.NoError(t, err)
	require.True(t, ok)

	// deleting a group removes its scope from the index
	buckets, err = s.Get(ctx, "buckets")
	require.NoError(t, err)
	require.NoError(t, s.Delete(ctx, buckets))
	a, err = lease.Admit(ctx, "other", "b1", "zzz")
	require.NoError(t, err)
	require.Empty(t, a.Group)
	idx, err := s.Redis.HGetAll(ctx, ScopeIndexKey).Result()
	require.NoError(t, err)
	require.Equal(t, map[string]string{indexVersionField: indexVersion, "u:tenant": "whole", "o:whole": "tenant", "p:whole": "a", "m:whole": "a,b"}, idx)
}

func frozenGroup(t *testing.T, s *Store) Group {
	g := saveGroup(t, s, Group{ID: "g", User: "tenant"})
	return g
}

func freeze(t *testing.T, s *Store, g Group) Group {
	g, err := s.Get(t.Context(), g.ID)
	require.NoError(t, err)
	g.Frozen = true
	g.State = "draining"
	require.NoError(t, s.Save(t.Context(), &g))
	return g
}

func report(id string, started, updated time.Time) proxystatus.Status {
	return proxystatus.Status{AppID: id, StartedAt: started, UpdatedAt: updated}
}

// NEW-02: a crashed proxy (no heartbeat, absent from status reports) is reaped after its lease.
func TestReapWritersCrashedProxy(t *testing.T) {
	s, mr, _ := newFenceStore(t)
	ctx := t.Context()
	g := frozenGroup(t, s)
	lease := s.NewLease("crashed", false, 2*time.Second)
	_, err := lease.Admit(ctx, "tenant", "x", "a") // never released: the process "crashed"
	require.NoError(t, err)
	g = freeze(t, s, g)

	n, err := s.ReapWriters(ctx, g.ID, nil, time.Now(), reportMaxAge)
	require.NoError(t, err)
	require.Zero(t, n, "lease still valid: the proxy may still be writing")

	mr.FastForward(3 * time.Second)
	n, err = s.ReapWriters(ctx, g.ID, nil, time.Now(), reportMaxAge)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	active, err := s.ActiveWriters(ctx, g.ID)
	require.NoError(t, err)
	require.Zero(t, active)
}

func TestReapWritersKeepsAliveProxies(t *testing.T) {
	s, mr, _ := newFenceStore(t)
	ctx := t.Context()
	g := frozenGroup(t, s)
	start := time.Now().Add(-time.Hour)
	lease := s.NewLease("slow", false, 2*time.Second)
	_, err := lease.Admit(ctx, "tenant", "x", "a")
	require.NoError(t, err)
	g = freeze(t, s, g)

	// the lease expired (e.g. heartbeat delayed), but the proxy still reports: keep waiting
	mr.FastForward(3 * time.Second)
	now := time.Now()
	n, err := s.ReapWriters(ctx, g.ID, []proxystatus.Status{report("slow", start, now)}, now, reportMaxAge)
	require.NoError(t, err)
	require.Zero(t, n)

	// a running heartbeat keeps the lease even without reports
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go lease.Run(runCtx)
	require.Eventually(t, func() bool { return mr.Exists(LeaseKey(kindProxy, "slow")) }, time.Second, 10*time.Millisecond)
	n, err = s.ReapWriters(ctx, g.ID, nil, time.Now(), reportMaxAge)
	require.NoError(t, err)
	require.Zero(t, n)
	active, err := s.ActiveWriters(ctx, g.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, active)
}

func TestReapWritersRestartedProxyAndLegacyRecords(t *testing.T) {
	s, _, _ := newFenceStore(t)
	ctx := t.Context()
	g := frozenGroup(t, s)
	old := s.NewLease("p", false, time.Minute)
	_, err := old.Admit(ctx, "tenant", "x", "a")
	require.NoError(t, err)
	require.NoError(t, s.Redis.HSet(ctx, writerKey(g.ID), "legacy", "p:1").Err())
	g = freeze(t, s, g)

	// the same proxy ID restarted: a new instance holds the lease and reports a later start time
	restarted := s.NewLease("p", false, time.Minute)
	restarted.tick(ctx)
	now := time.Now().Add(time.Second)
	n, err := s.ReapWriters(ctx, g.ID, []proxystatus.Status{report("p", now.Add(-500*time.Millisecond), now)}, now, reportMaxAge)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	w, err := s.Writers(ctx, g.ID)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"legacy": "p:1"}, w, "legacy records need ClearWriter")
}

func TestLeaseSelfFencesAndRetriesReleases(t *testing.T) {
	s, mr, _ := newFenceStore(t)
	ctx := t.Context()
	g := frozenGroup(t, s)
	lease := s.NewLease("p", false, 2*time.Second)
	clock := time.Now()
	lease.now = func() time.Time { return clock }

	a, err := lease.Admit(ctx, "tenant", "x", "a")
	require.NoError(t, err)
	writeCtx, done := lease.track(ctx, a)

	// Redis unreachable: a release fails and is kept for retry; after ttl/2 the write is canceled
	mr.SetError("LOADING simulated outage")
	lease.release(ctx, a)
	_, pending := lease.InFlight()
	require.Equal(t, 1, pending)
	clock = clock.Add(500 * time.Millisecond)
	lease.tick(ctx)
	require.NoError(t, writeCtx.Err(), "short blip must not cancel writes")
	clock = clock.Add(time.Second)
	lease.tick(ctx)
	require.ErrorIs(t, writeCtx.Err(), context.Canceled)
	done()

	mr.SetError("")
	lease.tick(ctx)
	_, pending = lease.InFlight()
	require.Zero(t, pending)
	active, err := s.ActiveWriters(ctx, g.ID)
	require.NoError(t, err)
	require.Zero(t, active)
}

func TestParseWriter(t *testing.T) {
	w, ok := parseWriter("host:with:colons:3:abcd:1700000000000")
	require.True(t, ok)
	require.Equal(t, "host:with:colons", w.owner)
	require.Equal(t, "abcd", w.instance)
	require.Equal(t, int64(1700000000000), w.admitted.UnixMilli())
	for _, bad := range []string{"p:1", "p:x:abcd:1", ":1:abcd:1", "p:1::1", "p:1:abcd:x"} {
		_, ok = parseWriter(bad)
		require.False(t, ok, bad)
	}
	require.True(t, strings.HasPrefix(LeaseKey(kindWorker, "w"), "tutti:protection:lease:worker:"))
}

func TestAdmitInfrastructureGroupWithoutUser(t *testing.T) {
	s, _, _ := newFenceStore(t)
	ctx := t.Context()
	// Create infrastructure group protecting bucket "invoices" across dc1 and dc2 with no User assigned
	g := saveGroup(t, s, Group{
		ID:      "pg-invoices",
		Buckets: []string{"invoices"},
		Members: []string{"dc1", "dc2"},
		Primary: "dc1",
		Copies:  2,
	})
	require.Empty(t, g.User)

	lease := s.NewLease("p1", false, time.Minute)

	// User "bob" writes to "invoices" on dc1 -> admitted!
	aBob, err := lease.Admit(ctx, "bob", "invoices", "dc1")
	require.NoError(t, err)
	require.Equal(t, "pg-invoices", aBob.Group)
	lease.release(ctx, aBob)

	// User "carol" writes to "invoices" on dc1 -> admitted without 403!
	aCarol, err := lease.Admit(ctx, "carol", "invoices", "dc1")
	require.NoError(t, err)
	require.Equal(t, "pg-invoices", aCarol.Group)
	lease.release(ctx, aCarol)

	// Scope index checks
	idx, err := s.Redis.HGetAll(ctx, ScopeIndexKey).Result()
	require.NoError(t, err)
	require.Equal(t, "pg-invoices", idx["b:invoices"])
	require.Equal(t, "dc1", idx["p:pg-invoices"])
	require.Empty(t, idx["o:pg-invoices"])

	// LookupSharedGroup confirms it is not treated as another identity's exclusive group
	group, ok, err := LookupSharedGroup(idx, "dc1", "bob", "invoices")
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, group, "infrastructure group without user is not reported as another identity")

	// Routing to wrong storage is rejected as stale
	_, err = lease.Admit(ctx, "carol", "invoices", "dc2")
	require.ErrorIs(t, err, dom.ErrUnavailable)
}
