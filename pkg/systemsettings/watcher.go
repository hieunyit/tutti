package systemsettings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/features"
)

// BufferExceedsBudgetError is returned when a request is larger than the whole buffer budget; retrying cannot help.
type BufferExceedsBudgetError struct{ Requested, Budget int64 }

func (e *BufferExceedsBudgetError) Error() string {
	return fmt.Sprintf("requested buffer %d bytes exceeds buffer budget %d bytes", e.Requested, e.Budget)
}

type bufferWaiter struct {
	bytes        int64
	grantedBytes int64
	oversize     bool
	budget       int64
	ready        chan struct{}
}

var (
	currentSettings atomic.Pointer[controlplane.SystemSettings]
	bandwidthMu     sync.RWMutex
	bandwidthLimit  *rate.Limiter

	bufferPoolMu  sync.Mutex
	bufferTotal   int64
	bufferUsed    int64
	bufferWaiters []*bufferWaiter
)

func init() {
	def := controlplane.SystemSettings{
		Worker: controlplane.SystemWorkerSettings{
			Concurrency:          10,
			BacklogHighWatermark: 10000,
			BacklogLowWatermark:  5000,
			BandwidthLimitBytes:  0,                 // 0 = unlimited
			BufferBudgetBytes:    256 * 1024 * 1024, // 256 MiB
		},
	}
	ApplySettings(def)
}

// Current returns a snapshot of the active system settings.
func Current() controlplane.SystemSettings {
	p := currentSettings.Load()
	if p == nil {
		return controlplane.SystemSettings{}
	}
	return *p
}

// ApplySettings sets in-memory feature flags, bandwidth throttler, and buffer budget semaphore.
func ApplySettings(s controlplane.SystemSettings) {
	currentSettings.Store(&s)
	ApplyFeatures(s)
	ApplyWorkerLimiting(s)
}

// ApplyFeatures sets in-memory feature flags from SystemSettings.
func ApplyFeatures(s controlplane.SystemSettings) {
	features.Set(&features.Config{
		Tagging:           s.Features.Tagging,
		ACL:               s.Features.ACL,
		Lifecycle:         s.Features.Lifecycle,
		Policy:            s.Features.Policy,
		PreserveACLGrants: s.Features.PreserveACLGrants,
		Versioning:        s.Features.Versioning,
	})
}

// ApplyWorkerLimiting updates dynamic bandwidth rate limiter and buffer budget pool.
func ApplyWorkerLimiting(s controlplane.SystemSettings) {
	bandwidthMu.Lock()
	if s.Worker.BandwidthLimitBytes > 0 {
		burst := int(s.Worker.BandwidthLimitBytes / 2)
		if burst < 256*1024 {
			burst = 256 * 1024
		}
		if burst > 4*1024*1024 {
			burst = 4 * 1024 * 1024
		}
		if bandwidthLimit == nil {
			bandwidthLimit = rate.NewLimiter(rate.Limit(s.Worker.BandwidthLimitBytes), burst)
		} else {
			bandwidthLimit.SetLimit(rate.Limit(s.Worker.BandwidthLimitBytes))
			bandwidthLimit.SetBurst(burst)
		}
	} else {
		bandwidthLimit = nil
	}
	bandwidthMu.Unlock()

	bufferPoolMu.Lock()
	bufferTotal = s.Worker.BufferBudgetBytes
	notifyBufferWaitersLocked()
	bufferPoolMu.Unlock()
}

func notifyBufferWaitersLocked() {
	for len(bufferWaiters) > 0 {
		first := bufferWaiters[0]
		grantSize := first.bytes
		if bufferTotal > 0 {
			if grantSize > bufferTotal {
				// Waiter requested more than the new total budget: fail it now instead of blocking the queue head.
				bufferWaiters = bufferWaiters[1:]
				first.oversize = true
				first.budget = bufferTotal
				close(first.ready)
				continue
			}
			if bufferUsed+grantSize > bufferTotal {
				break
			}
		}
		bufferWaiters = bufferWaiters[1:]
		bufferUsed += grantSize
		first.grantedBytes = grantSize
		close(first.ready)
	}
}

// BandwidthLimiter returns the active token bucket rate limiter for bulk copying (or nil if unlimited).
func BandwidthLimiter() *rate.Limiter {
	bandwidthMu.RLock()
	defer bandwidthMu.RUnlock()
	return bandwidthLimit
}

