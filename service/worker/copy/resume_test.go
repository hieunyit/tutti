package copy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/features"
)

type memResumeStore struct {
	mu   sync.Mutex
	data map[string][]byte
	sets atomic.Int32
	dels atomic.Int32
}

func newMemResumeStore() *memResumeStore { return &memResumeStore{data: map[string][]byte{}} }

func (m *memResumeStore) Get(_ context.Context, k string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data[k], nil
}
func (m *memResumeStore) Set(_ context.Context, k string, v []byte, _ time.Duration) error {
	m.sets.Add(1)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[k] = append([]byte(nil), v...)
	return nil
}
func (m *memResumeStore) Delete(_ context.Context, k string) error {
	m.dels.Add(1)
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, k)
	return nil
}
func (m *memResumeStore) has() bool { m.mu.Lock(); defer m.mu.Unlock(); return len(m.data) > 0 }

const resumeKeyName = "p:mpu:{r:1}:test"

func resumeEnv(t *testing.T, threads int) (*copyEnv, *memResumeStore) {
	e := newCopyEnv(t, controlplane.SystemWorkerSettings{CopyEngine: "pipeline", MultipartThreads: threads, MultipartPartSizeMiB: 5, ResumeMultipart: true}, features.Config{})
	return e, newMemResumeStore()
}

func resumeCtx(ctx context.Context, st *memResumeStore) context.Context {
	return WithResume(WithKeyLocked(ctx), st, resumeKeyName)
}

// crashAtPart cancels the copy when the Nth part upload starts: parts before it are complete (1 uploader).
func crashAtPart(n int, cancel context.CancelFunc) func(int, *http.Request) *memFault {
	var once atomic.Bool
	return func(_ int, r *http.Request) *memFault {
		if r.Method == http.MethodPut && r.URL.Query().Get("partNumber") == strconv.Itoa(n) && once.CompareAndSwap(false, true) {
			cancel()
			return &memFault{Status: 500, Code: "InternalError"}
		}
		return nil
	}
}

func TestResumeContinuesAfterKill(t *testing.T) {
	e, st := resumeEnv(t, 1)
	data := patterned(6*testPart+777, 11) // 7 parts
	e.putSource("o", data, nil)

	ctx, cancel := context.WithCancel(t.Context())
	e.dst.Hook = crashAtPart(4, cancel)
	require.Error(t, e.copy(resumeCtx(ctx, st), "o"))
	cancel()
	ids := e.dst.OpenUploads()
	require.Len(t, ids, 1, "the interrupted upload is kept for the retry")
	require.True(t, st.has(), "the record survives")
	require.Len(t, e.dst.UploadParts(ids[0]), 3)

	e.dst.Hook = nil
	putsBefore, getsBefore := e.dst.partsPut.Load(), e.src.gets.Load()
	require.NoError(t, e.copy(resumeCtx(t.Context(), st), "o"))
	require.Equal(t, data, e.destBytes("o"))
	require.EqualValues(t, 4, e.dst.partsPut.Load()-putsBefore, "only parts 4..7 are uploaded again")
	require.Equal(t, []string{fmt.Sprintf("bytes=%d-%d", 3*testPart, len(data)-1)}, e.src.Ranges(), "the source is read from the first missing part with a ranged GET")
	require.EqualValues(t, 1, e.src.gets.Load()-getsBefore)
	require.Empty(t, e.dst.OpenUploads())
	require.False(t, st.has(), "the record is deleted on success")
}

func TestResumeSourceChangedRestartsFromZero(t *testing.T) {
	e, st := resumeEnv(t, 1)
	e.putSource("o", patterned(6*testPart, 11), nil)
	ctx, cancel := context.WithCancel(t.Context())
	e.dst.Hook = crashAtPart(4, cancel)
	require.Error(t, e.copy(resumeCtx(ctx, st), "o"))
	cancel()
	old := e.dst.OpenUploads()
	require.Len(t, old, 1)

	newData := patterned(6*testPart, 99)
	e.putSource("o", newData, nil)
	e.dst.Hook = nil
	require.NoError(t, e.copy(resumeCtx(t.Context(), st), "o"))
	require.Equal(t, newData, e.destBytes("o"))
	require.Empty(t, e.src.Ranges(), "restarted from zero: no ranged GET")
	require.Empty(t, e.dst.OpenUploads(), "the stale upload was aborted")
	_, stillThere := e.dst.uploads[old[0]]
	require.False(t, stillThere)
	require.False(t, st.has())
}

