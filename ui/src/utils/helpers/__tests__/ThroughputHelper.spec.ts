import { describe, expect, it } from 'vitest';
import {
  bufferPercent,
  diagnoseThroughput,
  formatBytesPerSec,
  normalizeWorkerStats,
  type ThroughputTotals,
} from '../ThroughputHelper';

const base: ThroughputTotals = {
  inFlight: 5,
  concurrency: 10,
  objectsPerSec1m: 20,
  bytesPerSec1m: 0,
  objectsPerSec5m: 20,
  bytesPerSec5m: 0,
  bufferTotalBytes: 1000,
  bufferUsedBytes: 100,
  bufferWaiters: 0,
  copiedObjectsTotal: 0,
  copiedBytesTotal: 0,
  failedTotal: 0,
};
const w = [normalizeWorkerStats({ workers: [{ workerId: 'a' }] }).workers[0]!];
const diag = (over: Partial<ThroughputTotals>, prev?: number) =>
  diagnoseThroughput({ ...base, ...over }, w, prev);

describe('diagnoseThroughput', () => {
  it('is quiet when healthy or without workers', () => {
    expect(diag({})).toEqual([]);
    expect(diagnoseThroughput(base, [])).toEqual([]);
  });

  it('flags the buffer budget when waiters queue and the buffer is 90% full', () => {
    expect(diag({ bufferWaiters: 2, bufferUsedBytes: 900 })).toContain(
      'bufferBottleneck',
    );
    expect(diag({ bufferWaiters: 2, bufferUsedBytes: 899 })).not.toContain(
      'bufferBottleneck',
    );
    expect(diag({ bufferWaiters: 0, bufferUsedBytes: 1000 })).not.toContain(
      'bufferBottleneck',
    );
    expect(diag({ bufferWaiters: 2, bufferTotalBytes: 0 })).not.toContain(
      'bufferBottleneck',
    );
  });

  it('flags busy workers when in flight is within one of concurrency', () => {
    expect(diag({ inFlight: 10 })).toContain('workersBusy');
    expect(diag({ inFlight: 9 })).toContain('workersBusy');
    expect(diag({ inFlight: 8 })).not.toContain('workersBusy');
    expect(diag({ inFlight: 0, concurrency: 1 })).not.toContain('workersBusy');
  });

  it('flags idle workers only when in flight is low and rate is low', () => {
    expect(diag({ inFlight: 1, objectsPerSec1m: 0.2 })).toContain(
      'workersIdle',
    );
    expect(diag({ inFlight: 1, objectsPerSec1m: 30 })).not.toContain(
      'workersIdle',
    );
    expect(diag({ inFlight: 8, objectsPerSec1m: 0.2 })).not.toContain(
      'workersIdle',
    );
  });

  it('flags rising failures against the previous poll', () => {
    expect(diag({ failedTotal: 5 }, 3)).toContain('failuresRising');
    expect(diag({ failedTotal: 5 }, 5)).not.toContain('failuresRising');
    expect(diag({ failedTotal: 5 })).not.toContain('failuresRising');
  });
});

describe('formatting', () => {
  it('computes buffer percent', () => {
    expect(bufferPercent({ bufferTotalBytes: 200, bufferUsedBytes: 50 })).toBe(
      25,
    );
    expect(bufferPercent({ bufferTotalBytes: 0, bufferUsedBytes: 5 })).toBe(
      null,
    );
  });

  it('formats bytes per second', () => {
    expect(formatBytesPerSec(1572864)).toBe('1.5 MB/s');
  });

  it('normalizes int64 strings', () => {
    const r = normalizeWorkerStats({
      totals: { failedTotal: '12', bufferTotalBytes: '1024' },
    });

    expect(r.totals.failedTotal).toBe(12);
    expect(
      normalizeWorkerStats({
        workers: [{ workerId: 'a', inFlight: '3', bytesPerSec1m: '2048.5' }],
      }).workers[0],
    ).toMatchObject({ inFlight: 3, bytesPerSec1m: 2048.5 });
    expect(r.totals.bufferTotalBytes).toBe(1024);
  });
});
