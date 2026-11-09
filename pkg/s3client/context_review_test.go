package s3client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type reviewTransport func(*http.Request) (*http.Response, error)

func (f reviewTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestReviewForwardPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := newTestClient("http://backend.invalid", nil)
	c.c.Transport = reviewTransport(func(r *http.Request) (*http.Response, error) {
		if r.Context().Err() == nil {
			t.Error("backend request lost canceled caller context")
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok")), Request: r}, nil
	})
	resp, _, _ := c.Do(httptest.NewRequest(http.MethodGet, "/bucket/key", nil).WithContext(ctx))
	if resp != nil {
		resp.Body.Close()
	}
}
