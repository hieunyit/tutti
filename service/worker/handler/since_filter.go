// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
)

// maxScannedPerPage bounds how many source keys one listing execution walks when a since cutoff skips most of them,
// so a bucket with few recent objects is still listed in bounded slices (each ends with a checkpoint and a continuation).
const maxScannedPerPage = 20 * MaxListingPageSize

// SetSinceResolver sets how the worker reads the optional cutoff of a replication ("since"). Without a resolver
// nothing is filtered (the whole source is copied).
func (s *svc) SetSinceResolver(f func(ctx context.Context, id entity.UniversalReplicationID) (*time.Time, error)) {
	s.sinceFor = f
}

// replicationSince returns the cutoff of the replication or nil. A replication that no longer exists has no cutoff
// (its tasks are dropped as stale by the generation middleware); any other error is returned so that the task retries
// instead of copying more than the user asked for.
func (s *svc) replicationSince(ctx context.Context, id entity.UniversalReplicationID) (*time.Time, error) {
	if s.sinceFor == nil {
		return nil, nil
	}
	since, err := s.sinceFor(ctx, id)
	if err != nil {
		if errors.Is(err, dom.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to read replication since: %w", err)
	}
	return since, nil
}

// keepSince reports whether an object last modified at lastModified is copied by a replication with the given cutoff.
// A nil cutoff keeps everything; an object exactly at the cutoff is kept.
func keepSince(lastModified time.Time, since *time.Time) bool {
	return since == nil || !lastModified.Before(*since)
}
