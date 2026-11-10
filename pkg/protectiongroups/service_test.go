// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/proxystatus"
	"github.com/stretchr/testify/require"
)

// fakeBackend emulates the policy backend: replication progress per destination storage.
type fakeBackend struct {
	store *Store
	mr    *miniredis.Miniredis
	mu    sync.Mutex
	// pending initial-migration objects and event lag per destination storage
	initPending map[string]int
	lag         map[string]int
	commitErr   error
	commits     int
}

func (b *fakeBackend) Validate(context.Context, Group, bool) error { return nil }
func (b *fakeBackend) Ensure(context.Context, Group, entity.UniversalReplicationID) (int64, error) {
	return 1, nil
}
func (b *fakeBackend) Seed(context.Context, entity.UniversalReplicationID, int64) error { return nil }
func (b *fakeBackend) Replication(_ context.Context, id entity.UniversalReplicationID) (entity.ReplicationStatusExtended, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return entity.ReplicationStatusExtended{ReplicationStatus: &entity.ReplicationStatus{},
		InitMigration:  entity.QueueStats{Unprocessed: b.initPending[id.ToStorage()]},
		EventMigration: entity.QueueStats{Unprocessed: b.lag[id.ToStorage()]}}, nil
}
func (b *fakeBackend) Pause(context.Context, Group) error { return nil }
func (b *fakeBackend) Commit(ctx context.Context, before Group, after *Group) error {
	b.mu.Lock()
	err := b.commitErr
	b.mu.Unlock()
	if err != nil {
		return err
	}
	for _, key := range []string{writerKey(before.ID), writerKey(before.ID) + ":workers"} {
		if n, _ := b.store.Redis.HLen(ctx, key).Result(); n != 0 {
			return errors.New("outstanding writes prevent promotion")
		}
	}
	next := *after
	next.Revision = before.Revision
	if err := b.store.Save(ctx, &next); err != nil {
		return err
	}
	after.Revision = next.Revision
	b.mu.Lock()
	b.commits++
	b.mu.Unlock()
	return nil
}

type fakeResyncer struct {
	mu      sync.Mutex
	calls   []string
	frozen  []bool
	failErr error
}

func (r *fakeResyncer) Resync(_ context.Context, g Group, target string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, target)
	r.frozen = append(r.frozen, g.Frozen)
	return r.failErr
}

func healthyReports(now time.Time, storages ...string) []proxystatus.Status {
	var st []proxystatus.StorageHealth
	for _, s := range storages {
		st = append(st, proxystatus.StorageHealth{Name: s, Available: true})
	}
	return []proxystatus.Status{{AppID: "observer", StartedAt: now.Add(-time.Hour), UpdatedAt: now, Storages: st}}
}

func newService(t *testing.T) (*Service, *fakeBackend, *fakeResyncer, *time.Time, *[]proxystatus.Status) {
	s, mr, _ := newFenceStore(t)
	backend := &fakeBackend{store: s, mr: mr, initPending: map[string]int{}, lag: map[string]int{}}
	resync := &fakeResyncer{}
	now := time.Now()
	reports := healthyReports(now, "a", "b", "c")
	svc := &Service{Store: s, Backend: backend, Resyncer: resync,
		Reports: func(context.Context) ([]proxystatus.Status, error) { return reports, nil },
		Now:     func() time.Time { return now }}
	return svc, backend, resync, &now, &reports
}

func reconcileN(t *testing.T, svc *Service, id string, n int) Group {
	t.Helper()
	for i := 0; i < n; i++ {
		require.NoError(t, svc.Reconcile(t.Context(), id))
	}
	g, err := svc.Store.Get(t.Context(), id)
	require.NoError(t, err)
	return g
}

// The bulk catch-up runs with writes admitted; the freeze covers only the final drain + resync.
func TestCreateCatchesUpBeforeFreezing(t *testing.T) {
	svc, backend, resync, _, _ := newService(t)
	ctx := t.Context()
	backend.initPending["b"] = 1000
	g, err := svc.Create(ctx, Group{ID: "g", User: "tenant", Members: []string{"a", "b", "c"}, Copies: 2}, false)
	require.NoError(t, err)
	require.True(t, g.Frozen)

	g = reconcileN(t, svc, "g", 1)
	require.Equal(t, "initializing", g.State)
	require.False(t, g.Frozen, "writes must be admitted during the bulk initial migration")
	a, err := svc.Store.Admit(ctx, "tenant", "x", "a", "p1")
	require.NoError(t, err)
	require.Equal(t, "g", a.Group)
	require.NoError(t, svc.Store.Release(ctx, a))
	require.Empty(t, resync.calls)

	backend.mu.Lock()
	backend.initPending["b"] = 0
	backend.mu.Unlock()
	g = reconcileN(t, svc, "g", 1)
	require.True(t, g.Frozen, "caught up: freeze for the final resync")
	require.Equal(t, "initializing", g.State)
	g = reconcileN(t, svc, "g", 1)
	require.Equal(t, "ready", g.State)
	require.False(t, g.Frozen)
	require.ElementsMatch(t, []string{"b", "c"}, resync.calls)
	require.Equal(t, []bool{true, true}, resync.frozen, "resync only under freeze")
	require.Equal(t, map[string]int64{"b": 1, "c": 1}, g.Verified)
}

func readyGroup(t *testing.T, svc *Service) Group {
	_, err := svc.Create(t.Context(), Group{ID: "g", User: "tenant", Members: []string{"a", "b", "c"}, Copies: 2}, false)
	require.NoError(t, err)
	g := reconcileN(t, svc, "g", 2)
	require.Equal(t, "ready", g.State)
	return g
}

