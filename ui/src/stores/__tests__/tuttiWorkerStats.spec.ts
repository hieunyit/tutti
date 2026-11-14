import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useTuttiWorkerStatsStore } from '../tuttiWorkerStatsStore';
import { TuttiService } from '@/services/TuttiService';

vi.mock('@/services/TuttiService', () => ({
  TuttiService: { getWorkerStats: vi.fn() },
}));

const response = (failed: number) => ({
  workers: [{ workerId: 'w1', concurrency: 10, inFlight: 5 }],
  totals: { concurrency: 10, inFlight: 5, failedTotal: failed },
});

function httpError(status: number) {
  return Object.assign(new Error(`http ${status}`), {
    isAxiosError: true,
    response: { status },
  });
}

describe('useTuttiWorkerStatsStore', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    setActivePinia(createPinia());
    vi.mocked(TuttiService.getWorkerStats).mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('loads once, then polls every 5 s and tracks rising failures', async () => {
    vi.mocked(TuttiService.getWorkerStats)
      .mockResolvedValueOnce(response(1))
      .mockResolvedValue(response(4));

    const store = useTuttiWorkerStatsStore();

    await store.start();

    expect(store.totals?.failedTotal).toBe(1);
    expect(store.isLoading).toBe(false);
    expect(store.hints).not.toContain('failuresRising');

    await vi.advanceTimersByTimeAsync(5_000);

    expect(TuttiService.getWorkerStats).toHaveBeenCalledTimes(2);
    expect(store.hints).toContain('failuresRising');

    await store.stop();
    await vi.advanceTimersByTimeAsync(20_000);

    expect(TuttiService.getWorkerStats).toHaveBeenCalledTimes(2);
  });

  it.each([404, 501])('hides the panel and stops polling on %s', async (s) => {
    vi.mocked(TuttiService.getWorkerStats).mockRejectedValue(httpError(s));

    const store = useTuttiWorkerStatsStore();

    await store.start();
    await vi.advanceTimersByTimeAsync(30_000);

    expect(store.unavailable).toBe(true);
    expect(store.hasError).toBe(false);
    expect(TuttiService.getWorkerStats).toHaveBeenCalledTimes(1);
  });

  it('reports other errors and recovers on retry', async () => {
    vi.mocked(TuttiService.getWorkerStats).mockRejectedValueOnce(
      httpError(500),
    );

    const store = useTuttiWorkerStatsStore();

    await store.start();

    expect(store.hasError).toBe(true);
    expect(store.unavailable).toBe(false);

    vi.mocked(TuttiService.getWorkerStats).mockResolvedValue(response(0));
    await store.retry();

    expect(store.hasError).toBe(false);
    expect(store.workers).toHaveLength(1);
    await store.stop();
  });
});
