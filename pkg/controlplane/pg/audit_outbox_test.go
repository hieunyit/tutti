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
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/dom"
)

func TestAuditRowForEveryMutation(t *testing.T) {
	s := newStore(t)
	ctx := actorCtx("alice@console")
	r := s.Repos()

	acc := controlplane.ProxyAccountRecord{AccessKeyID: "TUT1", User: "bob", Status: "active", CreatedAt: t0, Secret: "SEALED-SECRET", PreviousSecret: "SEALED-OLD"}
	_, err := r.ProxyAccounts.Create(ctx, acc)
	require.NoError(t, err)
	_, err = r.ProxyAccounts.Mutate(ctx, "TUT1", func(v *controlplane.ProxyAccountRecord) error { v.Status = "disabled"; return nil })
	require.NoError(t, err)
	require.NoError(t, r.ProxyAccounts.Delete(ctx, "TUT1"))

	es, err := r.Audit.List(ctx, controlplane.AuditFilter{ObjectType: "proxy_accounts", ObjectID: "TUT1"})
	require.NoError(t, err)
	require.Len(t, es, 3)
	// newest first
	require.Equal(t, "delete", es[0].Action)
	require.Equal(t, "update", es[1].Action)
	require.Equal(t, "create", es[2].Action)
	for _, e := range es {
		require.Equal(t, "alice@console", e.Actor)
		require.Equal(t, "req-1", e.RequestID)
		require.WithinDuration(t, time.Now(), e.At, time.Minute)
	}
	require.Nil(t, es[2].Before)
	require.Nil(t, es[0].After)

	// before/after of the update, with secrets redacted
	var before, after map[string]any
	require.NoError(t, json.Unmarshal(es[1].Before, &before))
	require.NoError(t, json.Unmarshal(es[1].After, &after))
	require.Equal(t, "active", before["status"])
	require.Equal(t, "disabled", after["status"])
	require.Equal(t, pg.Redacted, after["secret"])
	require.Equal(t, pg.Redacted, after["previousSecret"])
	for _, e := range es {
		require.NotContains(t, string(e.Before)+string(e.After), "SEALED")
	}
	// the stored document itself keeps the (sealed) secret
	_, err = r.ProxyAccounts.Create(ctx, acc)
	require.NoError(t, err)
	got, _ := r.ProxyAccounts.Get(ctx, "TUT1")
	require.Equal(t, "SEALED-SECRET", got.Value.Secret)

	// other families redact their secrets too
	_, err = r.Storages.Create(ctx, controlplane.StorageRecord{Name: "st", Address: "http://x", ConnectionAccount: "SEALED-CONN", PreviousAccount: "SEALED-PREV", CreatedAt: t0, UpdatedAt: t0})
	require.NoError(t, err)
	_, err = r.ConsoleUsers.Create(ctx, controlplane.ConsoleUser{Username: "root", Role: "admin", PasswordHash: "$2a$12$HASH", CreatedAt: t0, UpdatedAt: t0, PasswordChangedAt: t0})
	require.NoError(t, err)
	require.NoError(t, r.DynamicCredentials.Set(ctx, controlplane.DynamicCredential{StorageType: "S3", Storage: "st", User: "bob", Sealed: "SEALED-CRED"}))
	_, err = r.DynamicCredentials.SetSaltIfAbsent(ctx, "4:SALTVALUE")
	require.NoError(t, err)
	all, err := r.Audit.List(ctx, controlplane.AuditFilter{Limit: 1000})
	require.NoError(t, err)
	for _, e := range all {
		for _, secret := range []string{"SEALED", "$2a$12$HASH", "SALTVALUE"} {
			require.NotContains(t, string(e.Before)+string(e.After), secret, "%s/%s", e.ObjectType, e.ObjectID)
		}
	}

	// filters and pagination
	byType, _ := r.Audit.List(ctx, controlplane.AuditFilter{ObjectType: "storages"})
	require.Len(t, byType, 1)
	page1, _ := r.Audit.List(ctx, controlplane.AuditFilter{Limit: 2})
	require.Len(t, page1, 2)
	page2, _ := r.Audit.List(ctx, controlplane.AuditFilter{Limit: 2, BeforeID: page1[1].ID})
	require.Less(t, page2[0].ID, page1[1].ID)
	byActor, _ := r.Audit.List(ctx, controlplane.AuditFilter{Actor: "nobody"})
	require.Empty(t, byActor)

	// the default actor
	_, err = r.S3Identities.Create(context.Background(), controlplane.S3Identity{Name: "u", CreatedAt: t0})
	require.NoError(t, err)
	e, _ := r.Audit.List(ctx, controlplane.AuditFilter{ObjectType: "s3_identities"})
	require.Equal(t, "system", e[0].Actor)
}

