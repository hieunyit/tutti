package objstore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/testutil"
)

func accessKeyOf(r *http.Request) string {
	_, rest, _ := strings.Cut(r.Header.Get("Authorization"), "Credential=")
	ak, _, _ := strings.Cut(rest, "/")
	return ak
}

// upstream is a fake storage that records the access key of every request and accepts the listed keys.
type upstream struct {
	*httptest.Server
	mu    sync.Mutex
	valid map[string]bool
	seen  map[string]int
	bad   atomic.Int64
}

func newUpstream(keys ...string) *upstream {
	u := &upstream{valid: map[string]bool{}, seen: map[string]int{}}
	for _, k := range keys {
		u.valid[k] = true
	}
	u.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ak := accessKeyOf(r)
		u.mu.Lock()
		ok := u.valid[ak]
		u.seen[ak]++
		u.mu.Unlock()
		w.Header().Set("Content-Type", "application/xml")
		if !ok {
			u.bad.Add(1)
			w.WriteHeader(403)
			_, _ = w.Write([]byte("<Error><Code>InvalidAccessKeyId</Code></Error>"))
			return
		}
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`<ListAllMyBucketsResult><Owner><ID>u</ID></Owner><Buckets/></ListAllMyBucketsResult>`))
			return
		}
		w.WriteHeader(404)
		_, _ = w.Write([]byte("<Error><Code>NoSuchBucket</Code></Error>"))
	}))
	return u
}

func (u *upstream) count(ak string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.seen[ak]
}

func cfgWith(addr string, conn *s3.CredentialsV4, creds map[string]s3.CredentialsV4) *Config {
	return &Config{Main: "main", Storages: map[string]Storage{
		"main": {CommonConfig: CommonConfig{Type: dom.S3}, S3: &s3.Storage{
			StorageAddress: s3.StorageAddress{Address: addr, Provider: s3.ProviderMinIO}, ConnectionAccount: conn, Credentials: creds}},
	}, DynamicCredentials: DynamicCredentialsConfig{Enabled: true, MasterPassword: "conn-test-password", PollInterval: time.Hour}}
}

func TestResolutionOrderOverrideConnectionMissing(t *testing.T) {
	rc := testutil.SetupRedis(t)
	conf := cfgWith("http://localhost:9000", &s3.CredentialsV4{AccessKeyID: "conn", SecretAccessKey: "connsecret"},
		map[string]s3.CredentialsV4{"alice": {AccessKeyID: "alice-key", SecretAccessKey: "alice-secret"}})
	conf.Storages["legacy"] = Storage{CommonConfig: CommonConfig{Type: dom.S3}, S3: &s3.Storage{
		StorageAddress: s3.StorageAddress{Address: "http://localhost:9001", Provider: s3.ProviderMinIO},
		Credentials:    map[string]s3.CredentialsV4{"alice": {AccessKeyID: "alice-legacy", SecretAccessKey: "s"}}}}
	svc, err := NewCredsSvc(t.Context(), conf, rc, WithManagedStorages(rc))
	require.NoError(t, err)

	// override wins over the connection account
	c, mode, err := svc.ResolveS3Credentials("main", "alice")
	require.NoError(t, err)
	require.Equal(t, ModeOverride, mode)
	require.Equal(t, "alice-key", c.AccessKeyID)
	// a dynamic override as well
	require.NoError(t, svc.SetS3Credentials(t.Context(), "main", "carol", s3.CredentialsV4{AccessKeyID: "carol-key", SecretAccessKey: "carol-secret"}))
	c, mode, err = svc.ResolveS3Credentials("main", "carol")
	require.NoError(t, err)
	require.Equal(t, ModeOverride, mode)
	require.Equal(t, "carol-key", c.AccessKeyID)
	// no override: the connection account
	c, mode, err = svc.ResolveS3Credentials("main", "bob")
	require.NoError(t, err)
	require.Equal(t, ModeConnection, mode)
	require.Equal(t, "conn", c.AccessKeyID)
	// no override, no connection account: the exact error
	_, mode, err = svc.ResolveS3Credentials("legacy", "bob")
	require.Equal(t, ModeMissing, mode)
	require.EqualError(t, err, "storage legacy has no credentials for user bob")
	require.ErrorIs(t, err, dom.ErrNoCredentials)
	// unknown storage
	_, _, err = svc.ResolveS3Credentials("nope", "bob")
	require.ErrorIs(t, err, dom.ErrNotFound)

	// GetS3Credentials stays "override only"
	_, err = svc.GetS3Credentials("main", "bob")
	require.ErrorIs(t, err, dom.ErrNotFound)
	// ListUsers: overrides plus identities (storages with a connection account); legacy storages only overrides
	require.NoError(t, svc.AddIdentity(t.Context(), "bob"))
	require.ElementsMatch(t, []string{"alice", "carol", "bob"}, svc.ListUsers("main"))
	require.ElementsMatch(t, []string{"alice", "carol"}, svc.ListOverrideUsers("main"))
	require.Equal(t, []string{"alice"}, svc.ListUsers("legacy"))
	// HasUser: identities and override users resolve through the connection account; unknown users do not
	require.NoError(t, svc.HasUser("main", "bob"))
	require.NoError(t, svc.HasUser("main", "alice"))
	require.Error(t, svc.HasUser("main", "stranger"))
	require.Error(t, svc.HasUser("legacy", "bob"))
	require.ErrorIs(t, svc.AddIdentity(t.Context(), "bob"), dom.ErrAlreadyExists)
	require.NoError(t, svc.RemoveIdentity(t.Context(), "bob"))
	require.False(t, svc.HasIdentity("bob"))
}

