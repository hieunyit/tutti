package pg_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
)

var t0 = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// testKV exercises the contract shared by all document repositories.
func testKV[T any](t *testing.T, repo controlplane.KV[T], mk func(i int) T, key func(T) string, change func(*T)) {
	t.Helper()
	ctx := actorCtx("tester")

	a, b := mk(1), mk(2)
	created, err := repo.Create(ctx, a)
	require.NoError(t, err)
	require.EqualValues(t, 1, created.Version)
	require.Equal(t, key(a), key(created.Value))

	_, err = repo.Create(ctx, a)
	require.ErrorIs(t, err, dom.ErrAlreadyExists)

	got, err := repo.Get(ctx, key(a))
	require.NoError(t, err)
	require.EqualValues(t, 1, got.Version)
	require.Equal(t, key(a), key(got.Value))

	_, err = repo.Get(ctx, "missing")
	require.ErrorIs(t, err, dom.ErrNotFound)

	_, err = repo.Put(ctx, b) // Put creates
	require.NoError(t, err)
	list, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Less(t, key(list[0].Value), key(list[1].Value), "sorted by key")

	// optimistic concurrency: the first writer wins, the second sees a conflict
	upd := got.Value
	change(&upd)
	u1, err := repo.Update(ctx, upd, got.Version)
	require.NoError(t, err)
	require.EqualValues(t, 2, u1.Version)
	_, err = repo.Update(ctx, upd, got.Version) // stale
	require.ErrorIs(t, err, controlplane.ErrVersionConflict)
	require.True(t, controlplane.IsConflict(err))
	_, err = repo.Update(ctx, mk(9), 1) // not found
	require.ErrorIs(t, err, dom.ErrNotFound)

	// Mutate serializes read-modify-write
	var wg sync.WaitGroup
	var mu sync.Mutex
	calls := 0
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Mutate(ctx, key(a), func(v *T) error {
				mu.Lock()
				calls++
				mu.Unlock()
				change(v)
				return nil
			})
			require.NoError(t, err)
		}()
	}
	wg.Wait()
	require.Equal(t, 5, calls)
	cur, err := repo.Get(ctx, key(a))
	require.NoError(t, err)
	require.EqualValues(t, 7, cur.Version, "every mutation is a new version, none is lost")

	// a failing mutate changes nothing
	_, err = repo.Mutate(ctx, key(a), func(v *T) error { change(v); return fmt.Errorf("boom") })
	require.ErrorContains(t, err, "boom")
	after, err := repo.Get(ctx, key(a))
	require.NoError(t, err)
	require.Equal(t, cur.Version, after.Version)
	_, err = repo.Mutate(ctx, "missing", func(*T) error { return nil })
	require.ErrorIs(t, err, dom.ErrNotFound)

	require.NoError(t, repo.Delete(ctx, key(a)))
	require.ErrorIs(t, repo.Delete(ctx, key(a)), dom.ErrNotFound)
	list, err = repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
}

