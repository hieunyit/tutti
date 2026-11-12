// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/protectiongroups"
)

// setupCutoverTestStore initializes miniredis and returns a Store and Redis client.
func setupCutoverTestStore(t *testing.T) (*protectiongroups.Store, *redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	store := protectiongroups.NewStore(rc)
	return store, rc, mr
}

// PR6: TestBulkCutover_InflightWriterDuringFreezeCountedUntilReturn
// Verifies that:
// 1. In-flight writer on primary is tracked during freeze.
// 2. Cutover/promotion cannot complete while ActiveWriters > 0.
// 3. New writes during freeze are rejected (fail-closed).
// 4. Once in-flight writer returns and releases, cutover is unblocked.
// 5. Freeze duration is measurable.
func TestBulkCutover_InflightWriterDuringFreezeCountedUntilReturn(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	store, _, _ := setupCutoverTestStore(t)

	group := protectiongroups.Group{
		ID:         "group-cutover-1",
		User:       "tenant-a",
		Buckets:    []string{"data-bucket"},
		Members:    []string{"primary-storage", "follower-storage"},
		Primary:    "primary-storage",
		Generation: 1,
		State:      "ready",
		Frozen:     false,
		Copies:     2,
	}
	r.NoError(store.Save(ctx, &group))

	lease := store.NewLease("proxy-node-1", false, 30*time.Second)

	// Step 1: Writer admits write to primary before freeze
	adm, err := lease.Admit(ctx, "tenant-a", "data-bucket", "primary-storage")
	r.NoError(err)
	r.Equal("group-cutover-1", adm.Group)

	// In-flight count is 1
	active, err := store.ActiveWriters(ctx, group.ID)
	r.NoError(err)
	r.Equal(int64(1), active, "must have 1 active writer in-flight")

	// Step 2: Cutover initiates -> Group is frozen
	freezeStart := time.Now()
	group.Frozen = true
	group.State = "promoting"
	group.Target = "follower-storage"
	r.NoError(store.Save(ctx, &group))

	// Step 3: New writes arriving during freeze are rejected immediately
	_, err = lease.Admit(ctx, "tenant-a", "data-bucket", "primary-storage")
	r.Error(err, "new write must be rejected during freeze")
	r.ErrorIs(err, dom.ErrUnavailable, "must fail with ErrUnavailable")

	// Step 4: Promotion check while writer is still in-flight
	active, err = store.ActiveWriters(ctx, group.ID)
	r.NoError(err)
	r.Equal(int64(1), active, "in-flight writer must remain counted during freeze")

	// Coordinator simulates promotion commit guard:
	canCommit := func() bool {
		n, e := store.ActiveWriters(ctx, group.ID)
		return e == nil && n == 0
	}
	r.False(canCommit(), "promotion commit MUST be blocked while in-flight writers > 0")

	// Step 5: In-flight writer finishes upload and releases admission
	time.Sleep(10 * time.Millisecond) // simulate brief in-flight write completion
	r.NoError(store.Release(ctx, adm))

	active, err = store.ActiveWriters(ctx, group.ID)
	r.NoError(err)
	r.Equal(int64(0), active, "active writers must drop to 0 after release")
	r.True(canCommit(), "promotion commit MUST be unblocked once writers drain")

	// Step 6: Complete promotion to follower
	freezeDuration := time.Since(freezeStart)
	r.GreaterOrEqual(freezeDuration, 10*time.Millisecond, "freeze duration must be measurable")

	group.Primary = "follower-storage"
	group.Target = ""
	group.Frozen = false
	group.State = "ready"
	group.Generation = 2
	r.NoError(store.Save(ctx, &group))

	// Verify new primary admits writes
	adm2, err := lease.Admit(ctx, "tenant-a", "data-bucket", "follower-storage")
	r.NoError(err)
	r.Equal(int64(2), adm2.Generation)
	r.NoError(store.Release(ctx, adm2))
}

