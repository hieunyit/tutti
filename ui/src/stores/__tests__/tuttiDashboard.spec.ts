import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  DASHBOARD_REFRESH_INTERVAL_MS,
  useTuttiDashboardStore,
} from '../tuttiDashboardStore';
import { ChorusService } from '@/services/ChorusService';
import { TuttiService } from '@/services/TuttiService';

vi.mock('@/services/ChorusService', () => ({
  ChorusService: { getStorages: vi.fn(), getReplications: vi.fn() },
}));
vi.mock('@/services/TuttiService', () => ({
  TuttiService: { getProxies: vi.fn(), getFailoverEvents: vi.fn() },
}));

beforeEach(() => {
  setActivePinia(createPinia());
  vi.resetAllMocks();
  vi.useFakeTimers();
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false);
  vi.mocked(ChorusService.getStorages).mockResolvedValue({ storages: [] });
  vi.mocked(ChorusService.getReplications).mockResolvedValue({
    replications: [],
  });
  vi.mocked(TuttiService.getProxies).mockResolvedValue({ proxies: [] });
  vi.mocked(TuttiService.getFailoverEvents).mockResolvedValue({ events: [] });
});

afterEach(() => {
  useTuttiDashboardStore().stopPolling();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe('tuttiDashboardStore', () => {
  it('polls every 30 seconds only while visible and catches up when visible', async () => {
    const store = useTuttiDashboardStore();

    store.startPolling();
    await vi.advanceTimersByTimeAsync(0);
    expect(ChorusService.getStorages).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(DASHBOARD_REFRESH_INTERVAL_MS);
    expect(ChorusService.getStorages).toHaveBeenCalledTimes(2);
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true);
    await vi.advanceTimersByTimeAsync(DASHBOARD_REFRESH_INTERVAL_MS * 2);
    expect(ChorusService.getStorages).toHaveBeenCalledTimes(2);
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false);
    document.dispatchEvent(new Event('visibilitychange'));
    await vi.advanceTimersByTimeAsync(0);
    expect(ChorusService.getStorages).toHaveBeenCalledTimes(3);
  });

  it('does not start requests when mounted in a hidden tab', async () => {
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true);
    useTuttiDashboardStore().startPolling();
    await vi.advanceTimersByTimeAsync(DASHBOARD_REFRESH_INTERVAL_MS);
    expect(ChorusService.getStorages).not.toHaveBeenCalled();
  });

  it('ignores stale completion after reset and permits a fresh request', async () => {
    let resolveOld!: (
      value: Awaited<ReturnType<typeof ChorusService.getStorages>>,
    ) => void;

    vi.mocked(ChorusService.getStorages).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveOld = resolve;
        }),
    );
    const store = useTuttiDashboardStore();
    const old = store.refresh();

    await store.refresh();
    expect(ChorusService.getStorages).toHaveBeenCalledTimes(1);
    store.$reset();
    await store.refresh();
    expect(store.storages).toEqual([]);
    resolveOld({ storages: [{ name: 'old' }] as never });
    await old;
    expect(store.storages).toEqual([]);
    expect(store.isRefreshing).toBe(false);
  });

  it('does not let optional event success hide total core-source failure', async () => {
    vi.mocked(ChorusService.getStorages).mockRejectedValue(
      new Error('offline'),
    );
    vi.mocked(ChorusService.getReplications).mockRejectedValue(
      new Error('offline'),
    );
    vi.mocked(TuttiService.getProxies).mockRejectedValue(new Error('offline'));
    const store = useTuttiDashboardStore();

    await store.refresh();
    expect(store.hasFatalError).toBe(true);
    expect(store.isLoading).toBe(false);
  });

  it('retains successful sources and suppresses an optional events 403', async () => {
    vi.mocked(TuttiService.getFailoverEvents).mockRejectedValue({
      isAxiosError: true,
      status: 403,
    });
    const store = useTuttiDashboardStore();

    await store.refresh();
    expect(store.hasFatalError).toBe(false);
    expect(store.failed.events).toBe(false);
    expect(store.events).toBeNull();
    vi.mocked(ChorusService.getStorages).mockRejectedValue(
      new Error('offline'),
    );
    await store.refresh();
    expect(store.storages).toEqual([]);
    expect(store.failed.storages).toBe(true);
  });
});
