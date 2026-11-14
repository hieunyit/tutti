package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/auth"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/storagetest/fakes3"
	"github.com/hieunyit/tutti/pkg/testutil"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

func connTestSetup(t *testing.T) (*resilienceHandlers, objstore.CredsService, *fakes3.Server) {
	t.Helper()
	fake := fakes3.New()
	t.Cleanup(fake.Close)
	rc := testutil.SetupRedis(t)
	cfg := &objstore.Config{Main: "main", Storages: map[string]objstore.Storage{"main": {CommonConfig: objstore.CommonConfig{Type: dom.S3}, S3: &s3.Storage{
		StorageAddress:    s3.StorageAddress{Address: fake.URL, Provider: s3.ProviderMinIO, DefaultRegion: "us-east-1"},
		ConnectionAccount: &s3.CredentialsV4{AccessKeyID: "tutti-svc", SecretAccessKey: "secret-main"}}}},
		DynamicCredentials: objstore.DynamicCredentialsConfig{Enabled: true, MasterPassword: "runtime-test-password", PollInterval: time.Hour}}
	creds, err := objstore.NewCredsSvc(t.Context(), cfg, rc, objstore.WithManagedStorages(rc))
	require.NoError(t, err)
	h := ResilienceHandlers(rc, nil, WithRuntimeStorages(creds, func(context.Context, string) ([]string, error) { return nil, nil }))
	return h.(*resilienceHandlers), creds, fake
}

func stepOf(r *pb.StorageTestResult, id string) *pb.StorageTestStep {
	for _, s := range r.Steps {
		if s.Id == id {
			return s
		}
	}
	return nil
}

func TestCreateStorageWithConnectionAccountRunsTestAndStoresIt(t *testing.T) {
	h, creds, fake := connTestSetup(t)
	ctx := auth.ContextWithSession(t.Context(), &auth.Session{Username: "admin", Role: "admin"})
	def := &pb.RuntimeStorage{Name: "archive", Address: fake.URL, Provider: "Minio", DefaultRegion: "us-east-1"}
	account := &pb.ConnectionAccount{AccessKeyId: "tutti-svc", SecretAccessKey: "secret-archive"}

	// a failing step refuses the save: nothing is stored
	fake.Deny["PutObject"] = true
	_, err := h.CreateRuntimeStorage(ctx, &pb.CreateRuntimeStorageRequest{Storage: def, ConnectionAccount: account})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Contains(t, err.Error(), "step 9 write")
	require.Contains(t, err.Error(), "PutObject")
	_, exists := creds.GetS3Address("archive")
	require.Error(t, exists)
	require.Empty(t, fake.Buckets(), "probe bucket cleaned up")

	// force: saved, who and when recorded, the failed test stored
	before := time.Now().Add(-time.Second)
	saved, err := h.CreateRuntimeStorage(ctx, &pb.CreateRuntimeStorageRequest{Storage: def, ConnectionAccount: account, Force: true})
	require.NoError(t, err)
	require.Equal(t, "admin", saved.LastForcedSaveBy)
	require.True(t, saved.LastForcedSaveAt.AsTime().After(before))
	require.True(t, saved.ConnectionAccount.Configured)
	require.Equal(t, "tutti-svc", saved.ConnectionAccount.AccessKeyId)
	require.Equal(t, "managed", saved.ConnectionAccount.Source)
	require.False(t, saved.LegacyCredentials)
	require.NotNil(t, saved.LastTest)
	require.False(t, saved.LastTest.Ok)
	require.Equal(t, "fail", stepOf(saved.LastTest, "write").Status)
	require.Empty(t, saved.Users, "no per-user override")
	// the secret is never returned
	raw := saved.String() + saved.ConnectionAccount.String()
	require.NotContains(t, raw, "secret-archive")

	// without force on an existing storage the update is refused while the step still fails
	_, err = h.UpdateRuntimeStorage(ctx, &pb.UpdateRuntimeStorageRequest{Storage: def})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	// fixed: the update runs the test and stores a passing result; the forced-save record is kept
	delete(fake.Deny, "PutObject")
	updated, err := h.UpdateRuntimeStorage(ctx, &pb.UpdateRuntimeStorageRequest{Storage: def})
	require.NoError(t, err)
	require.True(t, updated.LastTest.Ok)
	require.Equal(t, "admin", updated.LastForcedSaveBy)

	// listing and GetRuntimeStorage expose connection account and last test
	l, err := h.ListRuntimeStorages(ctx, &pb.ListRuntimeStoragesRequest{})
	require.NoError(t, err)
	byName := map[string]*pb.RuntimeStorage{}
	for _, s := range l.Storages {
		byName[s.Name] = s
	}
	require.Equal(t, "config", byName["main"].ConnectionAccount.Source)
	require.True(t, byName["archive"].LastTest.Ok)
	require.Nil(t, byName["main"].LastTest)
	got, err := h.GetRuntimeStorage(ctx, &pb.RuntimeStorageName{Name: "archive"})
	require.NoError(t, err)
	require.True(t, got.LastTest.Ok)
	_, err = h.GetRuntimeStorage(ctx, &pb.RuntimeStorageName{Name: "nope"})
	require.ErrorIs(t, err, dom.ErrNotFound)

	// clearing the account gives the storage back to legacy per-user credentials
	cleared, err := h.UpdateRuntimeStorage(ctx, &pb.UpdateRuntimeStorageRequest{Storage: def, ClearConnectionAccount: true, Force: true, Credentials: []*pb.RuntimeStorageCredential{{User: "u", AccessKeyId: "tutti-svc", SecretAccessKey: "s"}}})
	require.NoError(t, err)
	require.True(t, cleared.LegacyCredentials)
	require.Len(t, cleared.Users, 1)
}

