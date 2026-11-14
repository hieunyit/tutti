package systemsettings

import (
	"sync/atomic"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/hostres"
)

// Hardware-derived ("auto") tuning. A knob that has no explicit value (stored setting 0, YAML/env 0) uses
// the value recommended for the detected hardware. Explicit values always win.

type autoState struct {
	hw  hostres.Hardware
	rec hostres.Recommended
}

var (
	autoVal         atomic.Pointer[autoState]
	yamlConcurrency atomic.Int64 // worker config / CFG_CONCURRENCY value, 0 = auto
)

// SetConcurrencyFallback records the YAML/env concurrency used when the stored setting is 0.
func SetConcurrencyFallback(v int) { yamlConcurrency.Store(int64(v)) }

// SetHardware records the detected hardware and the recommendation derived from it.
func SetHardware(h hostres.Hardware) {
	autoVal.Store(&autoState{hw: h, rec: hostres.Recommend(h.CPU, h.MemoryBytes)})
	ApplyWorkerLimiting(Current()) // re-evaluate the auto buffer budget
}

// Hardware returns the detected hardware and recommendation (ok=false before SetHardware).
func Hardware() (hostres.Hardware, hostres.Recommended, bool) {
	a := autoVal.Load()
	if a == nil {
		return hostres.Hardware{}, hostres.Recommended{}, false
	}
	return a.hw, a.rec, true
}

// Knob source values.
const (
	SourceAuto     = "auto"
	SourceExplicit = "explicit"
)

// EffectiveValue is a resolved setting and whether it came from an explicit value or hardware detection.
type EffectiveValue struct {
	Value  int64
	Source string
}

// Effective is the resolved view of the tunables.
type Effective struct {
	Concurrency, BulkConcurrencyPercent, BufferBudgetBytes, MultipartThreads, MultipartPartSizeMiB EffectiveValue
}

func pickInt(explicit int, auto int64, haveAuto bool, fallback int64) EffectiveValue {
	switch {
	case explicit > 0:
		return EffectiveValue{int64(explicit), SourceExplicit}
	case haveAuto:
		return EffectiveValue{auto, SourceAuto}
	}
	return EffectiveValue{fallback, SourceAuto}
}

// ResolveEffective resolves every tunable: stored system setting, then YAML/env concurrency, then auto.
func ResolveEffective(w controlplane.SystemWorkerSettings) Effective {
	_, rec, ok := Hardware()
	conc := w.Concurrency
	if conc <= 0 {
		conc = int(yamlConcurrency.Load())
	}
	e := Effective{
		Concurrency:            pickInt(conc, int64(rec.Concurrency), ok, 10),
		BulkConcurrencyPercent: pickInt(ResolveBulkPercentExplicit(w.BulkConcurrencyPercent, int(yamlBulkPercent.Load())), int64(rec.BulkConcurrencyPercent), ok, DefaultBulkConcurrencyPercent),
		BufferBudgetBytes:      pickInt(0, rec.BufferBudgetBytes, ok, 256<<20),
		MultipartThreads:       pickInt(firstPositive(w.MultipartThreads, int(yamlMultipartThreads.Load())), int64(rec.MultipartThreads), ok, DefaultMultipartThreads),
		MultipartPartSizeMiB:   pickInt(firstPartSize(w.MultipartPartSizeMiB, int(yamlMultipartPartMiB.Load())), int64(rec.MultipartPartSizeMiB), ok, DefaultMultipartPartSizeMiB),
	}
	if w.BufferBudgetBytes > 0 {
		e.BufferBudgetBytes = EffectiveValue{w.BufferBudgetBytes, SourceExplicit}
	}
	return e
}

func firstPositive(a, b int) int {
	if a > 0 {
		return a
	}
	if b > 0 {
		return b
	}
	return 0
}

func firstPartSize(a, b int) int {
	if a >= MinMultipartPartSizeMiB {
		return a
	}
	if b >= MinMultipartPartSizeMiB {
		return b
	}
	return 0
}

// ResolveBulkPercentExplicit returns the explicit bulk percent (setting, then YAML) or 0 when unset/invalid.
func ResolveBulkPercentExplicit(setting, yaml int) int {
	for _, v := range []int{setting, yaml} {
		if v >= MinBulkConcurrencyPercent && v <= MaxBulkConcurrencyPercent {
			return v
		}
	}
	return 0
}

// autoBufferBudget returns the derived buffer budget (256 MiB before hardware detection).
func autoBufferBudget() int64 {
	if a := autoVal.Load(); a != nil {
		return a.rec.BufferBudgetBytes
	}
	return 256 << 20
}
