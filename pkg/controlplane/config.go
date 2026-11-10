// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"fmt"
	"time"

	"github.com/hieunyit/tutti/pkg/dom"
)

// Backend values of Config.Backend.
const (
	BackendRedis    = "redis"
	BackendPostgres = "postgres"
)

// Config is the `controlPlane` section of the worker and proxy configuration (docs/tutti/control-plane-db.md).
// It is not wired into the services yet.
type Config struct {
	// Backend selects the source of truth of control-plane config: "redis" (default, today's behaviour) or "postgres".
	Backend string `yaml:"backend,omitempty" mapstructure:"backend"`
	// ReadOnly makes the PostgreSQL backend reject every mutation with dom.ErrFailedPrecondition (read-only fallback
	// while PostgreSQL is being repaired or restored). Reads keep working; proxies keep serving from the Redis cache.
	ReadOnly bool `yaml:"readOnly,omitempty" mapstructure:"readOnly"`
	// Postgres is used when Backend is "postgres".
	Postgres PostgresConfig `yaml:"postgres,omitempty" mapstructure:"postgres"`
	// Outbox configures the publisher that propagates committed changes to Redis.
	Outbox OutboxConfig `yaml:"outbox,omitempty" mapstructure:"outbox"`
}

// PostgresConfig is the connection pool configuration.
type PostgresConfig struct {
	// DSN is a libpq URL or key/value string, e.g. postgres://user:pass@host:5432/tutti?sslmode=verify-full.
	DSN string `yaml:"dsn,omitempty" mapstructure:"dsn"`
	// MaxConns is the pool size (default 10).
	MaxConns int32 `yaml:"maxConns,omitempty" mapstructure:"maxConns"`
	// MinConns is the number of connections kept open (default 0).
	MinConns int32 `yaml:"minConns,omitempty" mapstructure:"minConns"`
	// ConnMaxLifetime recycles connections (default 30m).
	ConnMaxLifetime time.Duration `yaml:"connMaxLifetime,omitempty" mapstructure:"connMaxLifetime"`
	// ConnMaxIdleTime closes idle connections (default 5m).
	ConnMaxIdleTime time.Duration `yaml:"connMaxIdleTime,omitempty" mapstructure:"connMaxIdleTime"`
	// ConnectTimeout bounds the initial connection and ping (default 5s).
	ConnectTimeout time.Duration `yaml:"connectTimeout,omitempty" mapstructure:"connectTimeout"`
	// StatementTimeout is applied as a server-side statement_timeout to every connection (default 10s, 0 = server default).
	StatementTimeout time.Duration `yaml:"statementTimeout,omitempty" mapstructure:"statementTimeout"`
	// AutoMigrate applies the embedded SQL migrations on start (default false: run `tutti-migrate-pg migrate-schema`).
	AutoMigrate bool `yaml:"autoMigrate,omitempty" mapstructure:"autoMigrate"`
}

// OutboxConfig configures the outbox publisher.
type OutboxConfig struct {
	// PollInterval is how often unpublished changes are looked for (default 500ms).
	PollInterval time.Duration `yaml:"pollInterval,omitempty" mapstructure:"pollInterval"`
	// BatchSize is the max number of entries per cycle (default 200).
	BatchSize int `yaml:"batchSize,omitempty" mapstructure:"batchSize"`
	// RetryBackoff is the first retry delay, doubled up to MaxBackoff (defaults 1s, 1m).
	RetryBackoff time.Duration `yaml:"retryBackoff,omitempty" mapstructure:"retryBackoff"`
	MaxBackoff   time.Duration `yaml:"maxBackoff,omitempty" mapstructure:"maxBackoff"`
	// Retention is how long published entries are kept before being purged (default 24h).
	Retention time.Duration `yaml:"retention,omitempty" mapstructure:"retention"`
}

// WithDefaults fills zero values.
func (c Config) WithDefaults() Config {
	if c.Backend == "" {
		c.Backend = BackendRedis
	}
	p := &c.Postgres
	if p.MaxConns <= 0 {
		p.MaxConns = 10
	}
	if p.ConnMaxLifetime <= 0 {
		p.ConnMaxLifetime = 30 * time.Minute
	}
	if p.ConnMaxIdleTime <= 0 {
		p.ConnMaxIdleTime = 5 * time.Minute
	}
	if p.ConnectTimeout <= 0 {
		p.ConnectTimeout = 5 * time.Second
	}
	if p.StatementTimeout == 0 {
		p.StatementTimeout = 10 * time.Second
	}
	o := &c.Outbox
	if o.PollInterval <= 0 {
		o.PollInterval = 500 * time.Millisecond
	}
	if o.BatchSize <= 0 {
		o.BatchSize = 200
	}
	if o.RetryBackoff <= 0 {
		o.RetryBackoff = time.Second
	}
	if o.MaxBackoff <= 0 {
		o.MaxBackoff = time.Minute
	}
	if o.Retention <= 0 {
		o.Retention = 24 * time.Hour
	}
	return c
}

// Validate checks the configuration (after WithDefaults).
func (c Config) Validate() error {
	switch c.Backend {
	case "", BackendRedis:
		return nil
	case BackendPostgres:
		if c.Postgres.DSN == "" {
			return fmt.Errorf("%w: controlPlane.postgres.dsn is required when controlPlane.backend is %q", dom.ErrInvalidArg, BackendPostgres)
		}
		if c.Postgres.MinConns > c.Postgres.MaxConns && c.Postgres.MaxConns > 0 {
			return fmt.Errorf("%w: controlPlane.postgres.minConns must not exceed maxConns", dom.ErrInvalidArg)
		}
		return nil
	default:
		return fmt.Errorf("%w: controlPlane.backend must be %q or %q, got %q", dom.ErrInvalidArg, BackendRedis, BackendPostgres, c.Backend)
	}
}
