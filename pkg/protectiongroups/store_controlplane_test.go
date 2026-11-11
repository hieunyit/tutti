// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
)

// fakeCP is an in-memory control plane holding only protection group rows, with injectable failures.
type fakeCP struct {
	mu        sync.Mutex
	rows      map[string][]byte
	failPut   bool
	failDel   bool
	failRead  bool
	readOnly  bool
	auditLogs []string
}

func newFakeCP() *fakeCP { return &fakeCP{rows: map[string][]byte{}} }

type fakeAudit struct {
	controlplane.Audit
	f *fakeCP
}

func (a *fakeAudit) Record(ctx context.Context, action, objectType, objectID string, details any) error {
	a.f.mu.Lock()
	defer a.f.mu.Unlock()
	a.f.auditLogs = append(a.f.auditLogs, action+":"+objectType+":"+objectID)
	return nil
}

func (f *fakeCP) Repos() controlplane.Repos {
	return controlplane.Repos{
		ProtectionGroups: &fakeKV{f: f},
		Audit:            &fakeAudit{f: f},
	}
}
func (f *fakeCP) Tx(context.Context, func(controlplane.Repos) error) error {
	return errors.New("unused")
}
func (f *fakeCP) Close()           {}
func (f *fakeCP) IsReadOnly() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.readOnly }

func (f *fakeCP) set(fn func(*fakeCP)) { f.mu.Lock(); fn(f); f.mu.Unlock() }
func (f *fakeCP) row(t *testing.T, id string) (Group, bool) {
	t.Helper()
	f.mu.Lock()
	raw, ok := f.rows[id]
	f.mu.Unlock()
	if !ok {
		return Group{}, false
	}
	var g Group
	require.NoError(t, json.Unmarshal(raw, &g))
	return g, true
}

// fakeKV embeds the interface so methods it does not need stay unimplemented (nil) instead of breaking the build.
type fakeKV struct {
	controlplane.ProtectionGroups
	f *fakeCP
}

var errPG = fmt.Errorf("connection refused")

func (k *fakeKV) Create(ctx context.Context, v controlplane.RawDoc) (controlplane.Versioned[controlplane.RawDoc], error) {
	return k.Put(ctx, v)
}
func (k *fakeKV) Get(_ context.Context, key string) (controlplane.Versioned[controlplane.RawDoc], error) {
	k.f.mu.Lock()
	defer k.f.mu.Unlock()
	if k.f.failRead {
		return controlplane.Versioned[controlplane.RawDoc]{}, errPG
	}
	raw, ok := k.f.rows[key]
	if !ok {
		return controlplane.Versioned[controlplane.RawDoc]{}, dom.ErrNotFound
	}
	return controlplane.Versioned[controlplane.RawDoc]{Value: controlplane.RawDoc{ID: key, JSON: raw}}, nil
}
func (k *fakeKV) List(context.Context) ([]controlplane.Versioned[controlplane.RawDoc], error) {
	k.f.mu.Lock()
	defer k.f.mu.Unlock()
	if k.f.failRead {
		return nil, errPG
	}
	var out []controlplane.Versioned[controlplane.RawDoc]
	for id, raw := range k.f.rows {
		out = append(out, controlplane.Versioned[controlplane.RawDoc]{Value: controlplane.RawDoc{ID: id, JSON: raw}})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Value.ID < out[j].Value.ID })
	return out, nil
}
func (k *fakeKV) Put(_ context.Context, v controlplane.RawDoc) (controlplane.Versioned[controlplane.RawDoc], error) {
	k.f.mu.Lock()
	defer k.f.mu.Unlock()
	if k.f.failPut {
		return controlplane.Versioned[controlplane.RawDoc]{}, errPG
	}
	if k.f.readOnly {
		return controlplane.Versioned[controlplane.RawDoc]{}, controlplane.ErrReadOnly
	}
	k.f.rows[v.ID] = append([]byte(nil), v.JSON...)
	return controlplane.Versioned[controlplane.RawDoc]{Value: v}, nil
}
func (k *fakeKV) Update(ctx context.Context, v controlplane.RawDoc, _ int64) (controlplane.Versioned[controlplane.RawDoc], error) {
	return k.Put(ctx, v)
}
func (k *fakeKV) Mutate(context.Context, string, func(*controlplane.RawDoc) error) (controlplane.Versioned[controlplane.RawDoc], error) {
	return controlplane.Versioned[controlplane.RawDoc]{}, errors.New("unused")
}
func (k *fakeKV) Delete(_ context.Context, key string) error {
	k.f.mu.Lock()
	defer k.f.mu.Unlock()
	if k.f.failDel {
		return errPG
	}
	if _, ok := k.f.rows[key]; !ok {
		return dom.ErrNotFound
	}
	delete(k.f.rows, key)
	return nil
}

