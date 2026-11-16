package copy

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/features"
	"github.com/hieunyit/tutti/pkg/systemsettings"
)

const testPart = 5 << 20

func pipelineEnv(t *testing.T, threads int, feat features.Config) *copyEnv {
	return newCopyEnv(t, controlplane.SystemWorkerSettings{CopyEngine: "pipeline", MultipartThreads: threads, MultipartPartSizeMiB: 5}, feat)
}

func TestPipelinePartSizePolicy(t *testing.T) {
	require.EqualValues(t, 16<<20, pipelinePartSize(1900<<20, 16<<20))
	require.EqualValues(t, 16<<20, pipelinePartSize(0, 0))
	for _, size := range []int64{1 << 30, 100 << 30, 1 << 40, 5 << 40} {
		for _, cfg := range []uint64{0, 5 << 20, 16 << 20, 512 << 20} {
			ps := pipelinePartSize(size, cfg)
			require.LessOrEqual(t, (size+ps-1)/ps, int64(9000), "size %d cfg %d", size, cfg)
			require.Zero(t, ps%(1<<20))
			require.LessOrEqual(t, ps, int64(5<<30))
		}
	}
	// the legacy path raises an explicit part size only when needed
	require.EqualValues(t, 0, legacyPartSize(1<<30, 0))
	require.EqualValues(t, 5<<20, legacyPartSize(1<<30, 5<<20))
	require.Greater(t, legacyPartSize(500<<30, 5<<20), uint64(5<<20))
	require.LessOrEqual(t, (int64(500<<30)+int64(legacyPartSize(500<<30, 5<<20))-1)/int64(legacyPartSize(500<<30, 5<<20)), int64(9000))
}

func TestPipelineCopiesByteIdentical(t *testing.T) {
	sizes := map[string]int{
		"zero":         0,
		"one":          1,
		"under-part":   testPart - 1,
		"exact-part":   testPart,     // exactly one part: single PUT
		"part-plus-1":  testPart + 1, // two parts, second tiny
		"two-exact":    2 * testPart,
		"five-and-bit": 4*testPart + 12345,
	}
	for _, threads := range []int{1, 2, 4} {
		for name, size := range sizes {
			t.Run(fmt.Sprintf("k%d/%s", threads, name), func(t *testing.T) {
				e := pipelineEnv(t, threads, features.Config{})
				data := patterned(size, byte(size))
				e.putSource("o", data, nil)
				require.NoError(t, e.copy(t.Context(), "o"))
				require.True(t, bytes.Equal(data, e.destBytes("o")))
				require.Empty(t, e.dst.OpenUploads())
				multipart := e.dst.CountRequests("POST /bucket/o?uploads") > 0
				require.Equal(t, size > testPart, multipart, "single PUT up to one part, multipart above")
				if multipart {
					require.EqualValues(t, (size+testPart-1)/testPart, e.dst.partsPut.Load())
					require.EqualValues(t, 1, e.src.gets.Load(), "one sequential source stream, no GET per part")
					require.Zero(t, e.src.rangedGets.Load())
					require.Zero(t, e.src.CountRequests("HEAD")-1, "exactly the stat HEAD, none per part")
				}
			})
		}
	}
}

func TestPipelineMatchesLegacyMetadata(t *testing.T) {
	h := http.Header{
		"Content-Type":        {"application/x-backup"},
		"Content-Encoding":    {"gzip"},
		"Cache-Control":       {"private, max-age=7"},
		"Content-Disposition": {"attachment; filename=a.bak"},
		"Content-Language":    {"vi"},
		"X-Amz-Meta-Owner":    {"alice"},
		"X-Amz-Meta-Tier":     {"cold"},
	}
	data := patterned(3*testPart+99, 7)
	run := func(engine string) *memObj {
		e := newCopyEnv(t, controlplane.SystemWorkerSettings{CopyEngine: engine, MultipartThreads: 2, MultipartPartSizeMiB: 5}, features.Config{Tagging: true})
		e.putSource("o", data, h)
		e.src.objs["bucket/o"].tagging = `<Tag><Key>k</Key><Value>v</Value></Tag>`
		e.src.objs["bucket/o"].header.Set("X-Amz-Tagging-Count", "1")
		require.NoError(t, e.copy(t.Context(), "o"))
		o, ok := e.dst.Get("bucket/o")
		require.True(t, ok)
		return o
	}
	legacy, pipe := run("legacy"), run("pipeline")
	require.Equal(t, legacy.data, pipe.data)
	require.Equal(t, legacy.header, pipe.header, "content type, encoding, disposition, language, cache control, user metadata and tags must be identical")
	require.Equal(t, "alice", pipe.header.Get("X-Amz-Meta-Owner"))
	require.Equal(t, "k=v", pipe.header.Get("X-Amz-Tagging"))
	require.Equal(t, "gzip", pipe.header.Get("Content-Encoding"))
}

