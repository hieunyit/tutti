package copy

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/features"
)

func legacyEnv(t *testing.T) *copyEnv {
	return newCopyEnv(t, controlplane.SystemWorkerSettings{MultipartThreads: 1, MultipartPartSizeMiB: 5}, features.Config{})
}

// cancelOnPart cancels the copy context when the Nth part upload arrives and fails that request.
func cancelOnPart(n int, cancel context.CancelFunc) func(int, *http.Request) *memFault {
	return func(_ int, r *http.Request) *memFault {
		if r.Method == http.MethodPut && r.URL.Query().Get("partNumber") == strconv.Itoa(n) {
			cancel()
			return &memFault{Status: 500, Code: "InternalError"}
		}
		return nil
	}
}

func TestOrphanUploadAbortedAfterCancelledCopy(t *testing.T) {
	e := legacyEnv(t)
	e.putSource("big", patterned(12<<20, 1), nil)
	ctx, cancel := context.WithCancel(WithKeyLocked(t.Context()))
	defer cancel()
	e.dst.Hook = cancelOnPart(2, cancel)
	require.Error(t, e.copy(ctx, "big"))
	require.Empty(t, e.dst.OpenUploads(), "the incomplete upload must be aborted even though the copy context was cancelled")
	_, ok := e.dst.Get("bucket/big")
	require.False(t, ok)
}

func TestOrphanUploadNotTouchedWithoutKeyLock(t *testing.T) {
	e := legacyEnv(t)
	e.putSource("big", patterned(12<<20, 1), nil)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	e.dst.Hook = cancelOnPart(2, cancel)
	require.Error(t, e.copy(ctx, "big"))
	// without the lock the service cannot know that the leftover is not another worker's upload: no sweep
	require.Len(t, e.dst.OpenUploads(), 1)
}

func TestStaleUploadSweptAtStart(t *testing.T) {
	e := legacyEnv(t)
	data := patterned(12<<20, 2)
	e.putSource("big", data, nil)
	stale := e.dst.SeedUpload("bucket/big", nil, map[int][]byte{1: patterned(5<<20, 9)})
	fresh := e.dst.SeedUpload("bucket/big", nil, map[int][]byte{1: patterned(5<<20, 8)})
	other := e.dst.SeedUpload("bucket/big2", nil, nil)
	longer := e.dst.SeedUpload("bucket/big/sub", nil, nil)
	e.dst.Age(stale, 10*time.Minute)
	e.dst.Age(fresh, 30*time.Second) // may belong to a concurrent attempt that just started: kept
	e.dst.Age(other, time.Hour)
	e.dst.Age(longer, time.Hour)

	require.NoError(t, e.copy(WithKeyLocked(t.Context()), "big"))
	require.Equal(t, data, e.destBytes("big"))
	require.ElementsMatch(t, []string{fresh, other, longer}, e.dst.OpenUploads(), "only the stale upload of exactly this key is aborted")
}

func TestStaleUploadNotSweptWithoutKeyLock(t *testing.T) {
	e := legacyEnv(t)
	e.putSource("big", patterned(12<<20, 2), nil)
	stale := e.dst.SeedUpload("bucket/big", nil, nil)
	e.dst.Age(stale, time.Hour)
	require.NoError(t, e.copy(t.Context(), "big"))
	require.Equal(t, []string{stale}, e.dst.OpenUploads())
}

func TestSweepDisabledByFlag(t *testing.T) {
	SetOrphanSweep(false)
	defer SetOrphanSweep(true)
	e := legacyEnv(t)
	e.putSource("big", patterned(12<<20, 2), nil)
	stale := e.dst.SeedUpload("bucket/big", nil, nil)
	e.dst.Age(stale, time.Hour)
	require.NoError(t, e.copy(WithKeyLocked(t.Context()), "big"))
	require.Equal(t, []string{stale}, e.dst.OpenUploads())
	require.Zero(t, e.dst.CountRequests("GET /bucket?uploads"))
}

func TestSmallObjectDoesNotListUploads(t *testing.T) {
	e := legacyEnv(t)
	e.putSource("small", []byte("tiny"), nil)
	require.NoError(t, e.copy(WithKeyLocked(t.Context()), "small"))
	for _, r := range e.dst.Requests() {
		require.False(t, strings.Contains(r, "uploads"), r)
	}
}

func TestAbortUploadRetriesWithFreshContext(t *testing.T) {
	e := legacyEnv(t)
	id := e.dst.SeedUpload("bucket/k", nil, nil)
	var calls int
	e.dst.Hook = func(_ int, r *http.Request) *memFault {
		if r.Method == http.MethodDelete {
			calls++
			if calls < 3 {
				return &memFault{Status: 503, Code: "SlowDown"}
			}
		}
		return nil
	}
	abortRetryDelay = time.Millisecond
	cl := comprehensiveClient(t, e.dst.URL())
	require.NoError(t, abortUpload(coreOf(cl), "bucket", "k", id))
	require.Equal(t, 3, calls)
	require.Empty(t, e.dst.OpenUploads())
	// already gone counts as success
	require.NoError(t, abortUpload(coreOf(cl), "bucket", "k", id))
}