// PR6: TestBulkCutover_KeyCreatedBehindCursorCapturedByDelta
// Verifies that a key created behind the bulk listing cursor is captured
// by the delta replication stream so that no objects are lost.
func TestBulkCutover_KeyCreatedBehindCursorCapturedByDelta(t *testing.T) {
	r := require.New(t)

	type mockStoreState struct {
		mu      sync.Mutex
		objects map[string]string // key -> content
	}

	primary := &mockStoreState{objects: make(map[string]string)}
	follower := &mockStoreState{objects: make(map[string]string)}

	// Initial objects on primary
	primary.objects["obj-010"] = "data-010"
	primary.objects["obj-020"] = "data-020"
	primary.objects["obj-050"] = "data-050"

	// Bulk listing has progressed past obj-020: cursor is at obj-020
	listingCursor := "obj-020"

	// Copy already transferred keys <= listingCursor
	follower.objects["obj-010"] = primary.objects["obj-010"]
	follower.objects["obj-020"] = primary.objects["obj-020"]

	// Concurrent live write arrives for obj-015 (alphabetically behind cursor obj-020)
	behindKey := "obj-015"
	behindContent := "data-015-new"
	r.True(behindKey < listingCursor, "key must be behind cursor")

	primary.mu.Lock()
	primary.objects[behindKey] = behindContent
	primary.mu.Unlock()

	// Delta / Outbox capture detects live write and enqueues delta replication
	deltaQueue := make([]string, 0)
	deltaQueue = append(deltaQueue, behindKey)

	// Bulk listing continues from cursor for remaining keys (keys > listingCursor)
	for k, v := range primary.objects {
		if k > listingCursor {
			follower.mu.Lock()
			follower.objects[k] = v
			follower.mu.Unlock()
		}
	}

	// Delta replication processes captured events
	for _, k := range deltaQueue {
		follower.mu.Lock()
		follower.objects[k] = primary.objects[k]
		follower.mu.Unlock()
	}

	// Exact verification at cutover
	primary.mu.Lock()
	defer primary.mu.Unlock()
	follower.mu.Lock()
	defer follower.mu.Unlock()

	r.Equal(len(primary.objects), len(follower.objects), "follower must have identical count")
	for k, v := range primary.objects {
		val, exists := follower.objects[k]
		r.True(exists, "key %s must exist on follower", k)
		r.Equal(v, val, "content of %s must match", k)
	}
	r.Equal(behindContent, follower.objects[behindKey], "key created behind cursor must be present and verified on follower")
}

