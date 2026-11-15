package store

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestMultipartResumeStore(t *testing.T) {
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	s := NewMultipartResumeStore(c)
	ctx := t.Context()
	key := "p:mpu:{r:1}:abc"

	b, err := s.Get(ctx, key)
	require.NoError(t, err)
	require.Nil(t, b, "a missing key is (nil, nil)")

	require.NoError(t, s.Set(ctx, key, []byte(`{"uploadId":"u"}`), 7*24*time.Hour))
	b, err = s.Get(ctx, key)
	require.NoError(t, err)
	require.JSONEq(t, `{"uploadId":"u"}`, string(b))
	require.Equal(t, 7*24*time.Hour, mr.TTL(key))

	require.NoError(t, s.Delete(ctx, key))
	b, err = s.Get(ctx, key)
	require.NoError(t, err)
	require.Nil(t, b)
}