func TestRepositoriesCRUD(t *testing.T) {
	s := newStore(t)
	r := s.Repos()

	t.Run("storages", func(t *testing.T) {
		testKV[controlplane.StorageRecord](t, r.Storages, func(i int) controlplane.StorageRecord {
			return controlplane.StorageRecord{Name: fmt.Sprintf("st%d", i), Address: "http://s3:9000", Provider: "Minio", CreatedAt: t0, UpdatedAt: t0, ConnectionAccount: "sealed-blob"}
		}, func(v controlplane.StorageRecord) string { return v.Name }, func(v *controlplane.StorageRecord) { v.RateRPM++ })
	})
	t.Run("s3 identities", func(t *testing.T) {
		testKV[controlplane.S3Identity](t, r.S3Identities, func(i int) controlplane.S3Identity {
			return controlplane.S3Identity{Name: fmt.Sprintf("u%d", i), CreatedAt: t0}
		}, func(v controlplane.S3Identity) string { return v.Name }, func(v *controlplane.S3Identity) { v.CreatedAt = v.CreatedAt.Add(time.Second) })
	})
	t.Run("proxy accounts", func(t *testing.T) {
		testKV[controlplane.ProxyAccountRecord](t, r.ProxyAccounts, func(i int) controlplane.ProxyAccountRecord {
			return controlplane.ProxyAccountRecord{AccessKeyID: fmt.Sprintf("TUT%d", i), User: "alice", Status: "active", CreatedAt: t0, Secret: "sealed"}
		}, func(v controlplane.ProxyAccountRecord) string { return v.AccessKeyID }, func(v *controlplane.ProxyAccountRecord) { v.Description += "x" })
	})
	t.Run("access policies", func(t *testing.T) {
		testKV[controlplane.AccessPolicy](t, r.AccessPolicies, func(i int) controlplane.AccessPolicy {
			return controlplane.AccessPolicy{User: fmt.Sprintf("u%d", i), Permission: "read-only", Buckets: []string{"a*"}}
		}, func(v controlplane.AccessPolicy) string { return v.User }, func(v *controlplane.AccessPolicy) { v.Description += "x" })
	})
	t.Run("console users", func(t *testing.T) {
		testKV[controlplane.ConsoleUser](t, r.ConsoleUsers, func(i int) controlplane.ConsoleUser {
			return controlplane.ConsoleUser{Username: fmt.Sprintf("c%d", i), Role: "admin", PasswordHash: "$2a$hash", CreatedAt: t0, UpdatedAt: t0, PasswordChangedAt: t0}
		}, func(v controlplane.ConsoleUser) string { return v.Username }, func(v *controlplane.ConsoleUser) { v.MustChangePassword = !v.MustChangePassword })
	})
	t.Run("failover policies", func(t *testing.T) {
		lag := 3
		testKV[controlplane.FailoverPolicy](t, r.FailoverPolicies, func(i int) controlplane.FailoverPolicy {
			return controlplane.FailoverPolicy{Source: fmt.Sprintf("s%d", i), Enabled: true, Targets: []string{"t"}, After: time.Minute, MaxEventLag: &lag}
		}, func(v controlplane.FailoverPolicy) string { return v.Source }, func(v *controlplane.FailoverPolicy) { v.After += time.Second })
	})
	t.Run("failover events", func(t *testing.T) {
		testKV[controlplane.FailoverEvent](t, r.FailoverEvents, func(i int) controlplane.FailoverEvent {
			return controlplane.FailoverEvent{User: fmt.Sprintf("u%d", i), Bucket: "b", From: "a", To: "b", Time: t0}
		}, func(v controlplane.FailoverEvent) string { return v.Key() }, func(v *controlplane.FailoverEvent) { v.EventLag++ })
	})
	raw := func(id string, n int) controlplane.RawDoc {
		return controlplane.RawDoc{ID: id, User: "alice", JSON: json.RawMessage(fmt.Sprintf(`{"id":%q,"user":"alice","n":%d}`, id, n))}
	}
	t.Run("protection groups", func(t *testing.T) {
		testKV[controlplane.RawDoc](t, r.ProtectionGroups, func(i int) controlplane.RawDoc { return raw(fmt.Sprintf("g%d", i), 0) },
			func(v controlplane.RawDoc) string { return v.ID }, func(v *controlplane.RawDoc) {
				v.JSON = json.RawMessage(fmt.Sprintf(`{"id":%q,"user":"alice","n":%d}`, v.ID, time.Now().UnixNano()))
			})
	})
	t.Run("storage tests", func(t *testing.T) {
		testKV[controlplane.RawDoc](t, r.StorageTests, func(i int) controlplane.RawDoc { return raw(fmt.Sprintf("st%d", i), 0) },
			func(v controlplane.RawDoc) string { return v.ID }, func(v *controlplane.RawDoc) { v.JSON = json.RawMessage(fmt.Sprintf(`{"n":%d}`, time.Now().UnixNano())) })
	})
	t.Run("system settings", func(t *testing.T) {
		testKV[controlplane.SystemSettings](t, r.SystemSettings, func(i int) controlplane.SystemSettings {
			return controlplane.SystemSettings{
				ID:       fmt.Sprintf("system_%d", i),
				Features: controlplane.SystemFeaturesSettings{Tagging: true, ACL: i%2 == 0},
				Worker:   controlplane.SystemWorkerSettings{Concurrency: 10 + i, ShutdownTimeout: 5 * time.Second},
			}
		}, func(v controlplane.SystemSettings) string { return v.ID }, func(v *controlplane.SystemSettings) {
			v.Worker.Concurrency += 5
		})
	})
}