func TestTestStorageConnectionUnsavedAndExisting(t *testing.T) {
	h, _, fake := connTestSetup(t)
	ctx := t.Context()
	// unsaved settings + account
	res, err := h.TestStorageConnection(ctx, &pb.TestStorageConnectionRequest{
		Storage:           &pb.RuntimeStorage{Name: "new", Address: fake.URL, Provider: "Minio"},
		ConnectionAccount: &pb.ConnectionAccount{AccessKeyId: "tutti-svc", SecretAccessKey: "x"},
		Options:           &pb.StorageTestOptions{ProbeBucketPrefix: "app-tutti-probe-"},
	})
	require.NoError(t, err)
	require.True(t, res.Ok, "%v", res.Steps)
	require.True(t, strings.HasPrefix(res.ProbeBucket, "app-tutti-probe-"))
	require.Len(t, res.Steps, 16, "step 17 only runs on request")
	require.Equal(t, "pass", stepOf(res, "authentication").Status)
	require.NotEmpty(t, stepOf(res, "authentication").Call)

	// a failing step is reported, not returned as an error
	fake.Deny["DeleteBucket"] = true
	res, err = h.TestStorageConnection(ctx, &pb.TestStorageConnectionRequest{Name: "main"})
	require.NoError(t, err)
	require.False(t, res.Ok)
	del := stepOf(res, "delete")
	require.Equal(t, "fail", del.Status)
	require.Equal(t, "s3:DeleteBucket", del.MissingPermission)
	require.NotEmpty(t, res.Leftovers)
	// the result of the stored account is stored as last test of the storage
	main, err := h.GetRuntimeStorage(ctx, &pb.RuntimeStorageName{Name: "main"})
	require.NoError(t, err)
	require.False(t, main.LastTest.Ok)

	// invalid requests
	_, err = h.TestStorageConnection(ctx, &pb.TestStorageConnectionRequest{})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, err = h.TestStorageConnection(ctx, &pb.TestStorageConnectionRequest{Storage: &pb.RuntimeStorage{Name: "x", Address: fake.URL, Provider: "Minio"}})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, err = h.TestStorageConnection(ctx, &pb.TestStorageConnectionRequest{Name: "nope"})
	require.ErrorIs(t, err, dom.ErrNotFound)
	_, err = h.TestStorageConnection(ctx, &pb.TestStorageConnectionRequest{Name: "main", Options: &pb.StorageTestOptions{ProbeBucketPrefix: "Bad Prefix"}})
	require.ErrorIs(t, err, dom.ErrInvalidArg)
}

