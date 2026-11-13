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

package replscope

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newTestLocker(t *testing.T) Locker {
	mr := miniredis.RunT(t)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return NewRedisLocker(c)
}

func TestLockerMutualExclusion(t *testing.T) {
	l := newTestLocker(t)
	ctx := context.Background()
	unlock1, err := l.Lock(ctx)
	require.NoError(t, err)

	got := make(chan func(), 1)
	go func() {
		u, err := l.Lock(ctx)
		if err == nil {
			got <- u
		}
	}()
	select {
	case <-got:
		t.Fatal("second Lock acquired while first held")
	case <-time.After(150 * time.Millisecond):
	}
	unlock1()
	select {
	case u := <-got:
		u()
	case <-time.After(3 * time.Second):
		t.Fatal("second Lock not acquired after unlock")
	}
}

func TestLockerContextCancel(t *testing.T) {
	l := newTestLocker(t)
	unlock, err := l.Lock(context.Background())
	require.NoError(t, err)
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = l.Lock(ctx)
	require.Error(t, err)
}
