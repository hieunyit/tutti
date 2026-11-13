package systemsettings

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

func TestSplitConcurrency(t *testing.T) {
	cases := []struct {
		total, pct, bulk, live int
	}{
		{10, 0, 2, 8},  // unset: default 20%
		{10, 20, 2, 8}, // current behavior
		{10, 50, 5, 5},
		{10, 95, 9, 1},  // live keeps at least one slot
		{10, 5, 1, 9},   // 0.5 rounds down, bulk keeps one slot
		{10, 1, 2, 8},   // below range: default
		{10, 100, 2, 8}, // above range: default
		{3, 20, 1, 2},
		{2, 95, 1, 1},
		{7, 50, 3, 4}, // rounds down
		{100, 33, 33, 67},
	}
	for _, c := range cases {
		bulk, live := SplitConcurrency(c.total, c.pct)
		require.Equal(t, c.bulk, bulk, "bulk total=%d pct=%d", c.total, c.pct)
		require.Equal(t, c.live, live, "live total=%d pct=%d", c.total, c.pct)
		require.Equal(t, c.total, bulk+live)
	}
}

func TestMultipartTuning(t *testing.T) {
	th, ps := MultipartTuning(0, 0)
	require.Equal(t, 1, th)
	require.Equal(t, uint64(16<<20), ps)
	th, ps = MultipartTuning(99, 2)
	require.Equal(t, 16, th)
	require.Equal(t, uint64(16<<20), ps) // below the 5 MiB minimum: default
	th, ps = MultipartTuning(4, 5)
	require.Equal(t, 4, th)
	require.Equal(t, uint64(5<<20), ps)
}

func TestParallelBufferBudgetScalesWithThreads(t *testing.T) {
	ApplyWorkerLimiting(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{BufferBudgetBytes: 100 << 20}})
	t.Cleanup(func() {
		ApplyWorkerLimiting(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{BufferBudgetBytes: 256 << 20}})
	})
	// 4 threads x 16 MiB = 64 MiB fits; a second one must wait instead of exceeding the budget
	rel, err := AcquireBufferBudget(t.Context(), 4*(16<<20))
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	_, err = AcquireBufferBudget(ctx, 4*(16<<20))
	require.Error(t, err)
	rel()
	// 8 x 16 MiB = 128 MiB exceeds the whole 100 MiB budget: fail fast
	_, err = AcquireBufferBudget(t.Context(), 8*(16<<20))
	var oversize *BufferExceedsBudgetError
	require.ErrorAs(t, err, &oversize)
}
