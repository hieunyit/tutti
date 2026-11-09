package router

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/replication"
	"github.com/hieunyit/tutti/pkg/tasks"
)

type failingReplSvc struct {
	replication.Service
	failFirst int32 // fail this many calls first; <0 means always fail
	calls     atomic.Int32
}

func (f *failingReplSvc) Replicate(context.Context, string, tasks.ReplicationTask) error {
	n := f.calls.Add(1)
	if f.failFirst < 0 || n <= f.failFirst {
		return errors.New("OOM command not allowed")
	}
	return nil
}

type cannedRouter struct {
	body  *trackedBody
	tasks []tasks.ReplicationTask
}

type trackedBody struct {
	io.Reader
	closed atomic.Bool
}

func (b *trackedBody) Close() error { b.closed.Store(true); return nil }

func (c *cannedRouter) Route(*http.Request) (*http.Response, []tasks.ReplicationTask, string, bool, error) {
	c.body = &trackedBody{Reader: strings.NewReader("storage-ok")}
	h := http.Header{}
	h.Set("ETag", "abc")
	return &http.Response{StatusCode: http.StatusOK, Header: h, Body: c.body}, c.tasks, "main", false, nil
}

func runServe(t *testing.T, svc replication.Service, opts ReplicationOptions, ctx context.Context) (*httptest.ResponseRecorder, *cannedRouter) {
	t.Helper()
	rt := &cannedRouter{tasks: []tasks.ReplicationTask{&tasks.ObjectSyncPayload{}}}
	h := serve(rt, svc, false, opts)
	req := httptest.NewRequest(http.MethodPut, "/b/o", nil).WithContext(xctx.SetRoutingPolicy(zerolog.Nop().WithContext(ctx), "main"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec, rt
}

func TestServe_ReplicationRetrySucceeds(t *testing.T) {
	r := require.New(t)
	svc := &failingReplSvc{failFirst: 2}
	rec, rt := runServe(t, svc, ReplicationOptions{OnEventError: "fail", EventRetryBudget: 2 * time.Second}, context.Background())
	r.Equal(http.StatusOK, rec.Code)
	r.Equal("storage-ok", rec.Body.String())
	r.Equal("abc", rec.Header().Get("ETag"))
	r.EqualValues(3, svc.calls.Load())
	r.True(rt.body.closed.Load())
}

func TestServe_ReplicationAlwaysFails_FailMode(t *testing.T) {
	r := require.New(t)
	svc := &failingReplSvc{failFirst: -1}
	rec, rt := runServe(t, svc, ReplicationOptions{OnEventError: "fail", EventRetryBudget: 300 * time.Millisecond}, context.Background())
	r.Equal(http.StatusServiceUnavailable, rec.Code)
	r.Contains(rec.Body.String(), "<Code>ServiceUnavailable</Code>")
	r.Contains(rec.Body.String(), "retry")
	r.NotContains(rec.Body.String(), "storage-ok")
	r.Empty(rec.Header().Get("ETag"))
	r.Greater(svc.calls.Load(), int32(1))
	r.True(rt.body.closed.Load())
}

func TestServe_ReplicationAlwaysFails_LogMode(t *testing.T) {
	r := require.New(t)
	svc := &failingReplSvc{failFirst: -1}
	rec, rt := runServe(t, svc, ReplicationOptions{OnEventError: "log", EventRetryBudget: 200 * time.Millisecond}, context.Background())
	r.Equal(http.StatusOK, rec.Code)
	r.Equal("storage-ok", rec.Body.String())
	r.True(rt.body.closed.Load())
}

func TestServe_ReplicationRetryRespectsContextCancel(t *testing.T) {
	r := require.New(t)
	svc := &failingReplSvc{failFirst: -1}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)
	start := time.Now()
	rec, _ := runServe(t, svc, ReplicationOptions{OnEventError: "fail", EventRetryBudget: 10 * time.Second}, ctx)
	r.Less(time.Since(start), 3*time.Second)
	r.Equal(http.StatusServiceUnavailable, rec.Code)
}

// A request producing many tasks (e.g. DeleteObjects with 500 keys) must be bounded by a
// single retry budget, not budget x number of tasks.
func TestServe_ReplicationRetryBudgetIsPerRequest(t *testing.T) {
	r := require.New(t)
	svc := &failingReplSvc{failFirst: -1}
	many := make([]tasks.ReplicationTask, 500)
	for i := range many {
		many[i] = &tasks.ObjectSyncPayload{}
	}
	rt := &cannedRouter{tasks: many}
	h := serve(rt, svc, false, ReplicationOptions{OnEventError: OnEventErrorFail, EventRetryBudget: 200 * time.Millisecond})
	req := httptest.NewRequest(http.MethodPost, "/b?delete", nil).
		WithContext(xctx.SetRoutingPolicy(zerolog.Nop().WithContext(context.Background()), "main"))
	rec := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(rec, req)
	r.Less(time.Since(start), 2*time.Second, "500 tasks must share one 200ms budget")
	r.Equal(http.StatusServiceUnavailable, rec.Code)
}
