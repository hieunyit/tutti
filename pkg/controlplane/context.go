// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

// Package controlplane defines the repositories of the control-plane configuration (storages, users, keys, policies,
// routing, replication and failover policies, protection groups, console users) and their PostgreSQL implementation
// (package pg). See docs/tutti/control-plane-db.md.
package controlplane

import (
	"context"
	"errors"
	"fmt"

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

// IsConflict reports whether err is ErrVersionConflict.
func IsConflict(err error) bool { return errors.Is(err, ErrVersionConflict) }
