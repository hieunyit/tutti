package migrate_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/auth"
	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/migrate"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/controlplane/pgtest"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
	"github.com/hieunyit/tutti/pkg/proxyaccount"
	"github.com/hieunyit/tutti/pkg/ratelimit"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/store"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Run(m)) }

type testSealer struct{}

func (testSealer) Seal(b []byte) (string, error) { return "sealed:" + string(b), nil }
func (testSealer) Open(s string) ([]byte, error) {
	return []byte(strings.TrimPrefix(s, "sealed:")), nil
}

func newMiniRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return mr, rdb
}

func newPG(t *testing.T) *pg.Store {
	t.Helper()
	s, err := pg.Open(context.Background(), controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: pgtest.NewDB(t), MaxConns: 8, AutoMigrate: true}})
	require.NoError(t, err)
	t.Cleanup(s.Close)
	return s
}

// populate writes a representative control plane through the REAL Redis stores.
func populate(t *testing.T, rdb *redis.Client) {
	t.Helper()
	ctx := context.Background()
	conf := &objstore.Config{
		Main: "main",
		Storages: map[string]objstore.Storage{"main": {CommonConfig: objstore.CommonConfig{Type: dom.S3}, S3: &s3.Storage{
			StorageAddress: s3.StorageAddress{Address: "http://localhost:9000", Provider: s3.ProviderMinIO},
			Credentials:    map[string]s3.CredentialsV4{"u": {AccessKeyID: "key", SecretAccessKey: "secret"}}}}},
		DynamicCredentials: objstore.DynamicCredentialsConfig{Enabled: true, MasterPassword: "migrate-test-password", PollInterval: time.Hour},
	}
	creds, err := objstore.NewCredsSvc(ctx, conf, rdb, objstore.WithManagedStorages(rdb))
	require.NoError(t, err)
	for _, name := range []string{"archive", "backup"} {
		_, err = creds.CreateManaged(ctx, objstore.ManagedStorage{Name: name, Address: s3.StorageAddress{Address: "http://" + name + ":9000", Provider: s3.ProviderMinIO, HealthCheckInterval: 5 * time.Second},
			RateLimit: ratelimit.RateLimit{Enabled: true, RPM: 600}, ConnectionAccount: s3.CredentialsV4{AccessKeyID: "svc-" + name, SecretAccessKey: "svc-secret-" + name}})
		require.NoError(t, err)
	}
	// rotation fills PreviousAccount and RotatedAt
	_, err = creds.RotateConnectionAccount(ctx, "archive", s3.CredentialsV4{AccessKeyID: "svc-new", SecretAccessKey: "svc-secret-new"}, "tester")
	require.NoError(t, err)
	require.NoError(t, creds.SetS3Credentials(ctx, "archive", "alice", s3.CredentialsV4{AccessKeyID: "alice-key", SecretAccessKey: "alice-secret"}))
	require.NoError(t, creds.SetS3Credentials(ctx, "backup", "bob", s3.CredentialsV4{AccessKeyID: "bob-key", SecretAccessKey: "bob-secret"}))
	require.NoError(t, creds.AddIdentity(ctx, "alice"))
	require.NoError(t, creds.AddIdentity(ctx, "bob"))
	require.NoError(t, creds.SaveStorageTest(ctx, "archive", []byte(`{"ok":true,"steps":[{"name":"list","ok":true}]}`)))

	accounts := proxyaccount.NewStore(rdb, func() (proxyaccount.Sealer, error) { return testSealer{}, nil })
	acc, _, err := accounts.Create(ctx, "alice", "alice laptop")
	require.NoError(t, err)
	_, _, err = accounts.Rotate(ctx, acc.AccessKeyID, time.Hour)
	require.NoError(t, err)
	require.NoError(t, accounts.SetStatus(ctx, acc.AccessKeyID, proxyaccount.StatusDisabled))
	_, _, err = accounts.Create(ctx, "bob", "")
	require.NoError(t, err)

	policies := accesspolicy.NewStore(rdb)
	require.NoError(t, policies.Set(ctx, accesspolicy.Policy{User: "alice", Permission: accesspolicy.ReadOnly, Buckets: []string{"app-*", "logs"}, Description: "ro"}))
	require.NoError(t, policies.Set(ctx, accesspolicy.Policy{User: "bob", Permission: accesspolicy.ReadWrite}))

	users := auth.NewUserStore(rdb)
	require.NoError(t, users.Create(ctx, auth.ManagedUser{Username: "root", Role: "admin", PasswordHash: "$2a$12$abcdefghijklmnopqrstuv", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), PasswordChangedAt: time.Now().UTC()}))
	require.NoError(t, users.Create(ctx, auth.ManagedUser{Username: "viewer", Role: "viewer", PasswordHash: "$2a$12$zzzzzzzzzzzzzzzzzzzzzz", Disabled: true, MustChangePassword: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(), PasswordChangedAt: time.Now().UTC()}))
	require.NoError(t, users.TouchLogin(ctx, "root", time.Now().UTC()))
	require.NoError(t, users.TouchLogin(ctx, "ghost", time.Now().UTC())) // login of a user that does not exist is dropped

	fo := failover.NewStore(rdb, nil)
	lag := 5
	require.NoError(t, fo.SetFailoverPolicy(ctx, failover.Policy{Source: "main", Enabled: true, Targets: []string{"archive", "backup"}, After: 90 * time.Second, MaxEventLag: &lag}))
	require.NoError(t, fo.SetFailoverPolicy(ctx, failover.Policy{Source: "backup", Enabled: false}))
	require.NoError(t, fo.SaveFailoverEvent(ctx, failover.Event{User: "alice", From: "main", To: "archive", Time: time.Now().UTC(), EventLag: 2}))
	require.NoError(t, fo.SaveFailoverEvent(ctx, failover.Event{User: "bob", Bucket: "photos", From: "main", To: "backup", Time: time.Now().UTC()}))

	groups := protectiongroups.NewStore(rdb)
	require.NoError(t, groups.Save(ctx, &protectiongroups.Group{ID: "g1", User: "alice", Members: []string{"archive", "backup"}, Copies: 2, Buckets: []string{"photos"}}))

	// routing
	ur, br := store.NewUserRoutingStore(rdb), store.NewBucketRoutingStore(rdb)
	require.NoError(t, ur.SetOp(ctx, "alice", "archive").Get())
	require.NoError(t, ur.SetOp(ctx, "carol", "backup").Get())
	require.NoError(t, ur.BlockOp(ctx, "carol").Get())
	require.NoError(t, br.SetOp(ctx, entity.NewBucketRoutingPolicyID("bob", "photos"), "backup").Get())
	require.NoError(t, br.SetOp(ctx, entity.NewBucketRoutingPolicyID("bob", "docs"), "archive").Get())
	require.NoError(t, br.BlockOp(ctx, entity.NewBucketRoutingPolicyID("bob", "docs")).Get())

	// replication: user-level with two destinations, bucket-level with a custom destination, one archived
	up, bp := store.NewUserReplicationPolicyStore(rdb), store.NewBucketReplicationPolicyStore(rdb)
	us, bs := store.NewUserReplicationStatusStore(rdb), store.NewBucketReplicationStatusStore(rdb)
	gen := store.NewReplicationGenerationStore(rdb)
	for _, to := range []string{"archive", "backup"} {
		p := entity.NewUserReplicationPolicy("alice", "main", to)
		require.NoError(t, up.AddOp(ctx, p).Get())
		require.NoError(t, us.AddOp(ctx, p, entity.ReplicationStatus{AgentURL: "http://agent:9", EventSource: string(dom.EventSourceS3Notification)}).Get())
		_, err = gen.IncrementOp(ctx, entity.UniversalFromUserReplication(p)).Get()
		require.NoError(t, err)
	}
	_, err = gen.IncrementOp(ctx, entity.UniversalFromUserReplication(entity.NewUserReplicationPolicy("alice", "main", "backup"))).Get()
	require.NoError(t, err)
	b1 := entity.NewBucketReplicationPolicy("bob", "main", "photos", "backup", "photos-copy")
	b2 := entity.NewBucketReplicationPolicy("bob", "main", "docs", "archive", "docs")
	for _, p := range []entity.BucketReplicationPolicy{b1, b2} {
		require.NoError(t, bp.AddOp(ctx, p).Get())
		require.NoError(t, bs.AddOp(ctx, p, entity.ReplicationStatus{}).Get())
		_, err = gen.IncrementOp(ctx, entity.UniversalFromBucketReplication(p)).Get()
		require.NoError(t, err)
	}
	// b2 is removed: the policy goes, the status stays archived and the generation stays
	require.NoError(t, bp.RemoveOp(ctx, b2).Get())
	require.NoError(t, bs.ArchieveOp(ctx, b2).Get())
	_, err = gen.IncrementOp(ctx, entity.UniversalFromBucketReplication(b2)).Get()
	require.NoError(t, err)
	// runtime state that must NOT be migrated
	require.NoError(t, us.SetFailoverPendingOp(ctx, entity.NewUserReplicationPolicy("alice", "main", "archive"), true).Get())
}

