// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
)

func TestProtectionStoreUsesSnapshotVersionKeys(t *testing.T) {
	require.Equal(t, SnapshotVersionKey, protectiongroups.ConfigVersionKey)
	require.Equal(t, SnapshotChannel, protectiongroups.ConfigChangedChannel)
}

// The proxy fence admits unprotected writes from the snapshot's copy of the scope index. A group created
// after a proxy cached "unprotected" must be enforced on the very next write: group creation bumps the config
// version in its transaction and every write validates the version before using the snapshot.
func TestSnapshot_ProtectionGroupEnforcedOnNextWrite(t *testing.T) {
	e := newSnapEnv(t)
	ctx := t.Context()
	require.NoError(t, e.svc.SetUserRouting(ctx, "user", "main"))
	// the group store uses its own client without the change hook, like the worker's protection service
	other := redis.NewClient(&redis.Options{Addr: e.mr.Addr()})
	t.Cleanup(func() { _ = other.Close() })
	groups := protectiongroups.NewStore(other)
	require.NoError(t, groups.EnsureIndex(ctx))
	lease := groups.NewLease("p1", false, 0)

	reached := 0
	// policy middleware (write path: validates the version) -> fence -> handler
	handler := func() http.Handler {
		fence := groups.Middleware(lease, e.svc.ProtectionScope)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached++ }))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rctx, err := e.svc.BuildProxyContext(xctx.SetBucket(xctx.SetUser(r.Context(), "user"), "bucket"), "user", "bucket")
			require.NoError(t, err)
			fence.ServeHTTP(w, r.WithContext(rctx))
		})
	}()
	put := func() int {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/bucket/key", nil).WithContext(ctx))
		return rec.Code
	}

	// unprotected: admitted from memory, no fence command at all
	require.Equal(t, http.StatusOK, put())
	before := e.mr.CommandCount()
	require.Equal(t, http.StatusOK, put())
	require.Equal(t, 1, e.mr.CommandCount()-before, "unprotected write: only the version read")
	require.Equal(t, 2, reached)

	// a group (frozen while initializing) is created through another client: the next write is fenced
	g := protectiongroups.Group{ID: "g", User: "user", Members: []string{"main", "follower"}, Copies: 2,
		Generation: 1, State: "initializing", Frozen: true}
	require.NoError(t, g.Validate())
	require.NoError(t, groups.Save(ctx, &g))
	require.Equal(t, http.StatusServiceUnavailable, put(), "a frozen group must refuse the very next write")
	require.Equal(t, 2, reached)

	// unfrozen: the write is admitted through Redis and registers a writer record (then releases it)
	g.Frozen = false
	require.NoError(t, groups.Save(ctx, &g))
	require.Equal(t, http.StatusOK, put())
	require.Equal(t, 3, reached)
	n, err := groups.ActiveWriters(ctx, "g")
	require.NoError(t, err)
	require.Zero(t, n)

	// a lost index is never taken as "unprotected": the fence falls back to Redis, which rebuilds it
	g.Frozen = true
	require.NoError(t, groups.Save(ctx, &g))
	require.NoError(t, other.Del(ctx, protectiongroups.ScopeIndexKey).Err())
	require.NoError(t, other.Incr(ctx, SnapshotVersionKey).Err())
	require.Equal(t, http.StatusServiceUnavailable, put())
	ok, err := other.HExists(ctx, protectiongroups.ScopeIndexKey, "u:user").Result()
	require.NoError(t, err)
	require.True(t, ok, "the Redis path rebuilt the index")

	// deleting the group: unprotected again, from memory
	g.Frozen, g.State = false, "ready"
	require.NoError(t, groups.Save(ctx, &g))
	require.NoError(t, groups.Delete(ctx, g))
	require.Equal(t, http.StatusOK, put())
	before = e.mr.CommandCount()
	require.Equal(t, http.StatusOK, put())
	require.Equal(t, 1, e.mr.CommandCount()-before)
}