func TestPipelineCancelAbortsPromptlyAndAborts(t *testing.T) {
	e := pipelineEnv(t, 2, features.Config{})
	e.putSource("o", patterned(8*testPart, 3), nil)
	e.src.StreamBps = 20 << 20
	ctx, cancel := context.WithCancel(t.Context())
	var started atomic.Bool
	e.dst.Hook = func(_ int, r *http.Request) *memFault {
		if r.Method == http.MethodPut && r.URL.Query().Has("partNumber") && started.CompareAndSwap(false, true) {
			go func() { time.Sleep(50 * time.Millisecond); cancel() }()
		}
		return nil
	}
	t0 := time.Now()
	err := e.copy(ctx, "o")
	require.Error(t, err)
	require.Less(t, time.Since(t0), 3*time.Second, "cancellation must stop the copy promptly")
	require.Empty(t, e.dst.OpenUploads(), "aborted with a fresh context although the copy context is cancelled (no key lock needed: the upload is ours)")
	_, ok := e.dst.Get("bucket/o")
	require.False(t, ok)
	require.Zero(t, partBuffers.outstanding.Load(), "all buffers returned to the pool")
}

func TestPipelinePartFailureAbortsUpload(t *testing.T) {
	e := pipelineEnv(t, 2, features.Config{})
	e.putSource("o", patterned(6*testPart, 3), nil)
	e.dst.Hook = func(_ int, r *http.Request) *memFault {
		if r.Method == http.MethodPut && r.URL.Query().Get("partNumber") == "3" {
			return &memFault{Status: 403, Code: "AccessDenied"}
		}
		return nil
	}
	err := e.copy(t.Context(), "o")
	require.ErrorContains(t, err, "part 3")
	require.Empty(t, e.dst.OpenUploads())
	require.Zero(t, partBuffers.outstanding.Load())
}

func TestPipelineSourceDropContinuesWithRange(t *testing.T) {
	e := pipelineEnv(t, 2, features.Config{})
	data := patterned(5*testPart+7, 4)
	e.putSource("o", data, nil)
	var dropped atomic.Bool
	e.src.Hook = func(_ int, r *http.Request) *memFault {
		if r.Method == http.MethodGet && dropped.CompareAndSwap(false, true) {
			return &memFault{Drop: true, DropAfter: 2*testPart + 1000}
		}
		return nil
	}
	require.NoError(t, e.copy(t.Context(), "o"))
	require.Equal(t, data, e.destBytes("o"))
	require.EqualValues(t, 2, e.src.gets.Load())
	require.EqualValues(t, 1, e.src.rangedGets.Load(), "the broken stream is continued at the current offset")
}

func TestPipelineSourceKeepsDroppingFails(t *testing.T) {
	e := pipelineEnv(t, 2, features.Config{})
	e.putSource("o", patterned(5*testPart, 4), nil)
	e.src.Hook = func(_ int, r *http.Request) *memFault {
		if r.Method == http.MethodGet {
			return &memFault{Drop: true, DropAfter: 100}
		}
		return nil
	}
	require.Error(t, e.copy(t.Context(), "o"))
	require.Empty(t, e.dst.OpenUploads())
}

func TestPipelineSourceGoneIsNotFound(t *testing.T) {
	e := pipelineEnv(t, 2, features.Config{})
	e.putSource("o", patterned(3*testPart, 4), nil)
	e.src.Hook = func(_ int, r *http.Request) *memFault {
		if r.Method == http.MethodGet {
			return &memFault{Status: 404, Code: "NoSuchKey"}
		}
		return nil
	}
	err := e.copy(t.Context(), "o")
	require.ErrorIs(t, err, errNotFoundForTest)
	require.Empty(t, e.dst.OpenUploads())
}

