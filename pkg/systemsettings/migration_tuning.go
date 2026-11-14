package systemsettings

import "sync/atomic"

const (
	DefaultBulkConcurrencyPercent = 20
	MinBulkConcurrencyPercent     = 5
	MaxBulkConcurrencyPercent     = 95

	DefaultMultipartThreads     = 1
	MaxMultipartThreads         = 16
	DefaultMultipartPartSizeMiB = 16
	MinMultipartPartSizeMiB     = 5
)

// SplitConcurrency splits total worker slots between the bulk (migration) and live (event) servers.
// percent is the bulk share; a value outside 5..95 (including 0 = unset) means the 20% default.
// Both sides always get at least one slot; fractional bulk slots are rounded down.
func SplitConcurrency(total, percent int) (bulk, live int) {
	if percent < MinBulkConcurrencyPercent || percent > MaxBulkConcurrencyPercent {
		percent = DefaultBulkConcurrencyPercent
	}
	bulk = total * percent / 100
	if bulk < 1 {
		bulk = 1
	}
	live = total - bulk
	if live < 1 {
		live = 1
	}
	return bulk, live
}

// MultipartTuning normalizes multipart upload settings: threads 1..16 (default 1) and part size in
// bytes (default 16 MiB, minimum 5 MiB).
func MultipartTuning(threads, partSizeMiB int) (int, uint64) {
	if threads < 1 {
		threads = DefaultMultipartThreads
	}
	if threads > MaxMultipartThreads {
		threads = MaxMultipartThreads
	}
	if partSizeMiB < MinMultipartPartSizeMiB {
		partSizeMiB = DefaultMultipartPartSizeMiB
	}
	return threads, uint64(partSizeMiB) << 20
}

// Precedence of the tuning knobs: system setting (non-zero) > YAML/env (non-zero) > hardware-derived value
// (see auto.go) > built-in default. The derived multipart values equal the built-in defaults (threads 1, 16 MiB).

var yamlBulkPercent, yamlMultipartThreads, yamlMultipartPartMiB atomic.Int64

// SetConfigFallbacks records the YAML/env values used when the system setting is 0.
func SetConfigFallbacks(bulkPercent, multipartThreads, multipartPartSizeMiB int) {
	yamlBulkPercent.Store(int64(bulkPercent))
	yamlMultipartThreads.Store(int64(multipartThreads))
	yamlMultipartPartMiB.Store(int64(multipartPartSizeMiB))
}

// ResolveBulkPercent applies the precedence for bulkConcurrencyPercent.
func ResolveBulkPercent(setting, yaml int) int {
	if setting >= MinBulkConcurrencyPercent && setting <= MaxBulkConcurrencyPercent {
		return setting
	}
	if yaml >= MinBulkConcurrencyPercent && yaml <= MaxBulkConcurrencyPercent {
		return yaml
	}
	return DefaultBulkConcurrencyPercent
}

// ResolveMultipart applies the precedence for the multipart knobs. partExplicit reports whether the part
// size was configured (setting or YAML) rather than the built-in default.
func ResolveMultipart(settingThreads, settingPartMiB, yamlThreads, yamlPartMiB int) (threads, partMiB int, partExplicit bool) {
	threads = settingThreads
	if threads < 1 {
		threads = yamlThreads
	}
	partMiB = settingPartMiB
	if partMiB < MinMultipartPartSizeMiB {
		partMiB = yamlPartMiB
	}
	partExplicit = partMiB >= MinMultipartPartSizeMiB
	t, p := MultipartTuning(threads, partMiB)
	return t, int(p >> 20), partExplicit
}

// EffectiveBulkPercent resolves bulkConcurrencyPercent from the active settings and YAML fallback.
func EffectiveBulkPercent() int {
	if v := ResolveBulkPercentExplicit(Current().Worker.BulkConcurrencyPercent, int(yamlBulkPercent.Load())); v > 0 {
		return v
	}
	if _, rec, ok := Hardware(); ok {
		return rec.BulkConcurrencyPercent
	}
	return DefaultBulkConcurrencyPercent
}

// EffectiveMultipart resolves the multipart knobs from the active settings (read at copy time, so changes
// apply to the next copy) and the YAML fallback.
func EffectiveMultipart() (threads int, partSize uint64, partExplicit bool) {
	w := Current().Worker
	t, mib, explicit := ResolveMultipart(w.MultipartThreads, w.MultipartPartSizeMiB, int(yamlMultipartThreads.Load()), int(yamlMultipartPartMiB.Load()))
	return t, uint64(mib) << 20, explicit
}
