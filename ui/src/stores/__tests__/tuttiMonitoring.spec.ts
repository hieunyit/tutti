import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TuttiService } from '@/services/TuttiService';
import { ChorusService } from '@/services/ChorusService';
import {
  MONITORING_HISTORY_REFRESH_MS,
  MONITORING_POLL_INTERVAL_MS,
  useTuttiMonitoringStore,
} from '@/stores/tuttiMonitoringStore';
import type {
  TuttiProxyStatusListResponse,
  TuttiStatsHistory,
} from '@/utils/types/tutti';
import type { ChorusReplicationListResponse } from '@/utils/types/chorus';

vi.mock('@/services/TuttiService', () => ({
  TuttiService: { getProxies: vi.fn(), getStatsHistory: vi.fn() },
}));
vi.mock('@/services/ChorusService', () => ({
  ChorusService: { getReplications: vi.fn() },
}));

const NOW = Date.parse('2026-01-01T12:00:00Z');

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((done, fail) => {
    resolve = done;
    reject = fail;
  });

  return { promise, resolve, reject };
}

const iso = (offsetMs: number) => new Date(NOW + offsetMs).toISOString();

function history(...offsets: number[]): TuttiStatsHistory {
  return {
    points: offsets.map((o) => ({
      time: iso(o),
      rates: { requestsPerSec: 5, latencyAvgMs: 3 },
    })),
  };
}

/** a proxies response whose proxy published at NOW + offset */
function live(offsetMs: number, rps = 7): TuttiProxyStatusListResponse {
  return {
    proxies: [
      {
        appId: 'p1',
        updatedAt: iso(offsetMs),
        rates: { requestsPerSec: rps },
        stats: { requestsByStorage: { main: '12' }, syncBelowQuorum: '2' },
      },
    ],
    totalRates: { requestsPerSec: rps, requestsByStorage: { main: rps } },
  };
}

beforeEach(() => {
  setActivePinia(createPinia());
  vi.resetAllMocks();
  window.localStorage.clear();
  vi.mocked(ChorusService.getReplications).mockResolvedValue({
    replications: [],
  });
  vi.mocked(TuttiService.getStatsHistory).mockResolvedValue({ points: [] });
  vi.mocked(TuttiService.getProxies).mockResolvedValue({ proxies: [] });
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
});

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
});

