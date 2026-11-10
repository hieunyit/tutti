package s3users

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3"
)

func withConnection(f *fakeCreds, storages ...string) *fakeCreds {
	if f.conn == nil {
		f.conn = map[string]objstore.ConnectionInfo{}
	}
	for _, st := range storages {
		f.conn[st] = objstore.ConnectionInfo{Credentials: s3.CredentialsV4{AccessKeyID: "svc-" + st, SecretAccessKey: "s"}, Source: objstore.StorageSourceManaged}
	}
	return f
}

func TestCreateWithoutCredentialsNeedsConnectionAccountEverywhere(t *testing.T) {
	ctx := context.Background()
	// only main has a connection account: backup would be unresolvable
	f := withConnection(newFake(true), "main")
	m := newManager(t, f)
	_, err := m.Create(ctx, CreateRequest{Name: "keyless"})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	require.Contains(t, err.Error(), "backup")
	require.False(t, f.HasIdentity("keyless"))
	// an override for the storage without connection account makes the user complete
	res, err := m.Create(ctx, CreateRequest{Name: "keyless", Credentials: []Credential{{Storage: "backup", AccessKeyID: "bk", SecretAccessKey: "bs"}}, Policy: &accesspolicy.Policy{Permission: accesspolicy.ReadWrite, Buckets: []string{"app-*"}}})
	require.NoError(t, err)
	require.True(t, res.User.Complete)

	// both storages have one: no credentials at all
	f = withConnection(newFake(true), "main", "backup")
	m = newManager(t, f)
	res, err = m.Create(ctx, CreateRequest{Name: "keyless", CreateAccessKey: true, Policy: &accesspolicy.Policy{Permission: accesspolicy.ReadWrite, Buckets: []string{"app-*"}}})
	require.NoError(t, err)
	require.True(t, res.User.Complete)
	require.NotEmpty(t, res.AccessSecret)
	for _, st := range res.User.Storages {
		require.Equal(t, objstore.ModeConnection, st.Mode, st.Storage)
		require.False(t, st.Configured)
		require.Empty(t, st.AccessKeyID)
	}
	require.True(t, f.HasIdentity("keyless"))
	require.Empty(t, f.dyn["main"], "no per-user credentials were stored")
	// duplicates are refused, also for identity-only users
	_, err = m.Create(ctx, CreateRequest{Name: "keyless"})
	require.ErrorIs(t, err, dom.ErrAlreadyExists)
}

func TestListShowsModesAndIdentityOnlyUsers(t *testing.T) {
	ctx := context.Background()
	f := withConnection(newFake(true), "main") // backup is legacy
	f.dyn["backup"]["over"] = s3.CredentialsV4{AccessKeyID: "ok", SecretAccessKey: "os"}
	f.dyn["main"]["over"] = s3.CredentialsV4{AccessKeyID: "ok2", SecretAccessKey: "os"}
	require.NoError(t, f.AddIdentity(ctx, "keyless"))
	m := newManager(t, f)
	l, err := m.List(ctx)
	require.NoError(t, err)
	by := map[string]User{}
	for _, u := range l.Users {
		by[u.Name] = u
	}
	require.Contains(t, by, "keyless", "users that exist only as identities are listed")
	modes := func(u User) map[string]objstore.CredMode {
		out := map[string]objstore.CredMode{}
		for _, s := range u.Storages {
			out[s.Storage] = s.Mode
		}
		return out
	}
	require.Equal(t, map[string]objstore.CredMode{"main": objstore.ModeConnection, "backup": objstore.ModeMissing}, modes(by["keyless"]))
	require.False(t, by["keyless"].Complete)
	require.Equal(t, map[string]objstore.CredMode{"main": objstore.ModeOverride, "backup": objstore.ModeOverride}, modes(by["over"]))
	require.True(t, by["over"].Complete)
	// a config user with an override on main and nothing on backup: main override, backup missing
	require.Equal(t, map[string]objstore.CredMode{"main": objstore.ModeOverride, "backup": objstore.ModeOverride}, modes(by["user1"]))
	// once backup gets a connection account, the identity is complete
	withConnection(f, "backup")
	l, err = m.List(ctx)
	require.NoError(t, err)
	for _, u := range l.Users {
		if u.Name == "keyless" {
			require.True(t, u.Complete)
			require.Equal(t, map[string]objstore.CredMode{"main": objstore.ModeConnection, "backup": objstore.ModeConnection}, modes(u))
		}
	}
}

