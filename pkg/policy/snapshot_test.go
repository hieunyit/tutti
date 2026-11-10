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
	"fmt"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// countHook counts the commands sent to Redis (pipeline members included; MULTI and EXEC count as commands).
type countHook struct{ n atomic.Int64 }

func (h *countHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *countHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		h.n.Add(1)
		return next(ctx, cmd)
	}
}
func (h *countHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		h.n.Add(int64(len(cmds)))
		return next(ctx, cmds)
	}
}

type snapEnv struct {
	mr     *miniredis.Miniredis
	client *redis.Client
	hook   *countHook
	svc    *policySvc // snapshot enabled
	direct *policySvc // reads Redis on every request
}

func newSnapEnv(t *testing.T) *snapEnv {
	t.Helper()
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1, DialTimeout: 200 * time.Millisecond, ReadTimeout: 500 * time.Millisecond})
	t.Cleanup(func() { _ = c.Close() })
	hook := &countHook{}
	c.AddHook(hook)
	e := &snapEnv{mr: mr, client: c, hook: hook}
	e.svc = NewService(c, &tasks.QueueServiceMock{}, "main")
	e.direct = NewService(c, &tasks.QueueServiceMock{}, "main")
	// long poll: tests drive refreshes explicitly or through notifications
	require.NoError(t, e.svc.EnableSnapshot(t.Context(), SnapshotConfig{PollInterval: time.Hour, FullReload: time.Hour}))
	return e
}

// view is a comparable rendering of what the proxy context carries.
func view(ctx context.Context, err error) string {
	if err != nil {
		return "err:" + err.Error()
	}
	var repls []string
	for _, id := range xctx.GetReplications(ctx) {
		repls = append(repls, id.AsString())
	}
	var gens []string
	for id, g := range xctx.GetReplicationGenerations(ctx) {
		id := id
		gens = append(gens, fmt.Sprintf("%s=%d", id.AsString(), g))
	}
	sort.Strings(gens)
	sw := ""
	if s := xctx.GetInProgressZeroDowntime(ctx); s != nil {
		rid := s.ReplicationID()
		sw = rid.AsString()
	}
	return fmt.Sprintf("route=%s switch=%s repls=%v gens=%v", xctx.GetRoutingPolicy(ctx), sw, repls, gens)
}

// requireSame checks that a (validated) write request and the direct Redis read give the same answer for
// every probe: the snapshot must never be observable on the write path.
func (e *snapEnv) requireSame(t *testing.T, step string) {
	t.Helper()
	for _, p := range [][2]string{{"user", "bucket"}, {"user", ""}, {"other", "bucket"}, {"user", "other-bucket"}} {
		var want, got string
		if p[1] == "" {
			c, err := e.direct.BuildProxyNoBucketContext(t.Context(), p[0])
			want = view(c, err)
			c, err = e.svc.BuildProxyNoBucketContext(t.Context(), p[0])
			got = view(c, err)
		} else {
			c, err := e.direct.BuildProxyContext(t.Context(), p[0], p[1])
			want = view(c, err)
			c, err = e.svc.BuildProxyContext(t.Context(), p[0], p[1])
			got = view(c, err)
		}
		require.Equal(t, want, got, "step %q probe %v", step, p)
	}
}

func TestSnapshot_WritePathMatchesRedisThroughSwitchAndFailover(t *testing.T) {
	e := newSnapEnv(t)
	ctx := t.Context()
	e.requireSame(t, "empty")

	pol := entity.UserReplicationPolicy{User: "user", FromStorage: "main", ToStorage: "follower"}
	uid := entity.UniversalFromUserReplication(pol)
	require.NoError(t, e.svc.SetUserRouting(ctx, "user", "main"))
	e.requireSame(t, "routing set")
	_, err := e.svc.AddUserReplicationPolicy(ctx, pol, entity.ReplicationOptions{})
	require.NoError(t, err)
	e.requireSame(t, "replication added")

	require.NoError(t, e.svc.BlockUserRouting(ctx, "user"))
	e.requireSame(t, "blocked")
	require.NoError(t, e.svc.UnblockUserRouting(ctx, "user"))
	e.requireSame(t, "unblocked")

	require.NoError(t, e.svc.PauseReplication(ctx, uid))
	e.requireSame(t, "paused")
	require.NoError(t, e.svc.ResumeReplication(ctx, uid))
	e.requireSame(t, "resumed")

	// failover: routing moves to the follower in one transaction, replications from main are dropped from the request
	require.NoError(t, e.svc.FailoverReplication(ctx, uid))
	e.requireSame(t, "failed over")
	require.Contains(t, view(mustCtx(e.svc.BuildProxyContext(ctx, "user", "bucket"))), "route=follower")
}

