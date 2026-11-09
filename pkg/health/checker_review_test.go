package health

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReviewProbeDoesNotOverlapAfterTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	defer close(release)
	defer cancel()
	var started atomic.Int32
	overlap := make(chan struct{}, 1)
	reg := NewRegistry(Config{})
	done := make(chan struct{})
	go func() {
		reg.RunChecks(ctx, Target{Storage: "main", Interval: 20 * time.Millisecond, Probe: func(context.Context) error {
			if started.Add(1) > 1 {
				select {
				case overlap <- struct{}{}:
				default:
				}
			}
			<-release
			return nil
		}})
		close(done)
	}()
	select {
	case <-overlap:
		t.Error("second probe started while first probe remained blocked after timeout")
	case <-time.After(150 * time.Millisecond):
	}
	cancel()
	<-done
	if reg.Available("main") {
		t.Error("a persistently timed-out probe must open the circuit breaker")
	}
}

func TestReviewProbeRecoversAfterBlockedCallReturns(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release := make(chan struct{})
	var calls atomic.Int32
	var releaseOnce sync.Once
	reg := NewRegistry(Config{FailureThreshold: 1})
	done := make(chan struct{})
	go func() {
		reg.RunChecks(ctx, Target{Storage: "main", Interval: 20 * time.Millisecond, Probe: func(context.Context) error {
			if calls.Add(1) == 1 {
				<-release
			}
			return nil
		}})
		close(done)
	}()
	// Release the blocked goroutine even if the assertion aborts the test.
	defer func() { releaseOnce.Do(func() { close(release) }); cancel(); <-done }()
	require.Eventually(t, func() bool { return !reg.Available("main") }, time.Second, time.Millisecond)
	releaseOnce.Do(func() { close(release) })
	require.Eventually(t, func() bool { return calls.Load() >= 2 && reg.Available("main") }, time.Second, time.Millisecond)
}
