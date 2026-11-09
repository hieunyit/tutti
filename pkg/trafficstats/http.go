package trafficstats

import (
	"context"
	"io"
	"net/http"
	"time"
)

// ReqInfo is filled by inner handlers once the request is routed.
type ReqInfo struct {
	class   string
	storage string
}

type infoKey struct{}

// SetRoute records the operation class of the request; no-op outside Middleware.
func SetRoute(ctx context.Context, class string) {
	if ri, _ := ctx.Value(infoKey{}).(*ReqInfo); ri != nil {
		ri.class = class
	}
}

// SetStorage records the storage the request was routed to; no-op outside Middleware.
func SetStorage(ctx context.Context, storage string) {
	if ri, _ := ctx.Value(infoKey{}).(*ReqInfo); ri != nil {
		ri.storage = storage
	}
}

// Middleware measures every request: count, class/storage (set by inner handlers), status, bytes and latency.
// It must be the outermost handler.
func (c *Collector) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ri := &ReqInfo{}
		r = r.WithContext(context.WithValue(r.Context(), infoKey{}, ri))
		if r.Body != nil && r.Body != http.NoBody {
			r.Body = &countingBody{ReadCloser: r.Body, c: c}
		}
		rw := &ResponseWriter{ResponseWriter: w, head: r.Method == http.MethodHead}
		defer func() {
			// finish even when the handler panics (e.g. http.ErrAbortHandler)
			status := rw.status
			if status == 0 {
				status = http.StatusOK
			}
			c.Request(ri.class, ri.storage)
			c.Response(status)
			c.BytesOut(rw.bytes)
			c.Latency(time.Since(start))
		}()
		next.ServeHTTP(rw, r)
	})
}

type countingBody struct {
	io.ReadCloser
	c *Collector
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.c.BytesIn(int64(n))
	return n, err
}

// ResponseWriter records the status code and the number of body bytes written.
// It keeps Flush and http.ResponseController support (Unwrap) so streaming responses work.
type ResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
	head   bool
}

// Status returns the status written (0 if nothing was written yet).
func (w *ResponseWriter) Status() int { return w.status }

// Bytes returns the body bytes written.
func (w *ResponseWriter) Bytes() int64 { return w.bytes }

// NewResponseWriter wraps w; head must be true for HEAD requests (no body bytes are counted).
func NewResponseWriter(w http.ResponseWriter, head bool) *ResponseWriter {
	return &ResponseWriter{ResponseWriter: w, head: head}
}

func (w *ResponseWriter) WriteHeader(code int) {
	// 1xx informational responses (except 101) are not final
	if w.status == 0 && (code >= 200 || code == http.StatusSwitchingProtocols) {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *ResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	if !w.head {
		w.bytes += int64(n)
	}
	return n, err
}

// ReadFrom keeps the io.ReaderFrom fast path of the underlying writer.
func (w *ResponseWriter) ReadFrom(r io.Reader) (int64, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	var n int64
	var err error
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		n, err = rf.ReadFrom(r)
	} else {
		n, err = io.Copy(struct{ io.Writer }{w.ResponseWriter}, r)
	}
	if !w.head {
		w.bytes += n
	}
	return n, err
}

// Flush implements http.Flusher.
func (w *ResponseWriter) Flush() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap supports http.ResponseController.
func (w *ResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
