// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package policy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
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

func TestSnapshot_ProtectionGroupSharedAccountUserInheritsGroup(t *testing.T) {
	e := newSnapEnv(t)
	ctx := t.Context()
	e.svc.WithSharedAccount(func(storage, user string) bool {
		return true // all users on storage share connection account
	})
	require.NoError(t, e.svc.SetUserRouting(ctx, "user1", "main"))
	_, err := e.svc.AddUserReplicationPolicy(ctx, entity.UserReplicationPolicy{User: "user1", FromStorage: "main", ToStorage: "follower"}, entity.ReplicationOptions{})
	require.NoError(t, err)

	other := redis.NewClient(&redis.Options{Addr: e.mr.Addr()})
	t.Cleanup(func() { _ = other.Close() })
	groups := protectiongroups.NewStore(other)
	require.NoError(t, groups.EnsureIndex(ctx))
	lease := groups.NewLease("p1", false, 0)

	g := protectiongroups.Group{
		ID:         "group-1",
		User:       "user1",
		Members:    []string{"main", "follower"},
		Copies:     2,
		Generation: 1,
		State:      "ready",
		Frozen:     false,
	}
	require.NoError(t, g.Validate())
	require.NoError(t, groups.Save(ctx, &g))

	// User "test" (no explicit routing or replications of its own)
	// BuildProxyContext for "test" on bucket "mybucket"
	reqCtx := xctx.SetBucket(xctx.SetUser(ctx, "test"), "mybucket")
	pCtx, err := e.svc.BuildProxyContext(reqCtx, "test", "mybucket")
	require.NoError(t, err)

	require.Equal(t, "main", xctx.GetRoutingPolicy(pCtx))
	require.Equal(t, "user1", xctx.GetProtectionUser(pCtx))
	repls := xctx.GetReplications(pCtx)
	require.Len(t, repls, 1)
	require.Equal(t, "user1:main:follower", repls[0].AsString())

	// Write fence with lease
	reached := 0
	fence := groups.Middleware(lease, e.svc.ProtectionScope)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached++
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/mybucket/file.txt", nil).WithContext(pCtx)
	fence.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, reached)

	// Now freeze group-1: write from "test" must be refused (503 Service Unavailable)
	g.Frozen = true
	g.State = "draining"
	require.NoError(t, groups.Save(ctx, &g))

	pCtx2, err := e.svc.BuildProxyContext(reqCtx, "test", "mybucket")
	require.NoError(t, err)
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPut, "/mybucket/file.txt", nil).WithContext(pCtx2)
	fence.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusServiceUnavailable, rec2.Code)
	require.Equal(t, 1, reached, "write should have been blocked while frozen")
}
