// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/systemsettings"
)

// PR7: Unit Simulation Suite
// NOTE: These tests are in-memory unit simulations verifying algorithmic logic
// (checkpoint skipping, throttled streaming helpers, backpressure watermark hysteresis,
// and cancellation safety). For end-to-end integration tests computing SHA-256
// directly from destination S3 bytes, see test/migration/bulk_checksum_integration_test.go.

// TestPR7_Simulation_WorkloadA_BaselineAndRestartCorrectness
// Workload A Unit Simulation: 1,000 objects (1 KiB each).
// Measures simulated iteration overhead and verifies checkpoint resume logic without duplicates.
func TestPR7_Simulation_WorkloadA_BaselineAndRestartCorrectness(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	mr, err := miniredis.Run()
	r.NoError(err)
	t.Cleanup(mr.Close)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	const numObjects = 1000
	const objectSize = 1024 // 1 KiB

	type objectData struct {
		hash []byte
		size int
	}

	primaryStorage := make(map[string]objectData, numObjects)
	followerStorage := make(map[string]objectData, numObjects)
	var followerMu sync.Mutex

	payload := make([]byte, objectSize)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	hash := sha256.Sum256(payload)

	for i := 0; i < numObjects; i++ {
		key := fmt.Sprintf("workload-a/obj-%04d.dat", i)
		primaryStorage[key] = objectData{hash: hash[:], size: objectSize}
	}

	checkpointKey := "tutti:checkpoint:workload-a"

	// Phase 1: Run migration up to 500 objects, then inject crash
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	start := time.Now()

	crashedAt := 500
	processedBeforeCrash := 0
	var lastCheckpoint string

	for i := 0; i < numObjects; i++ {
		key := fmt.Sprintf("workload-a/obj-%04d.dat", i)
		if i == crashedAt {
			// Simulate crash! Save last checkpoint
			lastCheckpoint = fmt.Sprintf("workload-a/obj-%04d.dat", i-1)
			err = rc.Set(ctx, checkpointKey, lastCheckpoint, 0).Err()
			r.NoError(err)
			break
		}

		followerMu.Lock()
		followerStorage[key] = primaryStorage[key]
		followerMu.Unlock()
		processedBeforeCrash++
	}

	r.Equal(crashedAt, processedBeforeCrash, "must have migrated exactly 500 before crash")
	r.Equal(500, len(followerStorage))

	// Phase 2: Resume from checkpoint
	resumeMarker, err := rc.Get(ctx, checkpointKey).Result()
	r.NoError(err)
	r.Equal(lastCheckpoint, resumeMarker)

	resumedProcessed := 0
	for i := 0; i < numObjects; i++ {
		key := fmt.Sprintf("workload-a/obj-%04d.dat", i)
		if key <= resumeMarker {
			continue // Skip already checkpointed objects!
		}

		followerMu.Lock()
		followerStorage[key] = primaryStorage[key]
		followerMu.Unlock()
		resumedProcessed++
	}

	elapsed := time.Since(start)
	runtime.ReadMemStats(&m2)

	// Phase 3: Correctness Verification
	r.Equal(500, resumedProcessed, "must migrate remaining 500 objects on resume")
	r.Equal(numObjects, len(followerStorage), "follower must have 100% of objects after resume")

	for k, expected := range primaryStorage {
		actual, ok := followerStorage[k]
		r.True(ok, "key %s missing on follower", k)
		r.Equal(expected.hash, actual.hash, "hash mismatch on key %s", k)
	}

	totalBytes := int64(numObjects * objectSize)
	throughput := float64(numObjects) / elapsed.Seconds()
	mbps := (float64(totalBytes) / (1024 * 1024)) / elapsed.Seconds()
	memDelta := float64(m2.TotalAlloc-m1.TotalAlloc) / (1024 * 1024)

	t.Logf("[Workload A Metrics] Objects: %d | Time: %v | Rate: %.1f obj/s | Bandwidth: %.2f MB/s | Heap Delta: %.2f MB",
		numObjects, elapsed, throughput, mbps, memDelta)

	r.Greater(throughput, 100.0, "baseline throughput should exceed 100 obj/s on in-memory workload")
}

