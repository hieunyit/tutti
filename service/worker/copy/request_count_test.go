package copy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/features"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/systemsettings"
	"github.com/hieunyit/tutti/pkg/workerstats"
)

type counted struct {
	mu   sync.Mutex
	reqs []string
}

func (c *counted) add(r *http.Request) {
	if strings.Contains(r.URL.Path, "probe-health-test") || strings.Trim(r.URL.Path, "/") == "bucket" {
		return // health probe and bucket-level (region discovery) requests are not per object
	}
	c.mu.Lock()
	c.reqs = append(c.reqs, r.Method)
	c.mu.Unlock()
}

func (c *counted) n() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.reqs)
}

func srcHandler(c *counted, body []byte, ranges *atomic.Int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "probe-health-test") {
			w.WriteHeader(403)
			return
		}
		c.add(r)
		w.Header().Set("ETag", `"8d777f385d3dfec8815d20f7496026dc"`)
		w.Header().Set("Last-Modified", "Wed, 11 Nov 2026 00:00:00 GMT")
		w.Header().Set("Content-Type", "application/octet-stream")
		data := body
		if rg := r.Header.Get("Range"); rg != "" {
			ranges.Add(1)
			var a, b int
			_, err := fmt.Sscanf(rg, "bytes=%d-%d", &a, &b)
			if err != nil {
				w.WriteHeader(400)
				return
			}
			if b >= len(body) {
				b = len(body) - 1
			}
			data = body[a : b+1]
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", a, b, len(body)))
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.WriteHeader(206)
		} else {
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		}
		if r.Method != http.MethodHead {
			w.Write(data)
		}
	})
}

// TestCopyObjectRequestCount pins the number of S3 requests per copied object.
// Before: 5 (HEAD source, HEAD destination, HEAD issued by minio GetObject, GET source, PUT destination).
// After, new unversioned object: 4 (HEAD destination, HEAD+GET of GetObject which also provides the source stat, PUT).
// The destination HEAD is kept: it decides the skip of an already equal object.
func TestCopyObjectRequestCount(t *testing.T) {
	old := &features.Config{Versioning: features.Versioning(t.Context()), Tagging: features.Tagging(t.Context()), ACL: features.ACL(t.Context())}
	t.Cleanup(func() { features.Set(old) })
	features.Set(&features.Config{})

	body := []byte("data")
	var srcC, dstC counted
	var ranges atomic.Int32
	src := httptest.NewServer(srcHandler(&srcC, body, &ranges))
	defer src.Close()
	dstPresent := false
	var put atomic.Int32
	dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "probe-health-test") {
			w.WriteHeader(403)
			return
		}
		dstC.add(r)
		switch r.Method {
		case http.MethodHead:
			if !dstPresent {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("ETag", `"8d777f385d3dfec8815d20f7496026dc"`)
			w.Header().Set("Content-Length", "4")
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Last-Modified", "Wed, 11 Nov 2026 00:00:00 GMT")
		case http.MethodPut:
			io.Copy(io.Discard, r.Body)
			put.Add(1)
			w.Header().Set("ETag", `"8d777f385d3dfec8815d20f7496026dc"`)
		}
	}))
	defer dst.Close()
	clients := comprehensiveClients{source: comprehensiveClient(t, src.URL), dest: comprehensiveClient(t, dst.URL)}
	svc := NewS3CopySvc(comprehensiveCreds{}, clients, metrics.NewS3Service(false))

	// new object: HEAD dst, then GetObject = HEAD src + GET src, then PUT dst
	require.NoError(t, svc.CopyObject(t.Context(), "u", NewFile("source", "bucket", "object"), NewFile("dest", "bucket", "object")))
	require.Equal(t, 4, srcC.n()+dstC.n(), "src=%v dst=%v", srcC.reqs, dstC.reqs)
	require.Equal(t, []string{"HEAD", "GET"}, srcC.reqs)
	require.Equal(t, int32(1), put.Load())

	// identical object already at the destination is still skipped: HEAD src + HEAD dst, no transfer
	dstPresent = true
	srcC.reqs, dstC.reqs = nil, nil
	require.NoError(t, svc.CopyObject(t.Context(), "u", NewFile("source", "bucket", "object"), NewFile("dest", "bucket", "object")))
	require.Equal(t, []string{"HEAD"}, srcC.reqs)
	require.Equal(t, []string{"HEAD"}, dstC.reqs)
	require.Equal(t, int32(1), put.Load())
}

