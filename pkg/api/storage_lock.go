// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0
package api

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/bsm/redislock"
	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/dom"
)

const storageLockTTL = 30 * time.Second

// StorageLocker serializes operations that create or remove references to a storage (replications, routings, failover
// policies) with runtime storage update/delete, across worker instances. It closes the check-then-act window
// between the reference scan and the deletion. A nil locker runs the function without locking.
type StorageLocker struct {
	rdb redis.Cmdable
	ttl time.Duration
}

func NewStorageLocker(rdb redis.Cmdable) *StorageLocker {
	return &StorageLocker{rdb: rdb, ttl: storageLockTTL}
}

// With holds the lock of every named storage (acquired in sorted order to avoid deadlocks) while fn runs.
// The locks are refreshed while fn runs and always released.
func (l *StorageLocker) With(ctx context.Context, storages []string, fn func(context.Context) error) error {
	if l == nil || l.rdb == nil {
		return fn(ctx)
	}
	names := append([]string(nil), storages...)
	sort.Strings(names)
	var locks []*redislock.Lock
	release := func() {
		rc, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		for i := len(locks) - 1; i >= 0; i-- {
			_ = locks[i].Release(rc)
		}
	}
	prev := ""
	for _, n := range names {
		if n == "" || n == prev {
			continue
		}
		prev = n
		acq, cancel := context.WithTimeout(ctx, 10*time.Second)
		lock, err := redislock.New(l.rdb).Obtain(acq, "tutti:storage:lock:"+n, l.ttl,
			&redislock.Options{RetryStrategy: redislock.LinearBackoff(10 * time.Millisecond)})
		cancel()
		if err != nil {
			release()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, redislock.ErrNotObtained) {
				return fmt.Errorf("%w: storage %q is being modified; retry", dom.ErrFailedPrecondition, n)
			}
			return fmt.Errorf("acquire storage lock %q: %w", n, err)
		}
		locks = append(locks, lock)
	}
	workCtx, cancelWork := context.WithCancel(ctx)
	stop, stopped := make(chan struct{}), make(chan struct{})
	lost := make(chan error, 1)
	go func() {
		defer close(stopped)
		t := time.NewTicker(l.ttl / 3)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				for _, lk := range locks {
					if err := lk.Refresh(workCtx, l.ttl, nil); err != nil {
						lost <- err
						cancelWork()
						return
					}
				}
			}
		}
	}()
	err := fn(workCtx)
	close(stop)
	<-stopped
	cancelWork()
	release()
	if err != nil {
		return err
	}
	select {
	case e := <-lost:
		return fmt.Errorf("storage lock lost: %w", e)
	default:
	}
	return nil
}
