package router

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/tasks"
)

type fakeSyncer struct {
	mu    sync.Mutex
	calls map[string]int
	fail  map[string]bool
	hang  map[string]bool
}

type syncerFunc func(context.Context, tasks.ObjectSyncPayload) error

func (f syncerFunc) Sync(ctx context.Context, p tasks.ObjectSyncPayload) error { return f(ctx, p) }

func (f *fakeSyncer) Sync(ctx context.Context, p tasks.ObjectSyncPayload) error {
	to := p.ID.ToStorage()
	f.mu.Lock()
	f.calls[to]++
	fail, hang := f.fail[to], f.hang[to]
	f.mu.Unlock()
	if hang {
		<-ctx.Done()
		return ctx.Err()
	}
	if fail {
		return errors.New("destination down")
	}
	return nil
}

func (f *fakeSyncer) callsTo(storage string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[storage]
}

func (f *fakeSyncer) totalCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	total := 0
	for _, n := range f.calls {
		total += n
	}
	return total
}

func newFakeSyncer(failTo ...string) *fakeSyncer {
	f := &fakeSyncer{calls: map[string]int{}, fail: map[string]bool{}, hang: map[string]bool{}}
	for _, s := range failTo {
		f.fail[s] = true
	}
	return f
}

func replID(to string) entity.UniversalReplicationID {
	return entity.UniversalFromBucketReplication(entity.BucketReplicationPolicy{
		User: "u", FromStorage: "main", FromBucket: "b", ToStorage: to, ToBucket: "b",
	})
}

func runSyncServe(t *testing.T, svc *failingReplSvc, syncOpts SyncOptions, dests []string, nTasks int) (*httptest.ResponseRecorder, time.Duration) {
	t.Helper()
	ts := make([]tasks.ReplicationTask, nTasks)
	for i := range ts {
		ts[i] = &tasks.ObjectSyncPayload{}
	}
	rt := &cannedRouter{tasks: ts}
	h := serve(rt, svc, false, ReplicationOptions{OnEventError: OnEventErrorFail, EventRetryBudget: 100 * time.Millisecond}, syncOpts)
	ids := make([]entity.UniversalReplicationID, 0, len(dests))
	for _, d := range dests {
		ids = append(ids, replID(d))
	}
	ctx := xctx.SetReplications(zerolog.Nop().WithContext(context.Background()), ids)
	ctx = xctx.SetRoutingPolicy(ctx, "main")
	req := httptest.NewRequest(http.MethodPut, "/b/o", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(rec, req)
	return rec, time.Since(start)
}

func TestServeSync_Disabled(t *testing.T) {
	r := require.New(t)
	fs := newFakeSyncer()
	rec, _ := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: false, Syncer: fs}, []string{"f1", "f2"}, 1)
	r.Equal(http.StatusOK, rec.Code)
	r.Empty(rec.Header().Get(ReplicasHeader))
	r.Zero(fs.totalCalls())
}

func TestServeSync_AllSucceed(t *testing.T) {
	r := require.New(t)
	fs := newFakeSyncer()
	rec, _ := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: time.Second}, []string{"f1"}, 1)
	r.Equal(http.StatusOK, rec.Code)
	r.Equal("storage-ok", rec.Body.String())
	r.Equal("2/2", rec.Header().Get(ReplicasHeader))
	r.Equal(1, fs.callsTo("f1"))
}

func TestServeSync_OneOfTwoFails_QuorumAll_503(t *testing.T) {
	r := require.New(t)
	fs := newFakeSyncer("f2")
	rec, _ := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: time.Second}, []string{"f1", "f2"}, 1)
	r.Equal(http.StatusServiceUnavailable, rec.Code)
	r.Contains(rec.Body.String(), "<Code>ServiceUnavailable</Code>")
	r.Contains(rec.Body.String(), "quorum")
	r.NotContains(rec.Body.String(), "storage-ok")
}

