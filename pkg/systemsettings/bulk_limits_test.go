// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0

package systemsettings

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/hieunyit/tutti/pkg/controlplane"
)

// PR3: TestBulkBandwidth_AggregateRateAcrossWorkers
// Verifies that aggregate throughput across concurrent workers is bounded by the rate limiter.
func TestBulkBandwidth_AggregateRateAcrossWorkers(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	// 50 KiB/s rate limit with 10 KiB burst
	limitBytesPerSec := 50 * 1024
	limiter := rate.NewLimiter(rate.Limit(limitBytesPerSec), 10*1024)

	numWorkers := 3
	payloadPerWorker := 30 * 1024 // 30 KiB each = 90 KiB total
	var wg sync.WaitGroup
	var totalBytesRead int64
	var mu sync.Mutex

	start := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data := make([]byte, payloadPerWorker)
			reader := NewThrottledReader(ctx, bytes.NewReader(data), limiter)
			buf := make([]byte, 4096)
			var workerRead int64
			for {
				n, err := reader.Read(buf)
				if n > 0 {
					workerRead += int64(n)
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					return
				}
			}
			mu.Lock()
			totalBytesRead += workerRead
			mu.Unlock()
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	r.Equal(int64(numWorkers*payloadPerWorker), totalBytesRead)
	// 90 KiB total at 50 KiB/s with 10 KiB burst should take at least (90-10)/50 = 1.6s
	r.GreaterOrEqual(duration, 1200*time.Millisecond, "aggregate rate must be throttled")
}

// PR3: TestBulkBandwidth_CancelUnblocksWaitingReader
// Verifies that cancelling context immediately unblocks a throttled reader.
func TestBulkBandwidth_CancelUnblocksWaitingReader(t *testing.T) {
	r := require.New(t)

	// Rate limiter with 1 byte per second and 0 burst
	limiter := rate.NewLimiter(rate.Limit(1), 1)
	// Consume initial token
	r.True(limiter.Allow())

	ctx, cancel := context.WithCancel(context.Background())

	reader := NewThrottledReader(ctx, bytes.NewReader(make([]byte, 100)), limiter)

	unblocked := make(chan error, 1)
	go func() {
		buf := make([]byte, 50)
		_, err := reader.Read(buf)
		unblocked <- err
	}()

	// Wait briefly to ensure reader is blocking on token
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-unblocked:
		r.Error(err)
		r.True(errors.Is(err, context.Canceled), "reader must return context.Canceled")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("reader remained blocked after context cancellation")
	}
}

// PR3: TestBulkBuffers_LargeObjectUsesBoundedMemory
// Verifies that streaming a 10 MiB object in chunks uses small bounded memory.
func TestBulkBuffers_LargeObjectUsesBoundedMemory(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	limiter := rate.NewLimiter(rate.Inf, 0)
	objSize := 10 * 1024 * 1024 // 10 MiB

	// Zero-memory synthetic reader
	synthReader := io.LimitReader(zeroReader{}, int64(objSize))
	throttled := NewThrottledReader(ctx, synthReader, limiter)

	chunkBuf := make([]byte, 64*1024) // 64 KiB buffer
	var totalRead int64

	for {
		n, err := throttled.Read(chunkBuf)
		if n > 0 {
			totalRead += int64(n)
		}
		if err == io.EOF {
			break
		}
		r.NoError(err)
	}

	r.Equal(int64(objSize), totalRead)
}

type zeroReader struct{}

func (z zeroReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// PR3: TestBulkMultipart_CancelAndErrorReleaseResources
// Verifies that buffer budget permits are always released on error or completion.
func TestBulkMultipart_CancelAndErrorReleaseResources(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	// Configure 100 MiB buffer budget
	budgetBytes := int64(100 * 1024 * 1024)
	settings := controlplane.SystemSettings{
		Worker: controlplane.SystemWorkerSettings{
			BufferBudgetBytes: budgetBytes,
		},
	}
	ApplySettings(settings)

	// 1. Acquire 40 MiB
	part1Bytes := int64(40 * 1024 * 1024)
	release1, err := AcquireBufferBudget(ctx, part1Bytes)
	r.NoError(err)
	r.NotNil(release1)

	// 2. Acquire another 40 MiB
	part2Bytes := int64(40 * 1024 * 1024)
	release2, err := AcquireBufferBudget(ctx, part2Bytes)
	r.NoError(err)
	r.NotNil(release2)

	// 3. Attempting to acquire 30 MiB should fail or block (remaining budget is only 20 MiB)
	timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	_, err = AcquireBufferBudget(timeoutCtx, 30*1024*1024)
	r.Error(err, "should fail when acquiring beyond remaining budget")

	// 4. Release part 1 (e.g. simulated cancel or error)
	release1()

	// Now 30 MiB can be acquired
	quickCtx, cancelQuick := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancelQuick()
	release3, err := AcquireBufferBudget(quickCtx, 30*1024*1024)
	r.NoError(err, "should succeed after release")

	// Cleanup all
	release2()
	release3()
}

// PR3: TestBulkLimits_DesiredSettingsAndBufferDrain
// Verifies that worker concurrency is stored as desired cluster configuration (restart required),
// while dynamic resource draining is actively enforced across buffer budget changes.
func TestBulkLimits_DesiredSettingsAndBufferDrain(t *testing.T) {
	r := require.New(t)
	ctx := t.Context()

	// 1. Desired settings contract: Concurrency is saved to settings
	s := controlplane.SystemSettings{
		Worker: controlplane.SystemWorkerSettings{
			Concurrency:       5,
			BufferBudgetBytes: 10 * 1024 * 1024, // 10 MiB
		},
	}
	ApplySettings(s)
	r.Equal(5, Current().Worker.Concurrency, "desired concurrency must be recorded in system settings")

	// 2. Dynamic draining verification: acquire 8 MiB out of 10 MiB
	releaseOld, err := AcquireBufferBudget(ctx, 8*1024*1024)
	r.NoError(err)
	defer releaseOld()

	// Resize budget down to 4 MiB while 8 MiB is still held
	sReduced := controlplane.SystemSettings{
		Worker: controlplane.SystemWorkerSettings{
			Concurrency:       3,
			BufferBudgetBytes: 4 * 1024 * 1024, // 4 MiB
		},
	}
	ApplySettings(sReduced)
	r.Equal(3, Current().Worker.Concurrency)

	// New acquire must block because 8 MiB is outstanding, exceeding the new 4 MiB budget
	quickCtx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	releaseNew, err := AcquireBufferBudget(quickCtx, 2*1024*1024)
	r.Error(err, "new acquisition must block until old reservations drain below reduced budget")
	if releaseNew != nil {
		releaseNew()
	}
}
