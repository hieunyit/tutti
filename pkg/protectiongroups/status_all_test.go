// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// StatusAll returns one status per group, in List order, equal to the per-id Status.
func TestStatusAllMatchesPerGroupStatus(t *testing.T) {
	svc, backend, _, _, _ := newService(t)
	ctx := t.Context()
	backend.mu.Lock()
	backend.lag["b"] = 3
	backend.mu.Unlock()
	for i := 0; i < 5; i++ {
		_, err := svc.Create(ctx, Group{ID: fmt.Sprintf("g%d", i), User: fmt.Sprintf("u%d", i), Buckets: []string{fmt.Sprintf("bk%d", i)}, Members: []string{"a", "b", "c"}, Copies: 2}, false)
		require.NoError(t, err)
	}
	require.NoError(t, svc.Store.Redis.HSet(ctx, writerKey("g2"), "w1", "x", "w2", "y").Err())
	groups, err := svc.Store.List(ctx)
	require.NoError(t, err)
	all, err := svc.StatusAll(ctx)
	require.NoError(t, err)
	require.Len(t, all, len(groups))
	for i, g := range groups {
		require.Equal(t, g.ID, all[i].Group.ID, "same order as List")
		one, err := svc.Status(ctx, g.ID)
		require.NoError(t, err)
		require.Equal(t, one, all[i])
	}
	require.EqualValues(t, 2, all[2].ActiveWriters)
}

// Concurrent creates conflict on the WATCHed keys; the bounded retry makes them all succeed.
func TestConcurrentCreateRetriesOptimisticConflict(t *testing.T) {
	svc, _, _, _, _ := newService(t)
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				_, err := svc.Create(t.Context(), Group{ID: fmt.Sprintf("g-%d-%d", w, i), User: fmt.Sprintf("u-%d-%d", w, i), Buckets: []string{fmt.Sprintf("bk-%d-%d", w, i)}, Members: []string{"a", "b", "c"}, Copies: 2}, false)
				errs <- err
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}
