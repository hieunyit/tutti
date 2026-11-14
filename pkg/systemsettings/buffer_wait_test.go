package systemsettings

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

func TestBufferWaitTimeIncreasesWhenBudgetExhausted(t *testing.T) {
	prev := Current()
	t.Cleanup(func() { ApplySettings(prev) })
	ApplySettings(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{BufferBudgetBytes: 10 << 20}})

	base := CurrentBufferStats()
	rel1, err := AcquireBufferBudget(t.Context(), 10<<20)
	require.NoError(t, err)
	require.EqualValues(t, 10<<20, CurrentBufferStats().UsedBytes)

	done := make(chan func())
	go func() {
		rel, err := AcquireBufferBudget(t.Context(), 5<<20)
		if err != nil {
			close(done)
			return
		}
		done <- rel
	}()
	require.Eventually(t, func() bool { return CurrentBufferStats().Waiters == 1 }, time.Second, time.Millisecond)
	time.Sleep(60 * time.Millisecond)
	rel1()
	rel2 := <-done
	require.NotNil(t, rel2)
	st := CurrentBufferStats()
	require.GreaterOrEqual(t, st.WaitSeconds-base.WaitSeconds, 0.05)
	require.Zero(t, st.Waiters)
	rel2()
	require.Zero(t, CurrentBufferStats().UsedBytes)
}