func TestForwardingSignsWithTheRightKey(t *testing.T) {
	up := newUpstream("alice-key", "conn")
	defer up.Close()
	rc := testutil.SetupRedis(t)
	conf := cfgWith(up.URL, &s3.CredentialsV4{AccessKeyID: "conn", SecretAccessKey: "connsecret"},
		map[string]s3.CredentialsV4{"alice": {AccessKeyID: "alice-key", SecretAccessKey: "alice-secret"}})
	conf.Storages["legacy"] = Storage{CommonConfig: CommonConfig{Type: dom.S3}, S3: &s3.Storage{
		StorageAddress: s3.StorageAddress{Address: up.URL, Provider: s3.ProviderMinIO},
		Credentials:    map[string]s3.CredentialsV4{"alice": {AccessKeyID: "alice-key", SecretAccessKey: "s"}}}}
	svc, err := NewCredsSvc(t.Context(), conf, rc)
	require.NoError(t, err)
	reg, err := NewRegistry(t.Context(), svc, metrics.NewS3Service(false))
	require.NoError(t, err)

	a, err := reg.AsS3(t.Context(), "main", "alice")
	require.NoError(t, err)
	_, err = a.S3().ListBuckets(t.Context())
	require.NoError(t, err)
	b, err := reg.AsS3(t.Context(), "main", "bob")
	require.NoError(t, err)
	before := up.count("conn")
	_, err = b.S3().ListBuckets(t.Context())
	require.NoError(t, err)
	require.Greater(t, up.count("conn"), before, "bob is signed with the connection account")
	require.Positive(t, up.count("alice-key"))
	require.Zero(t, up.bad.Load())

	// a storage without connection account: the exact error, no request
	_, err = reg.AsS3(t.Context(), "legacy", "bob")
	require.EqualError(t, err, "storage legacy has no credentials for user bob")
	require.ErrorIs(t, err, dom.ErrNoCredentials)
	// and through AsCommon (worker path)
	_, err = reg.AsCommon(t.Context(), "legacy", "bob")
	require.ErrorIs(t, err, dom.ErrNoCredentials)
}

func newManagedPair(t *testing.T, conf *Config) (a, b *credsSvc, rc redis.UniversalClient) {
	t.Helper()
	rc = testutil.SetupRedis(t)
	confRC := redis.NewClient(&redis.Options{Addr: rc.(*redis.Client).Options().Addr, DB: 1})
	t.Cleanup(func() { _ = confRC.Close() })
	var err error
	a, err = NewCredsSvc(t.Context(), conf, rc, WithManagedStorages(confRC))
	require.NoError(t, err)
	b, err = NewCredsSvc(t.Context(), conf, rc, WithManagedStorages(confRC))
	require.NoError(t, err)
	return a, b, confRC
}

