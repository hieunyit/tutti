package pg_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/controlplane/pgtest"
)

func openRaw(t *testing.T) *pg.Store {
	t.Helper()
	s, err := pg.Open(context.Background(), controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: pgtest.NewDB(t)}})
	require.NoError(t, err)
	t.Cleanup(s.Close)
	return s
}

func TestMigrateUpFromEmpty(t *testing.T) {
	ctx := context.Background()
	s := openRaw(t)
	v, err := pg.SchemaVersion(ctx, s.Pool())
	require.NoError(t, err)
	require.Equal(t, 0, v)

	applied, err := pg.Migrate(ctx, s.Pool())
	require.NoError(t, err)
	latest, err := pg.LatestVersion()
	require.NoError(t, err)
	require.Equal(t, []int{1, 2}, applied)
	require.Equal(t, 2, latest)

	v, err = pg.SchemaVersion(ctx, s.Pool())
	require.NoError(t, err)
	require.Equal(t, latest, v)

	for _, table := range []string{"audit_log", "outbox", "storages", "s3_identities", "dynamic_credentials", "proxy_accounts", "access_policies",
		"console_users", "routes", "route_blocks", "replication_policies", "replication_status", "replication_generations",
		"failover_policies", "failover_events", "protection_groups", "storage_tests", "schema_migrations"} {
		var ok bool
		require.NoError(t, s.Pool().QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&ok))
		require.True(t, ok, table)
	}

	// idempotent
	applied, err = pg.Migrate(ctx, s.Pool())
	require.NoError(t, err)
	require.Empty(t, applied)

	// a database ahead of the binary is rejected
	_, err = s.Pool().Exec(ctx, `INSERT INTO schema_migrations (version, name, checksum) VALUES (999,'future','x')`)
	require.NoError(t, err)
	_, err = pg.Migrate(ctx, s.Pool())
	require.ErrorContains(t, err, "newer than this build")
	_, err = s.Pool().Exec(ctx, `DELETE FROM schema_migrations WHERE version=999`)
	require.NoError(t, err)

	// a modified applied migration is detected
	_, err = s.Pool().Exec(ctx, `UPDATE schema_migrations SET checksum='tampered' WHERE version=1`)
	require.NoError(t, err)
	_, err = pg.Migrate(ctx, s.Pool())
	require.ErrorContains(t, err, "checksum mismatch")
}

func TestMigrateConcurrent(t *testing.T) {
	ctx := context.Background()
	s := openRaw(t)
	var wg sync.WaitGroup
	errs := make([]error, 6)
	total := make([]int, 6)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, err := pg.Migrate(ctx, s.Pool())
			errs[i], total[i] = err, len(a)
		}()
	}
	wg.Wait()
	sum := 0
	for i := range errs {
		require.NoError(t, errs[i])
		sum += total[i]
	}
	require.Equal(t, 2, sum, "exactly one migrator applies each migration (versions 1 and 2)")
}
