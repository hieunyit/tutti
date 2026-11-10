// Copyright 2025 Clyso GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package policy

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestNextBackoff_DoublesAndCaps(t *testing.T) {
	d := nextBackoff(resubscribeMin)
	require.Equal(t, 1*time.Second, d)
	d = nextBackoff(d)
	require.Equal(t, 2*time.Second, d)
	for i := 0; i < 20; i++ {
		d = nextBackoff(d)
	}
	require.Equal(t, resubscribeMax, d, "backoff must cap at the max")
	require.Equal(t, resubscribeMax, nextBackoff(resubscribeMax))
}

func TestSnapshot_ResubscribeReceivesNotifications(t *testing.T) {
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = c.Close() })
	ctx := t.Context()

	s := &snapshot{rc: c}
	ps, err := s.resubscribe(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ps.Close() })

	msgs := ps.Channel()
	require.NoError(t, c.Publish(ctx, SnapshotChannel, "v2").Err())
	select {
	case m := <-msgs:
		require.Equal(t, SnapshotChannel, m.Channel)
	case <-time.After(3 * time.Second):
		t.Fatal("resubscribed pubsub did not receive the change notification")
	}
}

func TestSnapshot_ResubscribeFailsWhenRedisDown(t *testing.T) {
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1, DialTimeout: 200 * time.Millisecond})
	t.Cleanup(func() { _ = c.Close() })
	mr.Close()

	s := &snapshot{rc: c}
	_, err := s.resubscribe(t.Context())
	require.Error(t, err, "a resubscribe while Redis is down must fail so the loop backs off")
}