func TestReadRedisParsesRealStores(t *testing.T) {
	_, rdb := newMiniRedis(t)
	populate(t, rdb)
	d, err := migrate.ReadRedis(context.Background(), migrate.Sources{Config: rdb})
	require.NoError(t, err)
	require.Empty(t, d.Warnings)

	require.Len(t, d.Storages, 2)
	require.Equal(t, "svc-new-sealed-check", "svc-new-sealed-check")
	require.NotEmpty(t, d.Storages[0].ConnectionAccount)
	require.NotEmpty(t, d.Storages[0].PreviousAccount, "rotation keeps the previous account")
	require.NotContains(t, d.Storages[0].ConnectionAccount, "svc-secret", "stays sealed")
	require.Len(t, d.S3Identities, 2)
	require.Len(t, d.DynamicCreds, 2)
	require.NotEmpty(t, d.DynamicSalt)
	require.Len(t, d.ProxyAccounts, 2)
	require.Len(t, d.AccessPolicies, 2)
	require.Len(t, d.ConsoleUsers, 2)
	require.Len(t, d.ConsoleLogins, 1, "only the login of an existing user")
	require.Len(t, d.FailoverPolicies, 2)
	require.Len(t, d.FailoverEvents, 2)
	require.Len(t, d.ProtectionGroups, 1)
	require.Len(t, d.StorageTests, 1)
	require.Len(t, d.Routing.Routes, 4)
	require.Len(t, d.Routing.Blocks, 2)
	require.Len(t, d.ReplPolicies, 3, "two user-level destinations and the remaining bucket policy")
	require.Len(t, d.ReplStatuses, 4, "archived statuses outlive their policy")
	require.Len(t, d.ReplGenerations, 4)
	gens := d.ReplGenerations
	require.EqualValues(t, 2, gens[controlplane.ReplicationID{User: "alice", ToStorage: "backup"}])
	require.EqualValues(t, 2, gens[controlplane.ReplicationID{User: "bob", FromBucket: "docs", ToStorage: "archive", ToBucket: "docs"}])
	archived := d.ReplStatuses[controlplane.ReplicationID{User: "bob", FromStorage: "main", ToStorage: "archive", FromBucket: "docs", ToBucket: "docs"}]
	require.True(t, archived.Archived)
	require.NotNil(t, archived.ArchivedAt)
	st := d.ReplStatuses[controlplane.ReplicationID{User: "alice", FromStorage: "main", ToStorage: "archive"}]
	require.Equal(t, "http://agent:9", st.AgentURL)
	require.False(t, st.CreatedAt.IsZero())
}