func TestRotateConnectionAccountTestsThenSwitches(t *testing.T) {
	h, creds, fake := connTestSetup(t)
	ctx := auth.ContextWithSession(t.Context(), &auth.Session{Username: "admin"})
	fake.AccessKeys["svc-v2"] = true
	def := &pb.RuntimeStorage{Name: "archive", Address: fake.URL, Provider: "Minio", DefaultRegion: "us-east-1"}
	_, err := h.CreateRuntimeStorage(ctx, &pb.CreateRuntimeStorageRequest{Storage: def, ConnectionAccount: &pb.ConnectionAccount{AccessKeyId: "tutti-svc", SecretAccessKey: "s1"}})
	require.NoError(t, err)

	// config storages are read-only
	_, err = h.RotateStorageConnectionAccount(ctx, &pb.RotateStorageConnectionAccountRequest{Name: "main", NewAccount: &pb.ConnectionAccount{AccessKeyId: "svc-v2", SecretAccessKey: "s2"}})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	// an unknown access key fails step 5: refused, the old account stays
	_, err = h.RotateStorageConnectionAccount(ctx, &pb.RotateStorageConnectionAccountRequest{Name: "archive", NewAccount: &pb.ConnectionAccount{AccessKeyId: "typo", SecretAccessKey: "s2"}})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Contains(t, err.Error(), "authentication")
	ci, _ := creds.ConnectionAccount("archive")
	require.Equal(t, "tutti-svc", ci.Credentials.AccessKeyID)
	// a permission-limited new account fails its step
	fake.Deny["PutObject"] = true
	_, err = h.RotateStorageConnectionAccount(ctx, &pb.RotateStorageConnectionAccountRequest{Name: "archive", NewAccount: &pb.ConnectionAccount{AccessKeyId: "svc-v2", SecretAccessKey: "s2"}})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	delete(fake.Deny, "PutObject")

	out, err := h.RotateStorageConnectionAccount(ctx, &pb.RotateStorageConnectionAccountRequest{Name: "archive", NewAccount: &pb.ConnectionAccount{AccessKeyId: "svc-v2", SecretAccessKey: "s2"}})
	require.NoError(t, err)
	require.Equal(t, "svc-v2", out.ConnectionAccount.AccessKeyId)
	require.Equal(t, "tutti-svc", out.ConnectionAccount.PreviousAccessKeyId)
	require.NotNil(t, out.ConnectionAccount.RotatedAt)
	require.True(t, out.ConnectionAccount.SafeToRevokePreviousAfter.AsTime().After(out.ConnectionAccount.RotatedAt.AsTime()))
	require.True(t, out.LastTest.Ok)
	require.Equal(t, "svc-v2", out.LastTest.AccessKeyId)
	c, mode, err := creds.ResolveS3Credentials("archive", "anyone")
	require.NoError(t, err)
	require.Equal(t, objstore.ModeConnection, mode)
	require.Equal(t, "s2", c.SecretAccessKey)

	// failing test + force: switched and recorded
	fake.AccessKeys["svc-v3"] = true
	fake.Deny["GetObject"] = true
	_, err = h.RotateStorageConnectionAccount(ctx, &pb.RotateStorageConnectionAccountRequest{Name: "archive", NewAccount: &pb.ConnectionAccount{AccessKeyId: "svc-v3", SecretAccessKey: "s3"}})
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	forced, err := h.RotateStorageConnectionAccount(ctx, &pb.RotateStorageConnectionAccountRequest{Name: "archive", Force: true, NewAccount: &pb.ConnectionAccount{AccessKeyId: "svc-v3", SecretAccessKey: "s3"}})
	require.NoError(t, err)
	require.Equal(t, "admin", forced.LastForcedSaveBy)
	require.False(t, forced.LastTest.Ok)
}
