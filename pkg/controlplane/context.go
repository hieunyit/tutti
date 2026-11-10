// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

// Package controlplane defines the repositories of the control-plane configuration (storages, users, keys, policies,
// routing, replication and failover policies, protection groups, console users) and their PostgreSQL implementation
// (package pg). See docs/tutti/control-plane-db.md.
package controlplane

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/hieunyit/tutti/pkg/dom"
)

// Actor identifies who performs a mutation; it is written to the audit log in the same transaction.
type Actor struct {
	// Name is the console user, API principal or component ("failover-controller", "tutti-migrate-pg").
	Name string
	// RequestID correlates the mutation with the API/proxy request log.
	RequestID string
}

type actorKey struct{}
type skipOutboxKey struct{}

// WithActor attaches the actor to the context.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

// ActorFrom returns the actor of the context, "system" if none.
func ActorFrom(ctx context.Context) Actor {
	if a, ok := ctx.Value(actorKey{}).(Actor); ok && a.Name != "" {
		return a
	}
	return Actor{Name: "system"}
}

// WithSkipOutbox makes mutations of the context not write outbox entries (used by the Redis to PostgreSQL migration,
// where Redis already holds the data).
func WithSkipOutbox(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipOutboxKey{}, true)
}

// SkipOutbox reports whether WithSkipOutbox was applied.
func SkipOutbox(ctx context.Context) bool {
	v, _ := ctx.Value(skipOutboxKey{}).(bool)
	return v
}

// ErrVersionConflict is returned by conditional updates when the stored version differs from the expected one.
var ErrVersionConflict = fmt.Errorf("%w: version conflict, the object was changed concurrently", dom.ErrFailedPrecondition)

// ErrReadOnly is returned by every mutation while the store is in read-only fallback mode.
var ErrReadOnly = fmt.Errorf("%w: control plane is read-only", dom.ErrFailedPrecondition)

// ErrCommitUncertain wraps the error of a failed COMMIT: the connection broke while the commit was in flight, so the
// transaction may or may not be applied. Callers that compensate a failed write must not roll back on it.
var ErrCommitUncertain = errors.New("controlplane: commit outcome unknown")

// IsConflict reports whether err is ErrVersionConflict.
func IsConflict(err error) bool { return errors.Is(err, ErrVersionConflict) }

// IsUnavailable reports whether err indicates that PostgreSQL is unreachable or timed out (rather than a domain error
// or a bug), in which case reads fall back to the Redis projection and writes return 503 Unavailable. It is an
// allow-list: only connection failures, timeouts, a closed pool/transaction and the PostgreSQL connection-exception,
// shutdown and resource-exhaustion SQLSTATEs qualify; everything else (constraint violations, programming errors,
// dom errors) is not "unavailable" and surfaces as the error it is.
func IsUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrCommitUncertain) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, pgx.ErrTxClosed) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	var ce *pgconn.ConnectError
	if errors.As(err, &ce) {
		return true
	}
	if pgconn.Timeout(err) {
		return true
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch {
		case strings.HasPrefix(pe.Code, "08"), // connection exception
			strings.HasPrefix(pe.Code, "57P"), // admin shutdown, crash shutdown, cannot connect now
			pe.Code == "53300",                // too many connections
			pe.Code == "57014",                // statement timeout (query_canceled)
			pe.Code == "25P03":                // idle_in_transaction_session_timeout
			return true
		}
		return false
	}
	// pgxpool reports a closed pool with a plain errors.New("closed pool")
	return strings.Contains(err.Error(), "closed pool")
}

// WrapWriteError translates a PostgreSQL write failure into dom.ErrUnavailable (503) while preserving domain errors
// and ErrReadOnly.
func WrapWriteError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrReadOnly) || !IsUnavailable(err) {
		return err
	}
	return fmt.Errorf("%w: control plane database unavailable: %w", dom.ErrUnavailable, err)
}
