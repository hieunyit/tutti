// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
)

type testCommon struct {
	objstore.Common
	files map[string][]byte
}

func (m *testCommon) GetObject(ctx context.Context, bucket string, name string, opts ...objstore.CommonObjectOption) (io.Reader, error) {
	data, ok := m.files[bucket+"/"+name]
	if !ok {
		return nil, dom.ErrNotFound
	}
	return bytes.NewReader(data), nil
}

type testClients struct {
	objstore.Clients
	storages map[string]*testCommon
}

func (m *testClients) AsCommon(ctx context.Context, storage string, user string) (objstore.Common, error) {
	c, ok := m.storages[storage]
	if !ok {
		return nil, dom.ErrNotFound
	}
	return c, nil
}

// Destructive diff fix on unfenced live writes must be rejected to prevent
// deleting newly created/replicated objects on replica (stale report TOCTOU hazard).
func TestDiffFix_StaleReportMustNotDeleteNewReplicatedObject(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	ctx := t.Context()
	diffSvc := NewDiffSvc(rc, nil, &tasks.QueueServiceMock{})

	locSource := entity.DiffLocation{Storage: "primary", Bucket: "b1"}
	locDest := entity.DiffLocation{Storage: "replica", Bucket: "b2"}
	diffID := entity.DiffID{
		Locations: []entity.DiffLocation{locSource, locDest},
	}
	diffFixID := entity.NewDiffFixID(diffID, locDest.Storage)

	// Case 1: Key K exists only in destination according to stale diff report
	setStore := store.NewDiffSetStore(rc)
	diffSetID := entity.NewNameDiffSetID(diffID, "stale-key-k")
	entry := entity.NewDiffSetEntry(locDest, false)
	require.NoError(t, setStore.Add(ctx, diffSetID, entry, 2))

	// CollectObjectsToFix must reject destructive deletion with ErrFailedPrecondition and SkipRetry
	err := diffSvc.CollectObjectsToFix(ctx, diffFixID, locSource.Storage, false)
	require.Error(t, err)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition, "destructive diff fix must require fenced writes")
	require.ErrorIs(t, err, asynq.SkipRetry, "failed destructive diff fix must not retry indefinitely")

	// Case 2: Even if remove set is directly enqueued, RemoveDiffObjects must also fail closed
	removeStore := store.NewDiffFixRemoveSetStore(rc)
	_, err = removeStore.Add(ctx, diffFixID, "stale-key-k")
	require.NoError(t, err)

	err = diffSvc.RemoveDiffObjects(ctx, diffFixID, locDest, "user1", false)
	require.Error(t, err)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.ErrorIs(t, err, asynq.SkipRetry)
}

// F-6: Bounded memory collection in CollectObjectsToFix flushes copy sets to Redis without in-memory accumulation.
func TestDiffFix_BoundedMemoryCollectsCopySet(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	ctx := t.Context()
	diffSvc := NewDiffSvc(rc, nil, &tasks.QueueServiceMock{})

	locSource := entity.DiffLocation{Storage: "primary", Bucket: "b1"}
	locDest := entity.DiffLocation{Storage: "replica", Bucket: "b2"}
	diffID := entity.DiffID{
		Locations: []entity.DiffLocation{locSource, locDest},
	}
	diffFixID := entity.NewDiffFixID(diffID, locDest.Storage)

	setStore := store.NewDiffSetStore(rc)

	// Add key1 only in source (needs copy)
	require.NoError(t, setStore.Add(ctx, entity.NewNameDiffSetID(diffID, "obj-to-copy-1"), entity.NewDiffSetEntry(locSource, false), 2))
	// Add key2 in both source and dest with differing size (needs copy, non-versioned)
	require.NoError(t, setStore.Add(ctx, entity.NewSizeDiffSetID(diffID, "obj-modified", 10), entity.NewDiffSetEntry(locSource, false), 2))
	require.NoError(t, setStore.Add(ctx, entity.NewSizeDiffSetID(diffID, "obj-modified", 20), entity.NewDiffSetEntry(locDest, false), 2))

	err := diffSvc.CollectObjectsToFix(ctx, diffFixID, locSource.Storage, false)
	require.NoError(t, err)

	fixCopyStore := store.NewDiffFixCopySetStore(rc)
	copied, err := fixCopyStore.Get(ctx, diffFixID)
	require.NoError(t, err)

	copiedNames := make(map[string]bool)
	for _, c := range copied {
		copiedNames[c.Name] = true
	}
	require.True(t, copiedNames["obj-to-copy-1"])
	require.True(t, copiedNames["obj-modified"])
	require.Equal(t, 2, len(copied))
}