func TestRedact(t *testing.T) {
	out := pg.Redact([]byte(`{"a":1,"secret":"x","nested":{"passwordHash":"h","keep":"v","empty":""},"list":[{"sealed":"s"}],"previousSecret":""}`))
	var m map[string]any
	require.NoError(t, json.Unmarshal(out, &m))
	require.Equal(t, "***", m["secret"])
	require.Equal(t, "", m["previousSecret"], "empty secrets stay empty so absence is visible")
	nested := m["nested"].(map[string]any)
	require.Equal(t, "***", nested["passwordHash"])
	require.Equal(t, "v", nested["keep"])
	require.Equal(t, "***", m["list"].([]any)[0].(map[string]any)["sealed"])
	require.Equal(t, `"***"`, string(pg.Redact([]byte(`not json`))))
}

func TestFailedTransactionLeavesNoAuditRow(t *testing.T) {
	s := newStore(t)
	ctx := actorCtx("tester")
	r := s.Repos()

	// a multi-object transaction that fails after its first mutation rolls back data, audit and outbox together
	err := s.Tx(ctx, func(tx controlplane.Repos) error {
		if _, err := tx.Storages.Create(ctx, controlplane.StorageRecord{Name: "st", Address: "http://x", CreatedAt: t0, UpdatedAt: t0}); err != nil {
			return err
		}
		if err := tx.Routing.Set(ctx, "alice", "", "st"); err != nil {
			return err
		}
		return fmt.Errorf("boom")
	})
	require.ErrorContains(t, err, "boom")
	_, err = r.Storages.Get(ctx, "st")
	require.ErrorIs(t, err, dom.ErrNotFound)
	require.Zero(t, countAudit(t, s))
	var n int
	require.NoError(t, s.Pool().QueryRow(ctx, `SELECT count(*) FROM outbox`).Scan(&n))
	require.Zero(t, n)

	// a failing statement inside the transaction (duplicate) aborts everything as well
	_, err = r.Storages.Create(ctx, controlplane.StorageRecord{Name: "dup", Address: "http://x", CreatedAt: t0, UpdatedAt: t0})
	require.NoError(t, err)
	before := countAudit(t, s)
	err = s.Tx(ctx, func(tx controlplane.Repos) error {
		if err := tx.Routing.Set(ctx, "bob", "", "dup"); err != nil {
			return err
		}
		_, err := tx.Storages.Create(ctx, controlplane.StorageRecord{Name: "dup"})
		return err
	})
	require.ErrorIs(t, err, dom.ErrAlreadyExists)
	require.Equal(t, before, countAudit(t, s))
	_, err = r.Routing.Get(ctx, "bob", "")
	require.ErrorIs(t, err, dom.ErrNotFound)

	// the successful transaction commits everything with consecutive audit rows
	require.NoError(t, s.Tx(ctx, func(tx controlplane.Repos) error {
		if _, err := tx.Storages.Create(ctx, controlplane.StorageRecord{Name: "ok", Address: "http://x", CreatedAt: t0, UpdatedAt: t0}); err != nil {
			return err
		}
		return tx.Routing.Set(ctx, "carol", "", "ok")
	}))
	require.Equal(t, before+2, countAudit(t, s))

	// failed single mutations leave no audit row either
	before = countAudit(t, s)
	_, err = r.Storages.Update(ctx, controlplane.StorageRecord{Name: "ok"}, 99)
	require.ErrorIs(t, err, controlplane.ErrVersionConflict)
	_, err = r.Storages.Mutate(ctx, "ok", func(v *controlplane.StorageRecord) error { return fmt.Errorf("nope") })
	require.Error(t, err)
	require.Equal(t, before, countAudit(t, s))
}

