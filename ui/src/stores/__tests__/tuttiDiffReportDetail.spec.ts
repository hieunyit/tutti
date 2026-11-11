import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useTuttiDiffReportDetailStore } from '../tuttiDiffReportDetailStore';
import { ChorusService } from '@/services/ChorusService';

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
    let resolveReq: (val: any) => void = () => {};
    const pendingPromise = new Promise((resolve) => {
      resolveReq = resolve;
    });

    vi.mocked(ChorusService.getDiffReport).mockImplementation(() => pendingPromise as any);

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
    });

    await initPromise;

    // After reset, report should remain null, not updated with the resolved response
    expect(store.report).toBeNull();

    // Fast-forward time past POLL_INTERVAL_MS (5000ms)
    await vi.advanceTimersByTimeAsync(10000);

    // getDiffReport should NOT have been called again (polling was stopped, not revived)
    expect(ChorusService.getDiffReport).toHaveBeenCalledTimes(1);
  });
});