// F-5: Multipart ETag reconciliation distinguishes inconclusive ETag mismatch from genuine content inconsistency.
func TestDiff_MultipartReconciliation(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	ctx := t.Context()

	locSource := entity.DiffLocation{Storage: "primary", Bucket: "b1"}
	locDest := entity.DiffLocation{Storage: "replica", Bucket: "b2"}
	diffID := entity.DiffID{
		Locations: []entity.DiffLocation{locSource, locDest},
	}

	contentIdentical := []byte("identical-file-content-spanning-multiple-megabytes")

	clients := &testClients{
		storages: map[string]*testCommon{
			"primary": {files: map[string][]byte{"b1/obj-multipart": contentIdentical}},
			"replica": {files: map[string][]byte{"b2/obj-multipart": contentIdentical}},
		},
	}

	qMock := &tasks.QueueServiceMock{
		Queues: map[string]int{
			tasks.DiffQueue(diffID): 0,
		},
	}

	diffSvc := NewDiffSvc(rc, clients, qMock)
	settings := entity.NewDiffSettings("alice", false, false, false)
	require.NoError(t, diffSvc.RegisterDiff(ctx, diffID, settings))

	setStore := store.NewDiffSetStore(rc)
	size := uint64(len(contentIdentical))

	// Primary has multipart ETag (e.g. ends with -2)
	idPrimary := entity.NewEtagDiffSetID(diffID, "obj-multipart", size, "d41d8cd98f00b204e9800998ecf8427e-2")
	require.NoError(t, setStore.Add(ctx, idPrimary, entity.NewDiffSetEntry(locSource, false), 2))

	// Replica has singlepart MD5 ETag
	idReplica := entity.NewEtagDiffSetID(diffID, "obj-multipart", size, "098f6bcd4621d373cade4e832627b4f6")
	require.NoError(t, setStore.Add(ctx, idReplica, entity.NewDiffSetEntry(locDest, false), 2))

	// Before status call: setStore has entries with differing ETags
	tokens, err := store.DiffIDToTokensConverter(diffID)
	require.NoError(t, err)
	has, err := setStore.HasIDs(ctx, tokens...)
	require.NoError(t, err)
	require.True(t, has, "setStore initially holds entries due to differing ETags")

	// Calling GetDiffStatus triggers ReconcileInconclusiveMultipart
	status, err := diffSvc.GetDiffStatus(ctx, diffID)
	require.NoError(t, err)
	require.True(t, status.Check.Queue.Ready)
	require.True(t, status.Check.Consistent, "identical multipart content must be reconciled as consistent")

	// Verify report entries are clean (no false diff)
	entriesPage, err := diffSvc.GetDiffReportEntries(ctx, diffID, 0, 50)
	require.NoError(t, err)
	require.Empty(t, entriesPage.Entries, "reconciled multipart object must not appear as inconsistent in diff report")

	// Now verify that if content genuinely differs, it remains reported as inconsistent:
	diffID2 := entity.DiffID{
		Locations: []entity.DiffLocation{
			{Storage: "primary", Bucket: "b3"},
			{Storage: "replica", Bucket: "b4"},
		},
	}
	clients.storages["primary"].files["b3/obj-diff-content"] = []byte("content-A")
	clients.storages["replica"].files["b4/obj-diff-content"] = []byte("content-B")

	require.NoError(t, diffSvc.RegisterDiff(ctx, diffID2, settings))
	idDiff1 := entity.NewEtagDiffSetID(diffID2, "obj-diff-content", 9, "hash1-2")
	idDiff2 := entity.NewEtagDiffSetID(diffID2, "obj-diff-content", 9, "hash2")
	require.NoError(t, setStore.Add(ctx, idDiff1, entity.NewDiffSetEntry(entity.DiffLocation{Storage: "primary", Bucket: "b3"}, false), 2))
	require.NoError(t, setStore.Add(ctx, idDiff2, entity.NewDiffSetEntry(entity.DiffLocation{Storage: "replica", Bucket: "b4"}, false), 2))
	qMock.Queues[tasks.DiffQueue(diffID2)] = 0

	status2, err := diffSvc.GetDiffStatus(ctx, diffID2)
	require.NoError(t, err)
	require.False(t, status2.Check.Consistent, "genuinely differing content must remain inconsistent")
}

