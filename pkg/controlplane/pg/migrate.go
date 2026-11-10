// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migrationLockID is the pg_advisory_xact_lock key that serializes concurrent migrators.
const migrationLockID int64 = 0x7475747469 // "tutti"

type migration struct {
	version  int
	name     string
	sql      string
	checksum string
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, err
	}
	var res []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		num, _, ok := strings.Cut(e.Name(), "_")
		v, err := strconv.Atoi(num)
		if !ok || err != nil {
			return nil, fmt.Errorf("migration file %q must be named <number>_<name>.sql", e.Name())
		}
		raw, err := migrationFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(raw)
		res = append(res, migration{version: v, name: strings.TrimSuffix(e.Name(), ".sql"), sql: string(raw), checksum: hex.EncodeToString(sum[:])})
	}
	sort.Slice(res, func(i, j int) bool { return res[i].version < res[j].version })
	for i := 1; i < len(res); i++ {
		if res[i].version == res[i-1].version {
			return nil, fmt.Errorf("duplicate migration version %d", res[i].version)
		}
	}
	return res, nil
}

// LatestVersion is the schema version this build expects.
func LatestVersion() (int, error) {
	ms, err := loadMigrations()
	if err != nil || len(ms) == 0 {
		return 0, err
	}
	return ms[len(ms)-1].version, nil
}

// Migrate applies all pending migrations, each in its own transaction, in version order. Safe to run concurrently
// from several processes (serialized by an advisory lock) and to repeat. It fails if an applied migration file changed
// or the database is newer than this build. It returns the versions applied by this call.
func Migrate(ctx context.Context, pool *pgxpool.Pool) ([]int, error) {
	ms, err := loadMigrations()
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version bigint PRIMARY KEY, name text NOT NULL, checksum text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}
	var applied []int
	for _, m := range ms {
		done, err := applyOne(ctx, pool, m)
		if err != nil {
			return applied, fmt.Errorf("migration %s: %w", m.name, err)
		}
		if done {
			applied = append(applied, m.version)
		}
	}
	// a database that is ahead of this build is a deployment error (rollback of the binary without the schema)
	var maxV int
	if err := pool.QueryRow(ctx, `SELECT COALESCE(max(version),0) FROM schema_migrations`).Scan(&maxV); err != nil {
		return applied, err
	}
	if len(ms) > 0 && maxV > ms[len(ms)-1].version {
		return applied, fmt.Errorf("database schema version %d is newer than this build (%d)", maxV, ms[len(ms)-1].version)
	}
	return applied, nil
}

func applyOne(ctx context.Context, pool *pgxpool.Pool, m migration) (bool, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, migrationLockID); err != nil {
		return false, err
	}
	var sum string
	err = tx.QueryRow(ctx, `SELECT checksum FROM schema_migrations WHERE version=$1`, m.version).Scan(&sum)
	switch {
	case err == nil:
		if sum != m.checksum {
			return false, fmt.Errorf("checksum mismatch: the applied migration %d was modified (applied %s, file %s)", m.version, sum, m.checksum)
		}
		return false, nil
	case err != pgx.ErrNoRows:
		return false, err
	}
	if _, err := tx.Exec(ctx, m.sql); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, name, checksum) VALUES ($1,$2,$3)`, m.version, m.name, m.checksum); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

// SchemaVersion returns the highest applied migration, 0 for an empty database.
func SchemaVersion(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var v int
	err := pool.QueryRow(ctx, `SELECT COALESCE(max(version),0) FROM schema_migrations`).Scan(&v)
	if err != nil {
		return 0, nil //nolint:nilerr // table missing: empty database
	}
	return v, nil
}
