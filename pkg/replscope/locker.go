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
	"errors"
	"fmt"
	"time"

	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
)

const (
	lockKey = "lock:repl_scope"
	lockTTL = 30 * time.Second
)

// Locker serializes every creation path that checks and writes replication scopes.
type Locker interface {
	Lock(ctx context.Context) (unlock func(), err error)
}

type redisLocker struct {
	client *redislock.Client
}

// NewRedisLocker returns a Locker using one global Redis key.
func NewRedisLocker(rdb redis.UniversalClient) Locker {
	return &redisLocker{client: redislock.New(rdb)}
}

func (l *redisLocker) Lock(ctx context.Context) (func(), error) {
	opt := &redislock.Options{RetryStrategy: redislock.LinearBackoff(20 * time.Millisecond)}
	lock, err := l.client.Obtain(ctx, lockKey, lockTTL, opt)
	if err != nil {
		if errors.Is(err, redislock.ErrNotObtained) && ctx.Err() != nil {
			return nil, fmt.Errorf("obtain %s: %w", lockKey, ctx.Err())
		}
		return nil, fmt.Errorf("obtain %s: %w", lockKey, err)
	}
	return func() { _ = lock.Release(context.Background()) }, nil
}
