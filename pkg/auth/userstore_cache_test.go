package auth

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/controlplane/pgtest"
)

type countingUsers struct {
	controlplane.ConsoleUsers
	gets *atomic.Int64
}

func (c countingUsers) Get(ctx context.Context, k string) (controlplane.Versioned[controlplane.ConsoleUser], error) {
	c.gets.Add(1)
	return c.ConsoleUsers.Get(ctx, k)
}

type countingStore struct {
	controlplane.Store
	gets *atomic.Int64
}

func (s countingStore) Repos() controlplane.Repos {
	r := s.Store.Repos()
	r.ConsoleUsers = countingUsers{r.ConsoleUsers, s.gets}
	return r
}

// Authenticate runs on every API request: PostgreSQL is asked once per TTL, and local changes apply at once.
func TestUserStoreCachesPostgresLookups(t *testing.T) {
	ctx := context.Background()
	store, err := pg.Open(ctx, controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: pgtest.NewDB(t), AutoMigrate: true}})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	var gets atomic.Int64
	us := NewUserStore(rdb).WithControlPlane(countingStore{store, &gets})

	now := time.Now().UTC()
	require.NoError(t, us.Create(ctx, ManagedUser{Username: "ops", Role: RoleViewer, PasswordHash: "x", CreatedAt: now, UpdatedAt: now}))
	for i := 0; i < 20; i++ {
		u, err := us.Get(ctx, "ops")
		require.NoError(t, err)
		require.Equal(t, RoleViewer, u.Role)
	}
	require.EqualValues(t, 1, gets.Load(), "one database read for 20 requests")

	// a local change is visible immediately
	u, _ := us.Get(ctx, "ops")
	u.Disabled = true
	require.NoError(t, us.Put(ctx, u))
	got, err := us.Get(ctx, "ops")
	require.NoError(t, err)
	require.True(t, got.Disabled)

	// another process' change becomes visible within the TTL
	_, err = store.Repos().ConsoleUsers.Put(ctx, controlplane.ConsoleUser{Username: "ops", Role: RoleViewer, PasswordHash: "x", Disabled: false, CreatedAt: now, UpdatedAt: now})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		g, err := us.Get(ctx, "ops")
		return err == nil && !g.Disabled
	}, 5*time.Second, 100*time.Millisecond)

	require.NoError(t, us.Delete(ctx, "ops"))
	_, err = us.Get(ctx, "ops")
	require.Error(t, err, "a deleted user is rejected at once")
}