func TestManagedConnectionAccountSealedAndPropagated(t *testing.T) {
	a, b, confRC := newManagedPair(t, managedTestConf())
	m := ManagedStorage{Name: "archive", Address: s3.StorageAddress{Address: "http://localhost:9001", Provider: s3.ProviderMinIO},
		ConnectionAccount: s3.CredentialsV4{AccessKeyID: "svc-key", SecretAccessKey: "very-secret-value"}}
	_, err := a.CreateManaged(t.Context(), m)
	require.NoError(t, err)
	raw, err := confRC.HGet(t.Context(), managedStoragesKey, "archive").Result()
	require.NoError(t, err)
	require.NotContains(t, raw, "very-secret-value")
	require.NotContains(t, raw, "svc-key")
	require.Contains(t, raw, "connectionAccount")
	require.Eventually(t, func() bool {
		ci, ok := b.ConnectionAccount("archive")
		return ok && ci.Credentials.AccessKeyID == "svc-key" && ci.Source == StorageSourceManaged
	}, 5*time.Second, 20*time.Millisecond)
	c, mode, err := b.ResolveS3Credentials("archive", "anyone")
	require.NoError(t, err)
	require.Equal(t, ModeConnection, mode)
	require.Equal(t, "very-secret-value", c.SecretAccessKey)

	// an update without account keeps it; clearing removes it; forced-save metadata is kept
	m.ConnectionAccount = s3.CredentialsV4{}
	m.LastForcedSaveBy, m.LastForcedSaveAt = "admin", time.Now().UTC().Truncate(time.Second)
	_, err = a.UpdateManaged(t.Context(), m)
	require.NoError(t, err)
	got, _ := a.GetManaged("archive")
	require.Equal(t, "svc-key", got.ConnectionAccount.AccessKeyID)
	require.Equal(t, "admin", got.LastForcedSaveBy)
	require.False(t, got.LastForcedSaveAt.IsZero())
	m.LastForcedSaveBy = ""
	_, err = a.UpdateManaged(t.Context(), m)
	require.NoError(t, err)
	got, _ = a.GetManaged("archive")
	require.Equal(t, "admin", got.LastForcedSaveBy, "kept when the update does not force")
	m.ClearConnectionAccount = true
	_, err = a.UpdateManaged(t.Context(), m)
	require.NoError(t, err)
	_, ok := a.ConnectionAccount("archive")
	require.False(t, ok)

	// config storages cannot be rotated
	_, err = a.RotateConnectionAccount(t.Context(), "main", s3.CredentialsV4{AccessKeyID: "x", SecretAccessKey: "y"}, "")
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
}

