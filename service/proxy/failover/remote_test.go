package failover

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/entity"
	pf "github.com/hieunyit/tutti/pkg/failover"
)

type recApplier struct {
	ids []string
	evs []controlplane.FailoverEvent
}

func (a *recApplier) FailoverReplicationWithEvent(_ context.Context, id entity.UniversalReplicationID, ev *controlplane.FailoverEvent) error {
	a.ids = append(a.ids, id.AsString())
	a.evs = append(a.evs, *ev)
	return nil
}

// With the postgres backend a proxy must not write routing or the event to Redis: the worker applies the failover.
func TestControllerDelegatesToWorkerWithPostgresBackend(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	e := newEnv(t, enabled("f1"))
	e.ctrl = New(Config{DefaultAfter: time.Minute, CheckInterval: time.Second, MainStorage: "main"},
		e.pol, e.hl, e.lock, e.store, zerolog.Nop(),
		WithClock(func() time.Time { return e.now }),
		WithRemoteExecutor(pf.Remote{Client: rdb, Wait: 5 * time.Second}),
		WithOnFailover(func(from, to string) { e.metric = append(e.metric, [2]string{from, to}) }))
	e.userRepl("u1", "f1", status(0))

	// redis backend: local failover as before
	require.NoError(t, pf.PublishBackend(t.Context(), rdb, "redis"))
	e.run(t)
	require.Len(t, e.pol.failedOver, 1)
	require.Len(t, e.store.events, 1)

	// postgres backend: the worker does it, the proxy touches neither routing nor events
	e.pol.failedOver, e.store.events, e.metric = nil, nil, nil
	e.pol.userRoutes = map[string]string{}
	require.NoError(t, pf.PublishBackend(t.Context(), rdb, "postgres"))
	ap := &recApplier{}
	go pf.RunExecutor(t.Context(), rdb, ap, zerolog.Nop())
	e.run(t)
	require.Empty(t, e.pol.failedOver, "no Redis routing write by the proxy")
	require.Empty(t, e.store.events, "no Redis event write by the proxy")
	require.Equal(t, []string{"u1:main:f1"}, ap.ids)
	require.Equal(t, "u1", ap.evs[0].User)
	require.Equal(t, [][2]string{{"main", "f1"}}, e.metric)
}
