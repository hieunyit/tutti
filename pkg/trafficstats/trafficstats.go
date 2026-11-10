// Package trafficstats keeps cumulative in-memory proxy traffic counters (since process start) without Prometheus.
//
// All hot-path operations are lock-free atomic increments. A Snapshot is published in the proxy status document so
// consumers can compute rates from two snapshots (CollectedAt differs, StartedAt identifies the process run).
package trafficstats

import (
	"sync"
	"sync/atomic"
	"time"
)

// Operation classes (see pkg/accesspolicy.Class).
const (
	ClassRead   = "read"
	ClassWrite  = "write"
	ClassExists = "exists"
	ClassManage = "manage"
	ClassOther  = "other"
)

// Sync replication outcomes.
const (
	SyncQuorum      = "quorum"
	SyncBelowQuorum = "belowQuorum"
	SyncSkipped     = "skipped"
)

// LatencyBounds are the upper bounds (seconds, inclusive) of the latency histogram buckets; one more +Inf bucket follows.
var LatencyBounds = []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

var latencyBoundsNs = func() []int64 {
	res := make([]int64, len(LatencyBounds))
	for i, b := range LatencyBounds {
		res[i] = int64(b * float64(time.Second))
	}
	return res
}()

// Collector holds the counters. The zero value is not usable, use New.
type Collector struct {
	startedAt time.Time

	classes [5]atomic.Uint64 // read, write, exists, manage, other
	storage sync.Map         // string -> *atomic.Uint64

	total   atomic.Uint64
	status  [4]atomic.Uint64 // 2xx 3xx 4xx 5xx
	statusO atomic.Uint64    // everything else (1xx, invalid)

	failFast       atomic.Uint64
	accessDenied   atomic.Uint64
	eventRecordErr atomic.Uint64
	sync           [3]atomic.Uint64 // quorum, belowQuorum, skipped
	chunkedFailed  atomic.Uint64
	noCredentials  atomic.Uint64

	bytesIn  atomic.Uint64
	bytesOut atomic.Uint64

	latCount   atomic.Uint64
	latSumNs   atomic.Uint64
	latBuckets [10]atomic.Uint64 // len(LatencyBounds)+1
}

// New creates a collector whose start time is now.
func New() *Collector { return &Collector{startedAt: time.Now().UTC()} }

// Default is the process-wide collector used by the proxy.
var Default = New()

func classIndex(class string) int {
	switch class {
	case ClassRead:
		return 0
	case ClassWrite:
		return 1
	case ClassExists:
		return 2
	case ClassManage:
		return 3
	}
	return 4
}

// Request counts one request of the operation class routed to storage ("" if not routed).
func (c *Collector) Request(class, storage string) {
	c.total.Add(1)
	c.classes[classIndex(class)].Add(1)
	if storage == "" {
		return
	}
	v, ok := c.storage.Load(storage)
	if !ok {
		v, _ = c.storage.LoadOrStore(storage, new(atomic.Uint64))
	}
	v.(*atomic.Uint64).Add(1)
}

// Response counts one response by status class.
func (c *Collector) Response(status int) {
	if i := status/100 - 2; i >= 0 && i < 4 {
		c.status[i].Add(1)
		return
	}
	c.statusO.Add(1)
}

// FailFast counts a request rejected because the storage circuit breaker is open.
func (c *Collector) FailFast() { c.failFast.Add(1) }

// AccessDenied counts a request denied by an access policy.
func (c *Collector) AccessDenied() { c.accessDenied.Add(1) }

// EventRecordFailed counts a request answered 503 because the replication event could not be recorded.
func (c *Collector) EventRecordFailed() { c.eventRecordErr.Add(1) }

// SyncOutcome counts a synchronous replication outcome (SyncQuorum, SyncBelowQuorum, SyncSkipped).
func (c *Collector) SyncOutcome(outcome string) {
	switch outcome {
	case SyncQuorum:
		c.sync[0].Add(1)
	case SyncBelowQuorum:
		c.sync[1].Add(1)
	case SyncSkipped:
		c.sync[2].Add(1)
	}
}

// NoCredentials counts a request answered 503 because the storage has neither an override nor a connection account
// for the user.
func (c *Collector) NoCredentials() { c.noCredentials.Add(1) }

// ChunkedVerifyFailed counts an aws-chunked payload verification failure.
func (c *Collector) ChunkedVerifyFailed() { c.chunkedFailed.Add(1) }

// BytesIn adds bytes read from clients.
func (c *Collector) BytesIn(n int64) {
	if n > 0 {
		c.bytesIn.Add(uint64(n))
	}
}

// BytesOut adds bytes written to clients.
func (c *Collector) BytesOut(n int64) {
	if n > 0 {
		c.bytesOut.Add(uint64(n))
	}
}