func TestServeSync_OneOfTwoFails_Accept(t *testing.T) {
	r := require.New(t)
	fs := newFakeSyncer("f2")
	rec, _ := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: time.Second, OnQuorumFailure: OnQuorumFailureAccept}, []string{"f1", "f2"}, 1)
	r.Equal(http.StatusOK, rec.Code)
	r.Equal("storage-ok", rec.Body.String())
	r.Equal("2/3", rec.Header().Get(ReplicasHeader))
}

func TestServeSync_WriteQuorumTwoSatisfied(t *testing.T) {
	r := require.New(t)
	fs := newFakeSyncer("f2")
	rec, _ := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: time.Second, WriteQuorum: 2}, []string{"f1", "f2"}, 1)
	r.Equal(http.StatusOK, rec.Code)
	r.Equal("2/3", rec.Header().Get(ReplicasHeader))
}

func TestServeSync_ReturnsWhenQuorumReached(t *testing.T) {
	r := require.New(t)
	slowStarted := make(chan struct{})
	slowCanceled := make(chan struct{})
	syncer := syncerFunc(func(ctx context.Context, p tasks.ObjectSyncPayload) error {
		if p.ID.ToStorage() == "f1" {
			<-slowStarted
			return nil
		}
		close(slowStarted)
		<-ctx.Done()
		close(slowCanceled)
		return ctx.Err()
	})
	svc := &failingReplSvc{}
	rec, took := runSyncServe(t, svc, SyncOptions{Enabled: true, Syncer: syncer, Timeout: 2 * time.Second, WriteQuorum: 2}, []string{"f1", "f2"}, 1)
	r.Less(took, time.Second)
	r.Equal(http.StatusOK, rec.Code)
	r.Equal("2/3", rec.Header().Get(ReplicasHeader))
	r.EqualValues(1, svc.calls.Load(), "asynchronous replication event must be recorded before early return")
	select {
	case <-slowCanceled:
	case <-time.After(time.Second):
		t.Fatal("pending destination was not canceled after quorum")
	}
}

func TestServeSync_QuorumOneSkipsDestinations(t *testing.T) {
	r := require.New(t)
	var calls atomic.Int32
	syncer := syncerFunc(func(context.Context, tasks.ObjectSyncPayload) error {
		calls.Add(1)
		return nil
	})
	rec, _ := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: syncer, Timeout: time.Second, WriteQuorum: 1}, []string{"f1", "f2"}, 1)
	r.Equal(http.StatusOK, rec.Code)
	r.Equal("1/3", rec.Header().Get(ReplicasHeader))
	r.Zero(calls.Load())
}

func TestServeSync_DefaultQuorumWaitsForSlowDestination(t *testing.T) {
	r := require.New(t)
	syncer := syncerFunc(func(ctx context.Context, p tasks.ObjectSyncPayload) error {
		if p.ID.ToStorage() == "f1" {
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	})
	rec, took := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: syncer, Timeout: 150 * time.Millisecond}, []string{"f1", "f2"}, 1)
	r.GreaterOrEqual(took, 100*time.Millisecond)
	r.Less(took, time.Second)
	r.Equal(http.StatusServiceUnavailable, rec.Code)
}

func TestServeSync_FailureStillAllowsQuorumFromOtherDestination(t *testing.T) {
	r := require.New(t)
	slowStarted := make(chan struct{})
	slowCanceled := make(chan struct{})
	syncer := syncerFunc(func(ctx context.Context, p tasks.ObjectSyncPayload) error {
		switch p.ID.ToStorage() {
		case "f1":
			return errors.New("destination down")
		case "f2":
			<-slowStarted
			return nil
		default:
			close(slowStarted)
			<-ctx.Done()
			close(slowCanceled)
			return ctx.Err()
		}
	})
	rec, took := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: syncer, Timeout: 2 * time.Second, WriteQuorum: 2}, []string{"f1", "f2", "f3"}, 1)
	r.Less(took, time.Second)
	r.Equal(http.StatusOK, rec.Code)
	r.Equal("2/4", rec.Header().Get(ReplicasHeader))
	select {
	case <-slowCanceled:
	case <-time.After(time.Second):
		t.Fatal("pending destination was not canceled after quorum")
	}
}

