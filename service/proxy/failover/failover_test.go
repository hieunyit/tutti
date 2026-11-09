package failover

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/entity"
	pf "github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/health"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/testutil"
)

type fakePolicy struct {
	userRoutes   map[string]string
	bucketRoutes map[string]map[string]string
	userRepls    map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended
	bucketRepls  map[string]map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended
	failedOver   []entity.UniversalReplicationID
}

func (f *fakePolicy) GetUserRoutings(context.Context) (map[string]string, error) {
	return f.userRoutes, nil
}
func (f *fakePolicy) GetBucketRoutings(_ context.Context, u string) (map[string]string, error) {
	return f.bucketRoutes[u], nil
}
func (f *fakePolicy) ListUserReplicationsInfo(context.Context) (map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended, error) {
	return f.userRepls, nil
}
func (f *fakePolicy) ListBucketReplicationsInfo(_ context.Context, u string) (map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended, error) {
	return f.bucketRepls[u], nil
}
func (f *fakePolicy) FailoverReplication(_ context.Context, id entity.UniversalReplicationID) error {
	f.failedOver = append(f.failedOver, id)
	if up, ok := id.AsUserID(); ok {
		f.userRoutes[up.User] = up.ToStorage
	} else if bp, ok := id.AsBucketID(); ok {
		if f.bucketRoutes[bp.User] == nil {
			f.bucketRoutes[bp.User] = map[string]string{}
		}
		f.bucketRoutes[bp.User][bp.FromBucket] = bp.ToStorage
	}
	return nil
}

type fakeHealth struct{ st []health.Status }

func (h *fakeHealth) Snapshot() []health.Status { return h.st }

type fakeLocker struct {
	held  bool
	calls int
}

func (l *fakeLocker) Obtain(context.Context, string, time.Duration) (func(), bool, error) {
	l.calls++
	if l.held {
		return nil, false, nil
	}
	return func() {}, true, nil
}

type fakePolicies struct {
	pols   []pf.Policy
	events []pf.Event
	err    error
}

func (f *fakePolicies) ListFailoverPolicies(context.Context) ([]pf.Policy, error) {
	return f.pols, f.err
}
func (f *fakePolicies) SaveFailoverEvent(_ context.Context, e pf.Event) error {
	f.events = append(f.events, e)
	return nil
}

var t0 = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

type env struct {
	pol    *fakePolicy
	hl     *fakeHealth
	lock   *fakeLocker
	store  *fakePolicies
	now    time.Time
	ctrl   *Controller
	metric [][2]string
}

func status(lag int) entity.ReplicationStatusExtended {
	return entity.ReplicationStatusExtended{
		ReplicationStatus: &entity.ReplicationStatus{},
		EventMigration:    entity.QueueStats{Unprocessed: lag},
	}
}

func newEnv(t *testing.T, policies ...pf.Policy) *env {
	e := &env{
		pol:   &fakePolicy{userRoutes: map[string]string{}, bucketRoutes: map[string]map[string]string{}, userRepls: map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended{}, bucketRepls: map[string]map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended{}},
		hl:    &fakeHealth{st: []health.Status{{Name: "main", Available: false, LastChange: t0}, {Name: "f1", Available: true}, {Name: "f2", Available: true}}},
		lock:  &fakeLocker{},
		store: &fakePolicies{pols: policies},
		now:   t0.Add(2 * time.Minute),
	}
	e.ctrl = New(Config{DefaultAfter: time.Minute, DefaultMaxEventLag: 0, CheckInterval: time.Second, MainStorage: "main"},
		e.pol, e.hl, e.lock, e.store, zerolog.Nop(),
		WithClock(func() time.Time { return e.now }),
		WithOnFailover(func(from, to string) { e.metric = append(e.metric, [2]string{from, to}) }))
	return e
}

func (e *env) userRepl(user, to string, st entity.ReplicationStatusExtended) {
	e.pol.userRepls[entity.UserReplicationPolicy{User: user, FromStorage: "main", ToStorage: to}] = st
}

func enabled(targets ...string) pf.Policy {
	return pf.Policy{Source: "main", Enabled: true, Targets: targets}
}

func (e *env) run(t *testing.T) {
	require.NoError(t, e.ctrl.RunOnce(t.Context()))
}

func TestNoPolicyNoFailover(t *testing.T) {
	e := newEnv(t)
	e.userRepl("u1", "f1", status(0))
	e.run(t)
	require.Empty(t, e.pol.failedOver)
}

func TestPolicyDisabledOrEmptyTargetsNoFailover(t *testing.T) {
	for name, p := range map[string]pf.Policy{
		"disabled":      {Source: "main", Enabled: false, Targets: []string{"f1"}},
		"empty targets": {Source: "main", Enabled: true},
		"other source":  {Source: "f1", Enabled: true, Targets: []string{"f2"}},
	} {
		e := newEnv(t, p)
		e.userRepl("u1", "f1", status(0))
		e.run(t)
		require.Empty(t, e.pol.failedOver, name)
		require.Zero(t, e.lock.calls, name)
	}
}

