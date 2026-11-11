import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useTuttiDiffReportDetailStore } from '../tuttiDiffReportDetailStore';
import { ChorusService } from '@/services/ChorusService';
import type { DiffReportDetailResponse } from '@/utils/types/chorus';

vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: {
      from: 's1:b1',
      to: 's2:b2',
    },
  }),
}));

vi.mock('@/services/ChorusService', () => ({
  ChorusService: {
    getDiffReport: vi.fn(),
    restartDiffReport: vi.fn(),
  },
}));

beforeEach(() => {
  setActivePinia(createPinia());
  vi.resetAllMocks();
  vi.useFakeTimers();
});

afterEach(() => {
  useTuttiDiffReportDetailStore().$reset();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe('tuttiDiffReportDetailStore', () => {
  it('does not revive polling after $reset during in-flight request', async () => {
    let resolveReq: (val: DiffReportDetailResponse) => void = () => {};
    const pendingPromise = new Promise<DiffReportDetailResponse>((resolve) => {
      resolveReq = resolve;
    });

    vi.mocked(ChorusService.getDiffReport).mockImplementation(() => pendingPromise);

    const store = useTuttiDiffReportDetailStore();

    expect(store.locations.length).toBe(2);

    const initPromise = store.initDiffReportDetailsPage();

    // Verify getDiffReport was called
    expect(ChorusService.getDiffReport).toHaveBeenCalledTimes(1);

    // Now unmount / reset the store while request is still pending
    store.$reset();

    // Now resolve the in-flight request
    resolveReq({
      check: {
        consistent: true,
        fixReady: false,
      },
    } as DiffReportDetailResponse);

    await initPromise;

    // After reset, report should remain null, not updated with the resolved response
    expect(store.report).toBeNull();

    // Fast-forward time past POLL_INTERVAL_MS (5000ms)
    await vi.advanceTimersByTimeAsync(10000);

    // getDiffReport should NOT have been called again (polling was stopped, not revived)
    expect(ChorusService.getDiffReport).toHaveBeenCalledTimes(1);
  });

  it('does not revive polling after $reset during in-flight restartDiffReport', async () => {
    let resolveRestart: (val: void) => void = () => {};
    const pendingRestart = new Promise<void>((resolve) => {
      resolveRestart = resolve;
    });

    vi.mocked(ChorusService.restartDiffReport).mockImplementation(() => pendingRestart);
    vi.mocked(ChorusService.getDiffReport).mockResolvedValue({
      check: { consistent: true, fixReady: false },
    } as DiffReportDetailResponse);

    const store = useTuttiDiffReportDetailStore();

    const restartPromise = store.restartDiffReport();

    expect(ChorusService.restartDiffReport).toHaveBeenCalledTimes(1);

    // Unmount / reset store while restartDiffReport is in-flight
    store.$reset();

    // Now resolve the restart call
    resolveRestart();
    await restartPromise;

    // Fast-forward time
    await vi.advanceTimersByTimeAsync(10000);

    // getDiffReport should not have been called to start new polling
    expect(ChorusService.getDiffReport).not.toHaveBeenCalled();
  });

  it('re-parses locations from route on initDiffReportDetailsPage after $reset', async () => {
    vi.mocked(ChorusService.getDiffReport).mockResolvedValue({
      check: { consistent: true, fixReady: false },
    } as DiffReportDetailResponse);

    const store = useTuttiDiffReportDetailStore();

    expect(store.locations.length).toBe(2);

    // Reset store on navigation unmount
    store.$reset();
    expect(store.locations.length).toBe(0);

    // Navigate back to the page in SPA (same singleton store)
    await store.initDiffReportDetailsPage();

    // Locations must be re-parsed and polling started
    expect(store.locations.length).toBe(2);
    expect(ChorusService.getDiffReport).toHaveBeenCalledTimes(1);
  });

  it('continues polling after transient 503 error without crashing timer loop', async () => {
    let callCount = 0;

    vi.mocked(ChorusService.getDiffReport).mockImplementation(async () => {
      callCount++;

      if (callCount === 1) {
        const err = new Error('Service Unavailable') as Error & { response?: { status: number } };

        err.response = { status: 503 };
        throw err;
      }

      return {
        check: { consistent: true, fixReady: false },
      } as DiffReportDetailResponse;
    });

    const store = useTuttiDiffReportDetailStore();

    await store.initDiffReportDetailsPage();
    expect(callCount).toBe(1);
    expect(store.hasError).toBe(true);

    // Advance timer to next poll cycle: must retry and succeed
    await vi.advanceTimersByTimeAsync(5000);
    expect(callCount).toBe(2);
    expect(store.hasError).toBe(false);
    expect(store.report?.consistent).toBe(true);
  });
});
