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

// Storage-level protection groups protect all users and buckets routed to their member storages.
// A write routed to a member storage must register a writer record under the storage group ID
// before the downstream handler starts, and keep it active until the handler returns.
func TestStorageGroup_WriteRegistersWriterUntilHandlerReturns(t *testing.T) {
	e := newSnapEnv(t)
	ctx := t.Context()
	require.NoError(t, e.svc.SetUserRouting(ctx, "bob", "main"))

	other := redis.NewClient(&redis.Options{Addr: e.mr.Addr()})
	t.Cleanup(func() { _ = other.Close() })
	groups := protectiongroups.NewStore(other)
	require.NoError(t, groups.EnsureIndex(ctx))
	lease := groups.NewLease("p1", false, 0)

	g := protectiongroups.Group{
		ID:         "storage-main",
		User:       "",
		Buckets:    nil,
		Members:    []string{"main", "follower"},
		Primary:    "main",
		Copies:     2,
		Generation: 1,
		State:      "ready",
		Frozen:     false,
	}
	require.NoError(t, g.Validate())
	require.NoError(t, groups.Save(ctx, &g))

	inHandler := make(chan struct{})
	releaseHandler := make(chan struct{})
	fence := groups.Middleware(lease, e.svc.ProtectionScope)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(inHandler)
		<-releaseHandler
		w.WriteHeader(http.StatusOK)
	}))

	done := make(chan int, 1)
	go func() {
		rctx, err := e.svc.BuildProxyContext(xctx.SetBucket(xctx.SetUser(ctx, "bob"), "data"), "bob", "data")
		require.NoError(t, err)
		rec := httptest.NewRecorder()
		fence.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/data/k", nil).WithContext(rctx))
		done <- rec.Code
	}()

	<-inHandler
	// While inside handler, the writer must be recorded in Redis for storage-main
	active, err := groups.ActiveWriters(ctx, "storage-main")
	require.NoError(t, err)
	require.EqualValues(t, 1, active, "storage group write must register writer record while downstream handler is running")

	close(releaseHandler)
	code := <-done
	require.Equal(t, http.StatusOK, code)

	// After handler returns, writer record must be released
	active, err = groups.ActiveWriters(ctx, "storage-main")
	require.NoError(t, err)
	require.Zero(t, active, "writer record must be released after downstream handler finishes")
}

// When a storage-level group is frozen, new writes must be blocked with HTTP 503 before downstream,
// while existing in-flight writes continue until they return and keep their writer records.
func TestStorageGroup_FreezeBlocksNewWriteButKeepsExistingWriter(t *testing.T) {
	e := newSnapEnv(t)
	ctx := t.Context()
	require.NoError(t, e.svc.SetUserRouting(ctx, "bob", "main"))

	other := redis.NewClient(&redis.Options{Addr: e.mr.Addr()})
	t.Cleanup(func() { _ = other.Close() })
	groups := protectiongroups.NewStore(other)
	require.NoError(t, groups.EnsureIndex(ctx))
	lease := groups.NewLease("p1", false, 0)

	g := protectiongroups.Group{
		ID:         "storage-main",
		User:       "",
		Buckets:    nil,
		Members:    []string{"main", "follower"},
		Primary:    "main",
		Copies:     2,
		Generation: 1,
		State:      "ready",
		Frozen:     false,
	}
	require.NoError(t, g.Validate())
	require.NoError(t, groups.Save(ctx, &g))

	write1InHandler := make(chan struct{})
	releaseWrite1 := make(chan struct{})
	write2Entered := false

	fence := groups.Middleware(lease, e.svc.ProtectionScope)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/data/write1" {
			close(write1InHandler)
			<-releaseWrite1
			w.WriteHeader(http.StatusOK)
			return
		}
		write2Entered = true
		w.WriteHeader(http.StatusOK)
	}))

	done1 := make(chan int, 1)
	go func() {
		rctx, err := e.svc.BuildProxyContext(xctx.SetBucket(xctx.SetUser(ctx, "bob"), "data"), "bob", "data")
		require.NoError(t, err)
		rec := httptest.NewRecorder()
		fence.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/data/write1", nil).WithContext(rctx))
		done1 <- rec.Code
	}()

	<-write1InHandler
	active, err := groups.ActiveWriters(ctx, "storage-main")
	require.NoError(t, err)
	require.EqualValues(t, 1, active)

	// Freeze group
	g.Frozen = true
	g.State = "draining"
	require.NoError(t, groups.Save(ctx, &g))

	// Send write 2: must be blocked by freeze (503) and must NOT enter downstream
	rctx2, err := e.svc.BuildProxyContext(xctx.SetBucket(xctx.SetUser(ctx, "bob"), "data"), "bob", "data")
	require.NoError(t, err)
	rec2 := httptest.NewRecorder()
	fence.ServeHTTP(rec2, httptest.NewRequest(http.MethodPut, "/data/write2", nil).WithContext(rctx2))
	require.Equal(t, http.StatusServiceUnavailable, rec2.Code, "new write to frozen storage group must return 503")
	require.False(t, write2Entered, "write 2 must not reach downstream handler")

	// Write 1 is still active
	active, err = groups.ActiveWriters(ctx, "storage-main")
	require.NoError(t, err)
	require.EqualValues(t, 1, active, "write 1 writer record must remain active")

	// Release write 1
	close(releaseWrite1)
	code1 := <-done1
	require.Equal(t, http.StatusOK, code1)

	// Now all writers drained
	active, err = groups.ActiveWriters(ctx, "storage-main")
	require.NoError(t, err)
	require.Zero(t, active)
}