func TestCopyObjectMissingSourceIsNotFound(t *testing.T) {
	old := &features.Config{Versioning: features.Versioning(t.Context()), Tagging: features.Tagging(t.Context()), ACL: features.ACL(t.Context())}
	t.Cleanup(func() { features.Set(old) })
	features.Set(&features.Config{})
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "probe-health-test") {
			w.WriteHeader(403)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(404)
		io.WriteString(w, `<Error><Code>NoSuchKey</Code><Message>x</Message></Error>`)
	}))
	defer src.Close()
	dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "probe-health-test") {
			w.WriteHeader(403)
			return
		}
		w.WriteHeader(404)
	}))
	defer dst.Close()
	clients := comprehensiveClients{source: comprehensiveClient(t, src.URL), dest: comprehensiveClient(t, dst.URL)}
	svc := NewS3CopySvc(comprehensiveCreds{}, clients, metrics.NewS3Service(false))
	err := svc.CopyObject(t.Context(), "u", NewFile("source", "bucket", "object"), NewFile("dest", "bucket", "object"))
	require.ErrorIs(t, err, dom.ErrNotFound)
}

func TestParallelSourceReadAt(t *testing.T) {
	body := make([]byte, 5<<20+123)
	for i := range body {
		body[i] = byte(i * 7)
	}
	var ranges atomic.Int32
	opened := 0
	var mu sync.Mutex
	ps := newParallelSource(t.Context(), int64(len(body)), 1<<20, nil, func(_ context.Context, start, end int64) (io.ReadCloser, error) {
		mu.Lock()
		opened++
		mu.Unlock()
		ranges.Add(1)
		return io.NopCloser(bytes.NewReader(body[start : end+1])), nil
	})
	defer ps.Close()
	var wg sync.WaitGroup
	got := make([]byte, len(body))
	for off := 0; off < len(body); off += 1 << 20 {
		wg.Add(1)
		go func(off int) {
			defer wg.Done()
			end := off + 1<<20
			if end > len(body) {
				end = len(body)
			}
			sr := io.NewSectionReader(ps, int64(off), int64(end-off))
			// chunked reads like an HTTP body copy
			_, err := io.CopyBuffer(&writerAt{got, off}, io.LimitReader(sr, int64(end-off)), make([]byte, 32*1024))
			require.NoError(t, err)
		}(off)
	}
	wg.Wait()
	require.Equal(t, body, got)
	require.Equal(t, 6, opened, "one ranged GET per part, not per chunk")
}

type writerAt struct {
	b   []byte
	off int
}

func (w *writerAt) Write(p []byte) (int, error) {
	n := copy(w.b[w.off:], p)
	w.off += n
	return n, nil
}

// TestCopyObjectParallelMultipart uploads an object above the part size with several threads and checks the
// bytes, the use of ranged source GETs and that the buffer reservation is released.
func TestCopyObjectParallelMultipart(t *testing.T) {
	ranges := runMultipartCopy(t, 4, 5)
	require.GreaterOrEqual(t, int(ranges), 3, "parts must be read with ranged GETs")
}

