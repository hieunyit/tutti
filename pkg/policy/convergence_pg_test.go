// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/controlplane/pgtest"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// failHook fails transactional pipelines that contain a command on a key with the given substring, while armed.
type failHook struct {
	match string
	armed atomic.Bool
	hits  atomic.Int64
}

func (h *failHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *failHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return next
}
func (h *failHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		if h.armed.Load() {
			for _, c := range cmds {
				for _, a := range c.Args() {
					if s, ok := a.(string); ok && strings.Contains(s, h.match) {
						h.hits.Add(1)
						return errors.New("injected redis failure")
					}
				}
			}
		}
		return next(ctx, cmds)
	}
}

type pgEnv struct {
	svc    *policySvc
	store  *pg.Store
	queues *tasks.QueueServiceMock
	hook   *failHook
	rdb    *redis.Client
	pub    *pg.Publisher
	uid    entity.UniversalReplicationID
}

// newPGEnv: PostgreSQL control plane + Redis whose "p:route_user" transactions can be made to fail.
func newPGEnv(t *testing.T) *pgEnv {
	t.Helper()
	store, err := pg.Open(context.Background(), controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: pgtest.NewDB(t), AutoMigrate: true}})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = rdb.Close() })
	hook := &failHook{match: "p:route_user"}
	rdb.AddHook(hook)
	q := &tasks.QueueServiceMock{}
	tasks.Reset(q)
	svc := NewService(rdb, q, "main").WithControlPlane(store)
	pub := pg.NewPublisher(store, rdb, controlplane.OutboxConfig{BatchSize: 100}, zerolog.Nop())
	pol := entity.UserReplicationPolicy{User: "user", FromStorage: "main", ToStorage: "follower"}
	uid := entity.UniversalFromUserReplication(pol)
	_, err = svc.AddUserReplicationPolicy(context.Background(), pol, entity.ReplicationOptions{})
	require.NoError(t, err)
	q.InitReplicationDone(uid)
	q.EventReplicationInProgress(uid)
	return &pgEnv{svc: svc, store: store, queues: q, hook: hook, rdb: rdb, pub: pub, uid: uid}
}

func (e *pgEnv) pgRoute(t *testing.T) string {
	t.Helper()
	to, err := e.store.Repos().Routing.Get(context.Background(), "user", "")
	if errors.Is(err, dom.ErrNotFound) {
		return "main"
	}
	require.NoError(t, err)
	return to
}

func (e *pgEnv) redisRoute(t *testing.T) string {
	t.Helper()
	to, err := e.rdb.HGet(context.Background(), pg.KeyRouteUser, "user").Result()
	if errors.Is(err, redis.Nil) {
		return ""
	}
	require.NoError(t, err)
	return to
}

func (e *pgEnv) pending(t *testing.T) (pending, paused bool) {
	t.Helper()
	info, err := e.svc.GetReplicationPolicyInfoExtended(context.Background(), e.uid)
	require.NoError(t, err)
	return info.FailoverPending, info.IsPaused
}

func TestFailoverConvergesAfterRedisFailureAfterControlPlaneCommit(t *testing.T) {
	e := newPGEnv(t)
	ctx := context.Background()

	e.hook.armed.Store(true)
	err := e.svc.FailoverReplicationWithEvent(ctx, e.uid, &controlplane.FailoverEvent{User: "user", From: "main", To: "follower", Time: time.Now().UTC()})
	require.Error(t, err)
	require.Positive(t, e.hook.hits.Load())
	require.Equal(t, "follower", e.pgRoute(t), "the control plane committed")
	pending, paused := e.pending(t)
	require.True(t, pending, "failover_pending marks the unfinished failover")
	require.True(t, paused)

	// a retry used to fail: the control plane already routes to the destination
	e.hook.armed.Store(false)
	require.NoError(t, e.svc.FailoverReplicationWithEvent(ctx, e.uid, nil))
	require.Equal(t, "follower", e.redisRoute(t))
	pending, paused = e.pending(t)
	require.False(t, pending, "failover_pending is cleared")
	require.True(t, paused, "replication from the failed storage stays paused")

	// once finished a further call is the usual rejection, not a silent success
	require.ErrorIs(t, e.svc.FailoverReplication(ctx, e.uid), dom.ErrInvalidArg)
	events, err := e.store.Repos().FailoverEvents.List(ctx)
	require.NoError(t, err)
	require.Len(t, events, 1, "the event was written once, in the control-plane transaction")
}

func TestFailoverRollsBackPendingWhenControlPlaneRejects(t *testing.T) {
	e := newPGEnv(t)
	ctx := context.Background()

	e.store.SetReadOnly(true)
	err := e.svc.FailoverReplication(ctx, e.uid)
	require.ErrorIs(t, err, controlplane.ErrReadOnly)
	pending, paused := e.pending(t)
	require.False(t, pending, "no stuck failover_pending after a definite control-plane failure")
	require.False(t, paused, "the queues are running again")
	require.Equal(t, "main", e.pgRoute(t))

	e.store.SetReadOnly(false)
	require.NoError(t, e.svc.FailoverReplication(ctx, e.uid))
	require.Equal(t, "follower", e.pgRoute(t))
	require.Equal(t, "follower", e.redisRoute(t))
}

