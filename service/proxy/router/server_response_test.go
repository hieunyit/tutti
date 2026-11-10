package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/replication"
	"github.com/hieunyit/tutti/pkg/tasks"
)

type respRouter struct{ resp func() *http.Response }

func (s respRouter) Route(*http.Request) (*http.Response, []tasks.ReplicationTask, string, bool, error) {
	return s.resp(), nil, "main", false, nil
}

func newRespProxy(t *testing.T, rt Router) *httptest.Server {
	t.Helper()
	pass := func(next http.Handler) http.Handler { return next }
	setPolicy := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(xctx.SetRoutingPolicy(r.Context(), "main")))
		})
	}
	h, err := New(Config{
		Storages: map[dom.StorageType]StorageProxy{dom.S3: {
			Router: rt, Replicator: struct{ replication.Service }{}, ReqParseMiddleware: pass,
		}},
		LogMiddleware: pass, PolicyMiddleware: setPolicy,
	})
	require.NoError(t, err)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

type cutBody struct{ sent bool }

func (c *cutBody) Read(p []byte) (int, error) {
	if !c.sent {
		c.sent = true
		return copy(p, "partial"), nil
	}
	return 0, io.ErrUnexpectedEOF
}
func (c *cutBody) Close() error { return nil }

func TestServe_BodyCopyFailureAbortsResponse(t *testing.T) {
	for _, cl := range []int64{100, -1} {
		srv := newRespProxy(t, respRouter{resp: func() *http.Response {
			h := http.Header{}
			if cl > 0 {
				h.Set("Content-Length", "100")
			}
			return &http.Response{StatusCode: 200, Header: h, ContentLength: cl, Body: &cutBody{}}
		}})
		resp, err := http.Get(srv.URL + "/bucket/object")
		if err == nil {
			defer resp.Body.Close()
			_, err = io.ReadAll(resp.Body)
		}
		require.Error(t, err, "client must see a broken response, not a truncated 200 (content-length %d)", cl)
	}
}

func TestServe_ForwardsAllHeaderValuesDropsHopByHop(t *testing.T) {
	srv := newRespProxy(t, respRouter{resp: func() *http.Response {
		h := http.Header{}
		h["X-Multi"] = []string{"a", "b"}
		h["Set-Cookie"] = []string{"c1=1", "c2=2"}
		h.Set("Connection", "X-Custom-Hop, close")
		h.Set("X-Custom-Hop", "1")
		h.Set("Keep-Alive", "timeout=5")
		h.Set("Proxy-Connection", "keep-alive")
		h.Set("Upgrade", "h2c")
		h.Set("Te", "trailers")
		h.Set("Trailer", "X-T")
		h.Set("ETag", `"e"`)
		return &http.Response{StatusCode: 200, Header: h, Body: io.NopCloser(strings.NewReader("ok"))}
	}})
	resp, err := http.Get(srv.URL + "/bucket/object")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, []string{"a", "b"}, resp.Header.Values("X-Multi"))
	require.Equal(t, []string{"c1=1", "c2=2"}, resp.Header.Values("Set-Cookie"))
	require.Equal(t, `"e"`, resp.Header.Get("ETag"))
	for _, h := range []string{"X-Custom-Hop", "Keep-Alive", "Proxy-Connection", "Upgrade", "Te", "Trailer"} {
		require.Empty(t, resp.Header.Values(h), h)
	}
}
