// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

// Package pg is the PostgreSQL implementation of the control-plane repositories (pkg/controlplane).
//
// Every mutation runs in one transaction that also writes the audit row and the outbox entry, so a change, its audit
// trail and its propagation to Redis commit or fail together.
package pg

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

// querier is implemented by *pgxpool.Pool and pgx.Tx.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// runner executes statements either on the pool (each write in its own transaction) or inside a shared transaction.
type runner struct {
	pool     *pgxpool.Pool
	tx       pgx.Tx
	readOnly *atomic.Bool
}

func (r *runner) q() querier {
	if r.tx != nil {
		return r.tx
	}
	return r.pool
}

// write runs fn in a transaction (the shared one inside Store.Tx) and commits it.
func (r *runner) write(ctx context.Context, fn func(tx pgx.Tx) error) error {
	if r.readOnly != nil && r.readOnly.Load() {
		return controlplane.ErrReadOnly
	}
	if r.tx != nil {
		return fn(r.tx)
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("controlplane: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("controlplane: commit: %w", err)
	}
	return nil
}

// Options of Open.
type Options struct {
	ReadOnly bool
	// AutoMigrate applies the embedded migrations after connecting.
	AutoMigrate bool
}

// Store is the PostgreSQL control-plane store.
type Store struct {
	pool     *pgxpool.Pool
	readOnly atomic.Bool
	repos    controlplane.Repos
}

var _ controlplane.Store = (*Store)(nil)

// Open connects with the configuration (WithDefaults is applied) and pings the server.
func Open(ctx context.Context, cfg controlplane.Config) (*Store, error) {
	cfg = cfg.WithDefaults()
	if cfg.Postgres.DSN == "" {
		return nil, fmt.Errorf("controlplane: postgres dsn is empty")
	}
	pc, err := pgxpool.ParseConfig(cfg.Postgres.DSN)
	if err != nil {
		return nil, fmt.Errorf("controlplane: parse dsn: %w", err)
	}
	pc.MaxConns = cfg.Postgres.MaxConns
	pc.MinConns = cfg.Postgres.MinConns
	pc.MaxConnLifetime = cfg.Postgres.ConnMaxLifetime
	pc.MaxConnIdleTime = cfg.Postgres.ConnMaxIdleTime
	pc.ConnConfig.ConnectTimeout = cfg.Postgres.ConnectTimeout
	if pc.ConnConfig.RuntimeParams == nil {
		pc.ConnConfig.RuntimeParams = map[string]string{}
	}
	if d := cfg.Postgres.StatementTimeout; d > 0 {
		pc.ConnConfig.RuntimeParams["statement_timeout"] = fmt.Sprintf("%d", d.Milliseconds())
	}
	pc.ConnConfig.RuntimeParams["application_name"] = "tutti-controlplane"
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("controlplane: connect: %w", err)
	}
	pctx, cancel := context.WithTimeout(ctx, cfg.Postgres.ConnectTimeout)
	defer cancel()
	if err := pool.Ping(pctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("controlplane: ping: %w", err)
	}
	s := NewFromPool(pool)
	s.readOnly.Store(cfg.ReadOnly)
	if cfg.Postgres.AutoMigrate {
		if _, err := Migrate(ctx, pool); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return s, nil
}

// NewFromPool wraps an existing pool (tests, tools). The schema must be migrated.
func NewFromPool(pool *pgxpool.Pool) *Store {
	s := &Store{pool: pool}
	s.repos = newRepos(&runner{pool: pool, readOnly: &s.readOnly})
	return s
}

// Pool exposes the connection pool (publisher, tests).
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// SetReadOnly switches the read-only fallback mode at runtime: mutations fail with controlplane.ErrReadOnly.
func (s *Store) SetReadOnly(v bool) { s.readOnly.Store(v) }

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Repos returns the repositories; every call is its own transaction.
func (s *Store) Repos() controlplane.Repos { return s.repos }

// Ping checks the connection.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Tx runs fn in one transaction.
func (s *Store) Tx(ctx context.Context, fn func(controlplane.Repos) error) error {
	if s.readOnly.Load() {
		return controlplane.ErrReadOnly
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("controlplane: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := fn(newRepos(&runner{pool: s.pool, tx: tx, readOnly: &s.readOnly})); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("controlplane: commit: %w", err)
	}
	return nil
}

func newRepos(r *runner) controlplane.Repos {
	return controlplane.Repos{
		Storages:           newStorages(r),
		S3Identities:       newS3Identities(r),
		DynamicCredentials: &dynCreds{rt: r},
		ProxyAccounts:      newProxyAccounts(r),
		AccessPolicies:     newAccessPolicies(r),
		ConsoleUsers:       newConsoleUsers(r),
		Routing:            &routing{rt: r},
		Replications:       &replications{rt: r},
		FailoverPolicies:   newFailoverPolicies(r),
		FailoverEvents:     newFailoverEvents(r),
		ProtectionGroups:   newProtectionGroups(r),
		StorageTests:       newStorageTests(r),
		Audit:              &auditRepo{rt: r},
	}
}