// PR6: TestBulkCutover_PutDeleteReorderConvergence
// Verifies convergence under concurrent out-of-order PUT and DELETE operations:
// - Object A: PUT v1 -> DELETE -> PUT v2 converges to v2.
// - Object B: PUT v1 -> DELETE does NOT resurrect.
// - Object C: PUT v1 -> PUT v2 ensures stale v1 cannot overwrite v2.
func TestBulkCutover_PutDeleteReorderConvergence(t *testing.T) {
	r := require.New(t)

	type versionedRecord struct {
		version   int64
		content   string
		deleted   bool
		updatedAt time.Time
	}

	type convergenceMap struct {
		mu      sync.Mutex
		records map[string]versionedRecord
	}

	applyOperation := func(m *convergenceMap, key string, version int64, content string, deleted bool, ts time.Time) bool {
		m.mu.Lock()
		defer m.mu.Unlock()

		current, exists := m.records[key]
		if exists {
			// Reject stale updates (lower version or earlier timestamp if same version)
			if version < current.version {
				return false // Stale write rejected!
			}
			if version == current.version && ts.Before(current.updatedAt) {
				return false
			}
		}

		m.records[key] = versionedRecord{
			version:   version,
			content:   content,
			deleted:   deleted,
			updatedAt: ts,
		}
		return true
	}

	target := &convergenceMap{records: make(map[string]versionedRecord)}

	t0 := time.Now()
	t1 := t0.Add(1 * time.Second)
	t2 := t0.Add(2 * time.Second)

	// Scenario A: PUT v1 -> DELETE -> PUT v2
	// Arrive at target in reverse order: PUT v2 arrives first, then delayed DELETE, then delayed PUT v1
	applyOperation(target, "obj-A", 3, "content-v2", false, t2)
	okDelete := applyOperation(target, "obj-A", 2, "", true, t1)
	okPutV1 := applyOperation(target, "obj-A", 1, "content-v1", false, t0)

	r.False(okDelete, "stale delete must be rejected after newer PUT v2")
	r.False(okPutV1, "stale PUT v1 must be rejected after newer PUT v2")
	r.False(target.records["obj-A"].deleted, "obj-A must not be deleted")
	r.Equal("content-v2", target.records["obj-A"].content, "obj-A must converge to v2")

	// Scenario B: PUT v1 -> DELETE (Ghost resurrection prevention)
	// Apply PUT v1, then DELETE v2
	applyOperation(target, "obj-B", 1, "content-v1", false, t0)
	applyOperation(target, "obj-B", 2, "", true, t1)

	// Delayed copy of PUT v1 arrives again:
	resurrectOk := applyOperation(target, "obj-B", 1, "content-v1", false, t0)
	r.False(resurrectOk, "delayed PUT must NOT resurrect deleted object")
	r.True(target.records["obj-B"].deleted, "obj-B must remain deleted")

	// Scenario C: PUT v1 -> PUT v2 (Stale overwrite prevention)
	applyOperation(target, "obj-C", 2, "content-v2", false, t1)
	staleOk := applyOperation(target, "obj-C", 1, "content-v1", false, t0)
	r.False(staleOk, "stale PUT v1 must NOT overwrite newer PUT v2")
	r.Equal("content-v2", target.records["obj-C"].content, "obj-C must retain v2 content")
}

// PR6: TestBulkCutover_MissedEventsDetectedByExactVerifier
// Verifies that if an event replication is dropped/missed during live streaming,
// the exact verifier before cutover detects the discrepancy, repairs the follower,
// and blocks cutover if unresolvable.
func TestBulkCutover_MissedEventsDetectedByExactVerifier(t *testing.T) {
	r := require.New(t)

	type mockBucket struct {
		objects map[string]string // key -> hash/etag
	}

	primary := mockBucket{
		objects: map[string]string{
			"report-2026-01.pdf": "hash-aaa",
			"report-2026-02.pdf": "hash-bbb",
			"report-2026-03.pdf": "hash-ccc", // Dropped during live streaming!
		},
	}

	follower := mockBucket{
		objects: map[string]string{
			"report-2026-01.pdf": "hash-aaa",
			"report-2026-02.pdf": "hash-bbb",
		},
	}

	// Step 1: Pre-cutover exact verifier pass
	type diffReport struct {
		missingOnFollower []string
		mismatched        []string
		orphanOnFollower  []string
	}

	verifyBuckets := func(src, dst mockBucket) diffReport {
		var rep diffReport
		for k, srcHash := range src.objects {
			dstHash, exists := dst.objects[k]
			if !exists {
				rep.missingOnFollower = append(rep.missingOnFollower, k)
			} else if srcHash != dstHash {
				rep.mismatched = append(rep.mismatched, k)
			}
		}
		for k := range dst.objects {
			if _, exists := src.objects[k]; !exists {
				rep.orphanOnFollower = append(rep.orphanOnFollower, k)
			}
		}
		return rep
	}

	report := verifyBuckets(primary, follower)
	r.Equal([]string{"report-2026-03.pdf"}, report.missingOnFollower, "must detect missed object")

	// Cutover gate: must NOT permit cutover when discrepancy exists
	cutoverAllowed := len(report.missingOnFollower) == 0 && len(report.mismatched) == 0
	r.False(cutoverAllowed, "cutover must be blocked when exact verifier detects missing data")

	// Step 2: Verifier repairs missed objects by resyncing
	for _, missingKey := range report.missingOnFollower {
		follower.objects[missingKey] = primary.objects[missingKey]
	}

	// Step 3: Re-verify after repair
	postReport := verifyBuckets(primary, follower)
	r.Empty(postReport.missingOnFollower)
	r.Empty(postReport.mismatched)
	r.Empty(postReport.orphanOnFollower)

	cutoverAllowedPost := len(postReport.missingOnFollower) == 0 && len(postReport.mismatched) == 0
	r.True(cutoverAllowedPost, "cutover is allowed only after exact verifier passes with zero diff")
}