// TestCopyObjectMultipartTuningAppliesLive changes the multipart threads in the system settings between two
// copies in the same process: the second copy must follow without any restart.
func TestCopyObjectMultipartTuningAppliesLive(t *testing.T) {
	before := workerstats.Default.Snapshot()
	require.GreaterOrEqual(t, int(runMultipartCopy(t, 4, 5)), 3)
	require.Zero(t, int(runMultipartCopy(t, 1, 5)), "1 thread must read the source sequentially")
	after := workerstats.Default.Snapshot()
	require.Equal(t, before.CopiedObjectsTotal+2, after.CopiedObjectsTotal)
	require.Equal(t, before.CopiedBytesTotal+2*int64(12<<20+17), after.CopiedBytesTotal)
	require.Zero(t, after.InFlight)
}

// runMultipartCopy copies a ~12 MiB object with the given tuning applied through the system settings and
// returns the number of ranged source GETs.
func runMultipartCopy(t *testing.T, threads, partMiB int) int32 {
	old := &features.Config{Versioning: features.Versioning(t.Context()), Tagging: features.Tagging(t.Context()), ACL: features.ACL(t.Context())}
	t.Cleanup(func() { features.Set(old) })
	features.Set(&features.Config{})
	prev := systemsettings.Current()
	t.Cleanup(func() { systemsettings.ApplySettings(prev) })
	systemsettings.ApplySettings(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{
		BufferBudgetBytes: 64 << 20, MultipartThreads: threads, MultipartPartSizeMiB: partMiB,
	}})

	body := make([]byte, 12<<20+17)
	for i := range body {
		body[i] = byte(i * 13)
	}
	var srcC counted
	var ranges atomic.Int32
	src := httptest.NewServer(srcHandler(&srcC, body, &ranges))
	defer src.Close()

	var mu sync.Mutex
	parts := map[int][]byte{}
	var complete atomic.Int32
	dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "probe-health-test") {
			w.WriteHeader(403)
			return
		}
		q := r.URL.Query()
		switch {
		case r.Method == http.MethodHead:
			w.WriteHeader(404)
		case r.Method == http.MethodPost && q.Has("uploads"):
			io.WriteString(w, `<InitiateMultipartUploadResult><Bucket>bucket</Bucket><Key>object</Key><UploadId>u1</UploadId></InitiateMultipartUploadResult>`)
		case r.Method == http.MethodPut && q.Has("partNumber"):
			n, _ := strconv.Atoi(q.Get("partNumber"))
			b, _ := io.ReadAll(r.Body)
			mu.Lock()
			parts[n] = b
			mu.Unlock()
			w.Header().Set("ETag", fmt.Sprintf(`"part%d"`, n))
		case r.Method == http.MethodPost && q.Has("uploadId"):
			io.Copy(io.Discard, r.Body)
			complete.Add(1)
			io.WriteString(w, `<CompleteMultipartUploadResult><Bucket>bucket</Bucket><Key>object</Key><ETag>"abc-3"</ETag></CompleteMultipartUploadResult>`)
		default:
			io.Copy(io.Discard, r.Body)
		}
	}))
	defer dst.Close()
	clients := comprehensiveClients{source: comprehensiveClient(t, src.URL), dest: comprehensiveClient(t, dst.URL)}
	svc := NewS3CopySvc(comprehensiveCreds{}, clients, metrics.NewS3Service(false))
	require.NoError(t, svc.CopyObject(t.Context(), "u", NewFile("source", "bucket", "object"), NewFile("dest", "bucket", "object")))

	var got []byte
	for i := 1; i <= len(parts); i++ {
		got = append(got, parts[i]...)
	}
	require.Equal(t, len(body), len(got))
	require.True(t, bytes.Equal(body, got), "uploaded bytes differ from source")
	require.Equal(t, int32(1), complete.Load())
	// budget fully released: the whole budget can be taken again
	rel, err := systemsettings.AcquireBufferBudget(t.Context(), 64<<20)
	require.NoError(t, err)
	rel()
	return ranges.Load()
}

func TestEstimateParallelCopyBuffer(t *testing.T) {
	require.Equal(t, int64(4*16<<20), EstimateParallelCopyBuffer(1<<30, 16<<20, 4))
	require.Equal(t, int64(20<<20), EstimateParallelCopyBuffer(20<<20, 16<<20, 8))
}
