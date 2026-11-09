package health

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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
	var inFlight atomic.Bool
	tick := func() {
		if !inFlight.CompareAndSwap(false, true) {
			return // previous probe is still running: skip this tick
		}
		go func() {
			defer inFlight.Store(false)
			pctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- t.Probe(pctx) }()
			var err error
			select {
			case err = <-done:
			case <-pctx.Done():
				// probe ignored its context: the bound still applies
				err = pctx.Err()
			}
			if errors.Is(err, ErrSkipProbe) || ctx.Err() != nil {
				return
			}
			r.Observe(t.Storage, err)
		}()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	tick()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick()
		}
	}
}
