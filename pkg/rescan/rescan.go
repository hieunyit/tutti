// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

// Package rescan starts periodic rescans of a replication source: a listing pass over the source bucket that copies
// the objects newer than the last successful scan which are missing at the destination. It is shared by the worker
// scheduler (due scans) and the API ("Sync now").
package rescan

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// Target is a replication eligible for a scan.
type Target struct {
	ID         entity.UniversalReplicationID
	Generation int64
	// Interval is the configured rescan interval (0 = off; a manual scan works regardless).
	Interval  time.Duration
	Since     *time.Time
	CreatedAt time.Time
}

// Starter takes ownership of a replication scan and enqueues its first task.
type Starter struct {
	Store *store.RescanStore
	Queue tasks.QueueService
	Now   func() time.Time
}

func (s *Starter) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Start begins a scan unless one is already running (started=false). The scan cutoff is derived from the stored
// watermark, see store.RescanCutoff.
func (s *Starter) Start(ctx context.Context, t Target) (started bool, runID string, err error) {
	now := s.now()
	st, err := s.Store.Get(ctx, t.ID.AsString(), t.Generation, now)
	if err != nil {
		return false, "", fmt.Errorf("rescan: unable to read state: %w", err)
	}
	if st.Running {
		return false, "", nil
	}
	cutoff := store.RescanCutoff(st, t.Since, t.CreatedAt, entity.RescanOverlap)
	runID = strconv.FormatInt(now.UnixMilli(), 10)
	ok, err := s.Store.Start(ctx, t.ID.AsString(), t.Generation, runID, now, cutoff)
	if err != nil {
		return false, "", fmt.Errorf("rescan: unable to start: %w", err)
	}
	if !ok {
		return false, "", nil
	}
	run := tasks.RescanRun{RunID: runID, Cutoff: cutoff}
	var task tasks.ReplicationTask
	if bucketID, isBucket := t.ID.AsBucketID(); isBucket {
		task = &tasks.RescanListObjectsPayload{Bucket: bucketID.FromBucket, RescanRun: run}
	} else {
		task = &tasks.RescanUserListBucketsPayload{RescanRun: run}
	}
	task.SetReplicationID(t.ID, t.Generation)
	if err := s.Queue.EnqueueTask(ctx, task); err != nil {
		_ = s.Store.Abort(context.WithoutCancel(ctx), t.ID.AsString(), t.Generation, runID, "unable to enqueue the scan: "+err.Error(), now)
		return false, "", fmt.Errorf("rescan: unable to enqueue first task: %w", err)
	}
	return true, runID, nil
}