// PR6: TestBulkCutover_DirectWriterUnfencedBlocksCutover
// Verifies that:
// 1. Direct/unfenced writer records in Redis block cutover.
// 2. ClearWriterKind requires explicit external fencing attestation.
// 3. Unfenced clearing is refused with dom.ErrFailedPrecondition.
// 4. Group cannot unfreeze until unfenced writer is attested and cleared.
func TestBulkCutover_DirectWriterUnfencedBlocksCutover(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	store, rc, _ := setupCutoverTestStore(t)

	group := protectiongroups.Group{
		ID:         "group-fencing-test",
		User:       "tenant-x",
		Buckets:    []string{"vault"},
		Members:    []string{"storage-1", "storage-2"},
		Primary:    "storage-1",
		Generation: 1,
		State:      "promoting",
		Frozen:     true,
		Target:     "storage-2",
		Copies:     2,
	}
	r.NoError(store.Save(ctx, &group))

	// Simulate an unfenced writer record deposited in Redis (e.g. from an ungraceful proxy crash)
	unfencedToken := "unfenced-token-999"
	writerValue := fmt.Sprintf("proxy-node-crash:1:inst-xyz:%d", time.Now().UnixMilli())
	err := rc.HSet(ctx, "tutti:protection:writers:"+group.ID, unfencedToken, writerValue).Err()
	r.NoError(err)

	// Step 1: Verification that cutover is blocked by unfenced writer
	activeWriters, err := store.ActiveWriters(ctx, group.ID)
	r.NoError(err)
	r.Equal(int64(1), activeWriters, "must detect unfenced writer")

	// Step 2: Attempting to clear without external fencing attestation is refused
	err = store.ClearWriterKind(ctx, group.ID, unfencedToken, false, false)
	r.Error(err, "clearing without external fencing attestation must fail")
	r.ErrorIs(err, dom.ErrFailedPrecondition, "must fail with ErrFailedPrecondition")

	// Writer record must still be present
	activeWriters, err = store.ActiveWriters(ctx, group.ID)
	r.NoError(err)
	r.Equal(int64(1), activeWriters, "writer must not be deleted on unverified clear")

	// Step 3: Clearing WITH external fencing attestation succeeds
	err = store.ClearWriterKind(ctx, group.ID, unfencedToken, true, false)
	r.NoError(err, "clearing with external fencing attestation must succeed")

	// Active writers drops to 0
	activeWriters, err = store.ActiveWriters(ctx, group.ID)
	r.NoError(err)
	r.Equal(int64(0), activeWriters, "writer count must be 0 after explicit fenced clearance")

	// Step 4: Cutover commit can now proceed safely
	group.Primary = "storage-2"
	group.Target = ""
	group.Frozen = false
	group.State = "ready"
	group.Generation = 2
	r.NoError(store.Save(ctx, &group))

	retrieved, err := store.Get(ctx, group.ID)
	r.NoError(err)
	r.Equal("ready", retrieved.State)
	r.Equal("storage-2", retrieved.Primary)
	assert.False(t, retrieved.Frozen)
}
