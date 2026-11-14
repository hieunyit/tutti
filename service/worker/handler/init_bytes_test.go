package handler

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/store"
)

func TestInitProgressThrottle(t *testing.T) {
	r := require.New(t)
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	st := store.NewInitBytesStore(rc)
	ctx := context.Background()

	old := initProgressInterval
	initProgressInterval = 50 * time.Millisecond
	t.Cleanup(func() { initProgressInterval = old })

	p := &initProgress{st: st, id: "u:a:b", gen: 1, field: store.ProgressField("big", ""), size: 1000, done: make(chan struct{})}
	r.NoError(st.AddTotal(ctx, "u:a:b", 1, 1000))
	lctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	go p.loop(lctx, ctx)

	done := func() int64 {
		got, err := st.Get(ctx, "u:a:b", 1, time.Now())
		r.NoError(err)
		return got.Done
	}
	// many reports within one interval: nothing is written until the tick, then only the latest value
	for i := int64(1); i <= 100; i++ {
		p.report(i)
	}
	p.report(50) // out of order, ignored
	r.Zero(done())
	r.Eventually(func() bool { return done() == 100 }, 2*time.Second, 10*time.Millisecond)

	// unchanged value: no further writes
	mr.SetTTL("p:repl_init_bytes:{u:a:b:1}:prog", time.Minute)
	time.Sleep(3 * initProgressInterval)
	r.Equal(time.Minute, mr.TTL("p:repl_init_bytes:{u:a:b:1}:prog"), "no write for an unchanged value")

	// stop waits for the loop; afterwards nothing is written, even for a new value
	p.stop()
	p.report(900)
	time.Sleep(3 * initProgressInterval)
	r.EqualValues(100, done())
	p.clear(ctx)
	r.Zero(done())

	var nilProgress *initProgress
	nilProgress.stop()
	nilProgress.clear(ctx)
}
