package s3users

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/proxyaccount"
	"github.com/hieunyit/tutti/pkg/s3"
)

type sealer struct{}

func (sealer) Seal(p []byte) (string, error) {
	b, _ := aes.NewCipher([]byte("0123456789abcdef0123456789abcdef"))
	g, _ := cipher.NewGCM(b)
	n := make([]byte, g.NonceSize())
	_, _ = rand.Read(n)
	return base64.StdEncoding.EncodeToString(g.Seal(n, n, p, nil)), nil
}
func (sealer) Open(v string) ([]byte, error) { return nil, errors.New("unused") }

// fakeCreds is an in-memory Credentials: config users are fixed, dynamic ones are mutable.
type fakeCreds struct {
	mu       sync.Mutex
	dynamic  bool
	config   map[string]map[string]s3.CredentialsV4 // storage -> user
	dyn      map[string]map[string]s3.CredentialsV4
	failSet  string // storage whose SetS3Credentials fails
	deleted  []string
	storages map[string]dom.StorageType
}

func newFake(dynamic bool) *fakeCreds {
	return &fakeCreds{
		dynamic:  dynamic,
		storages: map[string]dom.StorageType{"main": dom.S3, "backup": dom.S3},
		config: map[string]map[string]s3.CredentialsV4{
			"main":   {"user1": {AccessKeyID: "ak1", SecretAccessKey: "sk1"}},
			"backup": {"user1": {AccessKeyID: "bk1", SecretAccessKey: "bs1"}},
		},
		dyn: map[string]map[string]s3.CredentialsV4{"main": {}, "backup": {}},
	}
}

func (f *fakeCreds) MainStorage() string                  { return "main" }
func (f *fakeCreds) Storages() map[string]dom.StorageType { return f.storages }
func (f *fakeCreds) DynamicCredentialsEnabled() bool      { return f.dynamic }
func (f *fakeCreds) HasUser(st, u string) error {
	if f.CredentialSource(st, u) == "" {
		return dom.ErrNotFound
	}
	return nil
}
func (f *fakeCreds) ListUsers(st string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var r []string
	for u := range f.config[st] {
		r = append(r, u)
	}
	for u := range f.dyn[st] {
		r = append(r, u)
	}
	return r
}
func (f *fakeCreds) CredentialSource(st, u string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.config[st][u]; ok {
		return "config"
	}
	if _, ok := f.dyn[st][u]; ok {
		return "dynamic"
	}
	return ""
}
func (f *fakeCreds) GetS3Credentials(st, u string) (s3.CredentialsV4, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.config[st][u]; ok {
		return c, nil
	}
	if c, ok := f.dyn[st][u]; ok {
		return c, nil
	}
	return s3.CredentialsV4{}, dom.ErrNotFound
}
func (f *fakeCreds) SetS3Credentials(_ context.Context, st, u string, c s3.CredentialsV4) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if st == f.failSet {
		return errors.New("boom")
	}
	if _, ok := f.config[st][u]; ok {
		return dom.ErrInvalidArg
	}
	f.dyn[st][u] = c
	return nil
}
func (f *fakeCreds) DeleteUserCredentials(_ context.Context, st, u string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.dyn[st][u]; !ok {
		return dom.ErrNotFound
	}
	delete(f.dyn[st], u)
	f.deleted = append(f.deleted, st+":"+u)
	return nil
}

func newManager(t *testing.T, creds *fakeCreds) *Manager {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	return &Manager{
		Creds:    creds,
		Accounts: proxyaccount.NewStore(rc, func() (proxyaccount.Sealer, error) { return sealer{}, nil }),
		Policies: accesspolicy.NewStore(rc),
		Redis:    rc,
		Verify: func(_ context.Context, st string, c s3.CredentialsV4) error {
			if c.SecretAccessKey == "bad" {
				return errors.New("SignatureDoesNotMatch: bad secret")
			}
			return nil
		},
	}
}

