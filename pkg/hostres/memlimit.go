package hostres

import (
	"os"
	"runtime/debug"
)

// ApplyMemoryLimit sets the Go soft memory limit (GOMEMLIMIT) to 90% of the cgroup memory limit so the GC
// works harder before the container is OOM-killed. It does nothing when GOMEMLIMIT is already set or no
// cgroup memory limit applies. It returns the applied limit in bytes (0 = not applied).
func ApplyMemoryLimit(h Hardware) int64 {
	if os.Getenv("GOMEMLIMIT") != "" || !h.MemoryLimited || h.MemoryBytes <= 0 {
		return 0
	}
	limit := h.MemoryBytes / 10 * 9
	debug.SetMemoryLimit(limit)
	return limit
}