func TestSnapshot_WritePathMatchesRedisThroughZeroDowntimeSwitch(t *testing.T) {
	e := newSnapEnv(t)
	ctx := t.Context()
	pol := entity.UserReplicationPolicy{User: "user", FromStorage: "main", ToStorage: "follower"}
	uid := entity.UniversalFromUserReplication(pol)
	_, err := e.svc.AddUserReplicationPolicy(ctx, pol, entity.ReplicationOptions{})
	require.NoError(t, err)
	e.requireSame(t, "replication added")
	before := view(mustCtx(e.svc.BuildProxyContext(ctx, "user", "bucket")))
	require.Contains(t, before, "route=main")

	// routing, switch info and policy removal change in one transaction: a write must never see a half applied switch
	require.NoError(t, e.svc.AddZeroDowntimeReplicationSwitch(ctx, uid, &entity.ReplicationSwitchZeroDowntimeOpts{MultipartTTL: time.Minute}))
	e.requireSame(t, "zero downtime switch in progress")
	sw := view(mustCtx(e.svc.BuildProxyContext(ctx, "user", "bucket")))
	require.Contains(t, sw, "switch="+uid.AsString())
	require.Contains(t, sw, "route=follower")

	require.NoError(t, e.svc.CompleteZeroDowntimeReplicationSwitch(ctx, uid))
	e.requireSame(t, "zero downtime switch done")
	require.NoError(t, e.svc.DeleteReplicationSwitch(ctx, uid))
	e.requireSame(t, "switch deleted")

	// replication generations: re-adding a replication increments it, a write must see the new value at once
	pol2 := entity.UserReplicationPolicy{User: "user", FromStorage: "follower", ToStorage: "main"}
	_, err = e.svc.AddUserReplicationPolicy(ctx, pol2, entity.ReplicationOptions{})
	require.NoError(t, err)
	e.requireSame(t, "replication from follower")
	require.NoError(t, e.svc.DeleteUserReplication(ctx, pol2))
	_, err = e.svc.AddUserReplicationPolicy(ctx, pol2, entity.ReplicationOptions{})
	require.NoError(t, err)
	e.requireSame(t, "replication re-added with a new generation")
}

func mustCtx(ctx context.Context, err error) (context.Context, error) { return ctx, err }

func TestSnapshot_RedisCommandsPerRequest(t *testing.T) {
	e := newSnapEnv(t)
	ctx := t.Context()
	pol := entity.UserReplicationPolicy{User: "user", FromStorage: "main", ToStorage: "follower"}
	_, err := e.svc.AddUserReplicationPolicy(ctx, pol, entity.ReplicationOptions{})
	require.NoError(t, err)
	require.NoError(t, e.svc.refreshSnapshotForTest(ctx))

	const n = 100
	measure := func(f func()) float64 {
		before := e.hook.n.Load()
		for i := 0; i < n; i++ {
			f()
		}
		return float64(e.hook.n.Load()-before) / n
	}
	read := policyReadCtx(ctx)

	// warm: the first request after a change loads the entry
	_, err = e.svc.BuildProxyContext(read, "user", "bucket")
	require.NoError(t, err)
	_, err = e.svc.BuildProxyNoBucketContext(read, "user")
	require.NoError(t, err)

	direct := measure(func() { _, _ = e.direct.BuildProxyContext(read, "user", "bucket") })
	getHead := measure(func() { _, _ = e.svc.BuildProxyContext(read, "user", "bucket") })
	list := measure(func() { _, _ = e.svc.BuildProxyContext(read, "user", "bucket") })
	put := measure(func() { _, _ = e.svc.BuildProxyContext(ctx, "user", "bucket") })
	listBuckets := measure(func() { _, _ = e.svc.BuildProxyNoBucketContext(read, "user") })
	t.Logf("redis commands per request (policy context only): without snapshot=%.1f (incl. MULTI/EXEC), GET/HEAD=%.2f, LIST=%.2f, ListBuckets=%.2f, PUT/DELETE/POST=%.2f", direct, getHead, list, listBuckets, put)
	require.Equal(t, 0.0, getHead, "GET/HEAD must be served from the snapshot")
	require.Equal(t, 0.0, list)
	require.Equal(t, 0.0, listBuckets)
	require.Equal(t, 1.0, put, "writes validate the version with one GET")
	require.Greater(t, direct, 8.0)
}