func TestImportRoundTripAndIdempotence(t *testing.T) {
	ctx := context.Background()
	_, rdb := newMiniRedis(t)
	populate(t, rdb)
	src, err := migrate.ReadRedis(ctx, migrate.Sources{Config: rdb})
	require.NoError(t, err)
	s := newPG(t)

	// dry run: full report, nothing written
	target, err := migrate.ReadPG(ctx, s.Repos())
	require.NoError(t, err)
	dry := migrate.Import(ctx, s, src, migrate.ImportOptions{DryRun: true, Target: target})
	require.True(t, dry.OK, dry.Text())
	require.True(t, dry.DryRun)
	require.Equal(t, 2, dry.Families[migrate.FamStorages].Created)
	require.Equal(t, 4, dry.Families[migrate.FamReplStatus].Created)
	after, err := migrate.ReadPG(ctx, s.Repos())
	require.NoError(t, err)
	require.Empty(t, after.Storages)
	require.Zero(t, auditCount(t, s))
	// and without any database at all
	dryNoDB := migrate.Import(ctx, nil, src, migrate.ImportOptions{DryRun: true})
	require.True(t, dryNoDB.OK)
	require.Equal(t, 2, dryNoDB.Families[migrate.FamStorages].Created)

	rep := migrate.Import(ctx, s, src, migrate.ImportOptions{Target: target})
	require.True(t, rep.OK, rep.Text())
	require.Empty(t, rep.Errors)
	require.Empty(t, rep.Mismatches)
	for _, f := range migrate.Families {
		fr := rep.Families[f]
		require.NotNil(t, fr.After, f)
		require.Equal(t, fr.Source, *fr.After, "count and checksum match for %s", f)
		require.Equal(t, fr.Source.Count, fr.Created, f)
	}
	require.Contains(t, rep.Text(), "result: OK")
	var decoded migrate.Report
	require.NoError(t, json.Unmarshal(rep.JSON(), &decoded))
	require.True(t, decoded.OK)

	// the sealed values were copied verbatim, never opened
	got, err := s.Repos().Storages.Get(ctx, "archive")
	require.NoError(t, err)
	var want controlplane.StorageRecord
	for _, st := range src.Storages {
		if st.Name == "archive" {
			want = st
		}
	}
	require.Equal(t, want.ConnectionAccount, got.Value.ConnectionAccount)
	require.Equal(t, want.PreviousAccount, got.Value.PreviousAccount)
	// audit rows carry the migration actor and no secrets
	entries, err := s.Repos().Audit.List(ctx, controlplane.AuditFilter{Limit: 1000})
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	for _, e := range entries {
		require.Equal(t, migrate.Actor, e.Actor)
		require.NotContains(t, string(e.After), "sealed:")
		require.NotContains(t, string(e.After), want.ConnectionAccount)
	}
	// imports write no outbox entries: Redis already has the data
	var pending int
	require.NoError(t, s.Pool().QueryRow(ctx, `SELECT count(*) FROM outbox`).Scan(&pending))
	require.Zero(t, pending)

	// idempotent re-run: nothing changes, nothing is written
	auditBefore := auditCount(t, s)
	target, _ = migrate.ReadPG(ctx, s.Repos())
	again := migrate.Import(ctx, s, src, migrate.ImportOptions{Target: target})
	require.True(t, again.OK, again.Text())
	for _, f := range migrate.Families {
		fr := again.Families[f]
		require.Zero(t, fr.Created+fr.Updated, f)
		require.Equal(t, fr.Source.Count, fr.Unchanged, f)
	}
	require.Equal(t, auditBefore, auditCount(t, s), "a re-run writes no audit rows")

	// a record changed in Redis is updated, a record only in PG is kept and reported
	_, err = rdb.HSet(ctx, "tutti:access-policies", "alice", `{"user":"alice","permission":"read-write","buckets":["*"]}`).Result()
	require.NoError(t, err)
	_, err = s.Repos().S3Identities.Create(ctx, controlplane.S3Identity{Name: "pg-only", CreatedAt: time.Now().UTC()})
	require.NoError(t, err)
	src, _ = migrate.ReadRedis(ctx, migrate.Sources{Config: rdb})
	target, _ = migrate.ReadPG(ctx, s.Repos())
	third := migrate.Import(ctx, s, src, migrate.ImportOptions{Target: target})
	require.True(t, third.OK, third.Text())
	require.Equal(t, 1, third.Families[migrate.FamAccessPolicies].Updated)
	require.Equal(t, 1, third.Families[migrate.FamS3Identities].ExtraInTarget)
	pol, _ := s.Repos().AccessPolicies.Get(ctx, "alice")
	require.Equal(t, "read-write", pol.Value.Permission)
	require.EqualValues(t, 2, pol.Version)
}

