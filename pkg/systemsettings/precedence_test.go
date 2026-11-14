package systemsettings

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

func TestResolveBulkPercentPrecedence(t *testing.T) {
	require.Equal(t, 40, ResolveBulkPercent(40, 60), "setting wins")
	require.Equal(t, 60, ResolveBulkPercent(0, 60), "yaml when setting is 0")
	require.Equal(t, 20, ResolveBulkPercent(0, 0), "built-in default")
	require.Equal(t, 20, ResolveBulkPercent(0, 99), "out-of-range yaml falls to default")
}

func TestResolveMultipartPrecedence(t *testing.T) {
	th, mib, explicit := ResolveMultipart(8, 64, 2, 32)
	require.Equal(t, []any{8, 64, true}, []any{th, mib, explicit})
	th, mib, explicit = ResolveMultipart(0, 0, 2, 32)
	require.Equal(t, []any{2, 32, true}, []any{th, mib, explicit})
	th, mib, explicit = ResolveMultipart(0, 0, 0, 0)
	require.Equal(t, []any{1, 16, false}, []any{th, mib, explicit})
	th, mib, explicit = ResolveMultipart(4, 0, 0, 0)
	require.Equal(t, []any{4, 16, false}, []any{th, mib, explicit})
}

func TestEffectiveReadsLiveSettings(t *testing.T) {
	prev := Current()
	t.Cleanup(func() { ApplySettings(prev); SetConfigFallbacks(0, 0, 0) })
	SetConfigFallbacks(30, 2, 8)

	ApplySettings(controlplane.SystemSettings{})
	th, part, explicit := EffectiveMultipart()
	require.Equal(t, 2, th)
	require.EqualValues(t, 8<<20, part)
	require.True(t, explicit)
	require.Equal(t, 30, EffectiveBulkPercent())

	ApplySettings(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{MultipartThreads: 6, MultipartPartSizeMiB: 64, BulkConcurrencyPercent: 50}})
	th, part, _ = EffectiveMultipart()
	require.Equal(t, 6, th)
	require.EqualValues(t, 64<<20, part)
	require.Equal(t, 50, EffectiveBulkPercent())
}
