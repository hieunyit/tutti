package health

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	DefaultProbeInterval = 10 * time.Second
	maxProbeTimeout      = 3 * time.Second
)

// ErrSkipProbe makes the checker ignore a probe round (e.g. no credentials to probe with).
var ErrSkipProbe = errors.New("skip probe")

// Target is one storage to probe periodically.
type Target struct {
	// Probe must be a cheap authenticated call. Its context carries the probe timeout.
	Probe   func(ctx context.Context) error
	Storage string
	// Interval is the fixed probe period. Default 10s.
	Interval time.Duration
}

func probeTimeout(interval time.Duration) time.Duration {
	return min(maxProbeTimeout, interval/2)
}

// RunChecks probes every target on a fixed-rate ticker and feeds the registry.
// A probe still running when the next tick fires makes that tick skipped, so the period never stretches.
// It blocks until ctx is done.
func (r *Registry) RunChecks(ctx context.Context, targets ...Target) {
	var wg sync.WaitGroup
	for _, t := range targets {
		r.Register(t.Storage)
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.runTarget(ctx, t)
		}()
	}
	wg.Wait()
}

func (r *Registry) runTarget(ctx context.Context, t Target) {
	interval := t.Interval
	if interval <= 0 {
		interval = DefaultProbeInterval
	}
	timeout := probeTimeout(interval)
	var result <-chan error
	var deadline <-chan struct{}
	var cancelProbe context.CancelFunc
	timedOut := false
	defer func() {
		if cancelProbe != nil {
			cancelProbe()
		}
	}()
	start := func() {
		pctx, cancel := context.WithTimeout(ctx, timeout)
		cancelProbe = cancel
		done := make(chan error, 1)
		result = done
		deadline = pctx.Done()
		timedOut = false
		go func() { done <- t.Probe(pctx) }()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	start()
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-result:
			cancelProbe()
			result, deadline = nil, nil
			if !timedOut && !errors.Is(err, ErrSkipProbe) && ctx.Err() == nil {
				r.ObserveProbe(t.Storage, err)
			}
		case <-deadline:
			deadline = nil
			timedOut = true
			if ctx.Err() == nil {
				r.Observe(t.Storage, context.DeadlineExceeded)
			}
		case <-ticker.C:
			if result == nil {
				start()
			} else if timedOut && ctx.Err() == nil {
				// Keep reporting a stuck probe as unavailable without creating
				// more goroutines. A late result must not count as a success.
				r.Observe(t.Storage, context.DeadlineExceeded)
			}
		}
	}
}