func TestResumeDestinationLostUpload(t *testing.T) {
	e, st := resumeEnv(t, 1)
	data := patterned(5*testPart, 12)
	e.putSource("o", data, nil)
	ctx, cancel := context.WithCancel(t.Context())
	e.dst.Hook = crashAtPart(3, cancel)
	require.Error(t, e.copy(resumeCtx(ctx, st), "o"))
	cancel()
	e.dst.DropUpload(e.dst.OpenUploads()[0])
	e.dst.Hook = nil
	require.NoError(t, e.copy(resumeCtx(t.Context(), st), "o"))
	require.Equal(t, data, e.destBytes("o"))
	require.Empty(t, e.src.Ranges())
	require.EqualValues(t, 5, e.dst.partsPut.Load()-2, "all five parts of the new upload")
}

func TestResumeRejectsBadDestinationParts(t *testing.T) {
	cases := map[string]func(e *copyEnv, id string){
		"part etag differs": func(e *copyEnv, id string) { e.dst.CorruptPart(id, 2, testPart) },
		"part size differs": func(e *copyEnv, id string) { e.dst.CorruptPart(id, 1, testPart-1) },
		"duplicate numbers": func(e *copyEnv, _ string) { e.dst.dupListing = true },
		"listing fails": func(e *copyEnv, _ string) {
			e.dst.Hook = func(_ int, r *http.Request) *memFault {
				if r.Method == http.MethodGet && r.URL.Query().Has("uploadId") {
					return &memFault{Status: 403, Code: "AccessDenied"}
				}
				return nil
			}
		},
	}
	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			e, st := resumeEnv(t, 1)
			data := patterned(5*testPart, 13)
			e.putSource("o", data, nil)
			ctx, cancel := context.WithCancel(t.Context())
			e.dst.Hook = crashAtPart(4, cancel)
			require.Error(t, e.copy(resumeCtx(ctx, st), "o"))
			cancel()
			old := e.dst.OpenUploads()[0]
			e.dst.Hook = nil
			corrupt(e, old)
			require.NoError(t, e.copy(resumeCtx(t.Context(), st), "o"))
			require.Equal(t, data, e.destBytes("o"))
			require.Empty(t, e.src.Ranges(), "nothing is reused after a mismatch")
			require.Empty(t, e.dst.OpenUploads(), "the untrusted upload was aborted")
			require.False(t, st.has())
		})
	}
}

func TestResumeFailureAfterAllPartsCompletesWithoutSource(t *testing.T) {
	e, st := resumeEnv(t, 2)
	data := patterned(4*testPart+5, 14)
	e.putSource("o", data, nil)
	e.dst.Hook = func(_ int, r *http.Request) *memFault {
		if r.Method == http.MethodPost && r.URL.Query().Has("uploadId") {
			return &memFault{Status: 503, Code: "SlowDown"}
		}
		return nil
	}
	require.Error(t, e.copy(resumeCtx(t.Context(), st), "o"))
	require.Len(t, e.dst.OpenUploads(), 1, "all parts uploaded, completion failed transiently: kept")
	e.dst.Hook = nil
	getsBefore := e.src.gets.Load()
	require.NoError(t, e.copy(resumeCtx(t.Context(), st), "o"))
	require.Equal(t, data, e.destBytes("o"))
	require.Zero(t, e.src.gets.Load()-getsBefore, "nothing left to read: the upload is just completed")
	require.False(t, st.has())
}

