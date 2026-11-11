package s3client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	mclient "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/health"
	"github.com/hieunyit/tutti/pkg/s3"
)

type noopMetrics struct{}

func (noopMetrics) Count(xctx.Flow, string, string)         {}
func (noopMetrics) Upload(xctx.Flow, string, string, int)   {}
func (noopMetrics) Download(xctx.Flow, string, string, int) {}

type fakeHealth struct {
	allow    bool
	observed []error
}

func (f *fakeHealth) Allow(string) bool         { return f.allow }
func (f *fakeHealth) Observe(_ string, e error) { f.observed = append(f.observed, e) }

func newTestClient(addr string, h Health) *client {
	return &client{
		c:           &http.Client{},
		conf:        s3.StorageAddress{Address: addr},
		cred:        s3.CredentialsV4{AccessKeyID: "a", SecretAccessKey: "b"},
		storageName: "main",
		metricsSvc:  noopMetrics{},
		health:      h,
	}
}

func TestDo_OpenBreakerFailsFastWithoutBackendCall(t *testing.T) {
	r := require.New(t)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()
	h := &fakeHealth{allow: false}
	c := newTestClient(srv.URL, h)
	req := httptest.NewRequest(http.MethodGet, "/bucket/key", nil)
	resp, isApiErr, err := c.Do(req)
	r.Nil(resp)
	r.False(isApiErr)
	var s3Err mclient.ErrorResponse
	r.True(errors.As(err, &s3Err))
	r.Equal("ServiceUnavailable", s3Err.Code)
	r.Equal(http.StatusServiceUnavailable, s3Err.StatusCode)
	r.Contains(s3Err.Message, "storage main is temporarily unavailable")
	r.Zero(hits.Load())
	r.Empty(h.observed)
}

func TestDo_ObservesOutcomes(t *testing.T) {
	r := require.New(t)
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
	h := &fakeHealth{allow: true}
	c := newTestClient(srv.URL, h)
	do := func() {
		resp, _, _ := c.Do(httptest.NewRequest(http.MethodGet, "/bucket/key", nil))
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
	do()
	status = http.StatusNotFound
	do()
	status = http.StatusServiceUnavailable
	do()
	srv.Close()
	do() // connection refused
	r.Len(h.observed, 4)
	r.Equal(health.Success, health.Classify(h.observed[0]))
	r.Equal(health.Success, health.Classify(h.observed[1]))
	r.Equal(health.Failure, health.Classify(h.observed[2]))
	r.Equal(health.Failure, health.Classify(h.observed[3]))

	// client cancellation is not observed
	h.observed = nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := c.Do(httptest.NewRequest(http.MethodGet, "/bucket/key", nil).WithContext(ctx))
	r.Error(err)
	r.Empty(h.observed)
}

func TestDo_StatusNotModified_NotTreatedAsApiErr(t *testing.T) {
	r := require.New(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("ETag", "\"12345\"")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()
	c := newTestClient(srv.URL, &fakeHealth{allow: true})
	req := httptest.NewRequest(http.MethodGet, "/bucket/key", nil)
	resp, isApiErr, err := c.Do(req)
	r.NoError(err)
	r.False(isApiErr)
	r.Equal(http.StatusNotModified, resp.StatusCode)
	r.Equal("\"12345\"", resp.Header.Get("ETag"))
}
