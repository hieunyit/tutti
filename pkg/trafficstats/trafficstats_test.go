package trafficstats

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCollector_ConcurrentIncrements(t *testing.T) {
	c := New()
	const g, n = 16, 1000
	var wg sync.WaitGroup
	for i := 0; i < g; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < n; j++ {
				c.Request(ClassRead, "main")
				c.Response(200)
				c.BytesIn(2)
				c.BytesOut(3)
				c.Latency(time.Millisecond)
				c.FailFast()
				c.SyncOutcome(SyncQuorum)
			}
		}()
	}
	wg.Wait()
	s := c.Snapshot()
	require.EqualValues(t, g*n, s.Requests.Total)
	require.EqualValues(t, g*n, s.Requests.ByClass["read"])
	require.EqualValues(t, g*n, s.Requests.ByStorage["main"])
	require.EqualValues(t, g*n, s.Responses.Status2xx)
	require.EqualValues(t, 2*g*n, s.BytesIn)
	require.EqualValues(t, 3*g*n, s.BytesOut)
	require.EqualValues(t, g*n, s.Latency.Count)
	require.EqualValues(t, g*n, s.FailFast)
	require.EqualValues(t, g*n, s.Sync.QuorumReached)
}

func TestCollector_StatusClasses(t *testing.T) {
	c := New()
	for _, st := range []int{200, 204, 301, 304, 404, 403, 503, 100, 0} {
		c.Response(st)
	}
	r := c.Snapshot().Responses
	require.EqualValues(t, 2, r.Status2xx)
	require.EqualValues(t, 2, r.Status3xx)
	require.EqualValues(t, 2, r.Status4xx)
	require.EqualValues(t, 1, r.Status5xx)
	require.EqualValues(t, 2, r.Other)
}

func TestCollector_HistogramBuckets(t *testing.T) {
	c := New()
	for _, d := range []time.Duration{
		time.Millisecond, 10 * time.Millisecond, // <=10ms
		11 * time.Millisecond, // <=50ms
		time.Second,           // <=1s
		10 * time.Second,      // <=10s
		11 * time.Second,      // +Inf
	} {
		c.Latency(d)
	}
	l := c.Snapshot().Latency
	require.Len(t, l.Buckets, len(LatencyBounds)+1)
	require.EqualValues(t, 6, l.Count)
	require.Equal(t, []uint64{2, 1, 0, 0, 0, 1, 0, 0, 1, 1}, l.Buckets)
	require.InDelta(t, 22.022, l.SumSeconds, 1e-6)
}

func TestMiddleware_CountsBytesStatusAndRoute(t *testing.T) {
	c := New()
	h := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetRoute(r.Context(), ClassWrite)
		SetStorage(r.Context(), "s1")
		b, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(b)
		_, _ = w.Write([]byte("xx"))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/b/k", strings.NewReader("hello")))
	s := c.Snapshot()
	require.EqualValues(t, 1, s.Requests.ByClass["write"])
	require.EqualValues(t, 1, s.Requests.ByStorage["s1"])
	require.EqualValues(t, 1, s.Responses.Status2xx)
	require.EqualValues(t, 5, s.BytesIn)
	require.EqualValues(t, 7, s.BytesOut)
	require.EqualValues(t, 1, s.Latency.Count)
}

func TestMiddleware_ImplicitStatusAndUnrouted(t *testing.T) {
	c := New()
	h := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("abc")) }))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	s := c.Snapshot()
	require.EqualValues(t, 1, s.Responses.Status2xx)
	require.EqualValues(t, 1, s.Requests.ByClass["other"])
	require.Empty(t, s.Requests.ByStorage)
}

func TestMiddleware_HeadCountsNoBody(t *testing.T) {
	c := New()
	srv := httptest.NewServer(c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "5")
		_, _ = w.Write([]byte("hello"))
	})))
	defer srv.Close()
	resp, err := http.Head(srv.URL)
	require.NoError(t, err)
	_ = resp.Body.Close()
	resp, err = http.Get(srv.URL)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	require.Eventually(t, func() bool { return c.Snapshot().Requests.Total == 2 }, time.Second, 5*time.Millisecond)
	require.EqualValues(t, 5, c.Snapshot().BytesOut)
}

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushes int
}

func (f *flushRecorder) Flush() { f.flushes++; f.ResponseRecorder.Flush() }

func TestResponseWriter_StreamingAndReadFrom(t *testing.T) {
	fr := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	w := NewResponseWriter(fr, false)
	var _ http.Flusher = w
	w.Flush()
	require.Equal(t, 200, w.Status())
	require.Equal(t, 1, fr.flushes)
	n, err := io.Copy(w, strings.NewReader(strings.Repeat("a", 100000)))
	require.NoError(t, err)
	require.EqualValues(t, 100000, n)
	require.EqualValues(t, 100000, w.Bytes())
	require.Equal(t, 100000, fr.Body.Len())
	require.Equal(t, http.ResponseWriter(fr), w.Unwrap())
}

func TestResponseWriter_InformationalIgnored(t *testing.T) {
	w := NewResponseWriter(httptest.NewRecorder(), false)
	w.WriteHeader(http.StatusContinue)
	require.Equal(t, 0, w.Status())
	w.WriteHeader(http.StatusNotFound)
	w.WriteHeader(http.StatusOK)
	require.Equal(t, 404, w.Status())
}

func BenchmarkMiddleware(b *testing.B) {
	c := New()
	body := []byte(strings.Repeat("x", 1024))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetRoute(r.Context(), ClassRead)
		SetStorage(r.Context(), "main")
		_, _ = w.Write(body)
	})
	wrapped := c.Middleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/b/k", nil)
	b.Run("bare", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			inner.ServeHTTP(nopWriter{}, req)
		}
	})
	b.Run("wrapped", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			wrapped.ServeHTTP(nopWriter{}, req)
		}
	})
}

type nopWriter struct{}

func (nopWriter) Header() http.Header         { return http.Header{} }
func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
func (nopWriter) WriteHeader(int)             {}
