package pg_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/controlplane/pgtest"
)

func TestOpenLazyStartsDegradedWhenPostgresIsDown(t *testing.T) {
	ctx := context.Background()
	cfg := controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{
		DSN: "postgres://nobody:x@127.0.0.1:1/none?sslmode=disable", ConnectTimeout: 300 * time.Millisecond}}
	s, err := pg.OpenLazy(ctx, cfg)
	require.NoError(t, err, "an unreachable database must not prevent the service from starting")
	defer s.Close()
	require.False(t, s.Ready())
	require.Error(t, s.TryReady(ctx))

	require.ErrorIs(t, s.Repos().Routing.Set(ctx, "alice", "", "s1"), controlplane.ErrReadOnly)
	_, err = s.Repos().Routing.Get(ctx, "alice", "")
	require.True(t, controlplane.IsUnavailable(err), "reads fall back to the Redis projection: %v", err)
}

func TestOpenLazyBecomesReadyWhenPostgresAppears(t *testing.T) {
	ctx := context.Background()
	dsn := pgtest.NewDB(t)
	s, err := pg.OpenLazy(ctx, controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: dsn, AutoMigrate: true}})
	require.NoError(t, err)
	defer s.Close()
	require.False(t, s.Ready())
	require.ErrorIs(t, s.Repos().Routing.Set(ctx, "alice", "", "s1"), controlplane.ErrReadOnly)

	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	s.RunReadiness(rctx, 50*time.Millisecond, nil)
	require.True(t, s.Ready())
	require.NoError(t, s.Repos().Routing.Set(ctx, "alice", "", "s1"))

	// the configured read-only mode survives becoming ready
	s2, err := pg.OpenLazy(ctx, controlplane.Config{ReadOnly: true, Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: dsn}})
	require.NoError(t, err)
	defer s2.Close()
	require.NoError(t, s2.TryReady(ctx))
	require.ErrorIs(t, s2.Repos().Routing.Set(ctx, "bob", "", "s1"), controlplane.ErrReadOnly)
}
