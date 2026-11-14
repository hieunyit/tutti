package handler

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/service/worker/copy"
)

// Replication activity feed (in-flight copies and the event log shown in the UI detail view).
//
// Writes are best effort: they use a short timeout detached from the task context, log at debug level on
// failure and never change the result of a copy.
//
// Sampling: the initial replication flow records every object. The live-event flow can be very high volume, so
// per replication at most liveActivityPerSecond objects per second are tracked (in-flight entry plus started and
// done/skipped events). Failures and retries are always recorded, whether the object was sampled or not.

const (
	activityWriteTimeout  = 250 * time.Millisecond
	liveActivityPerSecond = 20
)

var liveActivityLimiter sync.Map // replication id string -> *activityBucket

type activityBucket struct {
	mu     sync.Mutex
	second int64
	used   int
}

func (b *activityBucket) allow(now time.Time, perSecond int) bool {
	sec := now.Unix()
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.second != sec {
		b.second, b.used = sec, 0
	}
	if b.used >= perSecond {
		return false
	}
	b.used++
	return true
}

func allowLiveActivity(id string, now time.Time) bool {
	v, ok := liveActivityLimiter.Load(id)
	if !ok {
		v, _ = liveActivityLimiter.LoadOrStore(id, &activityBucket{})
	}
	return v.(*activityBucket).allow(now, liveActivityPerSecond)
}

func (s *svc) activity() *store.ActivityStore {
	if rc := s.redisClient(); rc != nil {
		return store.NewActivityStore(rc)
	}
	return nil
}

// activityTracker follows one object copy attempt. A nil tracker is valid and does nothing.
type activityTracker struct {
	st      *store.ActivityStore
	id      string
	gen     int64
	key     string
	ver     string
	size    int64
	flow    string
	attempt int
	start   time.Time
	sampled bool // in-flight entry and started event were written (always true for the init flow)
}

func (s *svc) activityBegin(ctx context.Context, id entity.UniversalReplicationID, generation int64, key, versionID string, size int64, flow string) *activityTracker {
	st := s.activity()
	if st == nil {
		return nil
	}
	now := time.Now()
	t := &activityTracker{st: st, id: id.AsString(), gen: generation, key: key, ver: versionID, size: size, flow: flow, start: now}
	retried, _ := asynq.GetRetryCount(ctx)
	t.attempt = retried + 1
	t.sampled = flow == store.ActivityFlowInit || allowLiveActivity(t.id, now)
	if !t.sampled {
		return t
	}
	wctx, cancel := activityCtx(ctx)
	defer cancel()
	err := st.Begin(wctx, t.id, t.gen,
		store.ActivityInFlight{Key: key, VersionID: versionID, Size: size, Flow: flow, Attempt: t.attempt},
		&store.ActivityEvent{Level: store.ActivityInfo, Type: store.ActivityStarted, Key: key, VersionID: versionID, Size: size, Flow: flow, Attempt: t.attempt})
	if err != nil {
		t.sampled = false
		zerolog.Ctx(ctx).Debug().Err(err).Msg("unable to record replication activity")
	}
	return t
}

func activityCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), activityWriteTimeout)
}

// finish records the result of the attempt and removes the in-flight entry.
func (t *activityTracker) finish(ctx context.Context, err error, out *copy.Outcome) {
	if t == nil {
		return
	}
	ev := store.ActivityEvent{Key: t.key, VersionID: t.ver, Size: t.size, Flow: t.flow, Attempt: t.attempt}
	d := time.Since(t.start)
	switch {
	case err != nil:
		retried, _ := asynq.GetRetryCount(ctx)
		maxRetry, _ := asynq.GetMaxRetry(ctx)
		ev.Error = err.Error()
		if !errors.Is(err, asynq.SkipRetry) && retried < maxRetry {
			ev.Level, ev.Type = store.ActivityWarn, store.ActivityRetry
		} else {
			ev.Level, ev.Type = store.ActivityError, store.ActivityFailed
		}
	case out != nil && out.Missing:
		ev.Level, ev.Type, ev.Error = store.ActivityWarn, store.ActivitySkipped, "object missing in source"
	case out == nil || !out.Called || out.Skipped:
		if t.flow == store.ActivityFlowLive && (out == nil || !out.Called) {
			// nothing was copied (version up to date or a delete): too noisy for the live log
			t.untrack(ctx)
			return
		}
		ev.Level, ev.Type = store.ActivityInfo, store.ActivitySkipped
		if out != nil && out.Skipped {
			ev.Error = "already up to date in destination"
		}
	default:
		ev.Level, ev.Type = store.ActivityInfo, store.ActivityDone
		ev.DurationMs = d.Milliseconds()
		if out.Bytes > 0 {
			ev.Size = out.Bytes
		}
		if d > 0 {
			ev.BytesPerSecond = int64(float64(ev.Size) / d.Seconds())
		}
	}
	if ev.Type == store.ActivityDone || ev.Type == store.ActivitySkipped {
		ev.DurationMs = d.Milliseconds()
	}
	if err == nil && !t.sampled {
		return // live flow, not sampled: successes are dropped, only errors are always kept
	}
	wctx, cancel := activityCtx(ctx)
	defer cancel()
	if werr := t.st.End(wctx, t.id, t.gen, t.key, t.ver, t.sampled, &ev); werr != nil {
		zerolog.Ctx(ctx).Debug().Err(werr).Msg("unable to record replication activity")
	}
}

func (t *activityTracker) untrack(ctx context.Context) {
	if !t.sampled {
		return
	}
	wctx, cancel := activityCtx(ctx)
	defer cancel()
	if err := t.st.End(wctx, t.id, t.gen, t.key, t.ver, true, nil); err != nil {
		zerolog.Ctx(ctx).Debug().Err(err).Msg("unable to record replication activity")
	}
}
