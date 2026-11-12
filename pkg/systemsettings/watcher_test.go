package systemsettings

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/features"
)

func TestSystemSettings_Watch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	// Initial features: all false
	features.Set(&features.Config{
		Tagging:   false,
		ACL:       false,
		Lifecycle: false,
	})
	require.False(t, features.Tagging(ctx))

	// Start watcher
	Watch(ctx, rdb, nil, zerolog.Nop())

	// Give watcher goroutine a moment to subscribe
	time.Sleep(50 * time.Millisecond)

	// Simulate updated settings stored in Redis
	newSettings := controlplane.SystemSettings{
		ID: "system",
		Features: controlplane.SystemFeaturesSettings{
			Tagging:   true,
			ACL:       true,
			Lifecycle: true,
		},
		Worker: controlplane.SystemWorkerSettings{
			Concurrency: 40,
		},
	}
	raw, err := json.Marshal(newSettings)
	require.NoError(t, err)

	require.NoError(t, rdb.Set(ctx, pg.KeySystemSettings, string(raw), 0).Err())
	require.NoError(t, rdb.Incr(ctx, pg.KeySystemSettingsVersion).Err())
	require.NoError(t, rdb.Publish(ctx, pg.SystemSettingsChannel, "2").Err())

	// Wait for watcher to apply features
	require.Eventually(t, func() bool {
		return features.Tagging(ctx) && features.ACL(ctx) && features.Lifecycle(ctx)
	}, 2*time.Second, 50*time.Millisecond)
}

func TestThrottledReader(t *testing.T) {
	ctx := context.Background()
	limiter := rate.NewLimiter(rate.Limit(10000), 5000)
	data := make([]byte, 1000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	reader := bytes.NewReader(data)
	throttled := NewThrottledReader(ctx, reader, limiter)

	buf := make([]byte, 500)
	n, err := throttled.Read(buf)
	require.NoError(t, err)
	require.Equal(t, 500, n)
	require.Equal(t, data[:500], buf)

	seeker, ok := throttled.(io.Seeker)
	require.True(t, ok)
	pos, err := seeker.Seek(0, io.SeekStart)
	require.NoError(t, err)
	require.Equal(t, int64(0), pos)
}

func TestBufferBudget(t *testing.T) {
	ctx := context.Background()
	settings := controlplane.SystemSettings{
		Worker: controlplane.SystemWorkerSettings{
			BufferBudgetBytes: 1024,
		},
	}
	ApplyWorkerLimiting(settings)

	release, err := AcquireBufferBudget(ctx, 512)
	require.NoError(t, err)
	require.NotNil(t, release)
	release()
	// Idempotent release
	release()
}

func TestBufferBudget_OversizeFailsImmediatelyAndDoesNotBlockSmallWaiters(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ApplyWorkerLimiting(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{BufferBudgetBytes: 1000}})
	t.Cleanup(func() {
		ApplyWorkerLimiting(controlplane.SystemSettings{})
	})

	// Fill most of the budget so later small requests must wait.
	releaseBig, err := AcquireBufferBudget(ctx, 900)
	require.NoError(t, err)

	// Oversize request errors immediately with the typed error.
	start := time.Now()
	_, err = AcquireBufferBudget(ctx, 5000)
	var oversize *BufferExceedsBudgetError
	require.ErrorAs(t, err, &oversize)
	require.EqualValues(t, 5000, oversize.Requested)
	require.EqualValues(t, 1000, oversize.Budget)
	require.Less(t, time.Since(start), time.Second)

	// A small waiter queued after it is granted once capacity frees up.
	type res struct {
		rel func()
		err error
	}
	ch := make(chan res, 1)
	go func() {
		rel, err := AcquireBufferBudget(ctx, 500)
		ch <- res{rel, err}
	}()
	time.Sleep(50 * time.Millisecond)
	releaseBig()
	select {
	case r := <-ch:
		require.NoError(t, r.err)
		r.rel()
	case <-time.After(3 * time.Second):
		t.Fatal("small waiter blocked behind oversize request")
	}

	// Waiter queued, then budget shrinks below its size: fails with the typed error and does not block the next one.
	releaseHold, err := AcquireBufferBudget(ctx, 900)
	require.NoError(t, err)
	bigCh := make(chan error, 1)
	go func() {
		_, err := AcquireBufferBudget(ctx, 800)
		bigCh <- err
	}()
	time.Sleep(50 * time.Millisecond)
	smallCh := make(chan error, 1)
	go func() {
		rel, err := AcquireBufferBudget(ctx, 50)
		if rel != nil {
			defer rel()
		}
		smallCh <- err
	}()
	time.Sleep(50 * time.Millisecond)
	ApplyWorkerLimiting(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{BufferBudgetBytes: 500}})
	require.ErrorAs(t, <-bigCh, &oversize)
	releaseHold()
	require.NoError(t, <-smallCh)
}
