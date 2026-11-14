package handler

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/service/worker/copy"
)

// Initial replication byte counters are best effort: a Redis hiccup must never fail or retry an object copy.

func (s *svc) initBytes() *store.InitBytesStore {
	if rc := s.redisClient(); rc != nil {
		return store.NewInitBytesStore(rc)
	}
	return nil
}

// addInitTotalBytes registers sizes of objects accepted by the broker as copy tasks.
func (s *svc) addInitTotalBytes(ctx context.Context, id entity.UniversalReplicationID, generation int64, sizes []int64) {
	var total int64
	for _, n := range sizes {
		total += n
	}
	st := s.initBytes()
	if st == nil || total <= 0 {
		return
	}
	if err := st.AddTotal(ctx, id.AsString(), generation, total); err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).Msg("unable to update initial replication total bytes")
	}
}

// addInitDoneBytes registers an object whose initial copy task completed successfully. The same script drops
// the object's in-progress bytes, so they are never counted twice.
func (s *svc) addInitDoneBytes(ctx context.Context, id entity.UniversalReplicationID, generation int64, key, versionID string, size int64) {
	st := s.initBytes()
	if st == nil {
		return
	}
	wctx, cancel := activityCtx(ctx)
	defer cancel()
	if err := st.AddDone(wctx, id.AsString(), generation, max(size, 0), store.ProgressField(key, versionID), time.Now()); err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).Msg("unable to update initial replication done bytes")
	}
}

// initProgressInterval is the minimum time between two progress writes of one object.
var initProgressInterval = time.Second

// initProgress publishes the bytes transferred so far of one running init copy. The copy only stores the
// latest value (atomic, never blocks); a goroutine writes it at most once per interval and only when it changed.
// Failures are logged at debug level and ignored.
type initProgress struct {
	st     *store.InitBytesStore
	id     string
	gen    int64
	field  string
	size   int64
	latest atomic.Int64
	cancel context.CancelFunc
	done   chan struct{}
}

// startInitProgress resets the progress of the object (a retry restarts from 0) and starts publishing it.
// Call stop before the attempt result is recorded. The returned context reports progress from CopyObject.
func (s *svc) startInitProgress(ctx context.Context, id entity.UniversalReplicationID, generation int64, key, versionID string, size int64) (context.Context, *initProgress) {
	st := s.initBytes()
	if st == nil {
		return ctx, nil
	}
	p := &initProgress{st: st, id: id.AsString(), gen: generation, field: store.ProgressField(key, versionID), size: size, done: make(chan struct{})}
	p.write(ctx, 0)
	var lctx context.Context
	lctx, p.cancel = context.WithCancel(context.WithoutCancel(ctx))
	go p.loop(lctx, ctx)
	return copy.WithProgress(ctx, p.report), p
}

// report keeps the highest value seen (parallel parts may report slightly out of order).
func (p *initProgress) report(n int64) {
	for {
		cur := p.latest.Load()
		if n <= cur || p.latest.CompareAndSwap(cur, n) {
			return
		}
	}
}

func (p *initProgress) write(ctx context.Context, n int64) {
	wctx, cancel := activityCtx(ctx)
	defer cancel()
	if err := p.st.SetProgress(wctx, p.id, p.gen, p.field, n, p.size, time.Now()); err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).Msg("unable to update initial replication progress")
	}
}

func (p *initProgress) loop(lctx, logCtx context.Context) {
	defer close(p.done)
	t := time.NewTicker(initProgressInterval)
	defer t.Stop()
	var written int64
	for {
		select {
		case <-lctx.Done():
			return
		case <-t.C:
			if n := p.latest.Load(); n != written {
				written = n
				p.write(logCtx, n)
			}
		}
	}
}

// stop ends publishing and waits for a write in flight, so no write can land after the result was recorded.
func (p *initProgress) stop() {
	if p == nil {
		return
	}
	p.cancel()
	<-p.done
}

// clear removes the in-progress bytes of an attempt that did not complete the object.
func (p *initProgress) clear(ctx context.Context) {
	if p == nil {
		return
	}
	wctx, cancel := activityCtx(ctx)
	defer cancel()
	if err := p.st.ClearProgress(wctx, p.id, p.gen, p.field); err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).Msg("unable to clear initial replication progress")
	}
}
