package proxystatus

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/trafficstats"
)

func TestPublisher_SerializationAndTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	started := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	p := NewPublisher(c, "app1", 10*time.Second, func() Status {
		return Status{
			Address: "http://proxy", StartedAt: started,
			Storages: []StorageHealth{{Name: "main", Available: false, ConsecutiveFailures: 3, LastError: "boom"}},
			Settings: Settings{RequireSignedContentType: true, Failover: FailoverSettings{Enabled: true, After: time.Minute}},
		}
	}, zerolog.Nop())
	require.NoError(t, p.PublishOnce(context.Background()))

	require.Equal(t, 30*time.Second, mr.TTL(Key("app1")))
	raw, err := mr.Get(Key("app1"))
	require.NoError(t, err)
	var st Status
	require.NoError(t, json.Unmarshal([]byte(raw), &st))
	require.Equal(t, "app1", st.AppID)
	require.Equal(t, "http://proxy", st.Address)
	require.True(t, st.StartedAt.Equal(started))
	require.False(t, st.UpdatedAt.IsZero())
	require.Equal(t, "boom", st.Storages[0].LastError)
	require.True(t, st.Settings.Failover.Enabled)

	list, err := List(context.Background(), c)
	require.NoError(t, err)
	require.Len(t, list, 1)

	mr.FastForward(31 * time.Second)
	list, err = List(context.Background(), c)
	require.NoError(t, err)
	require.Empty(t, list)
}

func TestPublisher_StatsRoundTrip(t *testing.T) {
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	col := trafficstats.New()
	col.Request(trafficstats.ClassRead, "main")
	col.Latency(20 * time.Millisecond)
	col.BytesOut(42)
	snap := col.Snapshot()
	p := NewPublisher(c, "app1", 10*time.Second, func() Status { return Status{Stats: &snap} }, zerolog.Nop())
	require.NoError(t, p.PublishOnce(context.Background()))
	list, err := List(context.Background(), c)
	require.NoError(t, err)
	require.Len(t, list, 1)
	got := list[0].Stats
	require.NotNil(t, got)
	require.True(t, got.CollectedAt.Equal(snap.CollectedAt))
	require.True(t, got.StartedAt.Equal(snap.StartedAt))
	require.EqualValues(t, 1, got.Requests.ByStorage["main"])
	require.EqualValues(t, 42, got.BytesOut)
	require.Equal(t, snap.Latency.Buckets, got.Latency.Buckets)

	raw, _ := mr.Get(Key("app1"))
	require.Contains(t, raw, `"stats":{"collectedAt"`)
}