func TestDeleteIdentityOnlyUserAndTestUsesConnectionAccount(t *testing.T) {
	ctx := context.Background()
	f := withConnection(newFake(true), "main", "backup")
	m := newManager(t, f)
	_, err := m.Create(ctx, CreateRequest{Name: "keyless", CreateAccessKey: true, Policy: &accesspolicy.Policy{Permission: accesspolicy.ReadWrite, Buckets: []string{"app-*"}}})
	require.NoError(t, err)
	// the stored credentials test runs with the connection account
	var used string
	m.Verify = func(_ context.Context, st string, c s3.CredentialsV4) error { used = c.AccessKeyID; return nil }
	r, err := m.Test(ctx, "keyless", "main", nil)
	require.NoError(t, err)
	require.True(t, r.OK)
	require.Equal(t, "svc-main", used)

	_, err = m.Delete(ctx, "keyless", false)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition, "active key blocks")
	res, err := m.Delete(ctx, "keyless", true)
	require.NoError(t, err)
	require.Equal(t, 1, res.DeletedAccessKeys)
	require.False(t, f.HasIdentity("keyless"))
	_, err = m.Delete(ctx, "keyless", true)
	require.ErrorIs(t, err, dom.ErrNotFound)

	// rollback: a failing access key creation removes the identity again
	f2 := withConnection(newFake(true), "main", "backup")
	m2 := newManager(t, f2)
	_, err = m2.Create(ctx, CreateRequest{Name: "u2", Policy: &accesspolicy.Policy{Permission: "root"}})
	require.Error(t, err)
	require.False(t, f2.HasIdentity("u2"))
}

func TestUpdateCredentialsOfIdentityOnlyUser(t *testing.T) {
	ctx := context.Background()
	f := withConnection(newFake(true), "main", "backup")
	m := newManager(t, f)
	_, err := m.Create(ctx, CreateRequest{Name: "keyless", Policy: &accesspolicy.Policy{Permission: accesspolicy.ReadWrite, Buckets: []string{"app-*"}}})
	require.NoError(t, err)
	// an override can be added later for tenant isolation on one storage
	require.NoError(t, m.UpdateCredentials(ctx, "keyless", Credential{Storage: "backup", AccessKeyID: "tenant", SecretAccessKey: "ts"}, false))
	u, err := m.Get(ctx, "keyless")
	require.NoError(t, err)
	for _, st := range u.Storages {
		if st.Storage == "backup" {
			require.Equal(t, objstore.ModeOverride, st.Mode)
			require.Equal(t, "tenant", st.AccessKeyID)
		} else {
			require.Equal(t, objstore.ModeConnection, st.Mode)
		}
	}
}

func TestSharedAccountUserNeedsPolicy(t *testing.T) {
	ctx := context.Background()
	f := withConnection(newFake(true), "main", "backup")
	m := newManager(t, f)
	// no policy: refused, nothing is created
	_, err := m.Create(ctx, CreateRequest{Name: "keyless"})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	require.Contains(t, err.Error(), "policy is required")
	require.False(t, f.HasIdentity("keyless"))
	// with a policy: created
	_, err = m.Create(ctx, CreateRequest{Name: "keyless", Policy: &accesspolicy.Policy{Permission: accesspolicy.ReadWrite, Buckets: []string{"app-*"}}})
	require.NoError(t, err)
	require.Equal(t, []string{"backup", "main"}, m.SharedStorages("keyless"))
	// the last policy cannot be deleted
	err = m.CheckPolicyDeletable("keyless")
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Contains(t, err.Error(), "lose all access")

	// a user with its own credentials on every storage needs no policy
	_, err = m.Create(ctx, CreateRequest{Name: "own", Credentials: []Credential{
		{Storage: "main", AccessKeyID: "k1", SecretAccessKey: "s1"}, {Storage: "backup", AccessKeyID: "k2", SecretAccessKey: "s2"}}})
	require.NoError(t, err)
	require.Empty(t, m.SharedStorages("own"))
	require.NoError(t, m.CheckPolicyDeletable("own"))
	// an unknown user (orphan policy) can be deleted
	require.NoError(t, m.CheckPolicyDeletable("ghost"))
	// only main has a connection account and the user has creds on backup only: still shared on main
	f2 := withConnection(newFake(true), "main")
	m2 := newManager(t, f2)
	_, err = m2.Create(ctx, CreateRequest{Name: "half", Credentials: []Credential{{Storage: "backup", AccessKeyID: "k", SecretAccessKey: "s"}}})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	require.Contains(t, err.Error(), "policy is required")
}