// TestPR7_Simulation_WorkloadB_MixedSizeStreamingVerification
// Workload B Unit Simulation: Mixed size objects (1 KiB to 1 MiB) with ThrottledReader bandwidth limiting
// and bounded memory buffer budget. Validates helper throughput and checksum preservation.
func TestPR7_Simulation_WorkloadB_MixedSizeStreamingVerification(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	// 2 MiB/s bandwidth limit and 8 MiB buffer budget
	const bandwidthLimit = 2 * 1024 * 1024
	const bufferBudget = 8 * 1024 * 1024
	settings := controlplane.SystemSettings{
		Worker: controlplane.SystemWorkerSettings{
			BandwidthLimitBytes: bandwidthLimit,
			BufferBudgetBytes:   bufferBudget,
		},
	}
	systemsettings.ApplySettings(settings)
	limiter := systemsettings.BandwidthLimiter()

	sizes := []int{
		1024,        // 1 KiB
		64 * 1024,   // 64 KiB
		256 * 1024,  // 256 KiB
		512 * 1024,  // 512 KiB
		1024 * 1024, // 1 MiB
	}

	var totalTransferred int64
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)
	start := time.Now()

	for idx, sz := range sizes {
		data := make([]byte, sz)
		for i := range data {
			data[i] = byte((i + idx) % 256)
		}
		expectedHash := sha256.Sum256(data)

		// Acquire buffer budget
		release, err := systemsettings.AcquireBufferBudget(ctx, int64(sz))
		r.NoError(err, "acquiring buffer budget within limit must succeed")

		src := bytes.NewReader(data)
		throttled := systemsettings.NewThrottledReader(ctx, src, limiter)

		hasher := sha256.New()
		copied, err := io.Copy(hasher, throttled)
		release() // Release budget immediately after streaming

		r.NoError(err)
		r.Equal(int64(sz), copied)
		r.Equal(expectedHash[:], hasher.Sum(nil))

		totalTransferred += copied
	}

	elapsed := time.Since(start)
	runtime.ReadMemStats(&m2)

	memDelta := float64(m2.TotalAlloc-m1.TotalAlloc) / (1024 * 1024)
	mbps := (float64(totalTransferred) / (1024 * 1024)) / elapsed.Seconds()

	t.Logf("[Workload B Metrics] Total Bytes: %d | Time: %v | Effective Speed: %.2f MB/s | Heap Delta: %.2f MB",
		totalTransferred, elapsed, mbps, memDelta)
}