func TestNoFailoverBeforeAfterThenFailover(t *testing.T) {
	r := require.New(t)
	e := newEnv(t, enabled("f1"))
	e.userRepl("u1", "f1", status(0))
	e.now = t0.Add(59 * time.Second)
	e.run(t)
	r.Empty(e.pol.failedOver)
	e.now = t0.Add(60 * time.Second)
	e.run(t)
	r.Len(e.pol.failedOver, 1)
	r.Equal("f1", e.pol.userRoutes["u1"])
	r.Len(e.store.events, 1)
	ev := e.store.events[0]
	r.Equal("u1", ev.User)
	r.Equal("main", ev.From)
	r.Equal("f1", ev.To)
	r.Equal(e.now, ev.Time)
	r.Equal([][2]string{{"main", "f1"}}, e.metric)
}

func TestPolicyAfterAndLagOverrideDefaults(t *testing.T) {
	r := require.New(t)
	lag := 5
	p := enabled("f1")
	p.After = 10 * time.Second
	p.MaxEventLag = &lag
	e := newEnv(t, p)
	e.userRepl("u1", "f1", status(5))
	e.now = t0.Add(10 * time.Second)
	e.run(t)
	r.Len(e.pol.failedOver, 1)
	r.Equal(5, e.store.events[0].EventLag)
}

func TestBreakerRecoveredNoFailover(t *testing.T) {
	e := newEnv(t, enabled("f1"))
	e.userRepl("u1", "f1", status(0))
	e.hl.st[0].Available = true
	e.run(t)
	require.Empty(t, e.pol.failedOver)
}

func TestNoCandidate(t *testing.T) {
	arch := status(0)
	arch.IsArchived = true
	paused := status(0)
	paused.IsPaused = true
	sw := status(0)
	sw.Switch = &entity.ReplicationSwitchInfo{}
	initNotDone := status(0)
	initNotDone.InitMigration.Unprocessed = 4
	for name, tc := range map[string]struct {
		st       entity.ReplicationStatusExtended
		destDown bool
	}{
		"lag above max":       {st: status(1)},
		"destination down":    {st: status(0), destDown: true},
		"init not done":       {st: initNotDone},
		"paused":              {st: paused},
		"archived":            {st: arch},
		"switch in progress":  {st: sw},
		"unknown destination": {st: status(0), destDown: true},
	} {
		e := newEnv(t, enabled("f1"))
		e.userRepl("u1", "f1", tc.st)
		if tc.destDown {
			e.hl.st[1].Available = false
		}
		e.run(t)
		e.run(t) // warn only once, still nothing
		require.Empty(t, e.pol.failedOver, name)
		require.Empty(t, e.store.events, name)
		require.Len(t, e.ctrl.warned, 1, name)
	}
}

func TestCustomDestinationBucketSkipped(t *testing.T) {
	r := require.New(t)
	e := newEnv(t, enabled("f1", "f2"))
	e.pol.userRoutes["u1"] = "main"
	e.pol.bucketRepls["u1"] = map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended{
		{User: "u1", FromStorage: "main", FromBucket: "b", ToStorage: "f1", ToBucket: "other"}: status(0),
		{User: "u1", FromStorage: "main", FromBucket: "b", ToStorage: "f2", ToBucket: "b"}:     status(9),
	}
	e.ctrl.cfg.DefaultMaxEventLag = 10
	e.run(t)
	r.Len(e.pol.failedOver, 1)
	r.Equal("f2", e.pol.failedOver[0].ToStorage())
	r.Equal("b", e.store.events[0].Bucket)
	r.Equal("f2", e.pol.bucketRoutes["u1"]["b"])

	// only custom bucket replication: nothing happens
	e2 := newEnv(t, enabled("f1"))
	e2.pol.userRoutes["u1"] = "main"
	e2.pol.bucketRepls["u1"] = map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended{
		{User: "u1", FromStorage: "main", FromBucket: "b", ToStorage: "f1", ToBucket: "other"}: status(0),
	}
	e2.run(t)
	r.Empty(e2.pol.failedOver)
}

func TestStorageNotInTargetsNeverChosen(t *testing.T) {
	e := newEnv(t, enabled("f2"))
	e.userRepl("u1", "f1", status(0)) // f1 is perfectly healthy but not a target
	e.run(t)
	require.Empty(t, e.pol.failedOver)
	require.Equal(t, "", e.pol.userRoutes["u1"])
}

func TestTargetsOrderRespected(t *testing.T) {
	r := require.New(t)
	// user has replications to both; f2 is preferred even though f1 has lower lag (and sorts first by name)
	e := newEnv(t, enabled("f2", "f1"))
	e.ctrl.cfg.DefaultMaxEventLag = 10
	e.userRepl("u1", "f1", status(0))
	e.userRepl("u1", "f2", status(7))
	e.run(t)
	r.Len(e.pol.failedOver, 1)
	r.Equal("f2", e.pol.userRoutes["u1"])

	// first target unavailable: falls through to the next one
	e = newEnv(t, enabled("f2", "f1"))
	e.hl.st[2].Available = false
	e.userRepl("u1", "f1", status(0))
	e.userRepl("u1", "f2", status(0))
	e.run(t)
	r.Equal("f1", e.pol.userRoutes["u1"])
}