func TestReadOnlyMode(t *testing.T) {
	s := newStore(t)
	ctx := actorCtx("tester")
	r := s.Repos()
	_, err := r.Storages.Create(ctx, controlplane.StorageRecord{Name: "st", Address: "http://x", CreatedAt: t0, UpdatedAt: t0})
	require.NoError(t, err)
	s.SetReadOnly(true)
	_, err = r.Storages.Create(ctx, controlplane.StorageRecord{Name: "st2"})
	require.ErrorIs(t, err, controlplane.ErrReadOnly)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.ErrorIs(t, r.Routing.Set(ctx, "a", "", "st"), controlplane.ErrReadOnly)
	require.ErrorIs(t, s.Tx(ctx, func(controlplane.Repos) error { return nil }), controlplane.ErrReadOnly)
	got, err := r.Storages.Get(ctx, "st") // reads keep working
	require.NoError(t, err)
	require.Equal(t, "st", got.Value.Name)
	s.SetReadOnly(false)
	_, err = r.Storages.Create(ctx, controlplane.StorageRecord{Name: "st2", Address: "http://x"})
	require.NoError(t, err)
}

func TestSkipOutbox(t *testing.T) {
	s := newStore(t)
	ctx := controlplane.WithSkipOutbox(actorCtx("migrate"))
	_, err := s.Repos().Storages.Create(ctx, controlplane.StorageRecord{Name: "st", Address: "http://x"})
	require.NoError(t, err)
	var n int
	require.NoError(t, s.Pool().QueryRow(ctx, `SELECT count(*) FROM outbox`).Scan(&n))
	require.Zero(t, n)
	require.Equal(t, 1, countAudit(t, s))
}

// ---- outbox publisher

func TestPublisherProjectsToRedis(t *testing.T) {
	s := newStore(t)
	env := newRedis(t)
	ctx := actorCtx("tester")
	pub := newPublisher(t, s, env.rdb)
	r := s.Repos()

	// subscribe to the snapshot notification channel
	sub := env.rdb.Subscribe(ctx, pg.SnapshotChannel)
	defer sub.Close()
	_, err := sub.Receive(ctx)
	require.NoError(t, err)

	// nothing is visible before the publisher runs: the change is committed in PostgreSQL only
	_, err = r.Storages.Create(ctx, controlplane.StorageRecord{Name: "st1", Address: "http://x", Provider: "Minio", CreatedAt: t0, UpdatedAt: t0})
	require.NoError(t, err)
	require.False(t, env.mr.Exists(pg.KeyStorages))
	n, oldest, err := pub.Pending(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	require.GreaterOrEqual(t, oldest, time.Duration(0))

	res, err := pub.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, res.Published)
	doc, err := env.rdb.HGet(ctx, pg.KeyStorages, "st1").Result()
	require.NoError(t, err)
	require.JSONEq(t, `{"name":"st1","address":"http://x","provider":"Minio","isSecure":false,"createdAt":"2026-11-10T12:00:00Z","updatedAt":"2026-11-10T12:00:00Z"}`, doc)
	v, _ := env.rdb.Get(ctx, pg.KeyStoragesVersion).Int()
	require.Equal(t, 1, v)

	// routing: data + snapshot version bump + pub/sub notification
	require.NoError(t, r.Routing.Set(ctx, "alice", "", "st1"))
	require.NoError(t, r.Routing.Set(ctx, "alice", "b", "st1"))
	require.NoError(t, r.Routing.Block(ctx, "alice", "b"))
	_, err = pub.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, "st1", env.mr.HGet(pg.KeyRouteUser, "alice"))
	require.Equal(t, "st1", env.mr.HGet("p:route_bucket:alice", "b"))
	members, _ := env.mr.Members("p:route_block_bucket:alice")
	require.Equal(t, []string{"b"}, members)
	ver, _ := env.rdb.Get(ctx, pg.SnapshotVersionKey).Int()
	require.Equal(t, 3, ver)
	msg, err := sub.ReceiveTimeout(ctx, 2*time.Second)
	require.NoError(t, err)
	require.Contains(t, fmt.Sprint(msg), pg.SnapshotChannel)

	// delete
	require.NoError(t, r.Routing.Unblock(ctx, "alice", "b"))
	require.NoError(t, r.Storages.Delete(ctx, "st1"))
	_, err = pub.RunOnce(ctx)
	require.NoError(t, err)
	require.False(t, env.mr.Exists("p:route_block_bucket:alice"))
	require.False(t, env.mr.Exists(pg.KeyStorages))
	v, _ = env.rdb.Get(ctx, pg.KeyStoragesVersion).Int()
	require.Equal(t, 2, v)

	// nothing left to do
	res, err = pub.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, res.Published+res.Failed)

	// a skip-outbox change is not published
	_, err = r.S3Identities.Create(controlplane.WithSkipOutbox(ctx), controlplane.S3Identity{Name: "quiet", CreatedAt: t0})
	require.NoError(t, err)
	_, _ = pub.RunOnce(ctx)
	require.False(t, env.mr.Exists(pg.KeyS3Users))
}

