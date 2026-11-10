// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/failover"
)

// scopeSrc is an in-memory failover.ScopeSource.
type scopeSrc struct {
	userRoutes   map[string]string
	bucketRoutes map[string]map[string]string
	user         map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended
	bucket       map[string]map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended
}

func newScopeSrc() *scopeSrc {
	return &scopeSrc{userRoutes: map[string]string{}, bucketRoutes: map[string]map[string]string{},
		user:   map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended{},
		bucket: map[string]map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended{}}
}
func (s *scopeSrc) GetUserRoutings(context.Context) (map[string]string, error) {
	return s.userRoutes, nil
}
func (s *scopeSrc) GetBucketRoutings(_ context.Context, u string) (map[string]string, error) {
	return s.bucketRoutes[u], nil
}
func (s *scopeSrc) ListUserReplicationsInfo(context.Context) (map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended, error) {
	return s.user, nil
}
func (s *scopeSrc) ListBucketReplicationsInfo(_ context.Context, u string) (map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended, error) {
	return s.bucket[u], nil
}
func (s *scopeSrc) addUser(user, from, to string) {
	s.user[entity.UserReplicationPolicy{User: user, FromStorage: from, ToStorage: to}] = entity.ReplicationStatusExtended{ReplicationStatus: &entity.ReplicationStatus{}}
}
func (s *scopeSrc) addBucket(user, from, to, bucket string) {
	if s.bucket[user] == nil {
		s.bucket[user] = map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended{}
	}
	s.bucket[user][entity.BucketReplicationPolicy{User: user, FromStorage: from, ToStorage: to, FromBucket: bucket, ToBucket: bucket}] = entity.ReplicationStatusExtended{ReplicationStatus: &entity.ReplicationStatus{}}
}

// seedCounter counts the initial migrations a group would enqueue and can reject validation.
type seedCounter struct {
	*fakeBackend
	mu          sync.Mutex
	seeds       int
	validateErr error
}

func (b *seedCounter) Seed(ctx context.Context, id entity.UniversalReplicationID, gen int64) error {
	b.mu.Lock()
	b.seeds++
	b.mu.Unlock()
	return b.fakeBackend.Seed(ctx, id, gen)
}
func (b *seedCounter) Validate(context.Context, Group, bool) error { return b.validateErr }

type migEnv struct {
	m       *Migrator
	svc     *Service
	backend *seedCounter
	src     *scopeSrc
	policy  *failover.Store
}

var electorate3 = []string{"p1", "p2", "p3"}

func newMigEnv(t *testing.T, pol failover.Policy) *migEnv {
	t.Helper()
	svc, fb, _, _, _ := newService(t)
	be := &seedCounter{fakeBackend: fb}
	svc.Backend = be
	src := newScopeSrc()
	ps := failover.NewStore(svc.Store.Redis, nil)
	require.NoError(t, ps.SetFailoverPolicy(t.Context(), pol))
	m := &Migrator{Service: svc, Policies: ps, Source: src, MainStorage: "a", Users: func() []string { return []string{"u1", "u2", "u3", "u4", "u5"} },
		Electorate: func(context.Context) []string { return electorate3 }}
	return &migEnv{m: m, svc: svc, backend: be, src: src, policy: ps}
}

func policyAB() failover.Policy {
	return failover.Policy{Source: "a", Enabled: true, Targets: []string{"b", "c"}}
}

func scopeOf(t *testing.T, p MigrationPlan, user, bucket string) MigrationScope {
	t.Helper()
	for _, s := range p.Scopes {
		if s.User == user && s.Bucket == bucket {
			return s
		}
	}
	t.Fatalf("scope %s/%s not in plan", user, bucket)
	return MigrationScope{}
}

func TestMigrationPlanStorageScope(t *testing.T) {
	e := newMigEnv(t, failover.Policy{Source: "a", Enabled: true, Targets: []string{"b", "c"}, After: 5e9, MaxEventLag: ptr(7)})

	p, err := e.m.Plan(t.Context(), "a", MigrationOptions{})
	require.NoError(t, err)
	require.Len(t, p.Scopes, 1, "exactly one storage-level group per failover policy")

	s := p.Scopes[0]
	require.Equal(t, MigrationOK, s.Status)
	require.Empty(t, s.Group.Buckets)
	require.Empty(t, s.Group.User)
	require.Equal(t, []string{"a", "b", "c"}, s.Group.Members)
	require.Equal(t, 3, s.Group.Copies)
	require.Equal(t, "auto", s.Group.Mode)
	require.Equal(t, 7, s.Group.MaxEventLag)
	require.EqualValues(t, 5e9, s.Group.After)
	require.Equal(t, electorate3, s.Group.Electorate)
	require.Equal(t, "storage-a", s.Group.ID)
	require.True(t, p.Complete())
}

