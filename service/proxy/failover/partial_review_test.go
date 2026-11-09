package failover

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type reviewRouteFailure struct {
	armed    bool
	failures int
}

func (*reviewRouteFailure) DialHook(next redis.DialHook) redis.DialHook          { return next }
func (*reviewRouteFailure) ProcessHook(next redis.ProcessHook) redis.ProcessHook { return next }
func (h *reviewRouteFailure) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			args := cmd.Args()
			if h.armed && cmd.Name() == "hset" && len(args) > 1 && strings.HasPrefix(fmt.Sprint(args[1]), "p:route_user") {
				h.failures++
				return errors.New("review: transient routing transaction failure")
			}
		}
		return next(ctx, cmds)
	}
}

func TestReviewFailoverRetriesAfterPauseAndRoutingFailure(t *testing.T) {
	ctx := t.Context()
	rc := testutil.SetupRedis(t)
	qs := &tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}
	svc := policy.NewService(rc, qs, "main")
	repl := entity.UserReplicationPolicy{User: "u1", FromStorage: "main", ToStorage: "follower"}
	_, err := svc.AddUserReplicationPolicy(ctx, repl, entity.ReplicationOptions{})
	require.NoError(t, err)
	id := entity.UniversalFromUserReplication(repl)
	for _, q := range tasks.AllReplicationQueues(id) {
		qs.Queues[q] = 0
	}
	hook := &reviewRouteFailure{armed: true}
	rc.(*redis.Client).AddHook(hook)
	e := newEnv(t, enabled("follower"))
	e.hl.st[1].Name = "follower"
	ctrl := New(Config{DefaultAfter: time.Minute, CheckInterval: time.Second, MainStorage: "main"}, svc, e.hl, e.lock, e.store, zerolog.Nop(), WithClock(func() time.Time { return e.now }))
	err = ctrl.RunOnce(ctx)
	require.ErrorContains(t, err, "transient routing transaction failure")
	require.Equal(t, 1, hook.failures)
	hook.armed = false // infrastructure recovered; retry should complete failover
	info, err := svc.GetReplicationPolicyInfoExtended(ctx, id)
	require.NoError(t, err)
	require.True(t, info.IsPaused)
	require.True(t, info.FailoverPending)
	// A fresh service/controller must recover from Redis state, not process memory.
	svc = policy.NewService(rc, qs, "main")
	ctrl = New(Config{DefaultAfter: time.Minute, CheckInterval: time.Second, MainStorage: "main"}, svc, e.hl, e.lock, e.store, zerolog.Nop(), WithClock(func() time.Time { return e.now }))
	require.NoError(t, ctrl.RunOnce(ctx))
	routes, err := svc.GetUserRoutings(ctx)
	require.NoError(t, err)
	t.Logf("after recovered retry: routing=%q, replication paused=%v", routes["u1"], info.IsPaused)
	require.Equal(t, "follower", routes["u1"], "failover should resume after its own partially completed attempt")
	info, err = svc.GetReplicationPolicyInfoExtended(ctx, id)
	require.NoError(t, err)
	require.True(t, info.IsPaused)
	require.False(t, info.FailoverPending, "routing and pending marker must commit together")
}

func TestReviewFailoverPreservesOperatorPause(t *testing.T) {
	ctx := t.Context()
	rc := testutil.SetupRedis(t)
	qs := &tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}
	svc := policy.NewService(rc, qs, "main")
	repl := entity.UserReplicationPolicy{User: "u1", FromStorage: "main", ToStorage: "follower"}
	_, err := svc.AddUserReplicationPolicy(ctx, repl, entity.ReplicationOptions{})
	require.NoError(t, err)
	id := entity.UniversalFromUserReplication(repl)
	for _, q := range tasks.AllReplicationQueues(id) {
		qs.Queues[q] = 0
	}
	require.NoError(t, svc.PauseReplication(ctx, id))
	e := newEnv(t, enabled("follower"))
	e.hl.st[1].Name = "follower"
	ctrl := New(Config{DefaultAfter: time.Minute, CheckInterval: time.Second, MainStorage: "main"}, svc, e.hl, e.lock, e.store, zerolog.Nop(), WithClock(func() time.Time { return e.now }))
	require.NoError(t, ctrl.RunOnce(ctx))
	routes, err := svc.GetUserRoutings(ctx)
	require.NoError(t, err)
	require.Equal(t, "main", routes["u1"])
	require.Error(t, svc.FailoverReplication(ctx, id), "direct calls must also reject operator pause")
	// An operator can also stop recovery of an already pending failover.
	require.NoError(t, svc.ResumeReplication(ctx, id))
	hook := &reviewRouteFailure{armed: true}
	rc.(*redis.Client).AddHook(hook)
	require.Error(t, svc.FailoverReplication(ctx, id))
	hook.armed = false
	require.NoError(t, svc.PauseReplication(ctx, id))
	info, err := svc.GetReplicationPolicyInfoExtended(ctx, id)
	require.NoError(t, err)
	require.False(t, info.FailoverPending, "operator pause must revoke recovery ownership")
	require.NoError(t, ctrl.RunOnce(ctx))
	routes, err = svc.GetUserRoutings(ctx)
	require.NoError(t, err)
	require.Equal(t, "main", routes["u1"])
}
