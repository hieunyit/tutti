package objstore

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/controlplane/pgtest"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/s3"
)

type listCounts struct{ storages, identities, creds atomic.Int64 }

type countingStorages struct {
	controlplane.Storages
	n *atomic.Int64
}

func (c countingStorages) List(ctx context.Context) ([]controlplane.Versioned[controlplane.StorageRecord], error) {
	c.n.Add(1)
	return c.Storages.List(ctx)
}

type countingIdentities struct {
	controlplane.S3Identities
	n *atomic.Int64
}

func (c countingIdentities) List(ctx context.Context) ([]controlplane.Versioned[controlplane.S3Identity], error) {
	c.n.Add(1)
	return c.S3Identities.List(ctx)
}

type countingCreds struct {
	controlplane.DynamicCredentials
	n *atomic.Int64
}

func (c countingCreds) List(ctx context.Context) ([]controlplane.DynamicCredential, error) {
	c.n.Add(1)
	return c.DynamicCredentials.List(ctx)
}

type countingStore struct {
	controlplane.Store
	c *listCounts
}

func (s countingStore) Repos() controlplane.Repos {
	r := s.Store.Repos()
	r.Storages = countingStorages{r.Storages, &s.c.storages}
	r.S3Identities = countingIdentities{r.S3Identities, &s.c.identities}
	r.DynamicCredentials = countingCreds{r.DynamicCredentials, &s.c.creds}
	return r
}

// In postgres mode the 2 s polls of every worker must not SELECT and decode every row while nothing changed.
func TestPostgresPollsSkipUnchangedTables(t *testing.T) {
	ctx := context.Background()
	store, err := pg.Open(ctx, controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: pgtest.NewDB(t), AutoMigrate: true}})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	counts := &listCounts{}
	conf := &Config{Main: "main",
		DynamicCredentials: DynamicCredentialsConfig{Enabled: true, MasterPassword: "super-secret-master-password", PollInterval: time.Hour},
		Storages: map[string]Storage{"main": {CommonConfig: CommonConfig{Type: dom.S3},
			S3: &s3.Storage{StorageAddress: s3.StorageAddress{Address: "http://127.0.0.1:9000", Provider: "Minio"},
				Credentials: map[string]s3.CredentialsV4{"u": {AccessKeyID: "A", SecretAccessKey: "S"}}}}}}
	require.NoError(t, conf.Validate())
	svc, err := NewCredsSvc(ctx, conf, rdb, WithManagedStorages(rdb), WithControlPlane(countingStore{store, counts}))
	require.NoError(t, err)
	impl := svc

	poll := func() {
		require.NoError(t, impl.syncManaged(ctx))
		require.NoError(t, impl.syncIdentities(ctx))
		require.NoError(t, impl.sync(ctx))
	}
	poll()
	base := [3]int64{counts.storages.Load(), counts.identities.Load(), counts.creds.Load()}
	for i := 0; i < 5; i++ {
		poll()
	}
	require.Equal(t, base, [3]int64{counts.storages.Load(), counts.identities.Load(), counts.creds.Load()}, "unchanged tables are not listed again")

	// a change by another process is picked up
	_, err = store.Repos().S3Identities.Create(ctx, controlplane.S3Identity{Name: "newuser", CreatedAt: time.Now()})
	require.NoError(t, err)
	require.NoError(t, impl.syncIdentities(ctx))
	require.Equal(t, base[1]+1, counts.identities.Load())
	require.True(t, impl.knownUser("newuser"))
}
