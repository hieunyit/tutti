package trafficstats

import (
	"math"
	"sync"
	"time"
)

// DefaultWindow is the sliding window over which proxies compute their current rates.
const DefaultWindow = 10 * time.Second

// Rates are per-second traffic rates over a short sliding window (see Window). Latencies are in milliseconds.
type Rates struct {
	// WindowSeconds is the actual time span the rates were computed over (0 if there are not enough samples yet).
	WindowSeconds float64 `json:"windowSeconds"`
	// RequestsPerSec is the total request rate; ByClass (read, write, exists, manage, other) and ByStorage break it down.
	RequestsPerSec float64            `json:"requestsPerSec"`
	ByClass        map[string]float64 `json:"byClass,omitempty"`
	ByStorage      map[string]float64 `json:"byStorage,omitempty"`
	BytesInPerSec  float64            `json:"bytesInPerSec"`
	BytesOutPerSec float64            `json:"bytesOutPerSec"`
	// Status4xxPerSec and Status5xxPerSec are response rates by status class.
	Status4xxPerSec    float64 `json:"status4xxPerSec"`
	Status5xxPerSec    float64 `json:"status5xxPerSec"`
	FailFastPerSec     float64 `json:"failFastPerSec"`
	AccessDeniedPerSec float64 `json:"accessDeniedPerSec"`
	// LatencyAvgMs is the mean request duration within the window, LatencyP95Ms the 95th percentile estimated from
	// the histogram bucket deltas (linear interpolation inside the bucket; values above the last bound report it).
	LatencyAvgMs float64 `json:"latencyAvgMs"`
	LatencyP95Ms float64 `json:"latencyP95Ms"`
}

// Window keeps timestamped snapshots of the last span and computes rates between the oldest and the newest one.
// It is safe for concurrent use.
type Window struct {
	span time.Duration
	mu   sync.Mutex
	s    []Snapshot
}

// NewWindow creates a window; span <= 0 means DefaultWindow.
func NewWindow(span time.Duration) *Window {
	if span <= 0 {
		span = DefaultWindow
	}
	return &Window{span: span}
}

// Add records a snapshot. A restart (different StartedAt) or a decreasing counter resets the window.
func (w *Window) Add(s Snapshot) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if n := len(w.s); n > 0 {
		last := w.s[n-1]
		if !last.StartedAt.Equal(s.StartedAt) || s.Requests.Total < last.Requests.Total {
			w.s = w.s[:0]
		} else if !s.CollectedAt.After(last.CollectedAt) {
			return // not newer
		}
	}
	w.s = append(w.s, s)
	// drop samples older than span, but keep the newest one at or before the cutoff as the baseline
	cut := s.CollectedAt.Add(-w.span)
	i := 0
	for i+1 < len(w.s) && !w.s[i+1].CollectedAt.After(cut) {
		i++
	}
	if i > 0 {
		w.s = append(w.s[:0], w.s[i:]...)
	}
}

// Rates returns the rates between the oldest and newest snapshot (zero Rates with WindowSeconds 0 if fewer than 2).
func (w *Window) Rates() Rates {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.s) < 2 {
		return Rates{}
	}
	return ComputeRates(w.s[0], w.s[len(w.s)-1])
}

// ComputeRates computes the rates between two snapshots of the same process run (old before cur).
func ComputeRates(old, cur Snapshot) Rates {
	dt := cur.CollectedAt.Sub(old.CollectedAt).Seconds()
	if dt <= 0 {
		return Rates{}
	}
	d := func(a, b uint64) float64 {
		if b < a {
			return 0
		}
		return float64(b-a) / dt
	}
	r := Rates{
		WindowSeconds:      dt,
		RequestsPerSec:     d(old.Requests.Total, cur.Requests.Total),
		ByClass:            map[string]float64{},
		ByStorage:          map[string]float64{},
		BytesInPerSec:      d(old.BytesIn, cur.BytesIn),
		BytesOutPerSec:     d(old.BytesOut, cur.BytesOut),
		Status4xxPerSec:    d(old.Responses.Status4xx, cur.Responses.Status4xx),
		Status5xxPerSec:    d(old.Responses.Status5xx, cur.Responses.Status5xx),
		FailFastPerSec:     d(old.FailFast, cur.FailFast),
		AccessDeniedPerSec: d(old.AccessDenied, cur.AccessDenied),
	}
	for k, v := range cur.Requests.ByClass {
		r.ByClass[k] = d(old.Requests.ByClass[k], v)
	}
	for k, v := range cur.Requests.ByStorage {
		r.ByStorage[k] = d(old.Requests.ByStorage[k], v)
	}
	if cnt := cur.Latency.Count - old.Latency.Count; cur.Latency.Count > old.Latency.Count && cnt > 0 {
		r.LatencyAvgMs = (cur.Latency.SumSeconds - old.Latency.SumSeconds) / float64(cnt) * 1000
		if r.LatencyAvgMs < 0 {
			r.LatencyAvgMs = 0
		}
		delta := make([]uint64, len(cur.Latency.Buckets))
		for i := range delta {
			if i < len(old.Latency.Buckets) && cur.Latency.Buckets[i] >= old.Latency.Buckets[i] {
				delta[i] = cur.Latency.Buckets[i] - old.Latency.Buckets[i]
			} else if i >= len(old.Latency.Buckets) {
				delta[i] = cur.Latency.Buckets[i]
			}
		}
		r.LatencyP95Ms = quantileMs(cur.Latency.BoundsSeconds, delta, 0.95)
	}
	return r
}