func TestPublisherRetriesWhenRedisFails(t *testing.T) {
	s := newStore(t)
	env := newRedis(t)
	ctx := actorCtx("tester")
	pub := newPublisher(t, s, env.rdb)
	r := s.Repos()

	env.mr.SetError("LOADING redis is starting")
	_, err := r.AccessPolicies.Create(ctx, controlplane.AccessPolicy{User: "alice", Permission: "read-only", Buckets: []string{"*"}})
	require.NoError(t, err, "the control-plane write does not depend on Redis")
	res, err := pub.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, res.Failed)
	require.Zero(t, res.Published)

	var attempts int
	var lastErr string
	require.NoError(t, s.Pool().QueryRow(ctx, `SELECT attempts, last_error FROM outbox`).Scan(&attempts, &lastErr))
	require.Equal(t, 1, attempts)
	require.Contains(t, lastErr, "LOADING")

	// still failing: the entry is retried after the backoff, not in a hot loop
	res, _ = pub.RunOnce(ctx)
	require.Equal(t, 0, res.Failed)
	require.Equal(t, 0, res.Published)
	n, _, err := pub.Pending(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, n, "the entry waits for its retry time (filtered in SQL, not counted as skipped)")

	env.mr.SetError("")
	require.Eventually(t, func() bool {
		res, err := pub.RunOnce(ctx)
		return err == nil && res.Published == 1
	}, 5*time.Second, 25*time.Millisecond)
	require.True(t, env.mr.Exists(pg.KeyAccessPolicies))
	var published bool
	require.NoError(t, s.Pool().QueryRow(ctx, `SELECT published_at IS NOT NULL AND last_error='' FROM outbox`).Scan(&published))
	require.True(t, published)

	// Redis down entirely (connection refused) behaves the same
	env.mr.Close()
	_, err = r.AccessPolicies.Put(ctx, controlplane.AccessPolicy{User: "alice", Permission: "read-write", Buckets: []string{"*"}})
	require.NoError(t, err)
	res, err = pub.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, res.Failed)
}

func TestPublisherKeepsOrderPerObject(t *testing.T) {
	s := newStore(t)
	env := newRedis(t)
	ctx := actorCtx("tester")
	pub := newPublisher(t, s, env.rdb)
	r := s.Repos()

	env.mr.SetError("down")
	for i := 1; i <= 3; i++ {
		_, err := r.AccessPolicies.Put(ctx, controlplane.AccessPolicy{User: "alice", Permission: "read-only", Buckets: []string{"*"}, Description: fmt.Sprint("v", i)})
		require.NoError(t, err)
	}
	_, err := r.AccessPolicies.Put(ctx, controlplane.AccessPolicy{User: "bob", Permission: "read-only", Buckets: []string{"*"}, Description: "bob"})
	require.NoError(t, err)

	res, err := pub.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, res.Failed, "the heads of alice and bob fail (Redis is down)")
	require.Equal(t, 2, res.Skipped, "alice's later entries are held back behind her failed head, never applied out of order")
	env.mr.SetError("")

	// let the retry delay pass
	require.Eventually(t, func() bool {
		res, err := pub.RunOnce(ctx)
		return err == nil && res.Published > 0
	}, 5*time.Second, 25*time.Millisecond)
	for i := 0; i < 5; i++ {
		_, _ = pub.RunOnce(ctx)
		time.Sleep(30 * time.Millisecond)
	}
	var doc struct{ Description string }
	require.NoError(t, json.Unmarshal([]byte(env.mr.HGet(pg.KeyAccessPolicies, "alice")), &doc))
	require.Equal(t, "v3", doc.Description, "the last change wins, never an older one")
	require.Contains(t, env.mr.HGet(pg.KeyAccessPolicies, "bob"), "bob")
	n, _, _ := pub.Pending(ctx)
	require.Zero(t, n)
}