func TestDiff_MultipartReconciliation_RestartInvalidatesCache(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	ctx := t.Context()
	locSource := entity.DiffLocation{Storage: "primary", Bucket: "b1"}
	locDest := entity.DiffLocation{Storage: "replica", Bucket: "b2"}
	diffID := entity.DiffID{Locations: []entity.DiffLocation{locSource, locDest}}

	contentV1 := []byte("v1-content-data")
	clients := &testClients{
		storages: map[string]*testCommon{
			"primary": {files: map[string][]byte{"b1/obj-mp": contentV1}},
			"replica": {files: map[string][]byte{"b2/obj-mp": contentV1}},
		},
	}
	qMock := &tasks.QueueServiceMock{
		Queues: map[string]int{tasks.DiffQueue(diffID): 0},
	}

	diffSvc := NewDiffSvc(rc, clients, qMock)
	settings := entity.NewDiffSettings("alice", false, false, false)
	require.NoError(t, diffSvc.StartDiff(ctx, diffID, settings))

	setStore := store.NewDiffSetStore(rc)
	sizeV1 := uint64(len(contentV1))
	id1 := entity.NewEtagDiffSetID(diffID, "obj-mp", sizeV1, "etag-v1-1")
	id2 := entity.NewEtagDiffSetID(diffID, "obj-mp", sizeV1, "etag-v1-single")
	require.NoError(t, setStore.Add(ctx, id1, entity.NewDiffSetEntry(locSource, false), 2))
	require.NoError(t, setStore.Add(ctx, id2, entity.NewDiffSetEntry(locDest, false), 2))

	status, err := diffSvc.GetDiffStatus(ctx, diffID)
	require.NoError(t, err)
	require.True(t, status.Check.Consistent, "v1 should reconcile as consistent")

	// Update both storages to contentV2
	contentV2 := []byte("v2-new-content-longer-data")
	clients.storages["primary"].files["b1/obj-mp"] = contentV2
	clients.storages["replica"].files["b2/obj-mp"] = contentV2

	// Restart diff
	require.NoError(t, diffSvc.RestartDiff(ctx, diffID))
	qMock.Queues[tasks.DiffQueue(diffID)] = 0

	// Re-add new diff discrepancies for contentV2
	sizeV2 := uint64(len(contentV2))
	id3 := entity.NewEtagDiffSetID(diffID, "obj-mp", sizeV2, "etag-v2-1")
	id4 := entity.NewEtagDiffSetID(diffID, "obj-mp", sizeV2, "etag-v2-single")
	require.NoError(t, setStore.Add(ctx, id3, entity.NewDiffSetEntry(locSource, false), 2))
	require.NoError(t, setStore.Add(ctx, id4, entity.NewDiffSetEntry(locDest, false), 2))

	statusAfterRestart, err := diffSvc.GetDiffStatus(ctx, diffID)
	require.NoError(t, err)
	require.True(t, statusAfterRestart.Check.Consistent, "after restart, new generation must reconcile freshly and not report false diff")
}

func TestDiffFix_Lifecycle_ArchivedQueueDoesNotHang(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	ctx := t.Context()
	locSource := entity.DiffLocation{Storage: "primary", Bucket: "b1"}
	locDest := entity.DiffLocation{Storage: "replica", Bucket: "b2"}
	diffID := entity.DiffID{Locations: []entity.DiffLocation{locSource, locDest}}

	clients := &testClients{
		storages: map[string]*testCommon{
			"primary": {files: map[string][]byte{"b1/obj": []byte("src")}},
			"replica": {files: map[string][]byte{"b2/obj": []byte("dst")}},
		},
	}
	qMock := &tasks.QueueServiceMock{
		Queues: map[string]int{
			tasks.DiffQueue(diffID):    0,
			tasks.DiffFixQueue(diffID): 0, // 0 unprocessed (archived task)
		},
	}

	diffSvc := NewDiffSvc(rc, clients, qMock)
	settings := entity.NewDiffSettings("alice", false, false, false)
	require.NoError(t, diffSvc.StartDiff(ctx, diffID, settings))

	setStore := store.NewDiffSetStore(rc)
	idDiff := entity.NewEtagDiffSetID(diffID, "obj", 3, "etag-diff")
	require.NoError(t, setStore.Add(ctx, idDiff, entity.NewDiffSetEntry(locSource, false), 2))

	status, err := diffSvc.GetDiffStatus(ctx, diffID)
	require.NoError(t, err)
	require.False(t, status.Check.Consistent)
	require.NotNil(t, status.FixQueue)
	require.True(t, status.FixQueue.Ready, "fix queue with 0 active tasks must be ready, not hanging")

	// Calling FixDiff again when previous fix is ready should purge previous queue and succeed
	require.NoError(t, diffSvc.FixDiff(ctx, diffID, locSource, dom.S3))
}