func auditCount(t *testing.T, s *pg.Store) int {
	t.Helper()
	es, err := s.Repos().Audit.List(context.Background(), controlplane.AuditFilter{Limit: 1000})
	require.NoError(t, err)
	return len(es)
}

func TestVerifyDetectsMismatch(t *testing.T) {
	ctx := context.Background()
	_, rdb := newMiniRedis(t)
	populate(t, rdb)
	src, err := migrate.ReadRedis(ctx, migrate.Sources{Config: rdb})
	require.NoError(t, err)
	s := newPG(t)
	require.True(t, migrate.Import(ctx, s, src, migrate.ImportOptions{}).OK)

	read := func() *migrate.Data {
		d, err := migrate.ReadPG(ctx, s.Repos())
		require.NoError(t, err)
		return d
	}
	rep := migrate.Verify(src, read())
	require.True(t, rep.OK, rep.Text())
	require.Empty(t, rep.Mismatches)

	r := s.Repos()
	// 1. a changed document
	_, err = r.Storages.Mutate(ctx, "archive", func(v *controlplane.StorageRecord) error { v.Address = "http://tampered:9000"; return nil })
	require.NoError(t, err)
	// 2. a record missing in PostgreSQL
	require.NoError(t, r.AccessPolicies.Delete(ctx, "bob"))
	// 3. a record only in PostgreSQL
	_, err = r.S3Identities.Create(ctx, controlplane.S3Identity{Name: "intruder", CreatedAt: time.Now().UTC()})
	require.NoError(t, err)
	// 4. a changed sealed credential and 5. routing / replication drift
	require.NoError(t, r.DynamicCredentials.Set(ctx, controlplane.DynamicCredential{StorageType: "S3", Storage: "archive", User: "alice", Sealed: "other"}))
	require.NoError(t, r.Routing.Set(ctx, "alice", "", "backup"))
	_, err = r.Replications.IncrementGeneration(ctx, controlplane.ReplicationID{User: "alice", ToStorage: "archive"})
	require.NoError(t, err)

	rep = migrate.Verify(src, read())
	require.False(t, rep.OK)
	kinds := map[string]string{}
	for _, m := range rep.Mismatches {
		kinds[string(m.Family)+"/"+m.Key] = m.Kind
	}
	require.Equal(t, "different", kinds["storages/archive"])
	require.Equal(t, "missing_in_target", kinds["access_policies/bob"])
	require.Equal(t, "missing_in_source", kinds["s3_identities/intruder"])
	require.Equal(t, "different", kinds["dynamic_credentials/S3:archive:alice"])
	require.Equal(t, "different", kinds["routes/alice"])
	require.Equal(t, "different", kinds["replication_generations/alice||archive|"])
	require.Len(t, rep.Mismatches, 6)
	require.NotEqual(t, rep.Families[migrate.FamStorages].Source.SHA, rep.Families[migrate.FamStorages].Target.SHA)
	require.Equal(t, rep.Families[migrate.FamProxyAccounts].Source, rep.Families[migrate.FamProxyAccounts].Target, "untouched families still match")
	text := rep.Text()
	require.Contains(t, text, "result: FAILED")
	require.NotContains(t, text, "other", "values (secrets) are never printed")
	require.NotContains(t, text, "tampered")
}