func TestPublisherIsIdempotent(t *testing.T) {
	s := newStore(t)
	env := newRedis(t)
	ctx := actorCtx("tester")
	pub := newPublisher(t, s, env.rdb)
	r := s.Repos()
	id := controlplane.ReplicationID{User: "alice", FromStorage: "s1", ToStorage: "s2"}
	require.NoError(t, r.Replications.Add(ctx, id, controlplane.ReplicationStatus{CreatedAt: t0}))
	_, err := r.Replications.IncrementGeneration(ctx, id)
	require.NoError(t, err)
	first1, err := pub.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, first1.Published, "next_attempt_at is compared with the database clock: new entries are always due")

	dump := func() string {
		keys := env.mr.Keys()
		out := ""
		for _, k := range keys {
			switch env.mr.Type(k) {
			case "hash":
				m, _ := env.mr.HKeys(k)
				for _, f := range m {
					out += k + "|" + f + "=" + env.mr.HGet(k, f) + "\n"
				}
			case "set":
				m, _ := env.mr.Members(k)
				out += k + "=" + fmt.Sprint(m) + "\n"
			}
		}
		return out
	}
	first := dump()
	require.Contains(t, first, "p:repl_user_gen:alice|s2=1")
	require.Contains(t, first, "p:repl_user_from:|alice=s1")
	// replay every entry (simulates a crash after Redis was written but before published_at was stored)
	_, err = s.Pool().Exec(ctx, `UPDATE outbox SET published_at=NULL`)
	require.NoError(t, err)
	res, err := pub.RunOnce(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, res.Published)
	require.Equal(t, first, dump(), "replaying leaves the same data (only the version counters move)")

	removed, err := pub.Purge(ctx)
	require.NoError(t, err)
	require.Zero(t, removed, "recent entries are kept for the retention period")
}

func TestOnlyOnePublisherRuns(t *testing.T) {
	s := newStore(t)
	env := newRedis(t)
	ctx := actorCtx("tester")
	pub := newPublisher(t, s, env.rdb)
	// hold the publisher lock from another session
	conn, err := s.Pool().Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	var got bool
	require.NoError(t, tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, int64(0x6f757462)).Scan(&got))
	require.True(t, got)
	res, err := pub.RunOnce(ctx)
	require.NoError(t, err)
	require.True(t, res.Busy)
	require.NoError(t, tx.Rollback(ctx))
	res, err = pub.RunOnce(ctx)
	require.NoError(t, err)
	require.False(t, res.Busy)
}

func TestReplicationScopeSingleOutboxKeyAndConcurrentAdds(t *testing.T) {
	s := newStore(t)
	env := newRedis(t)
	ctx := actorCtx("tester")
	pub := newPublisher(t, s, env.rdb)
	r := s.Repos()

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := controlplane.ReplicationID{User: "alice", FromBucket: "b", ToBucket: "b", FromStorage: "s0", ToStorage: fmt.Sprint("dst", i)}
			if err := r.Replications.Add(ctx, id, controlplane.ReplicationStatus{CreatedAt: t0}); err != nil {
				errs[i] = err
				return
			}
			_, errs[i] = r.Replications.IncrementGeneration(ctx, id)
		}()
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	var keys int
	require.NoError(t, s.Pool().QueryRow(ctx, `SELECT count(DISTINCT object_key) FROM outbox`).Scan(&keys))
	require.Equal(t, 1, keys, "every replication entry of a scope shares one outbox object key")

	// remove a destination concurrently with the generation bumps of the others
	require.NoError(t, r.Replications.Remove(ctx, controlplane.ReplicationID{User: "alice", FromBucket: "b", ToBucket: "b", FromStorage: "s0", ToStorage: "dst0"}))
	for i := 0; i < 3; i++ {
		_, err := pub.RunOnce(ctx)
		require.NoError(t, err)
	}
	members, err := env.mr.Members("p:repl_bucket_to:alice:b")
	require.NoError(t, err)
	require.Len(t, members, n-1, "no destination is lost or resurrected by an out-of-order absolute projection")
	require.NotContains(t, members, "dst0:b")
	require.Len(t, env.mr.HGet("p:repl_bucket_gen:alice:b", "dst3:b"), 1)
}

