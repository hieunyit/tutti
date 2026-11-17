// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/stretchr/testify/require"
)

type guardStore struct {
	renewals int
	fail     bool
}

func (*guardStore) AcquireLifecycleLease(context.Context, int64, string, time.Duration, bool) (bool, error) {
	return true, nil
}
func (s *guardStore) RenewLifecycleLease(context.Context, int64, string, time.Duration) (bool, error) {
	s.renewals++
	if s.fail {
		return false, errors.New("unavailable")
	}
	return true, nil
}
func (*guardStore) CompleteLifecycleLease(context.Context, int64, string, time.Time, time.Time, controlplane.LifecycleRunStats) (bool, error) {
	return true, nil
}

func TestLeaseGuardUsesConfirmedDeadlineAndFailsClosed(t *testing.T) {
	store := &guardStore{}
	guard := &leaseGuard{store: store, validUntil: time.Now().Add(lifecycleLeaseDuration)}
	for i := 0; i < 100; i++ {
		require.NoError(t, guard.ensureValid(context.Background()))
	}
	require.Zero(t, store.renewals)
	guard.validUntil = time.Now().Add(lifecycleRenewMargin - time.Second)
	require.NoError(t, guard.ensureValid(context.Background()))
	require.Equal(t, 1, store.renewals)
	require.NoError(t, guard.ensureValid(context.Background()))
	require.Equal(t, 1, store.renewals)
	guard.validUntil = time.Now().Add(-time.Second)
	store.fail = true
	require.Error(t, guard.ensureValid(context.Background()))
	require.Equal(t, 2, store.renewals)
}
