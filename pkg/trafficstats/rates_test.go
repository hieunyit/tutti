package trafficstats

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWindow_Rates(t *testing.T) {
	c := New()
	w := NewWindow(10 * time.Second)
	base := c.Snapshot()
	require.Equal(t, Rates{}, w.Rates(), "empty window")
	w.Add(base)
	require.Equal(t, Rates{}, w.Rates(), "one sample")

	for i := 0; i < 100; i++ {
		c.Request(ClassRead, "main")
		c.Response(200)
		c.Latency(20 * time.Millisecond) // bucket (0.01,0.05]
	}
	for i := 0; i < 20; i++ {
		c.Request(ClassWrite, "b")
		c.Response(503)
		c.Latency(300 * time.Millisecond)
		c.FailFast()
	}
	c.BytesIn(2000)
	c.BytesOut(4000)
	c.AccessDenied()
	s := c.Snapshot()
	s.CollectedAt = base.CollectedAt.Add(4 * time.Second)
	w.Add(s)

	r := w.Rates()
	require.InDelta(t, 4, r.WindowSeconds, 1e-9)
	require.InDelta(t, 30, r.RequestsPerSec, 1e-9)
	require.InDelta(t, 25, r.ByClass[ClassRead], 1e-9)
	require.InDelta(t, 5, r.ByClass[ClassWrite], 1e-9)
	require.InDelta(t, 5, r.ByStorage["b"], 1e-9)
	require.InDelta(t, 500, r.BytesInPerSec, 1e-9)
	require.InDelta(t, 1000, r.BytesOutPerSec, 1e-9)
	require.InDelta(t, 5, r.Status5xxPerSec, 1e-9)
	require.InDelta(t, 5, r.FailFastPerSec, 1e-9)
	require.InDelta(t, 0.25, r.AccessDeniedPerSec, 1e-9)
	require.InDelta(t, (100*20.0+20*300)/120, r.LatencyAvgMs, 0.01)
	// 120 samples, p95 rank 114 -> falls in (0.25,0.5] bucket: 100 in (0.01,0.05], 20 in (0.25,0.5]
	require.Greater(t, r.LatencyP95Ms, 250.0)
	require.LessOrEqual(t, r.LatencyP95Ms, 500.0)
}

func TestWindow_SlidesAndResets(t *testing.T) {
	c := New()
	w := NewWindow(10 * time.Second)
	t0 := c.Snapshot()
	w.Add(t0)
	at := func(total uint64, sec int) Snapshot {
		s := t0
		s.Requests.Total = total
		s.CollectedAt = t0.CollectedAt.Add(time.Duration(sec) * time.Second)
		return s
	}
	w.Add(at(1000, 5)) // burst early
	w.Add(at(1000, 12))
	w.Add(at(1010, 20))
	// window is 10s: baseline is the newest sample at/before t=10 -> (t=5, 1000); 10 requests over 15s
	r := w.Rates()
	require.InDelta(t, 15, r.WindowSeconds, 1e-9)
	require.InDelta(t, 10.0/15, r.RequestsPerSec, 1e-9)
	w.Add(at(1020, 30))
	w.Add(at(1030, 40))
	r = w.Rates()
	require.InDelta(t, 1.0, r.RequestsPerSec, 1e-9, "old burst slid out")

	// counter decrease resets
	w.Add(at(5, 41))
	require.Equal(t, Rates{}, w.Rates())
	// restart (different StartedAt) resets
	w.Add(at(10, 42))
	require.InDelta(t, 5, w.Rates().RequestsPerSec, 1e-9)
	s := at(500, 43)
	s.StartedAt = s.StartedAt.Add(time.Hour)
	w.Add(s)
	require.Equal(t, Rates{}, w.Rates())
}

func TestQuantileMs(t *testing.T) {
	bounds := []float64{0.01, 0.1}
	require.Equal(t, 0.0, quantileMs(bounds, []uint64{0, 0, 0}, 0.95))
	// all in the first bucket: interpolation within (0, 10ms]
	require.InDelta(t, 9.5, quantileMs(bounds, []uint64{100, 0, 0}, 0.95), 1e-9)
	// +Inf bucket reports the last bound
	require.InDelta(t, 100, quantileMs(bounds, []uint64{0, 0, 10}, 0.95), 1e-9)
}

func TestSumAndMeanRates(t *testing.T) {
	a := Rates{RequestsPerSec: 10, LatencyAvgMs: 10, LatencyP95Ms: 30, ByClass: map[string]float64{"read": 10}}
	b := Rates{RequestsPerSec: 30, LatencyAvgMs: 50, LatencyP95Ms: 20, ByClass: map[string]float64{"read": 20, "write": 10}, Status5xxPerSec: 2}
	s := SumRates([]Rates{a, b})
	require.InDelta(t, 40, s.RequestsPerSec, 1e-9)
	require.InDelta(t, 40, s.LatencyAvgMs, 1e-9, "request-weighted")
	require.InDelta(t, 30, s.LatencyP95Ms, 1e-9, "max")
	require.InDelta(t, 30, s.ByClass["read"], 1e-9)
	require.InDelta(t, 2, s.Status5xxPerSec, 1e-9)
	m := MeanRates([]Rates{a, b})
	require.InDelta(t, 20, m.RequestsPerSec, 1e-9)
	require.InDelta(t, 40, m.LatencyAvgMs, 1e-9)
	require.InDelta(t, 15, m.ByClass["read"], 1e-9)
	require.Equal(t, Rates{}, MeanRates(nil))
}
