// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/controlplane/pgtest"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// A promotion commits in Redis first. When PostgreSQL is unavailable (here: read-only) the promotion still
// completes, a pending marker is recorded in the same Redis transaction, and SyncControlPlane later applies the
// routing and topology change to PostgreSQL without touching replications created from the new primary.
func TestProtectionCommitRedisFirstThenControlPlane(t *testing.T) {
	ctx := context.Background()
	cp, err := pg.Open(ctx, controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: pgtest.NewDB(t), AutoMigrate: true}})
	require.NoError(t, err)
	t.Cleanup(cp.Close)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	q := &tasks.QueueServiceMock{}
	tasks.Reset(q)
	svc := NewService(rdb, q, "a").WithControlPlane(cp)
	backend := svc.ProtectionBackend().(*protectionBackend)

	groups := protectiongroups.NewStore(rdb).WithControlPlane(cp)
	g := protectiongroups.Group{ID: "g", User: "u", Members: []string{"a", "b", "c"}, Copies: 2, State: "ready", Generation: 1}
	require.NoError(t, g.Validate())
	require.NoError(t, groups.Save(ctx, &g))
	for _, id := range g.Replications("a") {
		uid, _ := id.AsUserID()
		_, err := svc.AddUserReplicationPolicy(ctx, uid, entity.ReplicationOptions{})
		require.NoError(t, err)
	}
	g.Frozen, g.State = true, "promoting"
	require.NoError(t, groups.Save(ctx, &g))

	// PostgreSQL rejects writes: the Redis commit must still happen
	cp.SetReadOnly(true)
	next := g
	next.Primary, next.Generation, next.State = "b", 2, "initializing"
	require.NoError(t, backend.Commit(ctx, g, &next))
	cur, err := groups.Get(ctx, "g")
	require.NoError(t, err)
	require.Equal(t, "b", cur.Primary)
	route, err := store.NewUserRoutingStore(rdb).GetOp(ctx, "u").Get()
	require.NoError(t, err)
	require.Equal(t, "b", route, "Redis routing switched (what the proxies read)")
	pending, err := rdb.HGet(ctx, commitPendingKey, "g").Result()
	require.NoError(t, err)
	var pc pendingCommit
	require.NoError(t, json.Unmarshal([]byte(pending), &pc))
	require.Equal(t, []string{"a"}, pc.OldPrimaries)
	pgRoute, err := cp.Repos().Routing.Get(ctx, "u", "")
	if !errors.Is(err, dom.ErrNotFound) {
		require.NoError(t, err)
		require.Equal(t, "a", pgRoute, "PostgreSQL not yet updated")
	}

	// PostgreSQL is back: the reconciler retries
	cp.SetReadOnly(false)
	require.NoError(t, backend.SyncControlPlane(ctx, cur))
	pgRoute, err = cp.Repos().Routing.Get(ctx, "u", "")
	require.NoError(t, err)
	require.Equal(t, "b", pgRoute)
	pols, err := cp.Repos().Replications.Policies(ctx, "u", "")
	if !errors.Is(err, dom.ErrNotFound) {
		require.NoError(t, err)
		for _, p := range pols {
			require.NotEqual(t, "a", p.FromStorage, "replications of the old primary removed in PostgreSQL")
		}
	}
	exists, err := rdb.HExists(ctx, commitPendingKey, "g").Result()
	require.NoError(t, err)
	require.False(t, exists, "marker cleared")
	doc, err := cp.Repos().ProtectionGroups.Get(ctx, "g")
	require.NoError(t, err)
	var stored protectiongroups.Group
	require.NoError(t, json.Unmarshal(doc.Value.JSON, &stored))
	require.Equal(t, "b", stored.Primary)
	require.True(t, stored.Frozen, "PostgreSQL copy is recovery-safe")
	// idempotent
	require.NoError(t, backend.SyncControlPlane(ctx, cur))
}