func newCPStore(t *testing.T) (*Store, *fakeCP) {
	s, _, _ := newFenceStore(t)
	cp := newFakeCP()
	s.WithControlPlane(cp)
	return s, cp
}

// Redis is written first; a failed PostgreSQL copy never fails or undoes the Redis commit and is re-pushed by
// the next read. The PostgreSQL copy is recovery-safe (frozen, initializing).
func TestControlPlaneCopyFollowsRedis(t *testing.T) {
	s, cp := newCPStore(t)
	ctx := t.Context()
	cp.set(func(f *fakeCP) { f.failPut = true })
	g := saveGroup(t, s, Group{ID: "g", User: "u", Buckets: []string{"b"}})
	_, ok := cp.row(t, "g")
	require.False(t, ok, "PostgreSQL write failed")
	got, err := s.Get(ctx, "g")
	require.NoError(t, err, "Redis is authoritative")
	require.Equal(t, "ready", got.State)

	cp.set(func(f *fakeCP) { f.failPut = false })
	_, err = s.Get(ctx, "g")
	require.NoError(t, err)
	row, ok := cp.row(t, "g")
	require.True(t, ok, "re-pushed on read")
	require.True(t, row.Spec().Equal(g.Spec()))
	require.Equal(t, "a", row.Primary)
	require.True(t, row.Frozen, "PostgreSQL copy is recovery-safe")
	require.Equal(t, "initializing", row.State)
	require.Empty(t, row.Verified)

	// a stale PostgreSQL copy (older primary, live runtime fields) never overrides Redis
	g.Primary, g.Generation = "b", 2
	require.NoError(t, s.Save(ctx, &g))
	stale, _ := json.Marshal(Group{ID: "g", User: "u", Buckets: []string{"b"}, Members: []string{"a", "b"}, Copies: 2, Primary: "a", Generation: 1, State: "ready"})
	cp.set(func(f *fakeCP) { f.rows["g"] = stale })
	got, err = s.Get(ctx, "g")
	require.NoError(t, err)
	require.Equal(t, "b", got.Primary)
	require.False(t, got.Frozen)
	row, _ = cp.row(t, "g")
	require.Equal(t, "b", row.Primary, "PostgreSQL resynced from Redis")
	require.Equal(t, int64(2), row.Generation)
}

// A delete commits in Redis with a tombstone: a PostgreSQL row left behind (failed delete) never resurrects the
// group and is removed by a later read.
func TestControlPlaneDeleteTombstone(t *testing.T) {
	s, cp := newCPStore(t)
	ctx := t.Context()
	g := saveGroup(t, s, Group{ID: "g", User: "u"})
	cp.set(func(f *fakeCP) { f.failDel = true })
	require.NoError(t, s.Delete(ctx, g))
	_, err := s.Get(ctx, "g")
	require.ErrorIs(t, err, dom.ErrNotFound)
	groups, err := s.List(ctx)
	require.NoError(t, err)
	require.Empty(t, groups)
	_, ok := cp.row(t, "g")
	require.True(t, ok, "delete still failing")
	a, err := s.Admit(ctx, "u", "x", "zzz", "p")
	require.NoError(t, err)
	require.Empty(t, a.Group, "the deleted group is no longer enforced")

	cp.set(func(f *fakeCP) { f.failDel = false })
	_, err = s.List(ctx)
	require.NoError(t, err)
	_, ok = cp.row(t, "g")
	require.False(t, ok, "leftover row removed")

	// re-creating the same ID clears the tombstone
	saveGroup(t, s, Group{ID: "g", User: "u"})
	deleted, err := s.Redis.HExists(ctx, DeletedKey, "g").Result()
	require.NoError(t, err)
	require.False(t, deleted)
	_, ok = cp.row(t, "g")
	require.True(t, ok)
}