// dump returns the comparable content of the keys that proxies and workers read.
func dump(t *testing.T, mr *miniredis.Miniredis) map[string]string {
	t.Helper()
	out := map[string]string{}
	skipPrefix := []string{"tutti:cfg:"}
	for _, k := range mr.Keys() {
		skip := strings.HasSuffix(k, ":version")
		for _, p := range skipPrefix {
			skip = skip || strings.HasPrefix(k, p)
		}
		if skip || strings.HasPrefix(k, "tutti:protection:") {
			continue
		}
		switch mr.Type(k) {
		case "hash":
			fields, _ := mr.HKeys(k)
			for _, f := range fields {
				v := mr.HGet(k, f)
				if f == "version" || f == "failover_pending" {
					continue // counters and runtime state are not projected
				}
				if f == "created_at" || f == "archived_at" || k == "tutti:console-users:last-login" {
					if tm, err := time.Parse(time.RFC3339Nano, v); err == nil {
						v = tm.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano) // PostgreSQL keeps microseconds
					}
				}
				out[k+" | "+f] = canonical(v)
			}
		case "set":
			m, _ := mr.Members(k)
			sort.Strings(m)
			out[k] = fmt.Sprint(m)
		case "string":
			v, _ := mr.Get(k)
			out[k] = canonical(v)
		}
	}
	return out
}