func TestResumeNonTransientErrorAbortsAndForgets(t *testing.T) {
	e, st := resumeEnv(t, 1)
	e.putSource("o", patterned(4*testPart, 15), nil)
	e.dst.Hook = func(_ int, r *http.Request) *memFault {
		if r.Method == http.MethodPut && r.URL.Query().Get("partNumber") == "2" {
			return &memFault{Status: 403, Code: "AccessDenied"}
		}
		return nil
	}
	require.Error(t, e.copy(resumeCtx(t.Context(), st), "o"))
	require.Empty(t, e.dst.OpenUploads(), "a client error is not worth resuming: aborted")
	require.False(t, st.has())
}

func TestResumeOffByDefault(t *testing.T) {
	e := newCopyEnv(t, controlplane.SystemWorkerSettings{CopyEngine: "pipeline", MultipartThreads: 1, MultipartPartSizeMiB: 5}, features.Config{})
	st := newMemResumeStore()
	data := patterned(4*testPart, 16)
	e.putSource("o", data, nil)
	ctx, cancel := context.WithCancel(t.Context())
	e.dst.Hook = crashAtPart(3, cancel)
	require.Error(t, e.copy(resumeCtx(ctx, st), "o"))
	cancel()
	require.Empty(t, e.dst.OpenUploads(), "without the switch a failed upload is aborted")
	require.Zero(t, st.sets.Load(), "and no record is written")
	e.dst.Hook = nil
	require.NoError(t, e.copy(resumeCtx(t.Context(), st), "o"))
	require.Equal(t, data, e.destBytes("o"))
	require.Empty(t, e.src.Ranges())
}

func TestResumeRecordWritesAreThrottled(t *testing.T) {
	e, st := resumeEnv(t, 2)
	e.putSource("o", patterned(12*testPart, 17), nil)
	require.NoError(t, e.copy(resumeCtx(t.Context(), st), "o"))
	require.LessOrEqual(t, st.sets.Load(), int32(3), "init plus at most a couple of throttled part writes, not one per part")
	require.GreaterOrEqual(t, st.dels.Load(), int32(1))
}

func TestResumeKeyFormat(t *testing.T) {
	k := ResumeKey("repl", 7, "a/b", "v1")
	require.Regexp(t, `^p:mpu:\{repl:7\}:[0-9a-f]{40}$`, k)
	require.NotEqual(t, k, ResumeKey("repl", 7, "a/b", "v2"))
	require.NotEqual(t, k, ResumeKey("repl", 8, "a/b", "v1"))
}

func TestKeepUploadOnError(t *testing.T) {
	require.False(t, keepUploadOnError(nil))
	require.True(t, keepUploadOnError(context.Canceled))
	require.True(t, keepUploadOnError(fmt.Errorf("x: %w", context.DeadlineExceeded)))
	require.True(t, keepUploadOnError(errors.New("connection reset by peer")))
	require.True(t, keepUploadOnError(minio.ErrorResponse{StatusCode: 503, Code: "SlowDown"}))
	require.True(t, keepUploadOnError(minio.ErrorResponse{StatusCode: 500}))
	require.False(t, keepUploadOnError(minio.ErrorResponse{StatusCode: 403, Code: "AccessDenied"}))
	require.False(t, keepUploadOnError(minio.ErrorResponse{StatusCode: 404, Code: "NoSuchUpload"}))
	require.False(t, keepUploadOnError(minio.ErrorResponse{StatusCode: 412, Code: "PreconditionFailed"}))
	require.False(t, keepUploadOnError(&sourceNotFoundError{err: errors.New("gone")}))
}

func TestResumeSweepKeepsTheResumableUpload(t *testing.T) {
	e, st := resumeEnv(t, 1)
	data := patterned(5*testPart, 18)
	e.putSource("o", data, nil)
	ctx, cancel := context.WithCancel(t.Context())
	e.dst.Hook = crashAtPart(4, cancel)
	require.Error(t, e.copy(resumeCtx(ctx, st), "o"))
	cancel()
	id := e.dst.OpenUploads()[0]
	e.dst.Age(id, time.Hour) // older than the sweep window: only the record protects it
	e.dst.Hook = nil
	require.NoError(t, e.copy(resumeCtx(t.Context(), st), "o"))
	require.Equal(t, data, e.destBytes("o"))
	require.NotEmpty(t, e.src.Ranges(), "the old upload was resumed, not swept")
}