// AcquireBufferBudget acquires estimated in-flight RAM buffer budget from the global pool.
// It returns a release closure and an error if context is cancelled.
// When the budget is reduced, existing reservations are not revoked, but new reservations
// will block until bufferUsed drops below the new budget (draining old reservations).
func AcquireBufferBudget(ctx context.Context, bytes int64) (func(), error) {
	if bytes <= 0 {
		return func() {}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	bufferPoolMu.Lock()
	if bufferTotal > 0 && bytes > bufferTotal {
		budget := bufferTotal
		bufferPoolMu.Unlock()
		return nil, &BufferExceedsBudgetError{Requested: bytes, Budget: budget}
	}

	if bufferTotal <= 0 || (len(bufferWaiters) == 0 && bufferUsed+bytes <= bufferTotal) {
		bufferUsed += bytes
		granted := bytes
		bufferPoolMu.Unlock()

		var released atomic.Bool
		return func() {
			if released.CompareAndSwap(false, true) {
				bufferPoolMu.Lock()
				bufferUsed -= granted
				notifyBufferWaitersLocked()
				bufferPoolMu.Unlock()
			}
		}, nil
	}

	w := &bufferWaiter{
		bytes: bytes,
		ready: make(chan struct{}),
	}
	bufferWaiters = append(bufferWaiters, w)
	bufferPoolMu.Unlock()

	select {
	case <-w.ready:
		if w.oversize {
			return nil, &BufferExceedsBudgetError{Requested: w.bytes, Budget: w.budget}
		}
		var released atomic.Bool
		return func() {
			if released.CompareAndSwap(false, true) {
				bufferPoolMu.Lock()
				bufferUsed -= w.grantedBytes
				notifyBufferWaitersLocked()
				bufferPoolMu.Unlock()
			}
		}, nil
	case <-ctx.Done():
		bufferPoolMu.Lock()
		select {
		case <-w.ready:
			if !w.oversize {
				bufferUsed -= w.grantedBytes
			}
			notifyBufferWaitersLocked()
			bufferPoolMu.Unlock()
			return nil, ctx.Err()
		default:
			for i, waiter := range bufferWaiters {
				if waiter == w {
					bufferWaiters = append(bufferWaiters[:i], bufferWaiters[i+1:]...)
					break
				}
			}
			notifyBufferWaitersLocked()
			bufferPoolMu.Unlock()
			return nil, ctx.Err()
		}
	}
}

// ThrottledReader wraps an io.Reader and throttles read throughput using rate.Limiter.
type ThrottledReader struct {
	reader  io.Reader
	ctx     context.Context
	limiter *rate.Limiter
}

func NewThrottledReader(ctx context.Context, r io.Reader, limiter *rate.Limiter) io.Reader {
	if limiter == nil {
		return r
	}
	return &ThrottledReader{
		reader:  r,
		ctx:     ctx,
		limiter: limiter,
	}
}

func (t *ThrottledReader) Read(p []byte) (int, error) {
	if t.limiter == nil {
		return t.reader.Read(p)
	}

	burst := t.limiter.Burst()
	chunk := p
	if burst > 0 && len(chunk) > burst {
		chunk = chunk[:burst]
	}

	n, err := t.reader.Read(chunk)
	if n > 0 {
		if waitErr := t.limiter.WaitN(t.ctx, n); waitErr != nil {
			return n, waitErr
		}
	}
	return n, err
}

func (t *ThrottledReader) Seek(offset int64, whence int) (int64, error) {
	if seeker, ok := t.reader.(io.Seeker); ok {
		return seeker.Seek(offset, whence)
	}
	return 0, errors.New("underlying reader does not implement io.Seeker")
}

// Load loads SystemSettings from PostgreSQL or Redis.
func Load(ctx context.Context, cpStore controlplane.Store, rdb redis.Cmdable) (*controlplane.SystemSettings, int64, error) {
	if cpStore != nil {
		v, err := cpStore.Repos().SystemSettings.Get(ctx, "system")
		if err == nil {
			return &v.Value, v.Version, nil
		}
		if !errors.Is(err, dom.ErrNotFound) && !controlplane.IsUnavailable(err) {
			return nil, 0, err
		}
	}
	if rdb != nil {
		raw, err := rdb.Get(ctx, pg.KeySystemSettings).Result()
		if err == nil && len(raw) > 0 {
			var s controlplane.SystemSettings
			if uerr := json.Unmarshal([]byte(raw), &s); uerr == nil {
				vInt, _ := rdb.Get(ctx, pg.KeySystemSettingsVersion).Int64()
				return &s, vInt, nil
			}
		}
	}
	return nil, 0, dom.ErrNotFound
}

// Watch subscribes to system settings update notifications in Redis and updates in-memory features live.
func Watch(ctx context.Context, rdb redis.UniversalClient, cpStore controlplane.Store, logger zerolog.Logger) {
	sub := rdb.Subscribe(ctx, pg.SystemSettingsChannel)
	go func() {
		defer sub.Close()
		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
				// Reload latest settings
				reloadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				s, v, err := Load(reloadCtx, cpStore, rdb)
				cancel()
				if err != nil {
					logger.Warn().Err(err).Msg("systemsettings watcher: failed to reload settings")
					continue
				}
				ApplySettings(*s)
				logger.Info().Int64("version", v).Msg("systemsettings watcher: dynamic features applied")
			}
		}
	}()
}
