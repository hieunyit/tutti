package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/migrate"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/controlplane/pgtest"
	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/store"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Run(m)) }

func runCLI(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestCLIEndToEnd(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	require.NoError(t, accesspolicy.NewStore(rdb).Set(ctx, accesspolicy.Policy{User: "alice", Permission: accesspolicy.ReadOnly, Buckets: []string{"a*"}}))
	require.NoError(t, failover.NewStore(rdb, nil).SetFailoverPolicy(ctx, failover.Policy{Source: "main", Enabled: true, Targets: []string{"b"}, After: time.Minute}))
	require.NoError(t, store.NewUserRoutingStore(rdb).SetOp(ctx, "alice", "b").Get())

	dsn := pgtest.NewDB(t)
	common := []string{"--redis-url", "redis://" + mr.Addr() + "/0", "--pg-dsn", dsn}
	with := func(cmd ...string) []string { return append(append([]string{}, common...), cmd...) }

	// migrate before the schema exists is refused with a hint
	code, _, stderr := runCLI(t, with("migrate")...)
	require.Equal(t, exitFailure, code)
	require.Contains(t, stderr, "schema")

	// a dry run works without touching PostgreSQL
	code, stdout, stderr := runCLI(t, "--redis-url", "redis://"+mr.Addr()+"/0", "--dry-run", "migrate")
	require.Equal(t, exitOK, code, stderr)
	require.Contains(t, stdout, "dry-run=true")
	require.Contains(t, stdout, "access_policies")

	code, stdout, stderr = runCLI(t, with("schema")...)
	require.Equal(t, exitOK, code, stderr)
	require.Contains(t, stdout, "applied migrations: [1]")

	// dry run against the empty schema, nothing written
	code, _, stderr = runCLI(t, with("--dry-run", "migrate")...)
	require.Equal(t, exitOK, code, stderr)
	code, _, _ = runCLI(t, with("verify")...)
	require.Equal(t, exitMismatch, code, "PostgreSQL is still empty")

	reportFile := filepath.Join(t.TempDir(), "report.json")
	code, stdout, stderr = runCLI(t, with("--out", reportFile, "migrate")...)
	require.Equal(t, exitOK, code, stderr+stdout)
	require.Contains(t, stdout, "result: OK")
	raw, err := os.ReadFile(reportFile)
	require.NoError(t, err)
	var rep migrate.Report
	require.NoError(t, json.Unmarshal(raw, &rep))
	require.True(t, rep.OK)
	require.Equal(t, 1, rep.Families[migrate.FamAccessPolicies].Created)

	// idempotent
	code, stdout, _ = runCLI(t, with("--format", "json", "migrate")...)
	require.Equal(t, exitOK, code)
	require.NoError(t, json.Unmarshal([]byte(stdout), &rep))
	require.Zero(t, rep.Families[migrate.FamAccessPolicies].Created+rep.Families[migrate.FamAccessPolicies].Updated)

	code, stdout, _ = runCLI(t, with("verify")...)
	require.Equal(t, exitOK, code)
	require.Contains(t, stdout, "result: OK")

	// drift in PostgreSQL is detected by --verify with a distinct exit code and a precise report
	s, err := pg.Open(ctx, controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: dsn}})
	require.NoError(t, err)
	defer s.Close()
	_, err = s.Repos().AccessPolicies.Put(ctx, controlplane.AccessPolicy{User: "alice", Permission: "read-write", Buckets: []string{"*"}})
	require.NoError(t, err)
	code, stdout, _ = runCLI(t, with("verify")...)
	require.Equal(t, exitMismatch, code)
	require.Contains(t, stdout, `mismatch: access_policies "alice" different`)

	// and drift in Redis too
	_, err = rdb.HDel(ctx, "tutti:failover-policies", "main").Result()
	require.NoError(t, err)
	code, stdout, _ = runCLI(t, with("--format", "json", "verify")...)
	require.Equal(t, exitMismatch, code)
	require.NoError(t, json.Unmarshal([]byte(stdout), &rep))
	require.False(t, rep.OK)
	require.Len(t, rep.Mismatches, 2)

	// republish rebuilds Redis from PostgreSQL and makes verify exit 0 again
	mr.FlushAll()
	code, stdout, stderr = runCLI(t, with("republish")...)
	require.Equal(t, exitOK, code, stderr+stdout)
	require.Contains(t, stdout, "result: OK")
	code, stdout, _ = runCLI(t, with("verify")...)
	require.Equal(t, exitOK, code, stdout)

	// usage errors
	code, _, _ = runCLI(t)
	require.Equal(t, exitFailure, code)
	code, _, stderr = runCLI(t, "--redis-url", "redis://"+mr.Addr(), "verify")
	require.Equal(t, exitFailure, code)
	require.Contains(t, stderr, "pg-dsn")
	code, _, _ = runCLI(t, with("frobnicate")...)
	require.Equal(t, exitFailure, code)
}
