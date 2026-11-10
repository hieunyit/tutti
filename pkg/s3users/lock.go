package s3users

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bsm/redislock"

	"github.com/hieunyit/tutti/pkg/dom"
)

const userLockTTL = 2 * time.Minute

// withUserLock serializes the multi-step credential, key and policy mutations
// of one S3 user across worker instances. The token-protected Redis lock is
// refreshed while the operation runs and released even if the client leaves.
func (m *Manager) withUserLock(ctx context.Context, user string, fn func(context.Context) error) error {
	if m.Redis == nil {
		return fmt.Errorf("%w: Redis is required for S3 user mutations", dom.ErrFailedPrecondition)
	}
	lockCtx, cancelAcquire := context.WithTimeout(ctx, 10*time.Second)
	defer cancelAcquire()
	lock, err := redislock.New(m.Redis).Obtain(lockCtx, "tutti:s3users:lock:"+user, userLockTTL,
		&redislock.Options{RetryStrategy: redislock.LinearBackoff(25 * time.Millisecond)})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, redislock.ErrNotObtained) {
			return fmt.Errorf("%w: S3 user %q is being modified; retry", dom.ErrFailedPrecondition, user)
		}
		return fmt.Errorf("S3 users: acquire lock for %q: %w", user, err)
	}
	workCtx, cancelWork := context.WithCancel(ctx)
	stop := make(chan struct{})
	stopped := make(chan struct{})
	refreshErrors := make(chan error, 1)
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(userLockTTL / 4)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if err := lock.Refresh(workCtx, userLockTTL, nil); err != nil {
					refreshErrors <- err
					cancelWork()
					return
				}
			}
		}
	}()
	workErr := fn(workCtx)
	close(stop)
	<-stopped
	cancelWork()
	releaseCtx, cancelRelease := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelRelease()
	releaseErr := lock.Release(releaseCtx)
	if workErr != nil {
		return workErr
	}
	select {
	case err := <-refreshErrors:
		return fmt.Errorf("S3 users: lost lock for %q: %w", user, err)
	default:
	}
	if releaseErr != nil {
		return fmt.Errorf("S3 users: release lock for %q: %w", user, releaseErr)
	}
	return nil
}