var both = []Credential{{Storage: "main", AccessKeyID: "nm", SecretAccessKey: "ns"}, {Storage: "backup", AccessKeyID: "nb", SecretAccessKey: "nbs"}}

func TestList_UnionAndNeverSecrets(t *testing.T) {
	ctx := context.Background()
	f := newFake(true)
	f.dyn["main"]["partial"] = s3.CredentialsV4{AccessKeyID: "pk", SecretAccessKey: "TOPSECRET"}
	m := newManager(t, f)
	require.NoError(t, m.Policies.Set(ctx, accesspolicy.Policy{User: "user1", Permission: accesspolicy.ReadOnly, Buckets: []string{"a*"}}))
	_, _, err := m.Accounts.Create(ctx, "user1", "k1")
	require.NoError(t, err)
	acc2, _, err := m.Accounts.Create(ctx, "user1", "k2")
	require.NoError(t, err)
	require.NoError(t, m.Accounts.SetStatus(ctx, acc2.AccessKeyID, proxyaccount.StatusDisabled))

	l, err := m.List(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"backup", "main"}, l.Storages)
	require.True(t, l.Dynamic)
	require.Len(t, l.Users, 2)
	partial, u1 := l.Users[0], l.Users[1]
	require.Equal(t, "partial", partial.Name)
	require.Equal(t, "user1", u1.Name)
	require.True(t, u1.Complete)
	require.Equal(t, StorageState{Storage: "backup", Configured: true, Source: "config", AccessKeyID: "bk1"}, u1.Storages[0])
	require.Equal(t, KeyCounts{DynamicActive: 1, DynamicDisabled: 1}, u1.KeyCounts)
	require.Equal(t, PolicySummary{HasPolicy: true, Permission: "read-only", Buckets: []string{"a*"}}, u1.Policy)
	require.False(t, partial.Complete)
	require.Equal(t, StorageState{Storage: "backup"}, partial.Storages[0])
	require.Equal(t, StorageState{Storage: "main", Configured: true, Source: "dynamic", AccessKeyID: "pk"}, partial.Storages[1])
	require.Equal(t, PolicySummary{Permission: "read-write", Buckets: []string{"*"}}, partial.Policy)
}

func TestCreate_Validation(t *testing.T) {
	ctx := context.Background()
	m := newManager(t, newFake(true))
	_, err := m.Create(ctx, CreateRequest{Name: "bad name", Credentials: both})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, err = m.Create(ctx, CreateRequest{Name: "u", Credentials: both[1:]})
	require.ErrorIs(t, err, dom.ErrInvalidArg, "main storage required")
	_, err = m.Create(ctx, CreateRequest{Name: "u"})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, err = m.Create(ctx, CreateRequest{Name: "u", Credentials: []Credential{{Storage: "nope", AccessKeyID: "a", SecretAccessKey: "b"}}})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, err = m.Create(ctx, CreateRequest{Name: "u", Credentials: []Credential{{Storage: "main", AccessKeyID: "a"}}})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, err = m.Create(ctx, CreateRequest{Name: "user1", Credentials: both})
	require.ErrorIs(t, err, dom.ErrAlreadyExists)
	_, err = m.Create(ctx, CreateRequest{Name: "u", Credentials: both, Policy: &accesspolicy.Policy{Permission: "root"}})
	require.ErrorIs(t, err, dom.ErrInvalidArg)

	noDyn := newManager(t, newFake(false))
	_, err = noDyn.Create(ctx, CreateRequest{Name: "u", Credentials: both})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Contains(t, err.Error(), "dynamic credentials are not enabled")
}

