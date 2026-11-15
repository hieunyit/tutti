package copy

// In-process benchmarks of the copy engines against synthetic S3 servers that run in CHILD processes, so the
// CPU time measured here (getrusage of this process) is the worker side only. The servers can cap bandwidth
// per stream (a per-flow limit, like a WAN path) or in total and add per-request latency. Loopback numbers are
// not WAN numbers: no TLS, no real RTT jitter, no loss; they compare engines under the same fake conditions.
//
//   GOMAXPROCS=2 go test ./service/worker/copy -run '^$' -bench 'Copy/' -benchtime 1x -benchmem
//   BENCH_GBYTES / BENCH_NET select a scenario, see benchScenarios.

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/features"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3client"
	"github.com/hieunyit/tutti/pkg/systemsettings"
)

func TestMain(m *testing.M) {
	if os.Getenv("MEMS3_CHILD") == "1" {
		runMemS3Child()
		return
	}
	os.Exit(m.Run())
}

// runMemS3Child serves a synthetic S3 until stdin is closed and prints its URL on the first line.
func runMemS3Child() {
	f := newMemS3()
	f.synthetic = true
	f.synthSource = os.Getenv("MEMS3_SOURCE") == "1"
	if v, _ := strconv.Atoi(os.Getenv("MEMS3_LATENCY_MS")); v > 0 {
		f.Latency = time.Duration(v) * time.Millisecond
	}
	if v, _ := strconv.ParseInt(os.Getenv("MEMS3_STREAM_BPS"), 10, 64); v > 0 {
		f.StreamBps = v
	}
	if v, _ := strconv.ParseInt(os.Getenv("MEMS3_TOTAL_BPS"), 10, 64); v > 0 {
		f.SetTotalBps(v)
	}
	fmt.Println(f.URL())
	_, _ = bufio.NewReader(os.Stdin).ReadByte()
	f.Close()
}

type benchServer struct {
	url  string
	stop func()
}

