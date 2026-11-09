package router

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/rs/zerolog"
)

type reviewDeadlineReplication struct {
	sawDeadline bool
	calls       int
}

type reviewBlockingReplication struct{ calls int }

func (s *reviewBlockingReplication) Replicate(ctx context.Context, _ string, _ tasks.ReplicationTask) error {
	s.calls++
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Second):
		return errors.New("dependency was not canceled by the retry budget")
	}
}

func TestReviewRequestStopsRecordingTasksWhenBudgetExpires(t *testing.T) {
	svc := &reviewBlockingReplication{}
	rt := &cannedRouter{tasks: []tasks.ReplicationTask{&tasks.ObjectSyncPayload{}, &tasks.ObjectSyncPayload{}, &tasks.ObjectSyncPayload{}}}
	h := serve(rt, svc, false, ReplicationOptions{EventRetryBudget: 20 * time.Millisecond}, SyncOptions{})
	req := httptest.NewRequest(http.MethodPost, "/bucket?delete", nil).WithContext(xctx.SetRoutingPolicy(zerolog.Nop().WithContext(context.Background()), "main"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if svc.calls != 1 {
		t.Errorf("expected one canceled dependency call, got %d", svc.calls)
	}
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func (s *reviewDeadlineReplication) Replicate(ctx context.Context, _ string, _ tasks.ReplicationTask) error {
	s.calls++
	_, s.sawDeadline = ctx.Deadline()
	return errors.New("temporary queue failure")
}

func TestReviewRetryPassesBudgetToDependency(t *testing.T) {
	svc := &reviewDeadlineReplication{}
	_ = replicateWithRetry(context.Background(), svc, "main", &tasks.ObjectSyncPayload{}, time.Now().Add(2*time.Millisecond))
	if !svc.sawDeadline {
		t.Error("Replicate invoked without the request budget deadline")
	}
}

func TestReviewRetryDoesNotCallAfterBudgetExhausted(t *testing.T) {
	svc := &reviewDeadlineReplication{}
	_ = replicateWithRetry(context.Background(), svc, "main", &tasks.ObjectSyncPayload{}, time.Now().Add(-time.Second))
	if svc.calls != 0 {
		t.Errorf("Replicate invoked %d time(s) after exhausted budget", svc.calls)
	}
}
