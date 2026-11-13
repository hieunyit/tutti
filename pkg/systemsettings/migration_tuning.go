package systemsettings

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