func startChildS3(b *testing.B, source bool, latencyMS int, streamBps, totalBps int64) benchServer {
	b.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	role := "0"
	if source {
		role = "1"
	}
	cmd.Env = append(os.Environ(), "MEMS3_CHILD=1", "MEMS3_SOURCE="+role, "GOMAXPROCS=1",
		"MEMS3_LATENCY_MS="+strconv.Itoa(latencyMS), "MEMS3_STREAM_BPS="+strconv.FormatInt(streamBps, 10), "MEMS3_TOTAL_BPS="+strconv.FormatInt(totalBps, 10))
	stdin, err := cmd.StdinPipe()
	require.NoError(b, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(b, err)
	require.NoError(b, cmd.Start())
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(b, err)
	return benchServer{url: strings.TrimSpace(line), stop: func() { _ = stdin.Close(); _ = cmd.Wait() }}
}

func cpuSeconds() float64 {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return float64(ru.Utime.Sec+ru.Stime.Sec) + float64(ru.Utime.Usec+ru.Stime.Usec)/1e6
}

type benchNet struct {
	name        string
	latencyMS   int
	srcStream   int64 // bytes/s per stream, 0 = unlimited
	dstStream   int64
	srcTotal    int64
	dstTotal    int64
	description string
}

const mbps = 1 << 20

var benchScenarios = []benchNet{
	{name: "unlimited", description: "no caps, no latency: CPU cost per GiB"},
	{name: "perflow50x50_30ms", latencyMS: 30, srcStream: 50 * mbps, dstStream: 50 * mbps, description: "source and destination 50 MiB/s per stream, 30 ms per request"},
	{name: "perflow100x25_30ms", latencyMS: 30, srcStream: 100 * mbps, dstStream: 25 * mbps, description: "fast source, slow destination per stream"},
	{name: "perflow25x100_30ms", latencyMS: 30, srcStream: 25 * mbps, dstStream: 100 * mbps, description: "slow source, fast destination per stream"},
	{name: "total50x50_30ms", latencyMS: 30, srcTotal: 50 * mbps, dstTotal: 50 * mbps, description: "50 MiB/s TOTAL on each side (shared pipe), 30 ms"},
}

type benchEngine struct {
	name    string
	engine  string
	threads int
}

var benchEngines = []benchEngine{
	{"legacy-t1", "legacy", 1},
	{"legacy-t2", "legacy", 2},
	{"pipeline-t2", "pipeline", 2},
	{"pipeline-t4", "pipeline", 4},
}

func BenchmarkCopy(b *testing.B) {
	sel := os.Getenv("BENCH_NET")
	for _, sc := range benchScenarios {
		if sel != "" && sel != sc.name {
			continue
		}
		for _, objs := range []int{1, 8} {
			for _, eng := range benchEngines {
				size := int64(256 << 20)
				if sc.name == "unlimited" {
					size = 512 << 20
				}
				b.Run(fmt.Sprintf("%s/%dx%dMiB/%s", sc.name, objs, size>>20, eng.name), func(b *testing.B) {
					runCopyBench(b, sc, eng, size, objs)
				})
			}
		}
	}
}

func runCopyBench(b *testing.B, sc benchNet, eng benchEngine, size int64, objs int) {
	old := &features.Config{}
	features.Set(old)
	prev := systemsettings.Current()
	b.Cleanup(func() { systemsettings.ApplySettings(prev) })
	systemsettings.ApplySettings(controlplane.SystemSettings{Worker: controlplane.SystemWorkerSettings{
		BufferBudgetBytes: 4 << 30, CopyEngine: eng.engine, MultipartThreads: eng.threads, MultipartPartSizeMiB: 16,
	}})
	features.Set(old)

	src := startChildS3(b, true, sc.latencyMS, sc.srcStream, sc.srcTotal)
	dst := startChildS3(b, false, sc.latencyMS, sc.dstStream, sc.dstTotal)
	defer src.stop()
	defer dst.stop()
	clients := comprehensiveClients{source: benchClient(b, src.url), dest: benchClient(b, dst.url)}
	svc := NewS3CopySvc(comprehensiveCreds{}, clients, metrics.NewS3Service(false))

	totalBytes := size * int64(objs)
	b.SetBytes(totalBytes)
	b.ReportAllocs()
	// warm up the connection pools and code paths with a small object
	require.NoError(b, svc.CopyObject(context.Background(), "u", NewFile("source", "bucket", "s1048576-warm"), NewFile("dest", "bucket", "s1048576-warm")))

	var peakHeap uint64
	stopSampler := make(chan struct{})
	var samplerWG sync.WaitGroup
	samplerWG.Add(1)
	go func() {
		defer samplerWG.Done()
		tk := time.NewTicker(20 * time.Millisecond)
		defer tk.Stop()
		var ms runtime.MemStats
		for {
			select {
			case <-stopSampler:
				return
			case <-tk.C:
				runtime.ReadMemStats(&ms)
				if ms.HeapInuse > peakHeap {
					peakHeap = ms.HeapInuse
				}
			}
		}
	}()

	runtime.GC()
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	partBuffers.peak.Store(0)
	cpu0 := cpuSeconds()
	b.ResetTimer()
	var wall time.Duration
	for i := 0; i < b.N; i++ {
		t0 := time.Now()
		var wg sync.WaitGroup
		errs := make(chan error, objs)
		for j := 0; j < objs; j++ {
			wg.Add(1)
			go func(j int) {
				defer wg.Done()
				name := fmt.Sprintf("s%d-%d-%d", size, i, j)
				errs <- svc.CopyObject(context.Background(), "u", NewFile("source", "bucket", name), NewFile("dest", "bucket", name))
			}(j)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(b, err)
		}
		wall += time.Since(t0)
	}
	b.StopTimer()
	cpu := cpuSeconds() - cpu0
	runtime.ReadMemStats(&m1)
	close(stopSampler)
	samplerWG.Wait()

	gib := float64(totalBytes) * float64(b.N) / float64(1<<30)
	b.ReportMetric(float64(totalBytes)*float64(b.N)/(1<<20)/wall.Seconds(), "MiB/s")
	b.ReportMetric(cpu/gib, "cpu-s/GiB")
	b.ReportMetric(float64(m1.Mallocs-m0.Mallocs)/gib, "allocs/GiB")
	b.ReportMetric(float64(m1.TotalAlloc-m0.TotalAlloc)/gib/(1<<20), "allocMiB/GiB")
	b.ReportMetric(float64(peakHeap)/(1<<20)/float64(objs), "peakHeapMiB/obj")
	b.ReportMetric(float64(partBuffers.peak.Load())/(1<<20)/float64(objs), "poolPeakMiB/obj")
	b.ReportMetric(wall.Seconds()/float64(b.N), "s/op-wall")
}

func benchClient(b *testing.B, url string) s3client.Client {
	c, err := s3client.NewClient(context.Background(), comprehensiveMetrics{}, s3.StorageAddress{Address: url, DefaultRegion: "us-east-1"},
		s3.CredentialsV4{AccessKeyID: "access", SecretAccessKey: "secret"}, "storage", "u")
	require.NoError(b, err)
	return c
}