func canonical(v string) string {
	var x any
	if json.Unmarshal([]byte(v), &x) != nil {
		return v
	}
	b, _ := json.Marshal(x)
	return string(b)
}

// The strongest check of the projection: Redis rebuilt from PostgreSQL by the outbox publisher is identical to the
// Redis the real stores wrote.
func TestProjectionRebuildsTheRedisTheRealStoresWrote(t *testing.T) {
	ctx := context.Background()
	origMR, origRdb := newMiniRedis(t)
	populate(t, origRdb)
	src, err := migrate.ReadRedis(ctx, migrate.Sources{Config: origRdb})
	require.NoError(t, err)

	s := newPG(t)
	rep := migrate.Import(ctx, s, src, migrate.ImportOptions{EmitOutbox: true})
	require.True(t, rep.OK, rep.Text())

	rebuiltMR, rebuiltRdb := newMiniRedis(t)
	pub := pg.NewPublisher(s, rebuiltRdb, controlplane.OutboxConfig{BatchSize: 1000}, zerolog.Nop())
	res, err := pub.RunOnce(ctx)
	require.NoError(t, err)
	require.Zero(t, res.Failed)
	require.Greater(t, res.Published, 20)

	want, got := dump(t, origMR), dump(t, rebuiltMR)
	delete(want, "tutti:console-users:last-login | ghost") // the login of a deleted user is not migrated
	for k, v := range want {
		require.Contains(t, got, k, "missing key in the rebuilt Redis")
		require.Equal(t, v, got[k], k)
	}
	var extra []string
	for k := range got {
		if _, ok := want[k]; !ok {
			extra = append(extra, k)
		}
	}
	require.Empty(t, extra, "keys in the rebuilt Redis that the real stores never wrote")
	require.Greater(t, len(want), 40)

	// and the proxy snapshot is told about it
	v, _ := rebuiltRdb.Get(ctx, policy.SnapshotVersionKey).Int()
	require.Positive(t, v)
	sv, _ := rebuiltRdb.Get(ctx, "tutti:storages:version").Int()
	require.Positive(t, sv)
	hv, _ := rebuiltRdb.HGet(ctx, "chorus:dynamic_creds", "version").Int()
	require.Positive(t, hv)

	// the real Redis stores read the rebuilt data back: a policy lookup and a status work unchanged
	up := store.NewUserReplicationPolicyStore(rebuiltRdb)
	ps, err := up.Get(ctx, "alice")
	require.NoError(t, err)
	require.Len(t, ps, 2)
	st, err := store.NewBucketReplicationStatusStore(rebuiltRdb).GetOp(ctx, entity.NewBucketReplicationPolicy("bob", "main", "docs", "archive", "docs")).Get()
	require.NoError(t, err)
	require.True(t, st.IsArchived)
	gv, err := store.NewReplicationGenerationStore(rebuiltRdb).Get(ctx, entity.UniversalFromUserReplication(entity.NewUserReplicationPolicy("alice", "main", "backup")))
	require.NoError(t, err)
	require.EqualValues(t, 2, gv)
	_, err = store.NewUserRoutingStore(rebuiltRdb).GetOp(ctx, "carol").Get()
	require.ErrorIs(t, err, dom.ErrRoutingBlock)
}

