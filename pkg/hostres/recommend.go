package hostres

const (
	mib = int64(1) << 20
	gib = int64(1) << 30

	// Floors equal the historical static defaults, so small hosts are never worse than before.
	floorConcurrency = 10
	floorBuffer      = 256 * mib
	floorBulkPercent = 20

	maxConcurrency = 128
	maxBuffer      = 8 * gib
	// bufferMemShare is the share of memory the copy buffer budget may take. The rest is left for the Go
	// heap overhead, Redis/DB clients, TLS, and GC headroom.
	bufferMemShare = 4 // 1/4 = 25%
)

// Recommended is the set of derived worker settings.
type Recommended struct {
	Concurrency            int
	BulkConcurrencyPercent int
	BufferBudgetBytes      int64
	MultipartThreads       int
	MultipartPartSizeMiB   int
}

// Recommend derives worker settings from the available CPU count and memory in bytes.
//
//   - concurrency: copy tasks are mostly network bound, so ~10 task slots per CPU, bounded by 128 and by
//     what the buffer budget can feed (each slot holds one 16 MiB part, so budget/32MiB), floor 10.
//   - bufferBudgetBytes: 25% of memory, clamped to 256 MiB..8 GiB.
//   - bulkConcurrencyPercent: 20 up to 3 CPUs, 30 for 4..7, 40 from 8 (more headroom for migration while
//     live events keep a majority of slots).
//   - multipartThreads stays 1: a local loopback measurement showed parallel parts were slower and the
//     WAN gain is unverified, so it is only raised by an explicit setting. Part size stays 16 MiB.
func Recommend(cpu int, memBytes int64) Recommended {
	if cpu < 1 {
		cpu = 1
	}
	buf := memBytes / bufferMemShare
	if buf > maxBuffer {
		buf = maxBuffer
	}
	if buf < floorBuffer {
		buf = floorBuffer
	}
	conc := cpu * 10
	if byBuf := int(buf / (32 * mib)); conc > byBuf {
		conc = byBuf
	}
	if conc > maxConcurrency {
		conc = maxConcurrency
	}
	if conc < floorConcurrency {
		conc = floorConcurrency
	}
	bulk := floorBulkPercent
	switch {
	case cpu >= 8:
		bulk = 40
	case cpu >= 4:
		bulk = 30
	}
	return Recommended{Concurrency: conc, BulkConcurrencyPercent: bulk, BufferBudgetBytes: buf, MultipartThreads: 1, MultipartPartSizeMiB: 16}
}
