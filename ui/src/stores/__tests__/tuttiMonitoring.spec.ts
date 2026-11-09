import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { TuttiService } from '@/services/TuttiService';
import { ChorusService } from '@/services/ChorusService';
import {
  MONITORING_PROXY_INTERVAL_MS,
  useTuttiMonitoringStore,
} from '@/stores/tuttiMonitoringStore';
import type { TuttiProxyStatusListResponse } from '@/utils/types/tutti';
import type { ChorusReplicationListResponse } from '@/utils/types/chorus';

vi.mock('@/services/TuttiService', () => ({
  TuttiService: { getProxies: vi.fn() },
}));
vi.mock('@/services/ChorusService', () => ({
  ChorusService: { getReplications: vi.fn() },
}));

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((done, fail) => {
    resolve = done;
    reject = fail;
  });

  return { promise, resolve, reject };
}

beforeEach(() => {
  setActivePinia(createPinia());
  vi.resetAllMocks();
  vi.mocked(ChorusService.getReplications).mockResolvedValue({
    replications: [],
  });
  vi.useFakeTimers();
});

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
});

it('skips overlapping polls while a proxy response is pending', async () => {
  const pending = deferred<TuttiProxyStatusListResponse>();
  const getProxies = vi.mocked(TuttiService.getProxies);

  getProxies
    .mockReturnValueOnce(pending.promise)
    .mockResolvedValue({ proxies: [] });
  const store = useTuttiMonitoringStore();

  store.startPolling();
  await vi.advanceTimersByTimeAsync(MONITORING_PROXY_INTERVAL_MS * 2);
  expect(getProxies).toHaveBeenCalledTimes(1);
  pending.resolve({ proxies: [{ appId: 'current-proxy' }] });
  await vi.advanceTimersByTimeAsync(0);
  expect(store.proxies.map((proxy) => proxy.appId)).toEqual(['current-proxy']);
  await vi.advanceTimersByTimeAsync(MONITORING_PROXY_INTERVAL_MS);
  expect(getProxies).toHaveBeenCalledTimes(2);
  store.stopPolling();
});

it('aborts pending requests and ignores late responses after reset', async () => {
  const oldRequest = deferred<TuttiProxyStatusListResponse>();

  vi.mocked(TuttiService.getProxies).mockReturnValueOnce(oldRequest.promise);
  const store = useTuttiMonitoringStore();
  const pending = store.loadProxies();
  const signal = vi.mocked(TuttiService.getProxies).mock.calls[0]?.[0];

  store.$reset();
  expect(signal?.aborted).toBe(true);
  oldRequest.resolve({ proxies: [{ appId: 'obsolete-proxy' }] });
  await pending;
  expect(store.proxies).toEqual([]);
  expect(store.lastUpdated).toBeNull();
  expect(store.isLoading).toBe(true);
});

it('does not let a previous polling session overwrite a restarted session', async () => {
  const oldRequest = deferred<TuttiProxyStatusListResponse>();

  vi.mocked(TuttiService.getProxies)
    .mockReturnValueOnce(oldRequest.promise)
    .mockResolvedValueOnce({ proxies: [{ appId: 'new-current-proxy' }] });
  const store = useTuttiMonitoringStore();

  store.startPolling();
  store.pause();
  store.resume();
  await vi.advanceTimersByTimeAsync(0);
  oldRequest.resolve({ proxies: [{ appId: 'old-removed-proxy' }] });
  await vi.advanceTimersByTimeAsync(0);
  expect(store.proxies.map((proxy) => proxy.appId)).toEqual([
    'new-current-proxy',
  ]);
  expect(store.hasError).toBe(false);
  store.stopPolling();
});

it('ignores canceled request failures after reset', async () => {
  const oldRequest = deferred<TuttiProxyStatusListResponse>();

  vi.mocked(TuttiService.getProxies).mockReturnValueOnce(oldRequest.promise);
  const store = useTuttiMonitoringStore();
  const pending = store.loadProxies();

  store.$reset();
  oldRequest.reject(new Error('canceled'));
  await pending;
  expect(store.hasError).toBe(false);
  expect(store.isLoading).toBe(true);
});

it('guards and invalidates replication requests as well as proxy requests', async () => {
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