describe('monitoring store', () => {
  it('loads history for the stored range and shows it at once', async () => {
    window.localStorage.setItem('tutti-monitoring-range', '1h');
    vi.mocked(TuttiService.getStatsHistory).mockResolvedValue(
      history(-60_000, -30_000),
    );
    const store = useTuttiMonitoringStore();

    store.start();
    await vi.advanceTimersByTimeAsync(0);
    expect(TuttiService.getStatsHistory).toHaveBeenCalledWith(
      '1h',
      expect.any(AbortSignal),
    );
    expect(store.points).toHaveLength(2);
    expect(store.isLoading).toBe(false);
    store.stop();
  });

  it('ignores a garbage stored range', () => {
    window.localStorage.setItem('tutti-monitoring-range', 'forever');
    expect(useTuttiMonitoringStore().range).toBe('15m');
  });

  it('polls every 2 s and appends the aggregated point', async () => {
    vi.mocked(TuttiService.getStatsHistory).mockResolvedValue(history(-10_000));
    vi.mocked(TuttiService.getProxies)
      .mockResolvedValueOnce(live(-2_000, 7))
      .mockResolvedValueOnce(live(0, 9));
    const store = useTuttiMonitoringStore();

    store.start();
    await vi.advanceTimersByTimeAsync(0);
    expect(store.points).toHaveLength(2);
    await vi.advanceTimersByTimeAsync(MONITORING_POLL_INTERVAL_MS);
    expect(TuttiService.getProxies).toHaveBeenCalledTimes(2);
    expect(store.points).toHaveLength(3);
    expect(store.points[2]?.rates.requests).toBe(9);
    expect(store.totalRates?.requests).toBe(9);
    expect(store.proxies[0]?.requestsByStorage).toEqual({ main: 12 });
    expect(store.syncTotals().below).toBe(2);
    store.stop();
  });

  it('does not append the same document twice', async () => {
    vi.mocked(TuttiService.getProxies).mockResolvedValue(live(-1_000));
    const store = useTuttiMonitoringStore();

    store.start();
    await vi.advanceTimersByTimeAsync(MONITORING_POLL_INTERVAL_MS * 3);
    expect(store.points).toHaveLength(1);
    store.stop();
  });

  it('keeps points within the range while polling for a long time', async () => {
    let i = 0;

    vi.mocked(TuttiService.getProxies).mockImplementation(async () =>
      live(Date.now() - NOW, 1 + (i += 1)),
    );
    // history refresh returns nothing: only live points remain
    const store = useTuttiMonitoringStore();

    store.start();
    store.setRange('5m');
    // 20 minutes of 2 s polls and history refreshes
    await vi.advanceTimersByTimeAsync(20 * 60_000);
    const span =
      (store.points[store.points.length - 1]?.t ?? 0) -
      (store.points[0]?.t ?? 0);

    expect(span).toBeLessThanOrEqual(5 * 60_000);
    // 5 min / 2 s = 150 points, never unbounded
    expect(store.points.length).toBeLessThanOrEqual(151);
    store.stop();
  });

  it('refetches history every 30 s', async () => {
    const store = useTuttiMonitoringStore();

    store.start();
    await vi.advanceTimersByTimeAsync(MONITORING_HISTORY_REFRESH_MS * 2);
    expect(TuttiService.getStatsHistory).toHaveBeenCalledTimes(3);
    store.stop();
  });

  it('skips overlapping polls while a proxy response is pending', async () => {
    const pending = deferred<TuttiProxyStatusListResponse>();

    vi.mocked(TuttiService.getProxies)
      .mockReturnValueOnce(pending.promise)
      .mockResolvedValue({ proxies: [] });
    const store = useTuttiMonitoringStore();

    store.start();
    await vi.advanceTimersByTimeAsync(MONITORING_POLL_INTERVAL_MS * 2);
    expect(TuttiService.getProxies).toHaveBeenCalledTimes(1);
    pending.resolve(live(0));
    await vi.advanceTimersByTimeAsync(0);
    expect(store.proxies.map((p) => p.appId)).toEqual(['p1']);
    await vi.advanceTimersByTimeAsync(MONITORING_POLL_INTERVAL_MS);
    expect(TuttiService.getProxies).toHaveBeenCalledTimes(2);
    store.stop();
  });

  it('flags failing updates and recovers', async () => {
    vi.mocked(TuttiService.getProxies)
      .mockRejectedValueOnce(new Error('down'))
      .mockResolvedValue(live(0));
    const store = useTuttiMonitoringStore();

    store.start();
    await vi.advanceTimersByTimeAsync(0);
    expect(store.hasError).toBe(true);
    await vi.advanceTimersByTimeAsync(MONITORING_POLL_INTERVAL_MS);
    expect(store.hasError).toBe(false);
    expect(store.lastUpdated).not.toBeNull();
    store.stop();
  });

  it('ignores a stale history response after a range change', async () => {
    const slow = deferred<TuttiStatsHistory>();

    vi.mocked(TuttiService.getStatsHistory)
      .mockReturnValueOnce(slow.promise)
      .mockResolvedValueOnce(history(-5_000));
    const store = useTuttiMonitoringStore();

    store.start();
    store.setRange('5m');
    await vi.advanceTimersByTimeAsync(0);
    slow.resolve(history(-1_000, -2_000, -3_000));
    await vi.advanceTimersByTimeAsync(0);
    expect(store.range).toBe('5m');
    expect(store.points).toHaveLength(1);
    expect(window.localStorage.getItem('tutti-monitoring-range')).toBe('5m');
    store.stop();
  });

  it('stops requests while paused and catches up on resume', async () => {
    const store = useTuttiMonitoringStore();

    store.start();
    await vi.advanceTimersByTimeAsync(0);
    store.pause();
    vi.mocked(TuttiService.getProxies).mockClear();
    vi.mocked(TuttiService.getStatsHistory).mockClear();
    await vi.advanceTimersByTimeAsync(MONITORING_HISTORY_REFRESH_MS * 2);
    expect(TuttiService.getProxies).not.toHaveBeenCalled();
    expect(TuttiService.getStatsHistory).not.toHaveBeenCalled();
    store.resume();
    await vi.advanceTimersByTimeAsync(0);
    expect(TuttiService.getProxies).toHaveBeenCalledTimes(1);
    expect(TuttiService.getStatsHistory).toHaveBeenCalledTimes(1);
    expect(store.isPaused).toBe(false);
    store.stop();
  });

  it('does not poll while the tab is hidden', async () => {
    const store = useTuttiMonitoringStore();

    store.start();
    await vi.advanceTimersByTimeAsync(0);
    vi.mocked(TuttiService.getProxies).mockClear();
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true);
    await vi.advanceTimersByTimeAsync(MONITORING_POLL_INTERVAL_MS * 5);
    expect(TuttiService.getProxies).not.toHaveBeenCalled();
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false);
    document.dispatchEvent(new Event('visibilitychange'));
    await vi.advanceTimersByTimeAsync(0);
    expect(TuttiService.getProxies).toHaveBeenCalledTimes(1);
    store.stop();
  });

  it('aborts pending requests and ignores late responses after reset', async () => {
    const oldRequest = deferred<TuttiProxyStatusListResponse>();

    vi.mocked(TuttiService.getProxies).mockReturnValueOnce(oldRequest.promise);
    const store = useTuttiMonitoringStore();

    store.start();
    const signal = vi.mocked(TuttiService.getProxies).mock.calls[0]?.[0];

    store.$reset();
    expect(signal?.aborted).toBe(true);
    oldRequest.resolve(live(0));
    await vi.advanceTimersByTimeAsync(0);
    expect(store.proxies).toEqual([]);
    expect(store.lastUpdated).toBeNull();
    expect(store.isLoading).toBe(true);
  });

  it('ignores canceled request failures after reset', async () => {
    const oldRequest = deferred<TuttiProxyStatusListResponse>();

    vi.mocked(TuttiService.getProxies).mockReturnValueOnce(oldRequest.promise);
    const store = useTuttiMonitoringStore();

    store.start();
    store.$reset();
    oldRequest.reject(new Error('canceled'));
    await vi.advanceTimersByTimeAsync(0);
    expect(store.hasError).toBe(false);
  });

  it('guards and invalidates replication requests', async () => {
    const oldRequest = deferred<ChorusReplicationListResponse>();
    const getReplications = vi.mocked(ChorusService.getReplications);

    getReplications.mockReturnValueOnce(oldRequest.promise);
    const store = useTuttiMonitoringStore();
    const pending = store.loadReplications();

    await store.loadReplications();
    expect(getReplications).toHaveBeenCalledTimes(1);
    const signal = getReplications.mock.calls[0]?.[0];

    store.$reset();
    expect(signal?.aborted).toBe(true);
    oldRequest.resolve({ replications: [] });
    await pending;
    expect(store.replicationsLoaded).toBe(false);
    expect(store.replicationsError).toBe(false);
  });
});
