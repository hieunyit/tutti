// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"time"

	"github.com/hieunyit/tutti/pkg/dom"
)

// MinReplicationSince is the earliest accepted replication cutoff. Anything older is almost certainly a client
// mistake (an unset timestamp is the Unix epoch) and equals a full copy anyway.
var MinReplicationSince = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

// NormalizeReplicationSince validates the optional cutoff of a replication. A nil cutoff is valid (copy everything).
// The cutoff must not be in the future and not earlier than year 2000. The result is UTC, truncated to the
// microsecond (the precision of the Postgres column) so that both backends round-trip the same value.
func NormalizeReplicationSince(since *time.Time, now time.Time) (*time.Time, error) {
	if since == nil {
		return nil, nil
	}
	t := since.UTC().Truncate(time.Microsecond)
	if t.After(now) {
		return nil, fmt.Errorf("%w: since must not be in the future", dom.ErrInvalidArg)
	}
	if t.Before(MinReplicationSince) {
		return nil, fmt.Errorf("%w: since must not be earlier than %s", dom.ErrInvalidArg, MinReplicationSince.Format("2006-01-02"))
	}
	return &t, nil
}
