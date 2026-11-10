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

package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/util"
)

const (
	defaultLockDuration = time.Second
)

type Lock struct {
	logger  *zerolog.Logger
	lock    *redislock.Lock
	overlap time.Duration
}

func NewLock(logger *zerolog.Logger, lock *redislock.Lock, overlap time.Duration) *Lock {
	return &Lock{
		logger:  logger,
		lock:    lock,
		overlap: overlap,
	}
}

func (r *Lock) Refresh(ctx context.Context, duration time.Duration) error {
	r.logger.Debug().Str("refresh", duration.String()).Msg("lock-service: refreshing the lock")
	err := r.lock.Refresh(ctx, duration, nil)
	r.logger.Debug().Err(err).Str("refresh", duration.String()).Msg("lock-service: lock refreshed")
	return err
}

func (r *Lock) Release(ctx context.Context) {
	r.logger.Debug().Msg("lock-service: releasing the lock")
	releaseErr := r.lock.Release(context.Background())
	if releaseErr != nil && !errors.Is(releaseErr, redislock.ErrLockNotHeld) {
		zerolog.Ctx(ctx).Warn().Err(releaseErr).Msg("unable to release lock")
	}
	r.logger.Debug().Err(releaseErr).Msg("lock-service: lock released")
}

// ErrLockLost is returned by Do/DoCtx when the lock could not be kept during the work: the lock
// expired or was taken over, or refreshing kept failing until the TTL would have expired.
// The context passed to the work function is canceled and the caller should retry the task.
var ErrLockLost = errors.New("lock lost during work")

// lockLostGrace is how long DoCtx waits for the canceled work to return before giving up on it.
const lockLostGrace = 5 * time.Second

// Do runs work holding the lock. Prefer DoCtx: work of Do is not canceled when the lock is lost.
func (r *Lock) Do(ctx context.Context, refresh time.Duration, work func() error) error {
	return r.DoCtx(ctx, refresh, func(context.Context) error { return work() })
}

// DoCtx runs work holding the lock and refreshes it while work runs. The context passed to work
// is canceled when the lock is lost (refresh reports it is not held, or refreshes keep failing
// until the TTL would have expired); DoCtx then returns an error wrapping ErrLockLost.
func (r *Lock) DoCtx(ctx context.Context, refresh time.Duration, work func(ctx context.Context) error) error {
	refreshInterval := refresh + r.overlap
	err := r.Refresh(ctx, refreshInterval)
	if errors.Is(err, redislock.ErrNotObtained) {
		return fmt.Errorf("%w: lock not obtained during refresh", &dom.ErrRateLimitExceeded{RetryIn: util.DurationJitter(time.Second*5, time.Second*30)})
	}
	if err != nil {
		return err
	}
	lastOK := time.Now()
	workCtx, cancelWork := context.WithCancel(ctx)
	defer cancelWork()
	errCh := make(chan error, 1)
	go func() {
		errCh <- work(workCtx)
	}()
	timer := time.NewTimer(refresh)
	defer timer.Stop()
	// keep the lock until work returns, even if ctx is canceled
	refreshCtx := ctx
	retryDelay := max(refresh/4, 10*time.Millisecond)
	i := 0
	for {
		select {
		case <-refreshCtx.Done():
			refreshCtx = context.WithoutCancel(ctx)
		case <-timer.C:
			i++
			err = r.Refresh(refreshCtx, refreshInterval)
			if err == nil {
				lastOK = time.Now()
				timer.Reset(refresh)
				continue
			}
			zerolog.Ctx(ctx).Err(err).
				Str("refresh_period", refresh.String()).
				Str("refresh_overlap", r.overlap.String()).
				Int("refresh_iteration", i).
				Msg("unable to refresh lock")
			lost := errors.Is(err, redislock.ErrNotObtained) || errors.Is(err, redislock.ErrLockNotHeld)
			if lost || time.Since(lastOK) >= refreshInterval {
				cancelWork()
				select {
				case werr := <-errCh:
					if werr != nil && !errors.Is(werr, context.Canceled) {
						return fmt.Errorf("%w: %w (work: %v)", ErrLockLost, err, werr)
					}
				case <-time.After(lockLostGrace):
				}
				return fmt.Errorf("%w: %w", ErrLockLost, err)
			}
			// transient error: retry sooner, within the remaining TTL
			timer.Reset(min(retryDelay, refresh))
		case err = <-errCh:
			return err
		}
	}
}

type lockOptions struct {
	duration time.Duration
	retry    bool
}

type LockOpt interface {
	apply(*lockOptions)
}

type durationOption time.Duration

func (d durationOption) apply(opts *lockOptions) {
	opts.duration = time.Duration(d)
}

func WithDuration(d time.Duration) LockOpt {
	return durationOption(d)
}

type retryOption bool

func (d retryOption) apply(opts *lockOptions) {
	opts.retry = bool(d)
}

func WithRetry(retry bool) LockOpt {
	return retryOption(retry)
}

type RedisIDKeyLocker[ID any] struct {
	locker *redislock.Client
	RedisIDCommonStore[ID]
	overlap time.Duration
}

func NewRedisIDKeyLocker[ID any](client redis.Cmdable, keyPrefix string,
	tokenizeID SingleToMultiValueConverter[ID, string], restoreID MultiToSingleValueConverter[string, ID],
	overlap time.Duration) *RedisIDKeyLocker[ID] {
	return &RedisIDKeyLocker[ID]{
		overlap:            overlap,
		locker:             redislock.New(client),
		RedisIDCommonStore: *NewRedisIDCommonStore[ID](client, keyPrefix, tokenizeID, restoreID),
	}
}

func (r *RedisIDKeyLocker[ID]) Lock(ctx context.Context, id ID, opts ...LockOpt) (*Lock, error) {
	lockKey, err := r.MakeKey(id)
	if err != nil {
		return nil, err
	}
	lockOpts := lockOptions{
		duration: defaultLockDuration,
	}
	for _, o := range opts {
		o.apply(&lockOpts)
	}

	logger := zerolog.Ctx(ctx).With().Str("lock_key", lockKey).Str("duration", lockOpts.duration.String()).Logger()
	var opt *redislock.Options
	if lockOpts.retry {
		opt = &redislock.Options{RetryStrategy: redislock.LimitRetry(redislock.ExponentialBackoff(time.Millisecond*50, time.Second*5), 100)}
	}
	logger.Debug().Msg("lock-service: obtaining lock")
	lock, err := r.locker.Obtain(ctx, lockKey, lockOpts.duration, opt)
	if errors.Is(err, redislock.ErrNotObtained) {
		return nil, fmt.Errorf("%w: lock not obtained %q", &dom.ErrRateLimitExceeded{RetryIn: util.DurationJitter(time.Second, time.Second*5)}, lockKey)
	}
	if err != nil {
		return nil, err
	}
	logger.Debug().Msg("lock-service: lock obtained")
	return NewLock(&logger, lock, r.overlap), nil
}