func TestZeroDowntimeSwitchStartConvergesAfterRedisFailure(t *testing.T) {
	e := newPGEnv(t)
	ctx := context.Background()
	opts := &entity.ReplicationSwitchZeroDowntimeOpts{MultipartTTL: time.Minute}

	e.hook.armed.Store(true)
	err := e.svc.AddZeroDowntimeReplicationSwitch(ctx, e.uid, opts)
	require.Error(t, err)
	require.Equal(t, "follower", e.pgRoute(t))
	_, perr := e.store.Repos().Replications.Policies(ctx, "user", "")
	require.ErrorIs(t, perr, dom.ErrNotFound, "the control plane committed the switch")

	e.hook.armed.Store(false)
	// a different TTL is a different request
	require.ErrorIs(t, e.svc.AddZeroDowntimeReplicationSwitch(ctx, e.uid, &entity.ReplicationSwitchZeroDowntimeOpts{MultipartTTL: time.Hour}), dom.ErrAlreadyExists)
	// the identical request finishes the Redis side
	require.NoError(t, e.svc.AddZeroDowntimeReplicationSwitch(ctx, e.uid, opts))
	require.Equal(t, "follower", e.redisRoute(t))
	info, err := e.svc.GetReplicationSwitchInfo(ctx, e.uid)
	require.NoError(t, err)
	require.Equal(t, entity.StatusInProgress, info.LastStatus)
	st, err := e.store.Repos().Replications.GetStatus(ctx, toCPReplicationID(e.uid))
	require.NoError(t, err)
	require.True(t, st.Archived)
	require.NoError(t, e.svc.CompleteZeroDowntimeReplicationSwitch(ctx, e.uid))
}

func TestZeroDowntimeSwitchStartLeavesNothingWhenControlPlaneRejects(t *testing.T) {
	e := newPGEnv(t)
	ctx := context.Background()
	opts := &entity.ReplicationSwitchZeroDowntimeOpts{MultipartTTL: time.Minute}

	e.store.SetReadOnly(true)
	require.ErrorIs(t, e.svc.AddZeroDowntimeReplicationSwitch(ctx, e.uid, opts), controlplane.ErrReadOnly)
	_, err := e.svc.GetReplicationSwitchInfo(ctx, e.uid)
	require.ErrorIs(t, err, dom.ErrNotFound, "the switch record was removed again")
	require.Equal(t, "main", e.pgRoute(t))

	e.store.SetReadOnly(false)
	require.NoError(t, e.svc.AddZeroDowntimeReplicationSwitch(ctx, e.uid, opts))
	require.Equal(t, "follower", e.pgRoute(t))
}

func TestDeleteSwitchAndDowntimeStatusConvergeAfterRedisFailure(t *testing.T) {
	ctx := context.Background()

	t.Run("delete zero downtime switch", func(t *testing.T) {
		e := newPGEnv(t)
		require.NoError(t, e.svc.AddZeroDowntimeReplicationSwitch(ctx, e.uid, &entity.ReplicationSwitchZeroDowntimeOpts{MultipartTTL: time.Minute}))
		e.hook.armed.Store(true)
		require.Error(t, e.svc.DeleteReplicationSwitch(ctx, e.uid))
		require.Equal(t, "main", e.pgRoute(t), "the control plane reverted the route")
		_, err := e.svc.GetReplicationSwitchInfo(ctx, e.uid)
		require.NoError(t, err, "the switch record is removed last, so the retry still finds it")
		e.hook.armed.Store(false)
		require.NoError(t, e.svc.DeleteReplicationSwitch(ctx, e.uid))
		require.Equal(t, "main", e.redisRoute(t))
		_, err = e.svc.GetReplicationSwitchInfo(ctx, e.uid)
		require.ErrorIs(t, err, dom.ErrNotFound)
	})

	t.Run("downtime switch done with continue replication", func(t *testing.T) {
		e := newPGEnv(t)
		require.NoError(t, e.svc.SetDowntimeReplicationSwitch(ctx, e.uid, &entity.ReplicationSwitchDowntimeOpts{Cron: stringPtr("0 0 * * *"), ContinueReplication: true}))
		require.NoError(t, e.svc.UpdateDowntimeSwitchStatus(ctx, e.uid, entity.StatusInProgress, "t"))
		require.NoError(t, e.svc.UpdateDowntimeSwitchStatus(ctx, e.uid, entity.StatusCheckInProgress, "t"))
		e.hook.armed.Store(true)
		require.Error(t, e.svc.UpdateDowntimeSwitchStatus(ctx, e.uid, entity.StatusDone, "t"))
		require.Equal(t, "follower", e.pgRoute(t))
		info, err := e.svc.GetReplicationSwitchInfo(ctx, e.uid)
		require.NoError(t, err)
		require.Equal(t, entity.StatusCheckInProgress, info.LastStatus, "status is written last, the transition can be retried")

		e.hook.armed.Store(false)
		require.NoError(t, e.svc.UpdateDowntimeSwitchStatus(ctx, e.uid, entity.StatusDone, "t"))
		info, err = e.svc.GetReplicationSwitchInfo(ctx, e.uid)
		require.NoError(t, err)
		require.Equal(t, entity.StatusDone, info.LastStatus)
		require.Equal(t, "follower", e.redisRoute(t))
		pols, err := e.store.Repos().Replications.Policies(ctx, "user", "")
		require.NoError(t, err)
		require.Len(t, pols, 1)
		require.Equal(t, "follower", pols[0].FromStorage, "exactly one backward replication")
	})
}
