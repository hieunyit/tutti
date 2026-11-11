package pg_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/controlplane/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Run(m)) }

func newStore(t *testing.T) *pg.Store {
	return newStoreWithMaxConns(t, 8)
}

func newStoreWithMaxConns(t *testing.T, maxConns int32) *pg.Store {
	t.Helper()
	dsn := pgtest.NewDB(t)
	s, err := pg.Open(context.Background(), controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: dsn, MaxConns: maxConns, AutoMigrate: true}})
	require.NoError(t, err)
	t.Cleanup(s.Close)
	return s
}

func actorCtx(name string) context.Context {
	return controlplane.WithActor(context.Background(), controlplane.Actor{Name: name, RequestID: "req-1"})
}

type redisEnv struct {
	mr  *miniredis.Miniredis
	rdb *redis.Client
}

func newRedis(t *testing.T) redisEnv {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return redisEnv{mr: mr, rdb: rdb}
}

func newPublisher(t *testing.T, s *pg.Store, rdb redis.UniversalClient) *pg.Publisher {
	t.Helper()
	return pg.NewPublisher(s, rdb, controlplane.OutboxConfig{RetryBackoff: 300 * time.Millisecond, MaxBackoff: 400 * time.Millisecond, BatchSize: 100}, zerolog.Nop())
}