// NEW-02: a crashed proxy (no heartbeat, absent from status) does not block promotion forever.
func TestSwitchoverCompletesAfterCrashedWriterLease(t *testing.T) {
	svc, backend, resync, _, _ := newService(t)
	ctx := t.Context()
	readyGroup(t, svc)
	resync.calls = nil

	crashed := svc.Store.NewLease("crashed-proxy", false, time.Second)
	_, err := crashed.Admit(ctx, "tenant", "x", "a") // never released
	require.NoError(t, err)

	require.NoError(t, svc.RequestSwitch(ctx, "g", "b"))
	g := reconcileN(t, svc, "g", 3)
	require.Equal(t, "draining", g.State, "the writer may still be running: wait")
	require.Equal(t, "a", g.Primary)

	backend.mr.FastForward(2 * time.Second) // lease TTL elapses in Redis
	g = reconcileN(t, svc, "g", 2)
	require.Equal(t, "b", g.Primary, "promotion must proceed once the lease expired")
	require.EqualValues(t, 2, g.Generation)
	require.Equal(t, "b", resync.calls[0], "manual switchover resyncs the target first")
}

// NEW-02: an alive but slow writer keeps blocking promotion as long as it runs.
func TestSwitchoverWaitsForAliveSlowWriter(t *testing.T) {
	svc, _, _, now, reports := newService(t)
	ctx := t.Context()
	readyGroup(t, svc)

	slow := svc.Store.NewLease("slow-proxy", false, time.Second)
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	go slow.Run(runCtx)
	a, err := slow.Admit(ctx, "tenant", "x", "a")
	require.NoError(t, err)
	*reports = append(*reports, proxystatus.Status{AppID: "slow-proxy", StartedAt: now.Add(-time.Hour), UpdatedAt: *now})

	require.NoError(t, svc.RequestSwitch(ctx, "g", "b"))
	deadline := time.Now().Add(2500 * time.Millisecond) // > 2 lease TTLs
	var g Group
	for time.Now().Before(deadline) {
		g = reconcileN(t, svc, "g", 1)
		require.Equal(t, "a", g.Primary, "must not promote while the writer runs")
		time.Sleep(100 * time.Millisecond)
	}
	require.Equal(t, "draining", g.State)

	require.NoError(t, svc.Store.Release(ctx, a))
	g = reconcileN(t, svc, "g", 2)
	require.Equal(t, "b", g.Primary)
}

// A failed commit must not leave a half-applied transition persisted by the LastError path.
func TestFailedCommitDoesNotPersistPartialTransition(t *testing.T) {
	svc, backend, _, _, _ := newService(t)
	_, err := svc.Create(t.Context(), Group{ID: "g", User: "tenant", Members: []string{"a", "b"}, Copies: 2}, false)
	require.NoError(t, err)
	backend.commitErr = fmt.Errorf("boom")
	require.Error(t, svc.Reconcile(t.Context(), "g"))
	g, err := svc.Store.Get(t.Context(), "g")
	require.NoError(t, err)
	require.Equal(t, "promoting", g.State)
	require.True(t, g.Frozen, "group must stay frozen until the commit succeeds")
	require.Equal(t, "boom", g.LastError)

	backend.commitErr = nil
	g = reconcileN(t, svc, "g", 1)
	require.Equal(t, "ready", g.State)
	require.False(t, g.Frozen)
	require.Empty(t, g.LastError)
}

// After a lossless automatic failover the new primary catches up the remaining member while
// writes are admitted, then verifies it under a short freeze.
func TestAutomaticFailoverKeepsReplicatingToRemainingMember(t *testing.T) {
	svc, backend, resync, now, reports := newService(t)
	ctx := t.Context()
	_, err := svc.Create(ctx, Group{ID: "g", User: "tenant", Members: []string{"a", "b", "c"}, Copies: 2, Mode: "auto",
		After: time.Second, Electorate: []string{"p1", "p2", "p3"}}, false)
	require.NoError(t, err)
	g := reconcileN(t, svc, "g", 2)
	require.Equal(t, "ready", g.State)

	down := time.Now().Add(-time.Minute)
	*reports = nil
	for _, p := range []string{"p1", "p2", "p3"} {
		*reports = append(*reports, proxystatus.Status{AppID: p, StartedAt: *now, UpdatedAt: *now, Storages: []proxystatus.StorageHealth{
			{Name: "a", Available: false, LastChange: down}, {Name: "b", Available: true}, {Name: "c", Available: true}}})
	}
	resync.calls = nil
	backend.lag["c"] = 5 // replication from the new primary has a backlog
	g = reconcileN(t, svc, "g", 2)
	require.Equal(t, "b", g.Primary)
	require.Equal(t, "initializing", g.State)
	require.Empty(t, g.Quarantined)
	g = reconcileN(t, svc, "g", 1)
	require.False(t, g.Frozen, "catch-up to c runs with writes admitted")
	a, err := svc.Store.Admit(ctx, "tenant", "x", "b", "p1")
	require.NoError(t, err)
	require.NoError(t, svc.Store.Release(ctx, a))

	backend.mu.Lock()
	backend.lag["c"] = 0
	backend.mu.Unlock()
	g = reconcileN(t, svc, "g", 2)
	require.Equal(t, "ready", g.State)
	require.Equal(t, []string{"c"}, resync.calls, "unavailable old primary is skipped")
	require.Len(t, g.History, 1)
}
