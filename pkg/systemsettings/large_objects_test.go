package systemsettings

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

func TestLargeObjectSlots(t *testing.T) {
	t.Cleanup(func() { ApplySettings(controlplane.SystemSettings{}) })
	ApplySettings(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{LargeObjectStreams: 2}})
	big := int64(LargeObjectThreshold)

	r1, ok := TryAcquireLargeObject("dst", big)
	require.True(t, ok)
	r2, ok := TryAcquireLargeObject("dst", big+1)
	require.True(t, ok)
	_, ok = TryAcquireLargeObject("dst", big)
	require.False(t, ok, "third large copy must wait")

	// other storages and small objects are not affected
	r3, ok := TryAcquireLargeObject("other", big)
	require.True(t, ok)
	small, ok := TryAcquireLargeObject("dst", big-1)
	require.True(t, ok)
	small()
	require.Equal(t, 2, LargeObjectsInFlight("dst"))

	r1()
	r1() // double release is harmless
	require.Equal(t, 1, LargeObjectsInFlight("dst"))
	r4, ok := TryAcquireLargeObject("dst", big)
	require.True(t, ok)
	r2()
	r3()
	r4()
	require.Equal(t, 0, LargeObjectsInFlight("dst"))
	require.Equal(t, 0, LargeObjectsInFlight("other"))
}

func TestLargeObjectStreamsLiveResize(t *testing.T) {
	t.Cleanup(func() { ApplySettings(controlplane.SystemSettings{}) })
	ApplySettings(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{LargeObjectStreams: 1}})
	r1, ok := TryAcquireLargeObject("d", LargeObjectThreshold)
	require.True(t, ok)
	_, ok = TryAcquireLargeObject("d", LargeObjectThreshold)
	require.False(t, ok)
	ApplySettings(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{LargeObjectStreams: 3}})
	r2, ok := TryAcquireLargeObject("d", LargeObjectThreshold)
	require.True(t, ok)
	r1()
	r2()
}

func TestLargeObjectStreamsPrecedence(t *testing.T) {
	t.Cleanup(func() { SetLargeObjectStreamsFallback(0); ApplySettings(controlplane.SystemSettings{}) })
	ApplySettings(controlplane.SystemSettings{})
	// no hardware detected yet or detected: auto source unless explicit
	require.Equal(t, SourceAuto, ResolveEffective(controlplane.SystemWorkerSettings{}).LargeObjectStreams.Source)
	SetLargeObjectStreamsFallback(7)
	e := ResolveEffective(controlplane.SystemWorkerSettings{}).LargeObjectStreams
	require.Equal(t, EffectiveValue{7, SourceExplicit}, e)
	e = ResolveEffective(controlplane.SystemWorkerSettings{LargeObjectStreams: 9}).LargeObjectStreams
	require.Equal(t, EffectiveValue{9, SourceExplicit}, e)
}