func TestImplicitMainScopeAndIdempotence(t *testing.T) {
	r := require.New(t)
	e := newEnv(t, enabled("f1"))
	// no explicit user routing: implicit main
	e.userRepl("u1", "f1", status(0))
	e.userRepl("u2", "f1", status(0))
	e.pol.userRoutes["u3"] = "f1" // routed elsewhere already: untouched
	e.pol.userRepls[entity.UserReplicationPolicy{User: "u3", FromStorage: "main", ToStorage: "f1"}] = status(0)
	e.run(t)
	r.Len(e.pol.failedOver, 2)
	r.Equal("f1", e.pol.userRoutes["u1"])
	r.Equal("f1", e.pol.userRoutes["u2"])
	r.Len(e.store.events, 2)
	// second run does nothing
	e.run(t)
	r.Len(e.pol.failedOver, 2)
	r.Len(e.store.events, 2)
}

func TestOnlyLockHolderActs(t *testing.T) {
	e := newEnv(t, enabled("f1"))
	e.userRepl("u1", "f1", status(0))
	e.lock.held = true
	e.run(t)
	require.Empty(t, e.pol.failedOver)
	require.Equal(t, 1, e.lock.calls)
	e.lock.held = false
	e.run(t)
	require.Len(t, e.pol.failedOver, 1)
}

func TestWarnClearedWhenStorageRecovers(t *testing.T) {
	e := newEnv(t, enabled("f1"))
	e.userRepl("u1", "f1", status(3))
	e.run(t)
	require.Len(t, e.ctrl.warned, 1)
	e.hl.st[0].Available = true
	e.run(t)
	require.Empty(t, e.ctrl.warned)
}

func TestPolicyListErrorReturned(t *testing.T) {
	e := newEnv(t, enabled("f1"))
	e.store.err = errors.New("boom")
	require.Error(t, e.ctrl.RunOnce(t.Context()))
}

// ---- integration with the real policy service (miniredis) ----

func TestIntegration_RealPolicyService(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()
	redisC := testutil.SetupRedis(t)
	qs := &tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}
	svc := policy.NewService(redisC, qs, "main")

	repl := entity.UserReplicationPolicy{User: "u1", FromStorage: "main", ToStorage: "follower"}
	_, err := svc.AddUserReplicationPolicy(ctx, repl, entity.ReplicationOptions{})
	r.NoError(err)
	id := entity.UniversalFromUserReplication(repl)
	for _, q := range tasks.AllReplicationQueues(id) {
		qs.Queues[q] = 0
	}

	reg := health.NewRegistry(health.Config{FailureThreshold: 1, OpenDuration: time.Hour}, health.WithLogger(zerolog.Nop()))
	reg.Register("main", "follower")
	reg.Observe("main", &net.OpError{Op: "dial", Err: errors.New("refused")})
	r.False(reg.Snapshot()[1].Available) // main (sorted: follower, main)

	pstore := pf.NewStore(redisC, nil)
	r.NoError(pstore.SetFailoverPolicy(ctx, pf.Policy{Source: "main", Enabled: true, Targets: []string{"follower"}}))

	now := time.Now()
	ctrl := New(Config{DefaultAfter: time.Minute, CheckInterval: time.Second, MainStorage: "main"}, svc, reg,
		RedisLocker{Client: redisC}, pstore, zerolog.Nop(), WithClock(func() time.Time { return now }))

	// not yet
	r.NoError(ctrl.RunOnce(ctx))
	rt, err := svc.GetUserRoutings(ctx)
	r.NoError(err)
	r.Equal("main", rt["u1"])
	paused, err := svc.GetReplicationPolicyInfoExtended(ctx, id)
	r.NoError(err)
	r.False(paused.IsPaused)

	// after the timeout
	now = now.Add(2 * time.Minute)
	r.NoError(ctrl.RunOnce(ctx))
	rt, err = svc.GetUserRoutings(ctx)
	r.NoError(err)
	r.Equal("follower", rt["u1"])
	info, err := svc.GetReplicationPolicyInfoExtended(ctx, id)
	r.NoError(err)
	r.True(info.IsPaused)
	events, err := pstore.ListFailoverEvents(ctx)
	r.NoError(err)
	r.Len(events, 1)
	r.Equal("follower", events[0].To)

	// idempotent
	r.NoError(ctrl.RunOnce(ctx))
	events, _ = pstore.ListFailoverEvents(ctx)
	r.Len(events, 1)

	// sync replication must not target the paused/failed replication: the request is routed to follower,
	// and the replication main->follower is dropped from the request context.
	reqCtx, err := svc.BuildProxyContext(ctx, "u1", "bkt")
	r.NoError(err)
	r.Equal("follower", xctx.GetRoutingPolicy(reqCtx))
	r.Empty(xctx.GetReplications(reqCtx))
}