// After Redis lost a group, it is restored from PostgreSQL frozen and re-initializing with its last primary, and
// the scope index enforces it again.
func TestControlPlaneRestoreAfterRedisLoss(t *testing.T) {
	s, cp := newCPStore(t)
	ctx := t.Context()
	g := saveGroup(t, s, Group{ID: "g", User: "u"})
	g.Primary, g.Generation = "b", 3
	g.Verified = map[string]int64{"a": 3}
	require.NoError(t, s.Save(ctx, &g))
	_, ok := cp.row(t, "g")
	require.True(t, ok)

	require.NoError(t, s.Redis.Del(ctx, GroupsKey, PromotionKey, ScopeIndexKey).Err())
	groups, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	r := groups[0]
	require.Equal(t, "b", r.Primary)
	require.Equal(t, int64(3), r.Generation)
	require.True(t, r.Frozen)
	require.Equal(t, "initializing", r.State)
	require.Empty(t, r.Verified, "replicas must be re-verified")
	_, err = s.Admit(ctx, "u", "x", "b", "p")
	require.ErrorIs(t, err, dom.ErrUnavailable, "restored group is enforced (frozen)")

	// restoring never overwrites a group that is present in Redis
	r.State, r.Frozen = "ready", false
	require.NoError(t, s.Save(ctx, &r))
	require.NoError(t, s.healRedis(ctx, g))
	cur, err := s.Get(ctx, "g")
	require.NoError(t, err)
	require.Equal(t, "ready", cur.State)
}

// In read-only control plane mode spec mutations are refused before Redis is touched; runtime transitions
// (freeze, promotion bookkeeping) keep working.
func TestControlPlaneReadOnly(t *testing.T) {
	s, cp := newCPStore(t)
	ctx := t.Context()
	g := saveGroup(t, s, Group{ID: "g", User: "u"})
	cp.set(func(f *fakeCP) { f.readOnly = true })
	n := Group{ID: "n", User: "v", Members: []string{"c", "d"}, Copies: 2, State: "ready", Generation: 1}
	require.NoError(t, n.Validate())
	require.ErrorIs(t, s.Save(ctx, &n), controlplane.ErrReadOnly)
	_, err := s.getRedis(ctx, "n")
	require.ErrorIs(t, err, dom.ErrNotFound)
	g.Frozen = true
	require.NoError(t, s.Save(ctx, &g), "runtime change allowed")
	require.ErrorIs(t, s.Delete(ctx, g), controlplane.ErrReadOnly)
}

