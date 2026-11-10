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

func TestMigrationPlanUserAndBucketScopes(t *testing.T) {
	e := newMigEnv(t, failover.Policy{Source: "a", Enabled: true, Targets: []string{"b", "c"}, After: 5e9, MaxEventLag: ptr(7)})
	e.src.addBucket("u2", "a", "b", "bk1")
	e.src.addBucket("u2", "a", "c", "bk1")
	e.src.addBucket("u2", "a", "b", "bk2") // no replication to c: member skipped
	e.src.addBucket("u3", "a", "x", "bk3") // replication to a non-target only: skipped scope
	e.src.addUser("u4", "z", "b")          // other source: not part of the policy
	e.src.userRoutes["u5"] = "b"           // routed away: the legacy controller ignores it
	e.src.addUser("u5", "a", "b")

	p, err := e.m.Plan(t.Context(), "a", MigrationOptions{})
	require.NoError(t, err)
	require.Len(t, p.Scopes, 3, "exactly the scopes the legacy controller acts on")

	s := scopeOf(t, p, "u2", "bk1")
	require.Equal(t, MigrationOK, s.Status)
	require.Equal(t, []string{"bk1"}, s.Group.Buckets)
	require.Equal(t, []string{"a", "b", "c"}, s.Group.Members)
	require.Equal(t, 3, s.Group.Copies)
	require.Equal(t, "auto", s.Group.Mode)
	require.Equal(t, 7, s.Group.MaxEventLag)
	require.EqualValues(t, 5e9, s.Group.After)
	require.Equal(t, electorate3, s.Group.Electorate)
	require.Equal(t, "legacy-a-u2-bk1", s.Group.ID)

	s = scopeOf(t, p, "u2", "bk2")
	require.Equal(t, MigrationOK, s.Status)
	require.Equal(t, []string{"a", "b"}, s.Group.Members)
	require.Equal(t, 2, s.Group.Copies)
	require.Equal(t, []string{"c"}, s.SkippedTargets)
	require.Contains(t, s.Reason, "no replication to c")

	s = scopeOf(t, p, "u3", "bk3")
	require.Equal(t, MigrationSkip, s.Status)
	require.False(t, p.Complete())
	require.NotEmpty(t, p.NotCovered)
}

func TestMigrationPlanWholeUserScope(t *testing.T) {
	e := newMigEnv(t, policyAB())
	e.src.addUser("u1", "a", "b")
	e.src.addUser("u1", "a", "c")
	p, err := e.m.Plan(t.Context(), "a", MigrationOptions{})
	require.NoError(t, err)
	require.Len(t, p.Scopes, 1)
	s := scopeOf(t, p, "u1", "")
	require.Equal(t, MigrationOK, s.Status)
	require.Equal(t, []string{"a", "b", "c"}, s.Group.Members)
	require.Empty(t, s.Group.Buckets)
	require.Equal(t, "legacy-a-u1", s.Group.ID)

	// groups of different users must not share a member when one of them covers the whole user
	e.src.addUser("u4", "a", "b")
	p, err = e.m.Plan(t.Context(), "a", MigrationOptions{})
	require.NoError(t, err)
	require.Equal(t, MigrationOK, scopeOf(t, p, "u1", "").Status)
	require.Equal(t, MigrationConflict, scopeOf(t, p, "u4", "").Status)
}

func TestMigrationPlanConflicts(t *testing.T) {
	e := newMigEnv(t, policyAB())
	e.src.addBucket("u1", "a", "b", "bk1")
	e.src.addBucket("u2", "a", "b", "bk")
	e.src.addBucket("u3", "a", "b", "bk")  // same bucket name on a shared member as u2
	e.src.addBucket("u5", "a", "b", "bk5") // credentials failure
	e.m.Service.ValidateCredentials = func(_ context.Context, g Group) error {
		if g.User == "u5" {
			return errors.New("no credentials for u5 on b")
		}
		return nil
	}
	// an existing group over u1's bucket with other members
	_, err := e.svc.Create(t.Context(), Group{ID: "mine", User: "u1", Buckets: []string{"bk1"}, Members: []string{"a", "c"}, Copies: 2}, false)
	require.NoError(t, err)

	p, err := e.m.Plan(t.Context(), "a", MigrationOptions{})
	require.NoError(t, err)
	s := scopeOf(t, p, "u1", "bk1")
	require.Equal(t, MigrationConflict, s.Status)
	require.Contains(t, s.Reason, "overlaps existing protection group mine")
	s = scopeOf(t, p, "u5", "bk5")
	require.Equal(t, MigrationConflict, s.Status)
	require.Contains(t, s.Reason, "no credentials")
	// u2 is proposed first, u3 (same bucket name, shared member) conflicts with that proposal
	require.Equal(t, MigrationOK, scopeOf(t, p, "u2", "bk").Status)
	s = scopeOf(t, p, "u3", "bk")
	require.Equal(t, MigrationConflict, s.Status)
	require.Contains(t, s.Reason, "another scope")
	require.False(t, p.Complete())
	require.Contains(t, joined(p.NotCovered), "u1/bk1: conflict")

	// the backend (routing, unmanaged or paused replications) has the last word
	e.backend.validateErr = dom.ErrFailedPrecondition
	p, err = e.m.Plan(t.Context(), "a", MigrationOptions{})
	require.NoError(t, err)
	require.Equal(t, MigrationConflict, scopeOf(t, p, "u2", "bk").Status)
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
	e.src.addUser("u1", "a", "b")
	e.m.Electorate = func(context.Context) []string { return []string{"p1"} }
	p, err := e.m.Plan(t.Context(), "a", MigrationOptions{})
	require.NoError(t, err)
	s := scopeOf(t, p, "u1", "")
	require.Equal(t, MigrationConflict, s.Status)
	require.Contains(t, s.Reason, "three explicit proxy voters")
	p, err = e.m.Plan(t.Context(), "a", MigrationOptions{Mode: "manual"})
	require.NoError(t, err)
	require.Equal(t, MigrationOK, scopeOf(t, p, "u1", "").Status)
	p, err = e.m.Plan(t.Context(), "a", MigrationOptions{Electorate: electorate3})
	require.NoError(t, err)
	require.Equal(t, MigrationOK, scopeOf(t, p, "u1", "").Status)
}