func TestSyncResult_PendingOnlyLogsAtDebug(t *testing.T) {
	r := require.New(t)
	var out bytes.Buffer
	logger := zerolog.New(&out).Level(zerolog.DebugLevel)
	result := syncResult{copies: 2, total: 3, quorum: 2, pending: []entity.UniversalReplicationID{replID("f2")}}
	result.logUnconfirmed(&logger, OnQuorumFailureFail)
	r.Contains(out.String(), `"level":"debug"`)
	r.Contains(out.String(), `"pending_destinations":"main->f2"`)
	r.Contains(out.String(), `"failed_destinations":""`)
}

func TestServeSync_QuorumCappedAtDestinations(t *testing.T) {
	r := require.New(t)
	fs := newFakeSyncer()
	rec, _ := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: time.Second, WriteQuorum: 10}, []string{"f1"}, 1)
	r.Equal(http.StatusOK, rec.Code)
	r.Equal("2/2", rec.Header().Get(ReplicasHeader))
}

func TestServeSync_MultipleTasksAllMustSucceed(t *testing.T) {
	r := require.New(t)
	fs := newFakeSyncer()
	rec, _ := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: time.Second}, []string{"f1"}, 3)
	r.Equal(http.StatusOK, rec.Code)
	r.Equal(3, fs.callsTo("f1"))
	r.Equal("2/2", rec.Header().Get(ReplicasHeader))
}

func TestServeSync_TimeoutRespected(t *testing.T) {
	r := require.New(t)
	fs := newFakeSyncer()
	fs.hang["f1"] = true
	rec, took := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: 200 * time.Millisecond}, []string{"f1"}, 1)
	r.Less(took, 3*time.Second)
	r.GreaterOrEqual(took, 150*time.Millisecond)
	r.Equal(http.StatusServiceUnavailable, rec.Code)
}

func TestServeSync_NoDestinationsNoSync(t *testing.T) {
	r := require.New(t)
	fs := newFakeSyncer()
	rec, _ := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: time.Second}, nil, 1)
	r.Equal(http.StatusOK, rec.Code)
	r.Empty(rec.Header().Get(ReplicasHeader))
	r.Zero(fs.totalCalls())
}

func TestServeSync_NonObjectTasksNotSynced(t *testing.T) {
	r := require.New(t)
	fs := newFakeSyncer()
	rt := &cannedRouter{tasks: []tasks.ReplicationTask{&tasks.ObjSyncACLPayload{}}}
	h := serve(rt, &failingReplSvc{}, false, ReplicationOptions{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: time.Second})
	ctx := xctx.SetRoutingPolicy(xctx.SetReplications(zerolog.Nop().WithContext(context.Background()), []entity.UniversalReplicationID{replID("f1")}), "main")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/b/o?acl", nil).WithContext(ctx))
	r.Equal(http.StatusOK, rec.Code)
	r.Empty(rec.Header().Get(ReplicasHeader))
	r.Zero(fs.totalCalls())
}

func TestServeSync_RecordFailureSkipsSync(t *testing.T) {
	r := require.New(t)
	fs := newFakeSyncer()
	rec, _ := runSyncServe(t, &failingReplSvc{failFirst: -1}, SyncOptions{Enabled: true, Syncer: fs, Timeout: time.Second}, []string{"f1"}, 1)
	r.Equal(http.StatusServiceUnavailable, rec.Code)
	r.Contains(rec.Body.String(), "could not be recorded")
	r.Zero(fs.totalCalls())
}