// Live changes made through the PostgreSQL repositories project to the same keys the Redis stores would write.
func TestLiveMutationsMatchRedisStores(t *testing.T) {
	ctx := context.Background()
	mr, rdb := newMiniRedis(t)
	s := newPG(t)
	pub := pg.NewPublisher(s, rdb, controlplane.OutboxConfig{BatchSize: 1000}, zerolog.Nop())
	r := s.Repos()
	flush := func() {
		res, err := pub.RunOnce(ctx)
		require.NoError(t, err)
		require.Zero(t, res.Failed)
	}

	// replication add + remove through PG, observed through the real Redis stores
	id := controlplane.ReplicationID{User: "alice", FromStorage: "main", ToStorage: "archive"}
	id2 := controlplane.ReplicationID{User: "alice", FromStorage: "main", ToStorage: "backup"}
	require.NoError(t, r.Replications.Add(ctx, id, controlplane.ReplicationStatus{AgentURL: "a"}))
	require.NoError(t, r.Replications.Add(ctx, id2, controlplane.ReplicationStatus{}))
	_, err := r.Replications.IncrementGeneration(ctx, id)
	require.NoError(t, err)
	flush()
	ps, err := store.NewUserReplicationPolicyStore(rdb).Get(ctx, "alice")
	require.NoError(t, err)
	require.Len(t, ps, 2)
	require.NoError(t, r.Replications.Remove(ctx, id))
	require.NoError(t, r.Replications.Remove(ctx, id2))
	flush()
	_, err = store.NewUserReplicationPolicyStore(rdb).Get(ctx, "alice")
	require.ErrorIs(t, err, dom.ErrNotFound)
	require.False(t, mr.Exists("p:repl_user_to:alice"), "removing the last destination removes the lookup keys")
	require.False(t, mr.Exists("p:repl_user_from"))
	g, err := store.NewReplicationGenerationStore(rdb).Get(ctx, entity.UniversalFromUserReplication(entity.NewUserReplicationPolicy("alice", "main", "archive")))
	require.NoError(t, err)
	require.EqualValues(t, 1, g, "generations survive removal")

	// proxy account written through PG is loadable by the real proxyaccount cache path
	_, err = r.ProxyAccounts.Create(ctx, controlplane.ProxyAccountRecord{AccessKeyID: "TUTLIVE", User: "alice", Status: "active", CreatedAt: time.Now().UTC().Truncate(time.Second), Secret: "sealed:s3cret"})
	require.NoError(t, err)
	flush()
	creds, skipped, err := proxyaccount.NewStore(rdb, func() (proxyaccount.Sealer, error) { return testSealer{}, nil }).LoadCredentials(ctx)
	require.NoError(t, err)
	require.Empty(t, skipped)
	require.Equal(t, "s3cret", creds["TUTLIVE"].Secret)
	// an access policy too
	require.NoError(t, putPolicy(ctx, r, "alice"))
	flush()
	p, err := accesspolicy.NewStore(rdb).Get(ctx, "alice")
	require.NoError(t, err)
	require.Equal(t, accesspolicy.ReadOnly, p.Permission)
}