func TestProxyAccountGeneratedColumns(t *testing.T) {
	s := newStore(t)
	ctx := actorCtx("tester")
	_, err := s.Repos().ProxyAccounts.Create(ctx, controlplane.ProxyAccountRecord{AccessKeyID: "TUT1", User: "alice", Status: "disabled", CreatedAt: t0, Secret: "x"})
	require.NoError(t, err)
	var user, status string
	require.NoError(t, s.Pool().QueryRow(ctx, `SELECT user_name, status FROM proxy_accounts WHERE k='TUT1'`).Scan(&user, &status))
	require.Equal(t, "alice", user)
	require.Equal(t, "disabled", status)
	_, err = s.Repos().ProxyAccounts.Create(ctx, controlplane.ProxyAccountRecord{AccessKeyID: "TUT2"})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
}

func TestConsoleUsersGuardAndLogin(t *testing.T) {
	s := newStore(t)
	ctx := actorCtx("tester")
	cu := s.Repos().ConsoleUsers
	mk := func(name, role string, disabled bool) controlplane.ConsoleUser {
		return controlplane.ConsoleUser{Username: name, Role: role, PasswordHash: "h", Disabled: disabled, CreatedAt: t0, UpdatedAt: t0, PasswordChangedAt: t0}
	}
	_, err := cu.Create(ctx, mk("root", "admin", false))
	require.NoError(t, err)
	_, err = cu.Create(ctx, mk("viewer", "viewer", false))
	require.NoError(t, err)

	demote := func(u *controlplane.ConsoleUser) (bool, bool, error) { u.Role = "viewer"; return false, true, nil }
	// the last enabled admin cannot be demoted
	err = cu.MutateGuarded(ctx, "root", controlplane.ConsoleUserGuard{}, demote)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	got, err := cu.Get(ctx, "root")
	require.NoError(t, err)
	require.Equal(t, "admin", got.Value.Role)
	// a config admin counts
	require.NoError(t, cu.MutateGuarded(ctx, "root", controlplane.ConsoleUserGuard{ConfigAdmins: 1}, demote))
	// removal
	_, err = cu.Create(ctx, mk("second", "admin", false))
	require.NoError(t, err)
	require.NoError(t, cu.MutateGuarded(ctx, "second", controlplane.ConsoleUserGuard{}, func(u *controlplane.ConsoleUser) (bool, bool, error) { return true, false, nil }))
	_, err = cu.Get(ctx, "second")
	require.ErrorIs(t, err, dom.ErrNotFound)
	require.ErrorIs(t, cu.MutateGuarded(ctx, "ghost", controlplane.ConsoleUserGuard{}, demote), dom.ErrNotFound)

	// two concurrent demotions of two admins: at least one admin must remain
	for _, n := range []string{"a1", "a2"} {
		_, err = cu.Create(ctx, mk(n, "admin", false))
		require.NoError(t, err)
	}
	require.NoError(t, cu.MutateGuarded(ctx, "root", controlplane.ConsoleUserGuard{}, func(u *controlplane.ConsoleUser) (bool, bool, error) { u.Role = "admin"; return false, false, nil }))
	require.NoError(t, cu.MutateGuarded(ctx, "root", controlplane.ConsoleUserGuard{}, demote))
	var wg sync.WaitGroup
	res := make([]error, 2)
	for i, n := range []string{"a1", "a2"} {
		wg.Add(1)
		go func() { defer wg.Done(); res[i] = cu.MutateGuarded(ctx, n, controlplane.ConsoleUserGuard{}, demote) }()
	}
	wg.Wait()
	failed := 0
	for _, e := range res {
		if e != nil {
			require.ErrorIs(t, e, dom.ErrFailedPrecondition)
			failed++
		}
	}
	require.Equal(t, 1, failed, "exactly one of two concurrent demotions is rejected")

	// last login lives in its own column and does not change the version
	before, _ := cu.Get(ctx, "viewer")
	at := t0.Add(time.Hour)
	require.NoError(t, cu.TouchLogin(ctx, "viewer", at))
	after, err := cu.Get(ctx, "viewer")
	require.NoError(t, err)
	require.True(t, at.Equal(after.Value.LastLoginAt))
	require.Equal(t, before.Version, after.Version)
	require.NoError(t, cu.TouchLogin(ctx, "nobody", at)) // never creates
	_, err = cu.Get(ctx, "nobody")
	require.ErrorIs(t, err, dom.ErrNotFound)
	// a Put cannot clobber it
	_, err = cu.Put(ctx, mk("viewer", "viewer", true))
	require.NoError(t, err)
	after, _ = cu.Get(ctx, "viewer")
	require.True(t, at.Equal(after.Value.LastLoginAt))
}

