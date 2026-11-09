package router

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
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
	r.Empty(fs.calls)
}

func TestServeSync_AllSucceed(t *testing.T) {
	r := require.New(t)
	fs := newFakeSyncer()
	rec, _ := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: time.Second}, []string{"f1"}, 1)
	r.Equal(http.StatusOK, rec.Code)
	r.Equal("storage-ok", rec.Body.String())
	r.Equal("2/2", rec.Header().Get(ReplicasHeader))
	r.Equal(1, fs.calls["f1"])
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
	r.Equal(3, fs.calls["f1"])
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
	r.Empty(fs.calls)
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
	r.Empty(fs.calls)
}

func TestServeSync_RecordFailureSkipsSync(t *testing.T) {
	r := require.New(t)
	fs := newFakeSyncer()
	rec, _ := runSyncServe(t, &failingReplSvc{failFirst: -1}, SyncOptions{Enabled: true, Syncer: fs, Timeout: time.Second}, []string{"f1"}, 1)
	r.Equal(http.StatusServiceUnavailable, rec.Code)
	r.Contains(rec.Body.String(), "could not be recorded")
	r.Empty(fs.calls)
}