func TestCreate_FullFlow(t *testing.T) {
	ctx := context.Background()
	f := newFake(true)
	m := newManager(t, f)
	res, err := m.Create(ctx, CreateRequest{
		Name: "user3", Credentials: both, Verify: true, CreateAccessKey: true, Description: "first",
		Policy: &accesspolicy.Policy{Permission: accesspolicy.ReadOnly, Buckets: []string{"x"}},
	})
	require.NoError(t, err)
	require.True(t, res.User.Complete)
	require.Len(t, res.Verification, 2)
	require.NotEmpty(t, res.AccessSecret)
	require.Equal(t, "user3", res.AccessKey.User)
	require.Equal(t, KeyCounts{DynamicActive: 1}, res.User.KeyCounts)
	require.True(t, res.User.Policy.HasPolicy)
	require.Equal(t, "user3", must(m.Policies.Get(ctx, "user3")).User)
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestCreate_VerifyFailureSavesNothing(t *testing.T) {
	ctx := context.Background()
	f := newFake(true)
	m := newManager(t, f)
	bad := []Credential{both[0], {Storage: "backup", AccessKeyID: "x", SecretAccessKey: "bad"}}
	res, err := m.Create(ctx, CreateRequest{Name: "user3", Credentials: bad, Verify: true, CreateAccessKey: true})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	require.Contains(t, err.Error(), `storage "backup": SignatureDoesNotMatch`)
	require.Len(t, res.Verification, 2)
	require.True(t, res.Verification[0].OK)
	require.False(t, res.Verification[1].OK)
	require.Empty(t, f.dyn["main"])
	accs, _ := m.Accounts.List(ctx)
	require.Empty(t, accs)
}

func TestCreate_RollbackOnCredentialFailure(t *testing.T) {
	ctx := context.Background()
	f := newFake(true)
	f.failSet = "backup"
	m := newManager(t, f)
	_, err := m.Create(ctx, CreateRequest{Name: "user3", Credentials: both, CreateAccessKey: true,
		Policy: &accesspolicy.Policy{Permission: accesspolicy.ReadWrite}})
	require.Error(t, err)
	require.Empty(t, f.dyn["main"], "credentials set before the failure are removed")
	require.Empty(t, f.dyn["backup"])
	pols, _ := m.Policies.List(ctx)
	require.Empty(t, pols)
}

func TestCreate_RollbackOnAccessKeyFailure(t *testing.T) {
	ctx := context.Background()
	f := newFake(true)
	m := newManager(t, f)
	calls := 0
	m.Accounts = proxyaccount.NewStore(m.Redis, func() (proxyaccount.Sealer, error) {
		calls++
		if calls > 1 { // available for the pre-check, fails on use
			return nil, errors.New("sealer gone")
		}
		return sealer{}, nil
	})
	_, err := m.Create(ctx, CreateRequest{Name: "user3", Credentials: both, CreateAccessKey: true,
		Policy: &accesspolicy.Policy{Permission: accesspolicy.ReadWrite}})
	require.Error(t, err)
	require.Empty(t, f.dyn["main"])
	require.Empty(t, f.dyn["backup"])
	pols, _ := m.Policies.List(ctx)
	require.Empty(t, pols, "policy rolled back")
}

func TestCreate_AccessKeyNeedsSealer(t *testing.T) {
	m := newManager(t, newFake(true))
	m.Accounts = proxyaccount.NewStore(m.Redis, nil)
	_, err := m.Create(context.Background(), CreateRequest{Name: "u", Credentials: both, CreateAccessKey: true})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Empty(t, m.Creds.(*fakeCreds).dyn["main"])
}

func TestUpdateCredentials(t *testing.T) {
	ctx := context.Background()
	f := newFake(true)
	m := newManager(t, f)
	_, err := m.Create(ctx, CreateRequest{Name: "user3", Credentials: both})
	require.NoError(t, err)
	require.NoError(t, m.UpdateCredentials(ctx, "user3", Credential{Storage: "main", AccessKeyID: "new", SecretAccessKey: "nsk"}, true))
	require.Equal(t, "new", f.dyn["main"]["user3"].AccessKeyID)
	require.ErrorIs(t, m.UpdateCredentials(ctx, "user3", Credential{Storage: "main", AccessKeyID: "n", SecretAccessKey: "bad"}, true), dom.ErrInvalidArg)
	require.Equal(t, "new", f.dyn["main"]["user3"].AccessKeyID, "failed verification keeps the old credentials")
	err = m.UpdateCredentials(ctx, "user1", Credential{Storage: "main", AccessKeyID: "n", SecretAccessKey: "s"}, false)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Contains(t, err.Error(), "defined in config")
	require.ErrorIs(t, m.UpdateCredentials(ctx, "ghost", Credential{Storage: "main", AccessKeyID: "n", SecretAccessKey: "s"}, false), dom.ErrNotFound)
}

func TestDelete_RefusalRules(t *testing.T) {
	ctx := context.Background()
	f := newFake(true)
	m := newManager(t, f)

	_, err := m.Delete(ctx, "ghost", false)
	require.ErrorIs(t, err, dom.ErrNotFound)

	// config credentials: always refused, force does not help
	_, err = m.Delete(ctx, "user1", true)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Contains(t, err.Error(), "defined in the worker config")

	_, err = m.Create(ctx, CreateRequest{Name: "user3", Credentials: both, CreateAccessKey: true,
		Policy: &accesspolicy.Policy{Permission: accesspolicy.ReadOnly}})
	require.NoError(t, err)

	// active key blocks unless forced
	_, err = m.Delete(ctx, "user3", false)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Contains(t, err.Error(), "active proxy access key")
	require.NotEmpty(t, f.dyn["main"]["user3"].AccessKeyID, "nothing deleted on refusal")

	// references block even with force
	m.Refs = func(context.Context, string) ([]string, error) { return []string{"1 replication(s) exist"}, nil }
	_, err = m.Delete(ctx, "user3", true)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Contains(t, err.Error(), "replication")
	_, err = m.Delete(ctx, "user3", false)
	require.Contains(t, err.Error(), "replication")
	require.Contains(t, err.Error(), "active proxy access key", "all reasons are listed")
	require.Len(t, f.dyn["main"], 1)

	m.Refs = func(context.Context, string) ([]string, error) { return nil, nil }
	res, err := m.Delete(ctx, "user3", true)
	require.NoError(t, err)
	require.Equal(t, 1, res.DeletedAccessKeys)
	require.True(t, res.DeletedPolicy)
	sort.Strings(res.DeletedStorages)
	require.Equal(t, []string{"backup", "main"}, res.DeletedStorages)
	require.Empty(t, f.dyn["main"])
	accs, _ := m.Accounts.List(ctx)
	require.Empty(t, accs)
	pols, _ := m.Policies.List(ctx)
	require.Empty(t, pols)
}

func TestDelete_DisabledKeysDoNotBlock(t *testing.T) {
	ctx := context.Background()
	m := newManager(t, newFake(true))
	res, err := m.Create(ctx, CreateRequest{Name: "user3", Credentials: both, CreateAccessKey: true})
	require.NoError(t, err)
	require.NoError(t, m.Accounts.SetStatus(ctx, res.AccessKey.AccessKeyID, proxyaccount.StatusDisabled))
	out, err := m.Delete(ctx, "user3", false)
	require.NoError(t, err)
	require.Equal(t, 1, out.DeletedAccessKeys)
}

func TestTest(t *testing.T) {
	ctx := context.Background()
	m := newManager(t, newFake(true))
	r, err := m.Test(ctx, "user1", "main", nil)
	require.NoError(t, err)
	require.True(t, r.OK)
	r, err = m.Test(ctx, "", "main", &Credential{AccessKeyID: "a", SecretAccessKey: "bad"})
	require.NoError(t, err)
	require.False(t, r.OK)
	require.Contains(t, r.Error, "SignatureDoesNotMatch")
	_, err = m.Test(ctx, "ghost", "main", nil)
	require.ErrorIs(t, err, dom.ErrNotFound)
	_, err = m.Test(ctx, "user1", "nope", nil)
	require.ErrorIs(t, err, dom.ErrInvalidArg)
}