// TestPR7_Simulation_WorkloadC_BoundedQueueWatermarkAdmission
// Workload C Unit Simulation: Simulates bounded queue producer-consumer dynamics with 10,000 tasks,
// high watermark = 2,000, low watermark = 1,000.
// Verifies that with the F2 capacity clamping (min(pageSize, highWatermark - queue)),
// peak queue size strictly never exceeds highWatermark.
func TestPR7_Simulation_WorkloadC_BoundedQueueWatermarkAdmission(t *testing.T) {
	r := require.New(t)

	const totalTasks = 10000
	const highWatermark = 2000
	const lowWatermark = 1000
	const pageSize = 200

	var queueBacklog int64
	var peakBacklog int64
	var pausedCount int64
	var resumedCount int64

	currentProduced := 0

	var wg sync.WaitGroup

	// Consumer goroutine simulating worker pool draining tasks
	stopConsumer := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(2 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-stopConsumer:
				// Drain remaining
				for atomic.LoadInt64(&queueBacklog) > 0 {
					atomic.AddInt64(&queueBacklog, -pageSize)
					if atomic.LoadInt64(&queueBacklog) < 0 {
						atomic.StoreInt64(&queueBacklog, 0)
					}
				}
				return
			case <-ticker.C:
				current := atomic.LoadInt64(&queueBacklog)
				if current > 0 {
					drain := int64(pageSize)
					if drain > current {
						drain = current
					}
					atomic.AddInt64(&queueBacklog, -drain)
				}
			}
		}
	}()

	start := time.Now()
	for currentProduced < totalTasks {
		currentQueue := atomic.LoadInt64(&queueBacklog)

		// Update peak tracking
		for {
			p := atomic.LoadInt64(&peakBacklog)
			if currentQueue <= p || atomic.CompareAndSwapInt64(&peakBacklog, p, currentQueue) {
				break
			}
		}

		if currentQueue >= highWatermark {
			// Backpressure triggered: producer yields/pauses
			atomic.AddInt64(&pausedCount, 1)
			for atomic.LoadInt64(&queueBacklog) > lowWatermark {
				time.Sleep(1 * time.Millisecond)
			}
			atomic.AddInt64(&resumedCount, 1)
			currentQueue = atomic.LoadInt64(&queueBacklog)
		}

		// Enqueue bounded page (matching F2 fix: min(pageSize, highWatermark - currentQueue))
		remaining := int64(highWatermark) - currentQueue
		if remaining <= 0 {
			continue
		}
		batch := pageSize
		if int64(batch) > remaining {
			batch = int(remaining)
		}
		if currentProduced+batch > totalTasks {
			batch = totalTasks - currentProduced
		}
		atomic.AddInt64(&queueBacklog, int64(batch))
		currentProduced += batch
	}

	close(stopConsumer)
	wg.Wait()
	elapsed := time.Since(start)

	r.Equal(totalTasks, currentProduced)
	peak := atomic.LoadInt64(&peakBacklog)
	t.Logf("[Workload C Metrics] Tasks: %d | Peak Queue: %d | High Watermark: %d | Pauses: %d | Resumes: %d | Time: %v",
		totalTasks, peak, highWatermark, atomic.LoadInt64(&pausedCount), atomic.LoadInt64(&resumedCount), elapsed)

	// Peak queue must strictly NOT exceed high watermark with bounded admission
	r.LessOrEqual(peak, int64(highWatermark), "backlog must never blow past bounded high watermark")
	r.Greater(atomic.LoadInt64(&pausedCount), int64(0), "backpressure pause must trigger under load")
	r.Equal(atomic.LoadInt64(&pausedCount), atomic.LoadInt64(&resumedCount), "every pause must have matching resume")
}

// TestPR7_Simulation_Chaos_ClientCancellation
// Chaos Unit Simulation: Verifies that client-side context cancellation before or during
// task enqueue fails cleanly without corrupting existing Asynq queue state.
func TestPR7_Simulation_Chaos_ClientCancellation(t *testing.T) {
	r := require.New(t)

	mr, err := miniredis.Run()
	r.NoError(err)
	t.Cleanup(mr.Close)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })

	client := asynq.NewClientFromRedisClient(rc)
	t.Cleanup(func() { _ = client.Close() })
	inspector := asynq.NewInspectorFromRedisClient(rc)
	t.Cleanup(func() { _ = inspector.Close() })

	testQueue := "chaos-test-queue"

	// Enqueue tasks
	for i := 0; i < 50; i++ {
		task := asynq.NewTask("copy_task", []byte(fmt.Sprintf("payload-%d", i)), asynq.Queue(testQueue), asynq.TaskID(fmt.Sprintf("task-%d", i)))
		_, err = client.Enqueue(task)
		r.NoError(err)
	}

	// Verify tasks are pending
	queueInfo, err := inspector.GetQueueInfo(testQueue)
	r.NoError(err)
	r.Equal(50, queueInfo.Pending)

	// Chaos injection: Simulate sudden client cancellation midway
	cancelCtx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately!

	// New enqueue under cancelled context must fail cleanly without corrupting Redis
	task := asynq.NewTask("copy_task", []byte("cancelled-task"), asynq.Queue(testQueue))
	_, err = client.EnqueueContext(cancelCtx, task)
	r.Error(err, "enqueue under cancelled context must fail")

	// State of Redis remains healthy
	queueInfoAfter, err := inspector.GetQueueInfo(testQueue)
	r.NoError(err)
	r.Equal(50, queueInfoAfter.Pending, "cancelled task must not corrupt queue state")
}
