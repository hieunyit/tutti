// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

// Package pgtest provides a throwaway PostgreSQL for tests: a postgres:16 container started with testcontainers-go
// (skipped when Docker is not available) or the server in TUTTI_TEST_PG_DSN. Each test gets its own database.
package pgtest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	mu        sync.Mutex
	adminDSN  string
	container testcontainers.Container
	startErr  error
	started   bool
	dbCounter atomic.Int64
)

func start(ctx context.Context) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if started {
		return adminDSN, startErr
	}
	started = true
	if dsn := os.Getenv("TUTTI_TEST_PG_DSN"); dsn != "" {
		adminDSN = dsn
		return adminDSN, nil
	}
	defer func() {
		if r := recover(); r != nil {
			startErr = fmt.Errorf("docker unavailable: %v", r)
		}
	}()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16",
		ExposedPorts: []string{"5432/tcp"},
		Env:          map[string]string{"POSTGRES_PASSWORD": "test", "POSTGRES_USER": "test", "POSTGRES_DB": "postgres"},
		Cmd:          []string{"postgres", "-c", "fsync=off", "-c", "max_connections=200"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		startErr = err
		return "", err
	}
	container = c
	host, err := c.Host(ctx)
	if err != nil {
		startErr = err
		return "", err
	}
	port, err := c.MappedPort(ctx, "5432/tcp")
	if err != nil {
		startErr = err
		return "", err
	}
	adminDSN = fmt.Sprintf("postgres://test:test@%s:%s/postgres?sslmode=disable", host, port.Port())
	return adminDSN, nil
}

// Run is for TestMain: it runs the tests and terminates the container.
func Run(m *testing.M) int {
	code := m.Run()
	if container != nil {
		_ = container.Terminate(context.Background())
	}
	return code
}

// NewDB creates an empty database and returns its DSN. The test is skipped if no PostgreSQL is available.
func NewDB(t testing.TB) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dsn, err := start(ctx)
	if err != nil {
		t.Skipf("PostgreSQL is not available (set TUTTI_TEST_PG_DSN or start Docker): %v", err)
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("PostgreSQL is not reachable: %v", err)
	}
	defer admin.Close()
	name := fmt.Sprintf("t_%d_%d", os.Getpid(), dbCounter.Add(1))
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		a, err := pgxpool.New(c, dsn)
		if err != nil {
			return
		}
		defer a.Close()
		_, _ = a.Exec(c, `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`)
	})
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}