// The storage fields of the index resolve the group of another identity managing a bucket on a storage.
func TestLookupSharedGroup(t *testing.T) {
	s, _, _ := newFenceStore(t)
	ctx := t.Context()
	saveGroup(t, s, Group{ID: "ga", User: "alice", Buckets: []string{"x"}, Members: []string{"s1", "s2"}})
	saveGroup(t, s, Group{ID: "gw", User: "whole", Members: []string{"s3", "s4"}})
	idx, err := s.Redis.HGetAll(ctx, ScopeIndexKey).Result()
	require.NoError(t, err)
	for _, tc := range []struct{ storage, user, bucket, want string }{
		{"s1", "bob", "x", "ga"},
		{"s2", "bob", "x", "ga"},
		{"s0", "bob", "x", ""},   // not a member
		{"s1", "bob", "y", ""},   // not a managed bucket
		{"s1", "alice", "x", ""}, // own group: the normal fence applies
		{"s3", "bob", "any", ""}, // whole-user groups of other users never apply
		{"s1", "bob", "", ""},    // no bucket
	} {
		got, ok, err := LookupSharedGroup(idx, tc.storage, tc.user, tc.bucket)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, tc.want, got, "%+v", tc)
	}
	_, ok, _ := LookupSharedGroup(map[string]string{indexVersionField: "1"}, "s1", "bob", "x")
	require.False(t, ok, "old index format is incomplete")
	idx["s:s1/x"] = conflictMarker
	_, _, err = LookupSharedGroup(idx, "s1", "bob", "x")
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)

	// an index of the old format is rebuilt by EnsureIndex
	require.NoError(t, s.Redis.HSet(ctx, ScopeIndexKey, indexVersionField, "1").Err())
	require.NoError(t, s.Redis.HDel(ctx, ScopeIndexKey, "s:s1/x").Err())
	require.NoError(t, s.EnsureIndex(ctx))
	idx, err = s.Redis.HGetAll(ctx, ScopeIndexKey).Result()
	require.NoError(t, err)
	require.Equal(t, "ga", idx["s:s1/x"])
}

// The worker applies the proxy's rule: a replication of another identity on a shared connection account never
// writes a bucket a group manages on the destination; the group's own replications pass.
func TestWorkerMiddlewareRefusesSharedDestination(t *testing.T) {
	s, _, _ := newFenceStore(t)
	ctx := t.Context()
	saveGroup(t, s, Group{ID: "ga", User: "alice", Buckets: []string{"x"}, Members: []string{"s1", "s2"}})
	s.SharedAccount = func(storage, user string) bool { return user != "own" }
	lease := s.NewLease("w1", true, 0)
	ran := 0
	h := s.WorkerMiddleware(lease)(asynq.HandlerFunc(func(context.Context, *asynq.Task) error { ran++; return nil }))
	task := func(user, from, to, bucket string) *asynq.Task {
		id := entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{User: user, FromStorage: from, ToStorage: to, FromBucket: bucket, ToBucket: bucket})
		raw, err := json.Marshal(struct{ ID entity.UniversalReplicationID }{id})
		require.NoError(t, err)
		return asynq.NewTask("t", raw)
	}
	err := h.ProcessTask(ctx, task("bob", "s0", "s1", "x"))
	require.ErrorIs(t, err, asynq.SkipRetry)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.NoError(t, h.ProcessTask(ctx, task("bob", "s0", "s3", "x")), "not a member")
	require.NoError(t, h.ProcessTask(ctx, task("bob", "s0", "s1", "y")), "not a managed bucket")
	require.NoError(t, h.ProcessTask(ctx, task("own", "s0", "s1", "x")), "own credentials, not the shared account")
	require.NoError(t, h.ProcessTask(ctx, task("alice", "s1", "s2", "x")), "the group's own replication")
	require.Equal(t, 4, ran)
}

func TestAuditRecordedForAutomaticFreezeAndClearWriter(t *testing.T) {
	s, cp := newCPStore(t)
	ctx := t.Context()

	// 1. Automatic freeze toggle audits
	require.NoError(t, s.SetAutomaticFrozen(ctx, true))
	require.NoError(t, s.SetAutomaticFrozen(ctx, false))

	cp.mu.Lock()
	logs := append([]string(nil), cp.auditLogs...)
	cp.mu.Unlock()

	require.Contains(t, logs, "ENABLE:protection_groups:automatic_freeze")
	require.Contains(t, logs, "DISABLE:protection_groups:automatic_freeze")

	// 2. ClearWriter audits
	g := saveGroup(t, s, Group{ID: "g-audit", User: "alice", Members: []string{"s1", "s2"}})
	g.Frozen = true
	require.NoError(t, s.Save(ctx, &g))

	require.NoError(t, s.ClearWriterKind(ctx, "g-audit", "test-token", true, false))

	cp.mu.Lock()
	logs = append([]string(nil), cp.auditLogs...)
	cp.mu.Unlock()

	require.Contains(t, logs, "CLEAR_WRITER:protection_groups:g-audit")
}

