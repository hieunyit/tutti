// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

// Command tutti-migrate-pg copies the Tutti control-plane data from Redis to PostgreSQL and verifies the result.
//
//	tutti-migrate-pg [flags] schema     apply the SQL migrations to PostgreSQL
//	tutti-migrate-pg [flags] migrate    copy Redis -> PostgreSQL (idempotent; --dry-run only reports)
//	tutti-migrate-pg [flags] verify     compare Redis and PostgreSQL; exit code 2 on any difference
//	tutti-migrate-pg [flags] republish  rebuild Redis projection from PostgreSQL via the outbox
//
// See docs/tutti/control-plane-db.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/migrate"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
)

const (
	exitOK       = 0
	exitFailure  = 1
	exitMismatch = 2
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("tutti-migrate-pg", flag.ContinueOnError)
	fs.SetOutput(stderr)
	redisURL := fs.String("redis-url", os.Getenv("TUTTI_REDIS_URL"), "config Redis (redis[s]://[user:pass@]host:port/db): the redis.configDB of the services [env TUTTI_REDIS_URL]")
	metaURL := fs.String("redis-meta-url", os.Getenv("TUTTI_REDIS_META_URL"), "Redis that holds tutti:dynamic_creds (the metaDB role); defaults to --redis-url [env TUTTI_REDIS_META_URL]")
	dsn := fs.String("pg-dsn", os.Getenv("TUTTI_PG_DSN"), "PostgreSQL DSN [env TUTTI_PG_DSN]")
	dry := fs.Bool("dry-run", false, "migrate: compute and print the report without writing to PostgreSQL")
	force := fs.Bool("force", false, "republish: overwrite Redis entries that differ from PostgreSQL (refused otherwise: they may be newer, e.g. after restoring PostgreSQL from an old backup)")
	prune := fs.Bool("prune", false, "republish: also remove Redis entries that have no PostgreSQL row (otherwise republish only adds and overwrites)")
	drainTimeout := fs.Duration("drain-timeout", 5*time.Minute, "republish: how long to wait for the outbox to drain")
	format := fs.String("format", "text", "report format: text or json")
	out := fs.String("out", "", "also write the JSON report to this file")
	timeout := fs.Duration("timeout", 30*time.Minute, "overall timeout")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: tutti-migrate-pg [flags] schema|migrate|verify|republish")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitFailure
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return exitFailure
	}
	cmd := fs.Arg(0)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	needPG := !(cmd == "migrate" && *dry)
	var store *pg.Store
	if *dsn != "" {
		var err error
		store, err = pg.Open(ctx, controlplane.Config{Backend: controlplane.BackendPostgres, Postgres: controlplane.PostgresConfig{DSN: *dsn, MaxConns: 4}})
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitFailure
		}
		defer store.Close()
	} else if needPG {
		fmt.Fprintln(stderr, "error: --pg-dsn (or TUTTI_PG_DSN) is required")
		return exitFailure
	}

	switch cmd {
	case "schema":
		applied, err := pg.Migrate(ctx, store.Pool())
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitFailure
		}
		fmt.Fprintf(stdout, "schema up to date; applied migrations: %v\n", applied)
		return exitOK
	case "migrate", "verify", "republish":
	default:
		fs.Usage()
		return exitFailure
	}

	if *redisURL == "" {
		fmt.Fprintln(stderr, "error: --redis-url (or TUTTI_REDIS_URL) is required")
		return exitFailure
	}
	src, closeRedis, err := openSources(*redisURL, *metaURL)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitFailure
	}
	defer closeRedis()

	var rep *migrate.Report
	if cmd == "republish" {
		rep, err = migrate.Republish(ctx, store, src, migrate.RepublishOptions{DryRun: *dry, Force: *force, Prune: *prune, DrainTimeout: *drainTimeout})
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return exitFailure
		}
	} else {
		data, err := migrate.ReadRedis(ctx, src)
		if err != nil {
			fmt.Fprintln(stderr, "error: reading Redis:", err)
			return exitFailure
		}
		switch cmd {
		case "migrate":
			var target *migrate.Data
			if store != nil {
				if v, err := pg.SchemaVersion(ctx, store.Pool()); err != nil || v == 0 {
					if *dry {
						fmt.Fprintln(stderr, "note: PostgreSQL schema is not applied yet; reporting against an empty target")
					} else {
						fmt.Fprintln(stderr, "error: PostgreSQL schema is not applied; run `tutti-migrate-pg schema` first")
						return exitFailure
					}
				} else if target, err = migrate.ReadPG(ctx, store.Repos()); err != nil {
					fmt.Fprintln(stderr, "error: reading PostgreSQL:", err)
					return exitFailure
				}
			}
			var cs controlplane.Store // keep a nil *pg.Store out of the interface
			if store != nil {
				cs = store
			}
			rep = migrate.Import(ctx, cs, data, migrate.ImportOptions{DryRun: *dry, Target: target})
		case "verify":
			target, err := migrate.ReadPG(ctx, store.Repos())
			if err != nil {
				fmt.Fprintln(stderr, "error: reading PostgreSQL:", err)
				return exitFailure
			}
			rep = migrate.Verify(data, target)
		}
	}

	if *out != "" {
		if err := os.WriteFile(*out, rep.JSON(), 0o600); err != nil {
			fmt.Fprintln(stderr, "error: writing report:", err)
			return exitFailure
		}
	}
	if *format == "json" {
		fmt.Fprintln(stdout, string(rep.JSON()))
	} else {
		fmt.Fprint(stdout, rep.Text())
	}
	switch {
	case rep.OK:
		return exitOK
	case len(rep.Errors) > 0:
		return exitFailure
	default:
		return exitMismatch
	}
}

func openSources(configURL, metaURL string) (migrate.Sources, func(), error) {
	open := func(u string) (*redis.Client, error) {
		opt, err := redis.ParseURL(u)
		if err != nil {
			return nil, fmt.Errorf("invalid redis url: %w", err)
		}
		return redis.NewClient(opt), nil
	}
	cfg, err := open(configURL)
	if err != nil {
		return migrate.Sources{}, nil, err
	}
	src := migrate.Sources{Config: cfg, Meta: cfg}
	closers := []func(){func() { _ = cfg.Close() }}
	if metaURL != "" && metaURL != configURL {
		meta, err := open(metaURL)
		if err != nil {
			_ = cfg.Close()
			return migrate.Sources{}, nil, err
		}
		src.Meta = meta
		closers = append(closers, func() { _ = meta.Close() })
	}
	return src, func() {
		for _, c := range closers {
			c()
		}
	}, nil
}