// A storage-level group created after an unprotected write was cached in memory must immediately
// fence subsequent writes on the storage.
func TestStorageGroup_CreatedAfterCachedUnprotectedWrite(t *testing.T) {
	e := newSnapEnv(t)
	ctx := t.Context()
	require.NoError(t, e.svc.SetUserRouting(ctx, "bob", "main"))

	other := redis.NewClient(&redis.Options{Addr: e.mr.Addr()})
	t.Cleanup(func() { _ = other.Close() })
	groups := protectiongroups.NewStore(other)
	require.NoError(t, groups.EnsureIndex(ctx))
	lease := groups.NewLease("p1", false, 0)

	reached := 0
	handler := func() http.Handler {
		fence := groups.Middleware(lease, e.svc.ProtectionScope)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached++ }))
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rctx, err := e.svc.BuildProxyContext(xctx.SetBucket(xctx.SetUser(r.Context(), "bob"), "data"), "bob", "data")
			require.NoError(t, err)
			fence.ServeHTTP(w, r.WithContext(rctx))
		})
	}()
	put := func() int {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/data/key", nil).WithContext(ctx))
		return rec.Code
	}

	// 1. Unprotected write warms the snapshot
	require.Equal(t, http.StatusOK, put())
	require.Equal(t, 1, reached)

	// 2. Storage group created as frozen
	g := protectiongroups.Group{
		ID:         "storage-main",
		User:       "",
		Buckets:    nil,
		Members:    []string{"main", "follower"},
		Primary:    "main",
		Copies:     2,
		Generation: 1,
		State:      "initializing",
		Frozen:     true,
	}
	require.NoError(t, g.Validate())
	require.NoError(t, groups.Save(ctx, &g))

	// 3. Next write must be refused (503)
	require.Equal(t, http.StatusServiceUnavailable, put(), "frozen storage group must refuse next write")
	require.Equal(t, 1, reached)

	// 4. Unfrozen admits write
	g.Frozen = false
	g.State = "ready"
	require.NoError(t, groups.Save(ctx, &g))
	require.Equal(t, http.StatusOK, put())
	require.Equal(t, 2, reached)
}

// ProtectionGroupOf must identify storage-level protection groups for scopes on member storages.
func TestProtectionGroupOf_RecognizesEffectiveStorageProtection(t *testing.T) {
	for _, snap := range []bool{true, false} {
		t.Run(fmt.Sprintf("snap=%v", snap), func(t *testing.T) {
			e := newSnapEnv(t)
			ctx := t.Context()
			svc := e.svc
			if !snap {
				svc = e.direct
			}
			require.NoError(t, svc.SetUserRouting(ctx, "bob", "main"))

			// Before group creation: empty
			group, err := svc.ProtectionGroupOf(ctx, "bob", "data")
			require.NoError(t, err)
			require.Empty(t, group)

			// Create storage-level group on main
			other := redis.NewClient(&redis.Options{Addr: e.mr.Addr()})
			t.Cleanup(func() { _ = other.Close() })
			groups := protectiongroups.NewStore(other)
			g := protectiongroups.Group{
				ID:         "storage-main",
				User:       "",
				Buckets:    nil,
				Members:    []string{"main", "follower"},
				Primary:    "main",
				Copies:     2,
				Generation: 1,
				State:      "ready",
			}
			require.NoError(t, g.Validate())
			require.NoError(t, groups.Save(ctx, &g))

			// ProtectionGroupOf must now report storage-main
			group, err = svc.ProtectionGroupOf(ctx, "bob", "data")
			require.NoError(t, err)
			require.Equal(t, "storage-main", group)

			// Scopes on other storage remain unprotected
			require.NoError(t, svc.SetUserRouting(ctx, "otheruser", "s0"))
			group, err = svc.ProtectionGroupOf(ctx, "otheruser", "data")
			require.NoError(t, err)
			require.Empty(t, group)
		})
	}
}

// Incomplete or unreadable index must never be assumed unprotected: write must fall back
// to Redis admission (fail closed if frozen or unavailable).
func TestUnknownScope_FallsBackToAdmission(t *testing.T) {
	e := newSnapEnv(t)
	ctx := t.Context()
	require.NoError(t, e.svc.SetUserRouting(ctx, "bob", "main"))

	other := redis.NewClient(&redis.Options{Addr: e.mr.Addr()})
	t.Cleanup(func() { _ = other.Close() })
	groups := protectiongroups.NewStore(other)
	require.NoError(t, groups.EnsureIndex(ctx))
	lease := groups.NewLease("p1", false, 0)

	g := protectiongroups.Group{
		ID:         "storage-main",
		User:       "",
		Buckets:    nil,
		Members:    []string{"main", "follower"},
		Primary:    "main",
		Copies:     2,
		Generation: 1,
		State:      "ready",
		Frozen:     true,
	}
	require.NoError(t, g.Validate())
	require.NoError(t, groups.Save(ctx, &g))

	handlerReached := false
	fence := groups.Middleware(lease, e.svc.ProtectionScope)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerReached = true
		w.WriteHeader(http.StatusOK)
	}))

	// Case 1: Corrupt the scope index key in Redis so snapshot cannot read it (returns error, ok=false)
	require.NoError(t, e.client.Set(ctx, protectiongroups.ScopeIndexKey, "corrupted-string", 0).Err())

	rctx, err := e.svc.BuildProxyContext(xctx.SetBucket(xctx.SetUser(ctx, "bob"), "data"), "bob", "data")
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	fence.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/data/k", nil).WithContext(rctx))
	// Must fail closed (503), not reach downstream!
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, "corrupted index must fail closed")
	require.False(t, handlerReached, "downstream handler must not be reached when index is unreadable")
}


