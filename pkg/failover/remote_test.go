package failover

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
)

type fakeApplier struct {
	mu   sync.Mutex
	ids  []string
	evs  []controlplane.FailoverEvent
	acts []string
	err  error
}

func (f *fakeApplier) FailoverReplicationWithEvent(ctx context.Context, id entity.UniversalReplicationID, ev *controlplane.FailoverEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ids = append(f.ids, id.AsString())
	f.acts = append(f.acts, controlplane.ActorFrom(ctx).Name)
	if ev != nil {
		f.evs = append(f.evs, *ev)
	}
	return f.err
}

func remoteEnv(t *testing.T) (redis.UniversalClient, *fakeApplier) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, &fakeApplier{}
}

func TestRemoteFailoverIsAppliedByTheWorker(t *testing.T) {
	rdb, ap := remoteEnv(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go RunExecutor(ctx, rdb, ap, zerolog.Nop())

	r := Remote{Client: rdb, Wait: 5 * time.Second}
	need, err := r.Required(ctx)
	require.NoError(t, err)
	require.False(t, need, "no backend announced: local failover")
	require.NoError(t, PublishBackend(ctx, rdb, "redis"))
	need, _ = r.Required(ctx)
	require.False(t, need)
	require.NoError(t, PublishBackend(ctx, rdb, "postgres"))
	need, _ = r.Required(ctx)
	require.True(t, need, "the postgres backend delegates")

	ev := Event{User: "u", From: "main", To: "f1", Time: time.Unix(100, 0).UTC(), EventLag: 3}
	require.NoError(t, r.Execute(ctx, "u:main:f1", ev))
	require.Equal(t, []string{"u:main:f1"}, ap.ids)
	require.Equal(t, []controlplane.FailoverEvent{toCPEvent(ev)}, ap.evs, "the event travels with the request and is written in the control-plane transaction")
	require.Equal(t, []string{"failover-controller"}, ap.acts)

	// errors keep their class
	ap.err = dom.ErrInvalidArg
	require.ErrorIs(t, r.Execute(ctx, "u:main:f1", ev), dom.ErrInvalidArg)
	ap.err = controlplane.ErrReadOnly
	require.ErrorIs(t, r.Execute(ctx, "u:main:f1", ev), dom.ErrFailedPrecondition)
	ap.err = errors.New("boom")
	require.ErrorContains(t, r.Execute(ctx, "u:main:f1", ev), "boom")
}

func TestRemoteFailoverWithoutWorkerTimesOutAndStaleRequestsAreDropped(t *testing.T) {
	rdb, ap := remoteEnv(t)
	r := Remote{Client: rdb, Wait: 300 * time.Millisecond}
	ev := Event{User: "u", From: "main", To: "f1", Time: time.Now()}
	err := r.Execute(t.Context(), "u:main:f1", ev)
	require.ErrorIs(t, err, ErrNoExecutor)
	require.ErrorIs(t, err, dom.ErrUnavailable)

	// a worker that starts later must not apply the request that nobody waits for any more
	n, _ := rdb.LLen(t.Context(), RequestsKey).Result()
	require.EqualValues(t, 1, n)
	raw, _ := rdb.LPop(t.Context(), RequestsKey).Result()
	stale := `{"id":"x","replication":"u:main:f1","event":{},"expires":"2000-01-01T00:00:00Z"}`
	_ = raw
	require.NoError(t, rdb.RPush(t.Context(), RequestsKey, stale).Err())
	ctx, cancel := context.WithCancel(t.Context())
	go RunExecutor(ctx, rdb, ap, zerolog.Nop())
	require.Eventually(t, func() bool { n, _ := rdb.LLen(t.Context(), RequestsKey).Result(); return n == 0 }, 3*time.Second, 20*time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)
	ap.mu.Lock()
	defer ap.mu.Unlock()
	require.Empty(t, ap.ids)
}
