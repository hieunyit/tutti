package health

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mclient "github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

var errNet = &net.OpError{Op: "dial", Err: errors.New("connection refused")}

func newTest(buf *bytes.Buffer) (*Registry, *fakeClock) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	return NewRegistry(Config{FailureThreshold: 3, OpenDuration: 30 * time.Second},
		withClock(clk.now), WithLogger(zerolog.New(buf))), clk
}

func TestClassify(t *testing.T) {
	r := require.New(t)
	r.Equal(Success, Classify(nil))
	r.Equal(Success, Classify(mclient.ErrorResponse{Code: "NoSuchKey", StatusCode: 404}))
	r.Equal(Success, Classify(mclient.ErrorResponse{Code: "AccessDenied", StatusCode: 403}))
	r.Equal(Failure, Classify(mclient.ErrorResponse{Code: "SlowDown", StatusCode: 503}))
	r.Equal(Failure, Classify(mclient.ErrorResponse{StatusCode: 500}))
	r.Equal(Failure, Classify(errNet))
	r.Equal(Failure, Classify(context.DeadlineExceeded))
	r.Equal(Neutral, Classify(context.Canceled))
	r.Equal(Neutral, Classify(errors.New("some local error")))
}

func TestBreaker_OpensAfterThreshold(t *testing.T) {
	r := require.New(t)
	var buf bytes.Buffer
	reg, _ := newTest(&buf)
	reg.Observe("s", errNet)
	reg.Observe("s", errNet)
	r.True(reg.Allow("s"))
	reg.Observe("s", errNet)
	r.False(reg.Allow("s"))
	r.False(reg.Available("s"))
	r.True(reg.Allow("other"))
	st := reg.Snapshot()
	r.Len(st, 1)
	r.False(st[0].Available)
	r.Equal(3, st[0].ConsecutiveFailures)
	r.NotEmpty(st[0].LastError)
}

func TestBreaker_4xxDoesNotCount(t *testing.T) {
	r := require.New(t)
	var buf bytes.Buffer
	reg, _ := newTest(&buf)
	for range 10 {
		reg.Observe("s", errNet)
		reg.Observe("s", errNet)
		reg.Observe("s", mclient.ErrorResponse{Code: "NoSuchKey", StatusCode: 404})
	}
	r.True(reg.Allow("s"))
	r.Equal(0, reg.Snapshot()[0].ConsecutiveFailures)
	// canceled requests neither count nor reset
	reg.Observe("s", errNet)
	reg.Observe("s", context.Canceled)
	r.Equal(1, reg.Snapshot()[0].ConsecutiveFailures)
}

func TestBreaker_HalfOpenSingleTrial(t *testing.T) {
	r := require.New(t)
	var buf bytes.Buffer
	reg, clk := newTest(&buf)
	for range 3 {
		reg.Observe("s", errNet)
	}
	clk.add(29 * time.Second)
	r.False(reg.Allow("s"))
	clk.add(2 * time.Second)
	r.True(reg.Available("s"))
	r.True(reg.Allow("s"), "first trial")
	r.False(reg.Allow("s"), "only one trial")
	r.False(reg.Available("s"))
	// failed trial re-opens
	reg.Observe("s", errNet)
	r.False(reg.Allow("s"))
	clk.add(31 * time.Second)
	r.True(reg.Allow("s"))
	// successful trial closes
	reg.Observe("s", nil)
	r.True(reg.Allow("s"))
	r.True(reg.Allow("s"))
	r.True(reg.Snapshot()[0].Available)
}

func TestBreaker_NeutralReleasesTrial(t *testing.T) {
	r := require.New(t)
	var buf bytes.Buffer
	reg, clk := newTest(&buf)
	for range 3 {
		reg.Observe("s", errNet)
	}
	clk.add(31 * time.Second)
	r.True(reg.Allow("s"))
	reg.Observe("s", context.Canceled)
	r.True(reg.Allow("s"))
}

func TestBreaker_TransitionsLoggedOnce(t *testing.T) {
	r := require.New(t)
	var buf bytes.Buffer
	var changes []bool
	clk := &fakeClock{t: time.Unix(1000, 0)}
	reg := NewRegistry(Config{FailureThreshold: 2, OpenDuration: time.Second}, withClock(clk.now),
		WithLogger(zerolog.New(&buf)), WithOnChange(func(_ string, a bool) { changes = append(changes, a) }))
	for range 10 {
		reg.Observe("s", errNet)
	}
	r.Equal(1, strings.Count(buf.String(), "opened"), buf.String())
	clk.add(2 * time.Second)
	reg.Allow("s")
	reg.Observe("s", errNet) // failed trial: still open, no new log
	r.Equal(1, strings.Count(buf.String(), "opened"))
	for range 5 {
		reg.Observe("s", nil)
	}
	r.Equal(1, strings.Count(buf.String(), "closed"), buf.String())
	r.Equal([]bool{false, true}, changes)
	r.Len(strings.Split(strings.TrimSpace(buf.String()), "\n"), 2)
}

func TestRunChecks_PeriodNotStretchedAndFailuresCounted(t *testing.T) {
	r := require.New(t)
	var buf bytes.Buffer
	reg := NewRegistry(Config{FailureThreshold: 3, OpenDuration: time.Hour}, WithLogger(zerolog.New(&buf)))
	var calls atomic.Int32
	probe := func(ctx context.Context) error {
		calls.Add(1)
		time.Sleep(time.Second) // ignores ctx and blocks longer than the timeout
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		reg.RunChecks(ctx, Target{Storage: "s", Interval: 100 * time.Millisecond, Probe: probe})
		close(done)
	}()
	// timeout is interval/2 = 50ms; a probe is started at ticks, never overlapping
	r.Eventually(func() bool { return !reg.Allow("s") }, 2*time.Second, 10*time.Millisecond)
	cancel()
	<-done
	r.GreaterOrEqual(int(calls.Load()), 3)
	// a stretched period (waiting 1s per probe) could not reach 3 calls in well under 1s
	st := reg.Snapshot()[0]
	r.False(st.Available)
	r.Contains(st.LastError, "deadline")
}

func TestRunChecks_SuccessAndSkip(t *testing.T) {
	r := require.New(t)
	reg := NewRegistry(Config{FailureThreshold: 1, OpenDuration: time.Hour}, WithLogger(zerolog.Nop()))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go reg.RunChecks(ctx,
		Target{Storage: "ok", Interval: 20 * time.Millisecond, Probe: func(context.Context) error { return nil }},
		Target{Storage: "skip", Interval: 20 * time.Millisecond, Probe: func(context.Context) error { return ErrSkipProbe }},
		Target{Storage: "down", Interval: 20 * time.Millisecond, Probe: func(context.Context) error { return errNet }},
	)
	r.Eventually(func() bool { return !reg.Available("down") }, time.Second, 10*time.Millisecond)
	r.True(reg.Available("ok"))
	r.True(reg.Available("skip"))
	r.Equal(0, snapshotOf(reg, "skip").ConsecutiveFailures)
}

func snapshotOf(reg *Registry, name string) Status {
	for _, s := range reg.Snapshot() {
		if s.Name == name {
			return s
		}
	}
	return Status{}
}