func TestMigrationPlanUnknownPolicy(t *testing.T) {
	e := newMigEnv(t, policyAB())
	_, err := e.m.Plan(t.Context(), "nope", MigrationOptions{})
	require.ErrorIs(t, err, dom.ErrNotFound)
	_, err = e.m.Plan(t.Context(), "", MigrationOptions{})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
}

// Applying adopts the existing replications (nothing is seeded, so no initial migration is enqueued), disables
// the legacy policy once everything is migrated and is idempotent.
func TestMigrationApplyAdoptsWithoutCopyAndIsIdempotent(t *testing.T) {
	e := newMigEnv(t, policyAB())
	ctx := t.Context()
	e.src.addBucket("u1", "a", "b", "bk1")
	e.src.addBucket("u1", "a", "c", "bk1")
	e.src.addBucket("u2", "a", "b", "bk2")
	// the policy is enabled: the legacy check must not stop the migration
	e.svc.ValidateLegacyFailover = LegacyFailoverValidator(e.policy.ListFailoverPolicies)

	res, err := e.m.Apply(ctx, "a", false, MigrationOptions{})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"legacy-a-u1-bk1", "legacy-a-u2-bk2"}, res.Created)
	require.Empty(t, res.Uncovered)
	require.True(t, res.PolicyDisabled)
	pol, err := e.policy.GetFailoverPolicy(ctx, "a")
	require.NoError(t, err)
	require.False(t, pol.Enabled)
	require.Equal(t, []string{"b", "c"}, pol.Targets, "the policy is disabled, not deleted")

	g, err := e.svc.Store.Get(ctx, "legacy-a-u1-bk1")
	require.NoError(t, err)
	require.Equal(t, "a", g.Primary)
	require.Len(t, g.Seeded, 2, "existing replications are recorded as seeded")
	for id, gen := range g.Seeded {
		require.NotZero(t, gen, id)
	}
	for i := 0; i < 3; i++ {
		require.NoError(t, e.svc.Reconcile(ctx, "legacy-a-u1-bk1"))
		require.NoError(t, e.svc.Reconcile(ctx, "legacy-a-u2-bk2"))
	}
	require.Zero(t, e.backend.seeds, "adopted replications must not be re-seeded (no data copy)")

	// re-run: nothing created, still migrated
	res, err = e.m.Apply(ctx, "a", false, MigrationOptions{})
	require.NoError(t, err)
	require.Empty(t, res.Created)
	require.ElementsMatch(t, []string{"legacy-a-u1-bk1", "legacy-a-u2-bk2"}, res.Migrated)
	require.True(t, res.PolicyDisabled)
	groups, err := e.svc.Store.List(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	require.Zero(t, e.backend.seeds)
}

// A partial failure keeps the legacy policy enabled; a re-run creates only the missing groups; force disables it
// and lists what is not covered.
func TestMigrationApplyPartialAndForce(t *testing.T) {
	e := newMigEnv(t, policyAB())
	ctx := t.Context()
	e.src.addBucket("u1", "a", "b", "ok")
	e.src.addBucket("u2", "a", "b", "bad")
	broken := true
	e.m.Service.ValidateCredentials = func(_ context.Context, g Group) error {
		if broken && g.User == "u2" {
			return errors.New("no credentials for u2")
		}
		return nil
	}
	res, err := e.m.Apply(ctx, "a", false, MigrationOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"legacy-a-u1-ok"}, res.Created)
	require.Len(t, res.Uncovered, 1)
	require.Equal(t, "u2", res.Uncovered[0].User)
	require.False(t, res.PolicyDisabled)
	pol, _ := e.policy.GetFailoverPolicy(ctx, "a")
	require.True(t, pol.Enabled, "legacy policy stays on while a scope is not migrated")

	// fix the cause: the re-run creates only the missing group
	broken = false
	res, err = e.m.Apply(ctx, "a", false, MigrationOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{"legacy-a-u2-bad"}, res.Created)
	require.Equal(t, []string{"legacy-a-u1-ok"}, res.Migrated)
	require.True(t, res.PolicyDisabled)

	// force with a remaining conflict
	e2 := newMigEnv(t, policyAB())
	e2.src.addBucket("u1", "a", "b", "ok")
	e2.src.addBucket("u2", "a", "x", "orphan") // replication to a non-target: skipped scope
	res, err = e2.m.Apply(ctx, "a", true, MigrationOptions{})
	require.NoError(t, err)
	require.Len(t, res.Uncovered, 1)
	require.Equal(t, MigrationSkip, res.Uncovered[0].Status)
	require.True(t, res.PolicyDisabled)
	pol, _ = e2.policy.GetFailoverPolicy(ctx, "a")
	require.False(t, pol.Enabled)
}

func TestMigrationApplyConflictingGroupKeepsPolicy(t *testing.T) {
	e := newMigEnv(t, policyAB())
	ctx := t.Context()
	e.src.addUser("u1", "a", "b")
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
