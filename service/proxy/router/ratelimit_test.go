package router

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/ratelimit"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3client"
)

type mockRPM struct {
	err error
}

func (m *mockRPM) StorReq(ctx context.Context, storage string, opts ...ratelimit.Opt) error {
	return m.err
}

func (m *mockRPM) StorReqN(ctx context.Context, storage string, n int, opts ...ratelimit.Opt) error {
	return m.err
}

type countingClient struct {
	s3client.Client
	calls int
}

func (c *countingClient) Do(req *http.Request) (*http.Response, bool, error) {
	c.calls++
	return &http.Response{StatusCode: http.StatusOK}, false, nil
}

func TestProxyLimit_PreventsBackendCall(t *testing.T) {
	client := &countingClient{}
	limit := &mockRPM{err: &dom.ErrRateLimitExceeded{RetryIn: time.Second}}
	r := &s3Router{
		limit: limit,
	}

	req := httptest.NewRequest(http.MethodPut, "/bucket/object", nil)
	req = req.WithContext(xctx.SetMethod(req.Context(), s3.PutObject))

	resp, isApiErr, err := r.forward(req, "storage1", client)
	require.Error(t, err)
	var rlErr *dom.ErrRateLimitExceeded
	require.True(t, errors.As(err, &rlErr))
	require.False(t, isApiErr)
	require.Nil(t, resp)
	require.Equal(t, 0, client.calls, "backend client must NOT be called when rate limit is exceeded")
}