func TestDynamicCredentials(t *testing.T) {
	s := newStore(t)
	ctx := actorCtx("tester")
	d := s.Repos().DynamicCredentials
	c := controlplane.DynamicCredential{StorageType: "S3", Storage: "st1", User: "alice", Sealed: "sealed1"}
	require.NoError(t, d.Set(ctx, c))
	c.Sealed = "sealed2"
	require.NoError(t, d.Set(ctx, c))
	got, err := d.Get(ctx, "S3", "st1", "alice")
	require.NoError(t, err)
	require.Equal(t, "sealed2", got.Sealed)
	require.NoError(t, d.Set(ctx, controlplane.DynamicCredential{StorageType: "SWIFT", Storage: "st1", User: "alice", Sealed: "sw"}))
	list, err := d.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.ErrorIs(t, d.Set(ctx, controlplane.DynamicCredential{}), dom.ErrInvalidArg)

	require.NoError(t, d.Delete(ctx, "st1", "alice")) // both types
	_, err = d.Get(ctx, "S3", "st1", "alice")
	require.ErrorIs(t, err, dom.ErrNotFound)
	require.NoError(t, d.Delete(ctx, "st1", "alice")) // absent is fine

	_, ok, err := d.Salt(ctx)
	require.NoError(t, err)
	require.False(t, ok)
	stored, err := d.SetSaltIfAbsent(ctx, "4:abc")
	require.NoError(t, err)
	require.True(t, stored)
	stored, err = d.SetSaltIfAbsent(ctx, "4:other")
	require.NoError(t, err)
	require.False(t, stored)
	salt, ok, _ := d.Salt(ctx)
	require.True(t, ok)
	require.Equal(t, "4:abc", salt)
}

func TestRouting(t *testing.T) {
	s := newStore(t)
	ctx := actorCtx("tester")
	r := s.Repos().Routing

	_, err := r.Get(ctx, "alice", "")
	require.ErrorIs(t, err, dom.ErrNotFound)
	require.ErrorIs(t, r.Update(ctx, "alice", "", "s2"), dom.ErrNotFound)
	require.NoError(t, r.Set(ctx, "alice", "", "s1"))
	require.NoError(t, r.Set(ctx, "alice", "b1", "s2"))
	require.NoError(t, r.Update(ctx, "alice", "", "s3"))
	st, err := r.Get(ctx, "alice", "")
	require.NoError(t, err)
	require.Equal(t, "s3", st)

	require.NoError(t, r.Block(ctx, "alice", "b1"))
	require.NoError(t, r.Block(ctx, "alice", "b1")) // idempotent
	_, err = r.Get(ctx, "alice", "b1")
	require.ErrorIs(t, err, dom.ErrRoutingBlock)
	bl, err := r.ListBucketBlocks(ctx, "alice")
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"b1": true}, bl)
	require.NoError(t, r.Unblock(ctx, "alice", "b1"))

	require.NoError(t, r.Block(ctx, "alice", ""))
	ub, _ := r.ListUserBlocks(ctx)
	require.Equal(t, map[string]bool{"alice": true}, ub)
	ur, _ := r.ListUserRoutes(ctx)
	require.Equal(t, map[string]string{"alice": "s3"}, ur)
	br, _ := r.ListBucketRoutes(ctx, "alice")
	require.Equal(t, map[string]string{"b1": "s2"}, br)

	require.NoError(t, r.DeleteAllUser(ctx, "alice"))
	_, err = r.Get(ctx, "alice", "")
	require.ErrorIs(t, err, dom.ErrNotFound)
	require.NoError(t, r.DeleteRoute(ctx, "alice", "missing")) // like HDEL
	exp, err := r.Export(ctx)
	require.NoError(t, err)
	require.Len(t, exp.Routes, 1) // the bucket route
	require.Empty(t, exp.Blocks)
}