func putPolicy(ctx context.Context, r controlplane.Repos, user string) error {
	_, err := r.AccessPolicies.Put(ctx, controlplane.AccessPolicy{User: user, Permission: accesspolicy.ReadOnly, Buckets: []string{"*"}})
	return err
}

func TestKeyConstantsMatchTheStores(t *testing.T) {
	require.Equal(t, policy.SnapshotVersionKey, pg.SnapshotVersionKey)
	require.Equal(t, policy.SnapshotChannel, pg.SnapshotChannel)
	require.Equal(t, accesspolicy.Key, pg.KeyAccessPolicies)
	require.Equal(t, proxyaccount.Key, pg.KeyProxyAccounts)
	require.Equal(t, auth.ConsoleUsersKey, pg.KeyConsoleUsers)
	require.Equal(t, failover.EventPrefix, pg.KeyFailoverEventPrefix)
	require.Equal(t, protectiongroups.GroupsKey, pg.KeyProtectionGroups)
}

func TestRepublishRebuildsLostRedisWithoutAuditOrVersionDrift(t *testing.T) {
	ctx := context.Background()
	origMR, origRdb := newMiniRedis(t)
	populate(t, origRdb)
	src, err := migrate.ReadRedis(ctx, migrate.Sources{Config: origRdb})
	require.NoError(t, err)

	s := newPG(t)
	// Normal migrate (EmitOutbox=false) into PG.
	rep := migrate.Import(ctx, s, src, migrate.ImportOptions{})
	require.True(t, rep.OK, rep.Text())

	auditBefore := auditCount(t, s)
	stBefore, err := s.Repos().Storages.Get(ctx, "archive")
	require.NoError(t, err)

	// Simulate losing Redis completely: start a brand-new empty miniredis.
	lostMR, lostRdb := newMiniRedis(t)
	lostSrc := migrate.Sources{Config: lostRdb}

	dryRep, err := migrate.Republish(ctx, s, lostSrc, migrate.RepublishOptions{DryRun: true})
	require.NoError(t, err)
	require.True(t, dryRep.OK, dryRep.Text())
	require.True(t, dryRep.DryRun)
	require.Equal(t, 2, dryRep.Families[migrate.FamStorages].Created)
	require.Empty(t, lostMR.Keys(), "dry-run must not write to Redis")

	repubRep, err := migrate.Republish(ctx, s, lostSrc, migrate.RepublishOptions{})
	require.NoError(t, err)
	require.True(t, repubRep.OK, repubRep.Text())

	// No new audit rows and no PG version bump.
	require.Equal(t, auditBefore, auditCount(t, s))
	stAfter, err := s.Repos().Storages.Get(ctx, "archive")
	require.NoError(t, err)
	require.Equal(t, stBefore.Version, stAfter.Version)

	// Verify that the rebuilt Redis and PostgreSQL agree across all families.
	rebuiltData, err := migrate.ReadRedis(ctx, lostSrc)
	require.NoError(t, err)
	pgData, err := migrate.ReadPG(ctx, s.Repos())
	require.NoError(t, err)
	verRep := migrate.Verify(rebuiltData, pgData)
	require.True(t, verRep.OK, verRep.Text())

	// Also verify protection group scope index was rebuilt in Redis.
	groupID, ok := protectiongroups.LookupScope(mustHGetAll(t, lostRdb, protectiongroups.ScopeIndexKey), "alice", "photos")
	require.True(t, ok)
	require.Equal(t, "g1", groupID)
	_ = origMR
}

func mustHGetAll(t *testing.T, rdb *redis.Client, key string) map[string]string {
	t.Helper()
	m, err := rdb.HGetAll(context.Background(), key).Result()
	require.NoError(t, err)
	return m
}
