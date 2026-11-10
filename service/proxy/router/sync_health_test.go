package router

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/health"
	"github.com/hieunyit/tutti/pkg/tasks"
)

func openBreaker(reg *health.Registry, storage string) {
	for range 3 {
		reg.Observe(storage, &net.OpError{Op: "dial", Err: errors.New("refused")})
	}
}

func newHealth() *health.Registry {
	return health.NewRegistry(health.Config{FailureThreshold: 3, OpenDuration: time.Hour}, health.WithLogger(zerolog.Nop()))
}

func TestServeSync_OpenBreakerDestinationSkipped_QuorumStillReachable(t *testing.T) {
	r := require.New(t)
	reg := newHealth()
	openBreaker(reg, "f2")
	fs := newFakeSyncer()
	fs.hang["f2"] = true // would block until timeout if attempted
	rec, took := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: 5 * time.Second, WriteQuorum: 2, Health: reg}, []string{"f1", "f2"}, 1)
	r.Less(took, time.Second)
	r.Equal(http.StatusOK, rec.Code)
	r.Equal("2/3", rec.Header().Get(ReplicasHeader))
	r.Equal(1, fs.callsTo("f1"))
	r.Zero(fs.callsTo("f2"), "open destination must not be attempted")
	r.True(reg.Available("f1"))
}

func TestServeSync_OpenBreaker_QuorumUnreachable_FailsFastWithoutSyncing(t *testing.T) {
	r := require.New(t)
	reg := newHealth()
	openBreaker(reg, "f2")
	fs := newFakeSyncer()
	// quorum = all (3 copies) but f2 is down: nothing is attempted
	rec, took := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: 5 * time.Second, Health: reg}, []string{"f1", "f2"}, 1)
	r.Less(took, time.Second)
	r.Equal(http.StatusServiceUnavailable, rec.Code)
	r.Contains(rec.Body.String(), "quorum")
	r.Zero(fs.totalCalls())
}

func TestServeSync_OpenBreaker_QuorumUnreachable_Accept(t *testing.T) {
	r := require.New(t)
	reg := newHealth()
	openBreaker(reg, "f2")
	fs := newFakeSyncer()
	rec, _ := runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: 5 * time.Second, Health: reg, OnQuorumFailure: OnQuorumFailureAccept}, []string{"f1", "f2"}, 1)
	r.Equal(http.StatusOK, rec.Code)
	r.Equal("1/3", rec.Header().Get(ReplicasHeader))
	r.Zero(fs.totalCalls())
}

func TestServeSync_BudgetTimeoutDoesNotOpenBreaker(t *testing.T) {
	r := require.New(t)
	reg := newHealth()
	fs := newFakeSyncer()
	fs.hang["f1"] = true
	// the destination exceeds our own sync budget every time: no storage outcome is reported
	for range 12 {
		runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: fs, Timeout: 20 * time.Millisecond, Health: reg}, []string{"f1"}, 1)
	}
	time.Sleep(50 * time.Millisecond)
	r.True(reg.Available("f1"), "budget timeouts must not open the breaker")
	r.Equal(12, fs.callsTo("f1"))
}

func TestServeSync_RealDestinationErrorsOpenBreaker(t *testing.T) {
	for name, syncErr := range map[string]error{
		"connection refused": syscall.ECONNREFUSED,
		// a storage call timeout that is not our budget (budget is 5s here)
		"per-call timeout": fmt.Errorf("storage call: %w", context.DeadlineExceeded),
	} {
		t.Run(name, func(t *testing.T) {
			r := require.New(t)
			reg := newHealth()
			sy := syncerFunc(func(context.Context, tasks.ObjectSyncPayload) error { return syncErr })
			for range 12 {
				runSyncServe(t, &failingReplSvc{}, SyncOptions{Enabled: true, Syncer: sy, Timeout: 5 * time.Second, Health: reg}, []string{"f1"}, 1)
			}
			r.Eventually(func() bool { return !reg.Available("f1") }, time.Second, 10*time.Millisecond)
		})
	}
}