func TestMigrationPlanNoTargets(t *testing.T) {
	e := newMigEnv(t, failover.Policy{Source: "a", Enabled: false, Targets: nil})
	p, err := e.m.Plan(t.Context(), "a", MigrationOptions{})
	require.NoError(t, err)
	require.Len(t, p.Scopes, 1)
	s := p.Scopes[0]
	require.Equal(t, MigrationSkip, s.Status)
	require.False(t, p.Complete())
	require.NotEmpty(t, p.NotCovered)
}

func TestMigrationPlanConflicts(t *testing.T) {
	e := newMigEnv(t, policyAB())
	e.m.Service.ValidateCredentials = func(_ context.Context, g Group) error {
		return errors.New("storage b must have a connection account")
	}

	p, err := e.m.Plan(t.Context(), "a", MigrationOptions{})
	require.NoError(t, err)
	s := p.Scopes[0]
	require.Equal(t, MigrationConflict, s.Status)
	require.Contains(t, s.Reason, "storage b must have a connection account")
	require.False(t, p.Complete())

	// overlapping group
	e.m.Service.ValidateCredentials = nil
	_, err = e.svc.Create(t.Context(), Group{ID: "mine", User: "u1", Buckets: []string{"bk1"}, Members: []string{"a", "c"}, Copies: 2}, false)
	require.NoError(t, err)

	p, err = e.m.Plan(t.Context(), "a", MigrationOptions{})
	require.NoError(t, err)
	s = p.Scopes[0]
	require.Equal(t, MigrationConflict, s.Status)
	require.Contains(t, s.Reason, "overlaps existing protection group mine")
}

func joined(s []string) string {
	out := ""
	for _, v := range s {
		out += v + "\n"
	}
	return out
}

func TestMigrationPlanAutoNeedsVoters(t *testing.T) {
	e := newMigEnv(t, policyAB())
	e.m.Electorate = func(context.Context) []string { return []string{"p1"} }
	p, err := e.m.Plan(t.Context(), "a", MigrationOptions{})
	require.NoError(t, err)
	s := p.Scopes[0]
	require.Equal(t, MigrationConflict, s.Status)
	require.Contains(t, s.Reason, "three explicit proxy voters")
	p, err = e.m.Plan(t.Context(), "a", MigrationOptions{Mode: "manual"})
	require.NoError(t, err)
	require.Equal(t, MigrationOK, p.Scopes[0].Status)
	p, err = e.m.Plan(t.Context(), "a", MigrationOptions{Electorate: electorate3})
	require.NoError(t, err)
	require.Equal(t, MigrationOK, p.Scopes[0].Status)
}

func TestMigrationPlanUnknownPolicy(t *testing.T) {
	e := newMigEnv(t, policyAB())
	_, err := e.m.Plan(t.Context(), "nope", MigrationOptions{})
	require.ErrorIs(t, err, dom.ErrNotFound)
	_, err = e.m.Plan(t.Context(), "", MigrationOptions{})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
}

// Applying creates the storage-level group, disables the legacy policy and is idempotent.
func TestMigrationApplyAdoptsWithoutCopyAndIsIdempotent(t *testing.T) {
	e := newMigEnv(t, policyAB())
	ctx := t.Context()
	// the policy is enabled: the legacy check must not stop the migration
	e.svc.ValidateLegacyFailover = LegacyFailoverValidator(e.policy.ListFailoverPolicies)

	res, err := e.m.Apply(ctx, "a", false, MigrationOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"storage-a"}, res.Created)
	require.Empty(t, res.Uncovered)
	require.True(t, res.PolicyDisabled)
	pol, err := e.policy.GetFailoverPolicy(ctx, "a")
	require.NoError(t, err)
	require.False(t, pol.Enabled)
	require.Equal(t, []string{"b", "c"}, pol.Targets, "the policy is disabled, not deleted")

	g, err := e.svc.Store.Get(ctx, "storage-a")
	require.NoError(t, err)
	require.Equal(t, "a", g.Primary)
	require.True(t, g.IsStorageScope())

	// re-run: nothing created, still migrated
	res, err = e.m.Apply(ctx, "a", false, MigrationOptions{})
	require.NoError(t, err)
	require.Empty(t, res.Created)
	require.Equal(t, []string{"storage-a"}, res.Migrated)
	require.True(t, res.PolicyDisabled)
	groups, err := e.svc.Store.List(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 1)
}

