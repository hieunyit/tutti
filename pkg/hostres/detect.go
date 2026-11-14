// Package hostres detects the CPU and memory actually available to the process (container and
// Kubernetes limits included) and derives worker tuning defaults from them.
package hostres

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Hardware sources reported in Hardware.Source.
const (
	SourceHost     = "host"
	SourceCgroupV1 = "cgroup-v1"
	SourceCgroupV2 = "cgroup-v2"
)

// Hardware is the resource envelope available to this process.
type Hardware struct {
	// CPU is the effective CPU count, rounded UP from CPUFractional (a 1500m limit gives 2), minimum 1.
	// Rounding up is deliberate: the work is I/O bound, so a fractional core still benefits from a whole
	// core's worth of goroutines, and floor would turn a 1500m pod into 1.
	CPU int
	// CPUFractional is the unrounded value, e.g. 1.5 for a quota of 150000/100000.
	CPUFractional float64
	// MemoryBytes is the effective memory: the smaller of host RAM and the cgroup limit.
	MemoryBytes int64
	// Source is host, cgroup-v1 or cgroup-v2: where the binding limit came from.
	Source string
	// MemoryLimited reports whether a cgroup memory limit (below host RAM) applies.
	MemoryLimited bool
}

// Options makes detection testable. Zero values mean the real system.
type Options struct {
	Root   string              // filesystem root, default "/"
	NumCPU int                 // default runtime.NumCPU()
	Getenv func(string) string // default os.Getenv
}

// Detect inspects the running system.
func Detect() Hardware { return DetectWith(Options{}) }

// DetectWith is Detect with an injectable root and environment. Missing or unparsable files are ignored.
func DetectWith(o Options) Hardware {
	if o.Root == "" {
		o.Root = "/"
	}
	if o.NumCPU <= 0 {
		o.NumCPU = runtime.NumCPU()
	}
	if o.Getenv == nil {
		o.Getenv = os.Getenv
	}
	read := func(p string) string {
		b, err := os.ReadFile(filepath.Join(o.Root, p))
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}

	cpu := float64(o.NumCPU)
	source := SourceHost

	hostMem := parseMeminfo(read("proc/meminfo"))
	mem := hostMem

	// cgroup v2
	cpuV2, memV2 := parseCPUMaxV2(read("sys/fs/cgroup/cpu.max")), parseLimit(read("sys/fs/cgroup/memory.max"))
	// cgroup v1 (CPU controller is mounted as cpu or cpu,cpuacct)
	var cpuV1 float64
	for _, d := range []string{"sys/fs/cgroup/cpu", "sys/fs/cgroup/cpu,cpuacct", "sys/fs/cgroup/cpuacct,cpu"} {
		if q, p := read(d+"/cpu.cfs_quota_us"), read(d+"/cpu.cfs_period_us"); q != "" {
			cpuV1 = parseQuotaV1(q, p)
			break
		}
	}
	memV1 := parseLimit(read("sys/fs/cgroup/memory/memory.limit_in_bytes"))

	cpuLim, memLim, src := cpuV2, memV2, SourceCgroupV2
	if cpuLim <= 0 && memLim <= 0 {
		cpuLim, memLim, src = cpuV1, memV1, SourceCgroupV1
	}
	if cpuLim > 0 && cpuLim < cpu {
		cpu, source = cpuLim, src
	}
	memoryLimited := false
	// A "limit" at or above host RAM (the v1 unlimited sentinel is ~2^63) is no limit.
	if memLim > 0 && (hostMem <= 0 || memLim < hostMem) {
		mem, source, memoryLimited = memLim, src, true
	}

	if g, err := strconv.Atoi(o.Getenv("GOMAXPROCS")); err == nil && g > 0 && float64(g) < cpu {
		cpu = float64(g)
	}
	if cpu < 1 {
		cpu = math.Max(cpu, 0.01)
	}
	n := int(math.Ceil(cpu - 1e-9))
	if n < 1 {
		n = 1
	}
	return Hardware{CPU: n, CPUFractional: cpu, MemoryBytes: mem, Source: source, MemoryLimited: memoryLimited}
}

func parseMeminfo(s string) int64 {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				if kb, err := strconv.ParseInt(f[1], 10, 64); err == nil {
					return kb * 1024
				}
			}
		}
	}
	return 0
}

// parseCPUMaxV2 parses "max 100000" (0 = unlimited) or "150000 100000" (1.5).
func parseCPUMaxV2(s string) float64 {
	f := strings.Fields(s)
	if len(f) < 1 || f[0] == "max" {
		return 0
	}
	return parseQuotaV1(f[0], pick(f, 1))
}

func pick(f []string, i int) string {
	if i < len(f) {
		return f[i]
	}
	return "100000"
}

// parseQuotaV1 returns quota/period CPUs, or 0 when unlimited (-1) or unparsable.
func parseQuotaV1(quota, period string) float64 {
	q, err := strconv.ParseFloat(quota, 64)
	if err != nil || q <= 0 {
		return 0
	}
	p, err := strconv.ParseFloat(period, 64)
	if err != nil || p <= 0 {
		return 0
	}
	return q / p
}

// parseLimit returns a byte limit, or 0 for "max", empty, or unparsable values.
func parseLimit(s string) int64 {
	if s == "" || s == "max" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}