// quantileMs estimates the q quantile (milliseconds) from non-cumulative histogram buckets.
func quantileMs(bounds []float64, buckets []uint64, q float64) float64 {
	var total uint64
	for _, b := range buckets {
		total += b
	}
	if total == 0 {
		return 0
	}
	rank := q * float64(total)
	var cum uint64
	for i, b := range buckets {
		if b == 0 {
			continue
		}
		if float64(cum+b) >= rank {
			if i >= len(bounds) { // +Inf bucket
				return bounds[len(bounds)-1] * 1000
			}
			lo := 0.0
			if i > 0 {
				lo = bounds[i-1]
			}
			frac := (rank - float64(cum)) / float64(b)
			return (lo + (bounds[i]-lo)*frac) * 1000
		}
		cum += b
	}
	return bounds[len(bounds)-1] * 1000
}

// SumRates adds the rates of several proxies (or of several samples of one entity, see MeanRates): request, byte,
// status, fail-fast and access-denied rates are summed; LatencyAvgMs is the request-weighted average; LatencyP95Ms is
// the maximum (an upper bound of the true combined p95 that cannot be recovered from per-proxy percentiles).
func SumRates(rs []Rates) Rates {
	out := Rates{ByClass: map[string]float64{}, ByStorage: map[string]float64{}}
	var wsum, latWeighted float64
	for _, r := range rs {
		if r.WindowSeconds > out.WindowSeconds {
			out.WindowSeconds = r.WindowSeconds
		}
		out.RequestsPerSec += r.RequestsPerSec
		out.BytesInPerSec += r.BytesInPerSec
		out.BytesOutPerSec += r.BytesOutPerSec
		out.Status4xxPerSec += r.Status4xxPerSec
		out.Status5xxPerSec += r.Status5xxPerSec
		out.FailFastPerSec += r.FailFastPerSec
		out.AccessDeniedPerSec += r.AccessDeniedPerSec
		for k, v := range r.ByClass {
			out.ByClass[k] += v
		}
		for k, v := range r.ByStorage {
			out.ByStorage[k] += v
		}
		wsum += r.RequestsPerSec
		latWeighted += r.LatencyAvgMs * r.RequestsPerSec
		out.LatencyP95Ms = math.Max(out.LatencyP95Ms, r.LatencyP95Ms)
	}
	if wsum > 0 {
		out.LatencyAvgMs = latWeighted / wsum
	}
	return out
}

// MeanRates averages the rates of consecutive samples of one proxy (used when downsampling history): rates are
// averaged, LatencyAvgMs is request-weighted, LatencyP95Ms is the maximum.
func MeanRates(rs []Rates) Rates {
	if len(rs) == 0 {
		return Rates{}
	}
	out := SumRates(rs)
	n := float64(len(rs))
	out.RequestsPerSec /= n
	out.BytesInPerSec /= n
	out.BytesOutPerSec /= n
	out.Status4xxPerSec /= n
	out.Status5xxPerSec /= n
	out.FailFastPerSec /= n
	out.AccessDeniedPerSec /= n
	for k := range out.ByClass {
		out.ByClass[k] /= n
	}
	for k := range out.ByStorage {
		out.ByStorage[k] /= n
	}
	return out
}

// Round rounds all rates to 3 decimals (compact JSON for history samples).
func (r Rates) Round() Rates {
	f := func(v float64) float64 { return math.Round(v*1000) / 1000 }
	r.WindowSeconds = f(r.WindowSeconds)
	r.RequestsPerSec = f(r.RequestsPerSec)
	r.BytesInPerSec = f(r.BytesInPerSec)
	r.BytesOutPerSec = f(r.BytesOutPerSec)
	r.Status4xxPerSec = f(r.Status4xxPerSec)
	r.Status5xxPerSec = f(r.Status5xxPerSec)
	r.FailFastPerSec = f(r.FailFastPerSec)
	r.AccessDeniedPerSec = f(r.AccessDeniedPerSec)
	r.LatencyAvgMs = f(r.LatencyAvgMs)
	r.LatencyP95Ms = f(r.LatencyP95Ms)
	for _, m := range []map[string]float64{r.ByClass, r.ByStorage} {
		for k, v := range m {
			if v = f(v); v == 0 {
				delete(m, k)
			} else {
				m[k] = v
			}
		}
	}
	return r
}
