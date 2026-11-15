package systemsettings

import (
	"sync"
	"sync/atomic"
)

const (
	// LargeObjectThreshold: objects of at least this size count as large (MinIO replication uses the same value).
	LargeObjectThreshold = 128 << 20
	// DefaultLargeObjectStreams applies before the hardware is detected.
	DefaultLargeObjectStreams = 4
	MaxLargeObjectStreams     = 64
)

var yamlLargeObjectStreams atomic.Int64

// SetLargeObjectStreamsFallback records the YAML/env value used when the system setting is 0.
func SetLargeObjectStreamsFallback(n int) { yamlLargeObjectStreams.Store(int64(n)) }

// EffectiveLargeObjectStreams resolves the per-destination-storage cap of concurrent large copies:
// system setting, then YAML/env, then the hardware-derived value.
func EffectiveLargeObjectStreams() int {
	e := ResolveEffective(Current().Worker).LargeObjectStreams
	n := int(e.Value)
	if n < 1 {
		n = DefaultLargeObjectStreams
	}
	return n
}

var largeSlots = struct {
	mu   sync.Mutex
	used map[string]int
}{used: map[string]int{}}

// TryAcquireLargeObject reserves one of the concurrent large-copy slots of a destination storage.
// Objects below LargeObjectThreshold never take or wait for a slot (ok is always true, release is a no-op).
// It never blocks: when all slots are taken ok is false and the caller is expected to retry later, so a
// waiting copy holds no worker slot. The limit is read on every call, so setting changes apply live.
func TryAcquireLargeObject(destStorage string, size int64) (release func(), ok bool) {
	if size < LargeObjectThreshold {
		return func() {}, true
	}
	limit := EffectiveLargeObjectStreams()
	largeSlots.mu.Lock()
	if largeSlots.used[destStorage] >= limit {
		largeSlots.mu.Unlock()
		return nil, false
	}
	largeSlots.used[destStorage]++
	largeSlots.mu.Unlock()
	var done atomic.Bool
	return func() {
		if done.CompareAndSwap(false, true) {
			largeSlots.mu.Lock()
			if largeSlots.used[destStorage]--; largeSlots.used[destStorage] <= 0 {
				delete(largeSlots.used, destStorage)
			}
			largeSlots.mu.Unlock()
		}
	}, true
}

// LargeObjectsInFlight returns the number of large copies holding a slot for the destination storage.
func LargeObjectsInFlight(destStorage string) int {
	largeSlots.mu.Lock()
	defer largeSlots.mu.Unlock()
	return largeSlots.used[destStorage]
}