// Latency records the duration of one request.
func (c *Collector) Latency(d time.Duration) {
	ns := d.Nanoseconds()
	if ns < 0 {
		ns = 0
	}
	c.latCount.Add(1)
	c.latSumNs.Add(uint64(ns))
	i := 0
	for i < len(latencyBoundsNs) && ns > latencyBoundsNs[i] {
		i++
	}
	c.latBuckets[i].Add(1)
}

// Snapshot is a point-in-time copy of the counters. All counters are cumulative since StartedAt.
type Snapshot struct {
	CollectedAt time.Time `json:"collectedAt"`
	StartedAt   time.Time `json:"startedAt"`
	// Requests is the number of requests seen by the proxy handler chain.
	Requests Requests `json:"requests"`
	// Responses by status class.
	Responses Responses `json:"responses"`
	// FailFast is the number of requests rejected because the storage breaker is open.
	FailFast uint64 `json:"failFast"`
	// AccessDenied is the number of requests denied by access policies.
	AccessDenied uint64 `json:"accessDenied"`
	// EventRecordFailed is the number of requests answered 503 because the replication event was not recorded.
	EventRecordFailed uint64 `json:"eventRecordFailed"`
	// Sync is the outcome of synchronous replication.
	Sync SyncStats `json:"sync"`
	// ChunkedVerifyFailed is the number of aws-chunked payload verification failures.
	ChunkedVerifyFailed uint64 `json:"chunkedVerifyFailed"`
	// NoCredentials is the number of requests failed with 503 because no credentials resolve for the user on the storage.
	NoCredentials uint64 `json:"noCredentials"`
	// BytesIn are bytes read from clients (request bodies), BytesOut bytes written to clients (response bodies).
	BytesIn  uint64 `json:"bytesIn"`
	BytesOut uint64 `json:"bytesOut"`
	// Latency is the histogram of the full request duration.
	Latency Latency `json:"latency"`
}

// Requests are request counters.
type Requests struct {
	Total uint64 `json:"total"`
	// ByClass: read, write, exists, manage, other.
	ByClass map[string]uint64 `json:"byClass"`
	// ByStorage is keyed by the storage the request was routed to.
	ByStorage map[string]uint64 `json:"byStorage"`
}

// Responses are counted by status class.
type Responses struct {
	Status2xx uint64 `json:"status2xx"`
	Status3xx uint64 `json:"status3xx"`
	Status4xx uint64 `json:"status4xx"`
	Status5xx uint64 `json:"status5xx"`
	Other     uint64 `json:"other"`
}

// SyncStats are synchronous replication outcomes.
type SyncStats struct {
	QuorumReached uint64 `json:"quorumReached"`
	BelowQuorum   uint64 `json:"belowQuorum"`
	Skipped       uint64 `json:"skipped"`
}

// Latency is a fixed-bucket histogram.
type Latency struct {
	Count      uint64  `json:"count"`
	SumSeconds float64 `json:"sumSeconds"`
	// BoundsSeconds are the inclusive upper bounds of the buckets; Buckets has one more (+Inf) element.
	// Buckets are not cumulative.
	BoundsSeconds []float64 `json:"boundsSeconds"`
	Buckets       []uint64  `json:"buckets"`
}

// Snapshot copies the counters.
func (c *Collector) Snapshot() Snapshot {
	s := Snapshot{
		CollectedAt: time.Now().UTC(),
		StartedAt:   c.startedAt,
		Requests: Requests{
			Total:     c.total.Load(),
			ByClass:   map[string]uint64{},
			ByStorage: map[string]uint64{},
		},
		Responses: Responses{
			Status2xx: c.status[0].Load(), Status3xx: c.status[1].Load(),
			Status4xx: c.status[2].Load(), Status5xx: c.status[3].Load(), Other: c.statusO.Load(),
		},
		FailFast:          c.failFast.Load(),
		AccessDenied:      c.accessDenied.Load(),
		EventRecordFailed: c.eventRecordErr.Load(),
		Sync: SyncStats{
			QuorumReached: c.sync[0].Load(), BelowQuorum: c.sync[1].Load(), Skipped: c.sync[2].Load(),
		},
		ChunkedVerifyFailed: c.chunkedFailed.Load(),
		NoCredentials:       c.noCredentials.Load(),
		BytesIn:             c.bytesIn.Load(),
		BytesOut:            c.bytesOut.Load(),
		Latency: Latency{
			Count:         c.latCount.Load(),
			SumSeconds:    float64(c.latSumNs.Load()) / float64(time.Second),
			BoundsSeconds: append([]float64(nil), LatencyBounds...),
			Buckets:       make([]uint64, len(c.latBuckets)),
		},
	}
	for i, name := range []string{ClassRead, ClassWrite, ClassExists, ClassManage, ClassOther} {
		s.Requests.ByClass[name] = c.classes[i].Load()
	}
	c.storage.Range(func(k, v any) bool {
		s.Requests.ByStorage[k.(string)] = v.(*atomic.Uint64).Load()
		return true
	})
	for i := range c.latBuckets {
		s.Latency.Buckets[i] = c.latBuckets[i].Load()
	}
	return s
}