func TestSnapshotCannotLandAfterNewerChange(t *testing.T) {
	s := newStore(t)
	env := newRedis(t)
	ctx := actorCtx("tester")
	pub := newPublisher(t, s, env.rdb)
	r := s.Repos()
	require.NoError(t, r.Routing.Set(ctx, "alice", "", "s0"))

	// a writer keeps its transaction open (outbox entry written, not committed) while a snapshot is requested
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- s.Tx(ctx, func(rp controlplane.Repos) error {
			if err := rp.Routing.Set(ctx, "alice", "", "s1"); err != nil {
				return err
			}
			close(started)
			<-release
			return nil
		})
	}()
	<-started
	snapDone := make(chan error, 1)
	go func() { _, err := s.EnqueueSnapshot(ctx); snapDone <- err }()
	select {
	case err := <-snapDone:
		t.Fatalf("snapshot must wait for the in-flight writer, finished with %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	require.NoError(t, <-done)
	require.NoError(t, <-snapDone)
	for i := 0; i < 3; i++ {
		_, err := pub.RunOnce(ctx)
		require.NoError(t, err)
	}
	require.Equal(t, "s1", env.mr.HGet(pg.KeyRouteUser, "alice"), "the snapshot saw the committed change, the older state did not land last")
}

func TestStorageTestsAreNotAuditedAndActionFilterRunsInSQL(t *testing.T) {
	s := newStore(t)
	env := newRedis(t)
	ctx := actorCtx("tester")
	pub := newPublisher(t, s, env.rdb)
	r := s.Repos()

	for i := 0; i < 5; i++ {
		_, err := r.StorageTests.Put(ctx, controlplane.RawDoc{ID: "st1", JSON: []byte(fmt.Sprintf(`{"ok":true,"n":%d}`, i))})
		require.NoError(t, err)
	}
	rows, err := r.Audit.List(ctx, controlplane.AuditFilter{ObjectType: "storage_tests"})
	require.NoError(t, err)
	require.Empty(t, rows, "connection test results do not spam the audit log")
	_, err = pub.RunOnce(ctx)
	require.NoError(t, err)
	require.True(t, env.mr.Exists(pg.KeyStorageTests), "the Redis projection is still written")

	_, err = r.AccessPolicies.Create(ctx, controlplane.AccessPolicy{User: "alice", Permission: "read-only", Buckets: []string{"*"}})
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		_, err = r.AccessPolicies.Put(ctx, controlplane.AccessPolicy{User: "alice", Permission: "read-write", Buckets: []string{"*"}, Description: fmt.Sprint(i)})
		require.NoError(t, err)
	}
	rows, err = r.Audit.List(ctx, controlplane.AuditFilter{Action: "CREATE", Limit: 1})
	require.NoError(t, err)
	require.Len(t, rows, 1, "the oldest, rare action is found although 5 newer rows of another action exist")
	require.Equal(t, "create", rows[0].Action)
}

func TestFingerprintChangesWithEveryRowChange(t *testing.T) {
	s := newStore(t)
	ctx := actorCtx("tester")
	r := s.Repos()
	fp := func() string {
		v, err := r.S3Identities.Fingerprint(ctx)
		require.NoError(t, err)
		return v
	}
	empty := fp()
	require.Equal(t, empty, fp(), "stable while nothing changes")
	_, err := r.S3Identities.Create(ctx, controlplane.S3Identity{Name: "a", CreatedAt: t0})
	require.NoError(t, err)
	created := fp()
	require.NotEqual(t, empty, created)
	_, err = r.S3Identities.Put(ctx, controlplane.S3Identity{Name: "a", CreatedAt: t0.Add(time.Hour)})
	require.NoError(t, err)
	updated := fp()
	require.NotEqual(t, created, updated, "an update bumps the version")
	require.NoError(t, r.S3Identities.Delete(ctx, "a"))
	require.Equal(t, empty, fp())
	d, err := r.DynamicCredentials.Fingerprint(ctx)
	require.NoError(t, err)
	require.Equal(t, "empty", d)
}

func TestPublisher_MaxConnsOneWithStats(t *testing.T) {
	s := newStoreWithMaxConns(t, 1)
	env := newRedis(t)
	ctx := actorCtx("tester")
	pub := newPublisher(t, s, env.rdb)

	statsCalled := false
	pub.OnStats = func(n int64, oldest time.Duration) {
		statsCalled = true
	}

	r := s.Repos()
	id := controlplane.ReplicationID{User: "alice", FromStorage: "s1", ToStorage: "s2"}
	require.NoError(t, r.Replications.Add(ctx, id, controlplane.ReplicationStatus{CreatedAt: t0}))

	runCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res, err := pub.RunOnce(runCtx)
	require.NoError(t, err)
	require.Equal(t, 1, res.Published)
	require.True(t, statsCalled)
}
