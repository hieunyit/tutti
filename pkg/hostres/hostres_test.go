package hostres

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const meminfo8G = "MemTotal:        8388608 kB\nMemFree: 1 kB\n"

func TestDetect(t *testing.T) {
	const g8 = int64(8) << 30
	cases := []struct {
		name   string
		files  map[string]string
		env    string
		cpu    int
		frac   float64
		mem    int64
		source string
		memLim bool
	}{
		{"bare host", map[string]string{"proc/meminfo": meminfo8G}, "", 8, 8, g8, SourceHost, false},
		{"missing files", nil, "", 8, 8, 0, SourceHost, false},
		{"v2 limits", map[string]string{"proc/meminfo": meminfo8G, "sys/fs/cgroup/cpu.max": "200000 100000", "sys/fs/cgroup/memory.max": "2147483648"}, "", 2, 2, 2 << 30, SourceCgroupV2, true},
		{"v2 max", map[string]string{"proc/meminfo": meminfo8G, "sys/fs/cgroup/cpu.max": "max 100000", "sys/fs/cgroup/memory.max": "max"}, "", 8, 8, g8, SourceHost, false},
		{"v2 fractional 1500m rounds up", map[string]string{"proc/meminfo": meminfo8G, "sys/fs/cgroup/cpu.max": "150000 100000"}, "", 2, 1.5, g8, SourceCgroupV2, false},
		{"v2 tiny quota min 1", map[string]string{"sys/fs/cgroup/cpu.max": "20000 100000"}, "", 1, 0.2, 0, SourceCgroupV2, false},
		{"v1 limits", map[string]string{"proc/meminfo": meminfo8G, "sys/fs/cgroup/cpu/cpu.cfs_quota_us": "300000", "sys/fs/cgroup/cpu/cpu.cfs_period_us": "100000", "sys/fs/cgroup/memory/memory.limit_in_bytes": "1073741824"}, "", 3, 3, 1 << 30, SourceCgroupV1, true},
		{"v1 unlimited sentinel", map[string]string{"proc/meminfo": meminfo8G, "sys/fs/cgroup/cpu/cpu.cfs_quota_us": "-1", "sys/fs/cgroup/cpu/cpu.cfs_period_us": "100000", "sys/fs/cgroup/memory/memory.limit_in_bytes": "9223372036854771712"}, "", 8, 8, g8, SourceHost, false},
		{"GOMAXPROCS lowers", map[string]string{"proc/meminfo": meminfo8G}, "3", 3, 3, g8, SourceHost, false},
		{"GOMAXPROCS above cap ignored", map[string]string{"sys/fs/cgroup/cpu.max": "200000 100000"}, "16", 2, 2, 0, SourceCgroupV2, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			for k, v := range c.files {
				write(t, root, k, v)
			}
			h := DetectWith(Options{Root: root, NumCPU: 8, Getenv: func(string) string { return c.env }})
			if h.CPU != c.cpu || h.CPUFractional != c.frac || h.MemoryBytes != c.mem || h.Source != c.source || h.MemoryLimited != c.memLim {
				t.Fatalf("got %+v", h)
			}
		})
	}
}

func TestRecommendTable(t *testing.T) {
	cases := []struct {
		cpu  int
		mem  int64
		want Recommended
	}{
		{1, 1 * gib, Recommended{10, 20, 256 * mib, 1, 16}},
		{2, 4 * gib, Recommended{20, 20, 1 * gib, 1, 16}},
		{4, 32 * gib, Recommended{40, 30, 8 * gib, 1, 16}},
		{16, 64 * gib, Recommended{128, 40, 8 * gib, 1, 16}},
		{0, 0, Recommended{10, 20, 256 * mib, 1, 16}},
		{64, 512 * mib, Recommended{10, 40, 256 * mib, 1, 16}}, // memory-starved: concurrency follows buffer, floor 10
	}
	for _, c := range cases {
		if got := Recommend(c.cpu, c.mem); got != c.want {
			t.Errorf("Recommend(%d,%d)=%+v want %+v", c.cpu, c.mem, got, c.want)
		}
	}
}