func TestSnapshot_NotificationRefreshesReaders(t *testing.T) {
	e := newSnapEnv(t)
	ctx := t.Context()
	// subscribe/poll loop is running with a 1h poll: only pub/sub can refresh the reader
	read := policyReadCtx(ctx)
	c, err := e.svc.BuildProxyContext(read, "user", "bucket")
	require.NoError(t, err)
	require.Equal(t, "main", xctx.GetRoutingPolicy(c))

	// give the subscriber time to attach
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, e.direct.SetUserRouting(ctx, "user", "follower")) // another process
	require.Eventually(t, func() bool {
		c, err := e.svc.BuildProxyContext(read, "user", "bucket")
		return err == nil && xctx.GetRoutingPolicy(c) == "follower"
	}, 3*time.Second, 20*time.Millisecond, "a change notification must refresh the snapshot")
}

func TestSnapshot_LastKnownGoodWhenRedisDown(t *testing.T) {
	e := newSnapEnv(t)
	ctx := t.Context()
	read := policyReadCtx(ctx)
	_, err := e.svc.BuildProxyContext(read, "user", "bucket")
	require.NoError(t, err)

	// a change makes the current state empty, the old one stays as last known good
	require.NoError(t, e.svc.SetUserRouting(ctx, "other", "follower"))
	require.NoError(t, e.svc.refreshSnapshotForTest(ctx))

	e.mr.Close()

	c, err := e.svc.BuildProxyContext(read, "user", "bucket")
	require.NoError(t, err, "reads keep working on the last known snapshot")
	require.Equal(t, "main", xctx.GetRoutingPolicy(c))

	_, err = e.svc.BuildProxyContext(ctx, "user", "bucket")
	require.ErrorIs(t, err, dom.ErrUnavailable, "writes fail closed")
	_, err = e.svc.BuildProxyContext(read, "never-seen", "bucket")
	require.ErrorIs(t, err, dom.ErrUnavailable, "unknown data cannot be served without Redis")
	require.False(t, e.svc.SnapshotStatus().RedisReachable)
}

func TestChangeNotifier_BumpsOnlyForSnapshotKeysAndInsideTransactions(t *testing.T) {
	e := newSnapEnv(t)
	ctx := t.Context()
	version := func() string {
		v, err := e.client.Get(ctx, SnapshotVersionKey).Result()
		if err == redis.Nil {
			return "0"
		}
		require.NoError(t, err)
		return v
	}
	v0 := version()
	// unrelated and read-only operations never bump
	require.NoError(t, e.client.Set(ctx, "other:key", "x", 0).Err())
	require.NoError(t, e.client.HSet(ctx, "p:repl_status_user:x", "a", "b").Err())
	_, _ = e.direct.GetUserRoutings(ctx)
	require.Equal(t, v0, version())

	require.NoError(t, e.svc.SetUserRouting(ctx, "user", "follower"))
	v1 := version()
	require.NotEqual(t, v0, v1)

	// a transaction (zero downtime switch) bumps exactly once
	pol := entity.UserReplicationPolicy{User: "u2", FromStorage: "main", ToStorage: "follower"}
	_, err := e.svc.AddUserReplicationPolicy(ctx, pol, entity.ReplicationOptions{})
	require.NoError(t, err)
	v2 := version()
	uid := entity.UniversalFromUserReplication(pol)
	require.NoError(t, e.svc.AddZeroDowntimeReplicationSwitch(ctx, uid, &entity.ReplicationSwitchZeroDowntimeOpts{MultipartTTL: time.Minute}))
	v3 := version()
	require.NotEqual(t, v2, v3)
	require.Equal(t, fmt.Sprint(atoiOrFail(t, v2)+1), v3, "a MULTI/EXEC change is announced by exactly one atomic bump")
}

func atoiOrFail(t *testing.T, s string) int {
	var n int
	_, err := fmt.Sscan(s, &n)
	require.NoError(t, err)
	return n
}

// refreshSnapshotForTest forces a version check.
func (r *policySvc) refreshSnapshotForTest(ctx context.Context) error {
	return r.snap.refresh(ctx, false)
}

// policyReadCtx marks ctx as a GET/HEAD/LIST request (same as ReadOnly), kept for tests in other packages.
func policyReadCtx(ctx context.Context) context.Context { return ReadOnly(ctx) }
