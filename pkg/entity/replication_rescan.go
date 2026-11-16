// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"time"

	"github.com/hieunyit/tutti/pkg/dom"
)

// Allowed range of the periodic rescan interval of a replication.
const (
	MinRescanInterval = 15 * time.Minute
	MaxRescanInterval = 24 * time.Hour
	// RescanOverlap is how far before the start of the previous successful scan the next one looks, to absorb clock
	// skew between workers and the source and objects whose upload finished after the previous listing started.
	RescanOverlap = 10 * time.Minute
)

// NormalizeRescanInterval validates the optional rescan interval. 0 is valid and means off. Otherwise it must be
// between MinRescanInterval and MaxRescanInterval. The result is truncated to whole seconds (the stored precision).
func NormalizeRescanInterval(d time.Duration) (time.Duration, error) {
	if d == 0 {
		return 0, nil
	}
	if d < MinRescanInterval || d > MaxRescanInterval {
		return 0, fmt.Errorf("%w: rescan_interval must be 0 (off) or between %s and %s", dom.ErrInvalidArg, MinRescanInterval, MaxRescanInterval)
	}
	return d.Truncate(time.Second), nil
}

// RescanInterval returns the configured interval of the status (0 = off).
func (s *ReplicationStatus) RescanInterval() time.Duration {
	if s == nil || s.RescanSeconds <= 0 {
		return 0
	}
	return time.Duration(s.RescanSeconds) * time.Second
}
