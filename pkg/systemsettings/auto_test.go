package systemsettings

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/hostres"
)

func TestResolveEffective_AutoVsExplicit(t *testing.T) {
	t.Cleanup(func() {
		autoVal.Store(nil)
		SetConcurrencyFallback(0)
		SetConfigFallbacks(0, 0, 0)
		ApplySettings(controlplane.SystemSettings{})
	})
	SetHardware(hostres.Hardware{CPU: 4, MemoryBytes: 32 << 30})
	SetConcurrencyFallback(0)
	SetConfigFallbacks(0, 0, 0)

	e := ResolveEffective(controlplane.SystemWorkerSettings{})
	require.EqualValues(t, 40, e.Concurrency.Value)
	require.Equal(t, SourceAuto, e.Concurrency.Source)
	require.EqualValues(t, 8<<30, e.BufferBudgetBytes.Value)
	require.EqualValues(t, 30, e.BulkConcurrencyPercent.Value)
	require.EqualValues(t, 1, e.MultipartThreads.Value)

	// Explicit stored values win; YAML concurrency is explicit when the stored one is 0.
	e = ResolveEffective(controlplane.SystemWorkerSettings{Concurrency: 7, BufferBudgetBytes: 64 << 20, BulkConcurrencyPercent: 50, MultipartThreads: 4, MultipartPartSizeMiB: 32})
	require.EqualValues(t, 7, e.Concurrency.Value)
	require.Equal(t, SourceExplicit, e.Concurrency.Source)
	require.EqualValues(t, 64<<20, e.BufferBudgetBytes.Value)
	require.Equal(t, SourceExplicit, e.BufferBudgetBytes.Source)
	require.EqualValues(t, 50, e.BulkConcurrencyPercent.Value)
	require.Equal(t, SourceExplicit, e.MultipartThreads.Source)
	require.Equal(t, SourceExplicit, e.MultipartPartSizeMiB.Source)
	SetConcurrencyFallback(25)
	e = ResolveEffective(controlplane.SystemWorkerSettings{})
	require.EqualValues(t, 25, e.Concurrency.Value)
	require.Equal(t, SourceExplicit, e.Concurrency.Source)

	// The buffer pool follows the auto value when nothing explicit is stored, and the explicit one otherwise.
	ApplySettings(controlplane.SystemSettings{})
	require.EqualValues(t, 8<<30, CurrentBufferStats().TotalBytes)
	ApplySettings(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{BufferBudgetBytes: 64 << 20}})
	require.EqualValues(t, 64<<20, CurrentBufferStats().TotalBytes)
}