func TestRotationWithoutDowntimeForTwoConsumers(t *testing.T) {
	// the storage accepts the old key until it is revoked, like a real rotation procedure
	up := newUpstream("key-v1", "key-v2")
	defer up.Close()
	a, b, _ := newManagedPair(t, cfgWith(up.URL, nil, nil))
	_, err := a.CreateManaged(t.Context(), ManagedStorage{Name: "archive", Address: s3.StorageAddress{Address: up.URL, Provider: s3.ProviderMinIO},
		ConnectionAccount: s3.CredentialsV4{AccessKeyID: "key-v1", SecretAccessKey: "secret-v1"}})
	require.NoError(t, err)
	require.Eventually(t, func() bool { _, ok := b.ConnectionAccount("archive"); return ok }, 5*time.Second, 20*time.Millisecond)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var failures, requests atomic.Int64
	var wg sync.WaitGroup
	consumer := func(svc *credsSvc) {
		defer wg.Done()
		reg, err := NewRegistry(ctx, svc, metrics.NewS3Service(false))
		if err != nil {
			failures.Add(1)
			return
		}
		for ctx.Err() == nil {
			c, err := reg.AsS3(ctx, "archive", "alice")
			if err == nil {
				_, err = c.S3().ListBuckets(ctx)
			}
			if err != nil && ctx.Err() == nil {
				failures.Add(1)
			}
			requests.Add(1)
			time.Sleep(10 * time.Millisecond)
		}
	}
	wg.Add(2)
	go consumer(a)
	go consumer(b)
	time.Sleep(200 * time.Millisecond)

	rotated, err := a.RotateConnectionAccount(t.Context(), "archive", s3.CredentialsV4{AccessKeyID: "key-v2", SecretAccessKey: "secret-v2"}, "")
	require.NoError(t, err)
	require.Equal(t, "key-v1", rotated.PreviousAccount.AccessKeyID)
	require.False(t, rotated.RotatedAt.IsZero())
	// the rotating process switches at once, the other one within the poll interval
	ci, _ := a.ConnectionAccount("archive")
	require.Equal(t, "key-v2", ci.Credentials.AccessKeyID)
	require.Equal(t, "key-v1", ci.Previous.AccessKeyID)
	require.WithinDuration(t, ci.RotatedAt.Add(RotationPropagationDelay), ci.SafeToRevokePreviousAfter(), time.Second)
	require.Eventually(t, func() bool {
		ci, _ := b.ConnectionAccount("archive")
		return ci.Credentials.AccessKeyID == "key-v2"
	}, 6*time.Second, 20*time.Millisecond)
	require.Eventually(t, func() bool { return up.count("key-v2") >= 4 }, 5*time.Second, 20*time.Millisecond, "v1=%d v2=%d fail=%d req=%d", up.count("key-v1"), up.count("key-v2"), failures.Load(), requests.Load())

	// both consumers are on the new key now: only then the old key is revoked on the storage
	up.mu.Lock()
	delete(up.valid, "key-v1")
	up.mu.Unlock()
	time.Sleep(300 * time.Millisecond)
	cancel()
	wg.Wait()
	require.Positive(t, requests.Load())
	require.Zero(t, failures.Load(), "no request failed during the rotation")
	require.Zero(t, up.bad.Load())
	require.Positive(t, up.count("key-v1"))

	// rotating to the same account or on a missing storage is refused
	_, err = a.RotateConnectionAccount(t.Context(), "archive", s3.CredentialsV4{AccessKeyID: "key-v2", SecretAccessKey: "secret-v2"}, "")
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, err = a.RotateConnectionAccount(t.Context(), "nope", s3.CredentialsV4{AccessKeyID: "k", SecretAccessKey: "s"}, "")
	require.ErrorIs(t, err, dom.ErrNotFound)
	// forced rotation records who
	m, err := a.RotateConnectionAccount(t.Context(), "archive", s3.CredentialsV4{AccessKeyID: "key-v3", SecretAccessKey: "secret-v3"}, "admin")
	require.NoError(t, err)
	require.Equal(t, "admin", m.LastForcedSaveBy)
}

func TestIdentitiesPropagateAndLastTestStored(t *testing.T) {
	a, b, confRC := newManagedPair(t, managedTestConf())
	require.NoError(t, a.AddIdentity(t.Context(), "bob"))
	require.Eventually(t, func() bool { return b.HasIdentity("bob") }, 5*time.Second, 20*time.Millisecond)
	require.Equal(t, []string{"bob"}, b.Identities())
	require.NoError(t, a.RemoveIdentity(t.Context(), "bob"))
	require.Eventually(t, func() bool { return !b.HasIdentity("bob") }, 5*time.Second, 20*time.Millisecond)

	require.NoError(t, a.SaveStorageTest(t.Context(), "archive", []byte(`{"ok":true}`)))
	tests, err := b.StorageTests(t.Context())
	require.NoError(t, err)
	require.JSONEq(t, `{"ok":true}`, tests["archive"])
	// deleting the storage drops its stored test
	_, err = a.CreateManaged(t.Context(), ManagedStorage{Name: "archive", Address: s3.StorageAddress{Address: "http://localhost:9001", Provider: s3.ProviderMinIO}})
	require.NoError(t, err)
	require.NoError(t, a.DeleteManaged(t.Context(), "archive"))
	require.False(t, confRC.HExists(t.Context(), storageTestsKey, "archive").Val())

	// without a config DB identities cannot be stored
	c, err := NewCredsSvc(t.Context(), managedTestConf(), testutil.SetupRedis(t))
	require.NoError(t, err)
	require.ErrorIs(t, c.AddIdentity(t.Context(), "x"), dom.ErrFailedPrecondition)
}

func TestConfigConnectionAccountValidation(t *testing.T) {
	st := &s3.Storage{StorageAddress: s3.StorageAddress{Address: "http://localhost:9000", Provider: s3.ProviderMinIO}, ConnectionAccount: &s3.CredentialsV4{AccessKeyID: "only-key"}}
	require.Error(t, st.Validate())
	st.ConnectionAccount.SecretAccessKey = "s"
	require.NoError(t, st.Validate())
}