// A partial failure keeps the legacy policy enabled; a re-run creates only the missing groups; force disables it
// and lists what is not covered.
func TestMigrationApplyPartialAndForce(t *testing.T) {
	e := newMigEnv(t, policyAB())
	ctx := t.Context()
	broken := true
	e.m.Service.ValidateCredentials = func(_ context.Context, g Group) error {
		if broken {
			return errors.New("no credentials for connection account")
		}
		return nil
	}
	res, err := e.m.Apply(ctx, "a", false, MigrationOptions{})
	require.NoError(t, err)
	require.Empty(t, res.Created)
	require.Len(t, res.Uncovered, 1)
	require.False(t, res.PolicyDisabled)
	pol, _ := e.policy.GetFailoverPolicy(ctx, "a")
	require.True(t, pol.Enabled, "legacy policy stays on while a scope is not migrated")

	// fix the cause: the re-run creates the group
	broken = false
	res, err = e.m.Apply(ctx, "a", false, MigrationOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"storage-a"}, res.Created)
	require.True(t, res.PolicyDisabled)

	// force with a remaining skip (no targets)
	e2 := newMigEnv(t, failover.Policy{Source: "a", Enabled: false, Targets: nil})
	res, err = e2.m.Apply(ctx, "a", true, MigrationOptions{})
	require.NoError(t, err)
	require.Len(t, res.Uncovered, 1)
	require.Equal(t, MigrationSkip, res.Uncovered[0].Status)
	require.True(t, res.PolicyDisabled)
}

func TestMigrationApplyConflictingGroupKeepsPolicy(t *testing.T) {
	e := newMigEnv(t, policyAB())
	ctx := t.Context()
	_, err := e.svc.Create(ctx, Group{ID: "other", User: "u1", Members: []string{"b", "c"}, Copies: 2}, false)
	require.NoError(t, err)
	res, err := e.m.Apply(ctx, "a", false, MigrationOptions{})
	require.NoError(t, err)
	require.Empty(t, res.Created)
	require.Len(t, res.Uncovered, 1)
	require.False(t, res.PolicyDisabled)
	groups, _ := e.svc.Store.List(ctx)
	require.Len(t, groups, 1, "the existing group is untouched")
}

func TestUnprotectedScopes(t *testing.T) {
	e := newMigEnv(t, failover.Policy{Source: "a", Enabled: false, Targets: []string{"b"}})
	ctx := t.Context()
	e.src.addBucket("u1", "a", "b", "covered")
	e.src.addBucket("u1", "a", "c", "covered")
	e.src.addBucket("u1", "a", "b", "open")
	e.src.addUser("u2", "a", "b")
	_, err := e.svc.Create(ctx, Group{ID: "g", User: "u1", Buckets: []string{"covered"}, Members: []string{"a", "b"}, Copies: 2}, false)
	require.NoError(t, err)
	opts := UnprotectedOptions{Storages: []string{"a", "b"}, MainStorage: "a", Users: func() []string { return []string{"u1", "u2"} }, Groups: e.svc.Store.List, Policies: e.policy.ListFailoverPolicies}
	list, err := Unprotected(ctx, e.src, opts)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, UnprotectedScope{Storage: "a", User: "u1", Bucket: "open", Targets: []string{"b"}}, list[0])
	require.Equal(t, "u2", list[1].User)

	// an enabled legacy policy covers every scope of its source
	require.NoError(t, e.policy.SetFailoverPolicy(ctx, failover.Policy{Source: "a", Enabled: true, Targets: []string{"b"}}))
	list, err = Unprotected(ctx, e.src, opts)
	require.NoError(t, err)
	require.Empty(t, list)
	require.NoError(t, CollectUnprotectedMetrics(ctx, e.src, opts))
}

func ptr[T any](v T) *T { return &v }