// Users on a shared connection account never inherit another identity's protection group: they keep their own
// routing and replications, and only their writes to a bucket the group manages on their routed storage are
// refused (same physical bucket, managed by the group). Both the snapshot and the direct Redis path.
func TestSharedAccountUserNeverUsesOtherUsersGroup(t *testing.T) {
	for _, snap := range []bool{true, false} {
		t.Run(fmt.Sprintf("snapshot=%v", snap), func(t *testing.T) {
			e := newSnapEnv(t)
			ctx := t.Context()
			svc := e.svc
			if !snap {
				svc = e.direct
			}
			// connection mode for everybody except "own" (override credentials)
			svc.WithSharedAccount(func(storage, user string) bool { return user != "own" })
			// alice: group on [main, follower] for bucket "shared" and her replication
			require.NoError(t, svc.SetUserRouting(ctx, "alice", "main"))
			_, err := svc.AddUserReplicationPolicy(ctx, entity.UserReplicationPolicy{User: "alice", FromStorage: "main", ToStorage: "follower"}, entity.ReplicationOptions{})
			require.NoError(t, err)
			// bob: routed to s0, which is not a group member; carol: routed to main without own policies
			require.NoError(t, svc.SetUserRouting(ctx, "bob", "s0"))

			other := redis.NewClient(&redis.Options{Addr: e.mr.Addr()})
			t.Cleanup(func() { _ = other.Close() })
			groups := protectiongroups.NewStore(other)
			lease := groups.NewLease("p1", false, 0)
			g := protectiongroups.Group{ID: "alice-g", User: "alice", Buckets: []string{"shared"}, Members: []string{"main", "follower"},
				Copies: 2, Generation: 1, State: "ready"}
			require.NoError(t, g.Validate())
			require.NoError(t, groups.Save(ctx, &g))

			reached := 0
			fence := groups.Middleware(lease, svc.ProtectionScope)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached++ }))
			write := func(user, bucket string) (int, context.Context) {
				rctx, err := svc.BuildProxyContext(xctx.SetBucket(xctx.SetUser(ctx, user), bucket), user, bucket)
				require.NoError(t, err)
				rec := httptest.NewRecorder()
				fence.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/"+bucket+"/k", nil).WithContext(rctx))
				return rec.Code, rctx
			}

			// bob keeps routing to s0, never gets alice's replications, and may write
			code, bctx := write("bob", "shared")
			require.Equal(t, http.StatusOK, code)
			require.Equal(t, "s0", xctx.GetRoutingPolicy(bctx))
			require.Empty(t, xctx.GetReplications(bctx))
			require.Empty(t, xctx.GetSharedProtectionGroup(bctx))

			// carol is routed to a member: own routing, no replications of alice, write refused (403)
			code, cctx := write("carol", "shared")
			require.Equal(t, http.StatusForbidden, code)
			require.Equal(t, "main", xctx.GetRoutingPolicy(cctx))
			require.Empty(t, xctx.GetReplications(cctx))
			require.Equal(t, "alice-g", xctx.GetSharedProtectionGroup(cctx))
			// other buckets of carol are not managed by the group
			code, _ = write("carol", "carol-bucket")
			require.Equal(t, http.StatusOK, code)

			// a user with its own credentials on the storage is not on the shared account
			code, octx := write("own", "shared")
			require.Equal(t, http.StatusOK, code)
			require.Empty(t, xctx.GetSharedProtectionGroup(octx))

			// alice herself is fenced by her group (ready, primary main): admitted with her replication
			code, actx := write("alice", "shared")
			require.Equal(t, http.StatusOK, code)
			require.Equal(t, []string{"alice:main:follower"}, replStrings(xctx.GetReplications(actx)))
			require.Equal(t, 4, reached, "bob, carol-bucket, own and alice were admitted; carol on the managed bucket was not")
		})
	}
}

func replStrings(ids []entity.UniversalReplicationID) []string {
	var out []string
	for _, id := range ids {
		out = append(out, id.AsString())
	}
	return out
}

// An unreadable scope index is never cached as "not protected": the request fails closed.
func TestSharedAccountGuardFailsClosedWhenIndexUnavailable(t *testing.T) {
	e := newSnapEnv(t)
	ctx := t.Context()
	e.direct.WithSharedAccount(func(string, string) bool { return true })
	e.mr.SetError("LOADING")
	_, err := e.direct.BuildProxyContext(xctx.SetBucket(xctx.SetUser(ctx, "bob"), "b"), "bob", "b")
	require.Error(t, err)
	e.mr.SetError("")
	// wrong type at the index key: HGETALL fails, the error is returned (and nothing is cached)
	require.NoError(t, e.client.Set(ctx, protectiongroups.ScopeIndexKey, "x", 0).Err())
	_, err = e.direct.BuildProxyContext(xctx.SetBucket(xctx.SetUser(ctx, "bob"), "b"), "bob", "b")
	require.ErrorIs(t, err, dom.ErrUnavailable)
	require.NoError(t, e.client.Del(ctx, protectiongroups.ScopeIndexKey).Err())
	// a missing index is rebuilt, then the lookup answers
	rctx, err := e.direct.BuildProxyContext(xctx.SetBucket(xctx.SetUser(ctx, "bob"), "b"), "bob", "b")
	require.NoError(t, err)
	require.Empty(t, xctx.GetSharedProtectionGroup(rctx))
	v, err := e.client.HGet(ctx, protectiongroups.ScopeIndexKey, "_v").Result()
	require.NoError(t, err)
	require.Equal(t, "2", v)
}

// The failover controller's protection check validates the config version and falls back to Redis.
func TestProtectionGroupOfUsesCurrentIndex(t *testing.T) {
	for _, snap := range []bool{true, false} {
		e := newSnapEnv(t)
		ctx := t.Context()
		svc := e.svc
		if !snap {
			svc = e.direct
		}
		group, err := svc.ProtectionGroupOf(ctx, "alice", "b")
		require.NoError(t, err)
		require.Empty(t, group)
		other := redis.NewClient(&redis.Options{Addr: e.mr.Addr()})
		t.Cleanup(func() { _ = other.Close() })
		groups := protectiongroups.NewStore(other)
		g := protectiongroups.Group{ID: "g", User: "alice", Members: []string{"main", "follower"}, Copies: 2, State: "ready"}
		require.NoError(t, g.Validate())
		require.NoError(t, groups.Save(ctx, &g))
		// no request validated the new version: the check must still see the group
		group, err = svc.ProtectionGroupOf(ctx, "alice", "b")
		require.NoError(t, err)
		require.Equal(t, "g", group)
		e.mr.SetError("LOADING")
		_, err = svc.ProtectionGroupOf(ctx, "alice", "other")
		require.Error(t, err, "unknown scope must be an error (fail closed), snapshot=%v", snap)
		e.mr.SetError("")
	}
}