func TestReplications(t *testing.T) {
	s := newStore(t)
	ctx := actorCtx("tester")
	r := s.Repos().Replications
	u := controlplane.ReplicationID{User: "alice", FromStorage: "s1", ToStorage: "s2"}
	bk := controlplane.ReplicationID{User: "bob", FromStorage: "s1", ToStorage: "s2", FromBucket: "b", ToBucket: "b2"}

	require.NoError(t, r.Add(ctx, u, controlplane.ReplicationStatus{AgentURL: "http://agent"}))
	require.NoError(t, r.Add(ctx, controlplane.ReplicationID{User: "alice", FromStorage: "s1", ToStorage: "s3"}, controlplane.ReplicationStatus{}))
	require.NoError(t, r.Add(ctx, bk, controlplane.ReplicationStatus{EventSource: "proxy"}))
	require.ErrorIs(t, r.Add(ctx, controlplane.ReplicationID{User: "x", FromStorage: "s", ToStorage: "s"}, controlplane.ReplicationStatus{}), dom.ErrInvalidArg)
	require.ErrorIs(t, r.Add(ctx, controlplane.ReplicationID{User: "x", FromStorage: "s", ToStorage: "t", FromBucket: "b"}, controlplane.ReplicationStatus{}), dom.ErrInvalidArg)

	ps, err := r.Policies(ctx, "alice", "")
	require.NoError(t, err)
	require.Len(t, ps, 2)
	require.Equal(t, "s2", ps[0].ToStorage)
	_, err = r.Policies(ctx, "nobody", "")
	require.ErrorIs(t, err, dom.ErrNotFound)

	ex, err := r.BucketReplicationExistsForUser(ctx, "bob")
	require.NoError(t, err)
	require.True(t, ex)
	ex, _ = r.BucketReplicationExistsForUser(ctx, "alice")
	require.False(t, ex)
	inUse, _ := r.IsDestinationInUse(ctx, "bob", "s2", "b2")
	require.True(t, inUse)
	inUse, _ = r.IsDestinationInUse(ctx, "bob", "s2", "other")
	require.False(t, inUse)

	g0, _ := r.Generation(ctx, u)
	require.EqualValues(t, 0, g0)
	g1, err := r.IncrementGeneration(ctx, u)
	require.NoError(t, err)
	require.EqualValues(t, 1, g1)
	g2, _ := r.IncrementGeneration(ctx, u)
	require.EqualValues(t, 2, g2)

	// removal keeps the status (archived) and the generation, like Redis
	require.NoError(t, r.Remove(ctx, u))
	require.NoError(t, r.Archive(ctx, u))
	st, err := r.GetStatus(ctx, u)
	require.NoError(t, err)
	require.True(t, st.Archived)
	require.NotNil(t, st.ArchivedAt)
	g, _ := r.Generation(ctx, u)
	require.EqualValues(t, 2, g)
	all, err := r.ListStatuses(ctx, "alice")
	require.NoError(t, err)
	require.Len(t, all, 2)
	require.NoError(t, r.DeleteStatus(ctx, u))
	_, err = r.GetStatus(ctx, u)
	require.ErrorIs(t, err, dom.ErrNotFound)
	require.ErrorIs(t, r.Archive(ctx, u), dom.ErrNotFound)

	// re-adding after removal brings back an active, not archived status
	require.NoError(t, r.Add(ctx, u, controlplane.ReplicationStatus{}))
	st, _ = r.GetStatus(ctx, u)
	require.False(t, st.Archived)
	list, err := r.ListAll(ctx)
	require.NoError(t, err)
	require.Len(t, list, 3)
	gens, err := r.ListGenerations(ctx)
	require.NoError(t, err)
	require.Len(t, gens, 1)

	require.NoError(t, r.SetGeneration(ctx, bk, 7))
	g, _ = r.Generation(ctx, bk)
	require.EqualValues(t, 7, g)
}

func TestIdenticalWritesAreNoOps(t *testing.T) {
	s := newStore(t)
	ctx := actorCtx("tester")
	r := s.Repos().Replications
	id := controlplane.ReplicationID{User: "alice", FromStorage: "s1", ToStorage: "s2"}
	st := controlplane.ReplicationStatus{CreatedAt: t0, AgentURL: "a"}
	require.NoError(t, r.Add(ctx, id, st))
	require.NoError(t, r.SetGeneration(ctx, id, 4))
	n := countAudit(t, s)
	require.NoError(t, r.Add(ctx, id, st))
	require.NoError(t, r.PutStatus(ctx, id, st))
	require.NoError(t, r.SetGeneration(ctx, id, 4))
	require.Equal(t, n, countAudit(t, s), "re-applying the same state writes no audit rows")
}

func countAudit(t *testing.T, s interface {
	Repos() controlplane.Repos
}) int {
	t.Helper()
	es, err := s.Repos().Audit.List(context.Background(), controlplane.AuditFilter{Limit: 1000})
	require.NoError(t, err)
	return len(es)
}