func TestPipelineSourceChangedFails(t *testing.T) {
	e := pipelineEnv(t, 2, features.Config{})
	e.putSource("o", patterned(3*testPart, 4), nil)
	e.src.Hook = func(_ int, r *http.Request) *memFault {
		if r.Method == http.MethodGet {
			return &memFault{Status: 412, Code: "PreconditionFailed"}
		}
		return nil
	}
	require.Error(t, e.copy(t.Context(), "o"))
	require.Empty(t, e.dst.OpenUploads())
	_, ok := e.dst.Get("bucket/o")
	require.False(t, ok)
}

func TestPipelineMemoryBoundedAndBudgetReleased(t *testing.T) {
	for _, threads := range []int{1, 2, 4} {
		e := pipelineEnv(t, threads, features.Config{})
		e.putSource("o", patterned(12*testPart, 5), nil)
		partBuffers.peak.Store(0)
		require.NoError(t, e.copy(t.Context(), "o"))
		require.LessOrEqual(t, partBuffers.peak.Load(), int64(threads+1)*testPart, "threads=%d", threads)
		require.Zero(t, partBuffers.outstanding.Load())
		rel, err := systemsettings.AcquireBufferBudget(t.Context(), 256<<20)
		require.NoError(t, err)
		rel()
	}
}

func TestPipelinePartialPartFitsReservedBuffer(t *testing.T) {
	for _, size := range []int{testPart + 1, 2*testPart + 1, 4*testPart + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			e := pipelineEnv(t, 2, features.Config{})
			e.putSource("o", patterned(size, 4), nil)
			partBuffers.peak.Store(0)
			// Keep uploaded parts alive while the producer allocates the last buffer.
			e.dst.Hook = func(_ int, req *http.Request) *memFault {
				if req.Method == http.MethodPut && req.URL.Query().Has("partNumber") {
					time.Sleep(50 * time.Millisecond)
				}
				return nil
			}
			require.NoError(t, e.copy(t.Context(), "o"))
			require.LessOrEqual(t, partBuffers.peak.Load(), pipelineBuffer(int64(size), testPart, 2))
			require.Zero(t, partBuffers.outstanding.Load())
		})
	}
}

func TestPipelineReportsProgressPerPart(t *testing.T) {
	e := pipelineEnv(t, 2, features.Config{})
	size := 6*testPart + 5
	e.putSource("o", patterned(size, 6), nil)
	var last atomic.Int64
	var calls atomic.Int32
	ctx := WithProgress(t.Context(), func(done int64) {
		calls.Add(1)
		require.GreaterOrEqual(t, done, int64(0))
		require.LessOrEqual(t, done, int64(size))
		for {
			l := last.Load()
			if done <= l || last.CompareAndSwap(l, done) {
				break
			}
		}
	})
	require.NoError(t, e.copy(ctx, "o"))
	require.EqualValues(t, size, last.Load())
	require.GreaterOrEqual(t, calls.Load(), int32(7))
}

func TestPipelineEngineSwitch(t *testing.T) {
	e := newCopyEnv(t, controlplane.SystemWorkerSettings{CopyEngine: "legacy", MultipartThreads: 1, MultipartPartSizeMiB: 5}, features.Config{})
	e.putSource("o", patterned(3*testPart, 1), nil)
	require.NoError(t, e.copy(t.Context(), "o"))
	require.EqualValues(t, 1, e.src.gets.Load())
	// legacy: part reads come from minio.Object, not through the pipeline pool
	require.Zero(t, partBuffers.peak.Load()-partBuffers.peak.Load())
	require.Equal(t, systemsettings.CopyEngineLegacy, systemsettings.EffectiveCopyEngine())
	systemsettings.ApplySettings(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{CopyEngine: "pipeline"}})
	require.Equal(t, systemsettings.CopyEnginePipeline, systemsettings.EffectiveCopyEngine())
	systemsettings.ApplySettings(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{CopyEngine: "bogus"}})
	require.Equal(t, systemsettings.DefaultCopyEngine, systemsettings.EffectiveCopyEngine())
}

func TestPipelineThreadsDefault(t *testing.T) {
	prev := systemsettings.Current()
	t.Cleanup(func() { systemsettings.ApplySettings(prev) })
	systemsettings.ApplySettings(controlplane.SystemSettings{})
	require.Equal(t, 2, systemsettings.EffectivePipelineThreads())
	systemsettings.ApplySettings(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{MultipartThreads: 5}})
	require.Equal(t, 5, systemsettings.EffectivePipelineThreads())
}
