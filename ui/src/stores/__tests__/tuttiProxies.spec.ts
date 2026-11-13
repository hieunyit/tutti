/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useTuttiProxiesStore } from '../tuttiProxiesStore';
import { useTuttiProxyFleetConfigStore } from '../tuttiProxyFleetConfigStore';
import { TuttiService } from '@/services/TuttiService';

vi.mock('@/services/TuttiService', () => ({
  TuttiService: {
    getProxies: vi.fn(),
    getProxyFleetConfig: vi.fn(),
    setProxyFleetConfig: vi.fn(),
  },
}));

beforeEach(() => {
  setActivePinia(createPinia());
  vi.useFakeTimers();
  vi.mocked(TuttiService.getProxies).mockReset();
  vi.mocked(TuttiService.getProxyFleetConfig).mockReset();
  vi.mocked(TuttiService.setProxyFleetConfig).mockReset();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('tuttiProxiesStore', () => {
  it('polls getProxies on the interval and stops after stopPolling', async () => {
    vi.mocked(TuttiService.getProxies).mockResolvedValue({
      proxies: [{ appId: 'p1' }],
      configVersion: 3,
    });
    const store = useTuttiProxiesStore();

    store.startPolling(15000);
    await vi.advanceTimersByTimeAsync(0);
    expect(TuttiService.getProxies).toHaveBeenCalledTimes(1);
    expect(store.proxies).toHaveLength(1);
    expect(store.configVersion).toBe(3);

    await vi.advanceTimersByTimeAsync(15000);
    expect(TuttiService.getProxies).toHaveBeenCalledTimes(2);

    store.stopPolling();
    await vi.advanceTimersByTimeAsync(60000);
    expect(TuttiService.getProxies).toHaveBeenCalledTimes(2);
  });

  it('flags an error and keeps the last data when a refresh fails', async () => {
    vi.mocked(TuttiService.getProxies)
      .mockResolvedValueOnce({ proxies: [{ appId: 'p1' }] })
      .mockRejectedValueOnce(new Error('down'));
    const store = useTuttiProxiesStore();

    await store.load();
    expect(store.error).toBe(false);
    await store.load();
    expect(store.error).toBe(true);
    expect(store.proxies).toHaveLength(1);
  });
});

describe('tuttiProxyFleetConfigStore', () => {
  const loaded = {
    version: 4,
    defaults: { writerLease: '30s' },
    groups: { edge: { health: { failureThreshold: 3 } } },
  };

  it('becomes dirty after editing the draft and clean after reset', async () => {
    vi.mocked(TuttiService.getProxyFleetConfig).mockResolvedValue(loaded);
    const store = useTuttiFleetStore();

    await store.load();
    expect(store.dirty).toBe(false);

    store.draft.defaults.writerLease = '60s';
    expect(store.dirty).toBe(true);
    expect(store.changes).toEqual([
      {
        scope: '__defaults__',
        path: 'writerLease',
        before: '30s',
        after: '60s',
      },
    ]);

    store.reset();
    expect(store.dirty).toBe(false);
  });

  it('setField sets and prunes nested values', async () => {
    vi.mocked(TuttiService.getProxyFleetConfig).mockResolvedValue(loaded);
    const store = useTuttiFleetStore();

    await store.load();
    store.setField('edge', ['health', 'failureThreshold'], undefined);
    expect(store.draft.groups.edge).toEqual({});
    store.setField('__defaults__', ['replication', 'sync', 'enabled'], true);
    expect(store.draft.defaults.replication?.sync?.enabled).toBe(true);
  });

  it('save calls setProxyFleetConfig once with the draft and the loaded version', async () => {
    vi.mocked(TuttiService.getProxyFleetConfig).mockResolvedValue(loaded);
    vi.mocked(TuttiService.setProxyFleetConfig).mockResolvedValue({
      ...loaded,
      version: 5,
      defaults: { writerLease: '60s' },
    });
    const store = useTuttiFleetStore();

    await store.load();
    store.draft.defaults.writerLease = '60s';
    await store.save();

    expect(TuttiService.setProxyFleetConfig).toHaveBeenCalledTimes(1);
    expect(TuttiService.setProxyFleetConfig).toHaveBeenCalledWith({
      defaults: { writerLease: '60s' },
      groups: loaded.groups,
      expectedVersion: 4,
    });
    expect(store.dirty).toBe(false);
    expect(store.version).toBe(5);
  });

  it('reports a version conflict instead of a generic error', async () => {
    vi.mocked(TuttiService.getProxyFleetConfig).mockResolvedValue(loaded);
    vi.mocked(TuttiService.setProxyFleetConfig).mockRejectedValue({
      response: { data: { code: 9 } },
    });
    const store = useTuttiFleetStore();

    await store.load();
    store.draft.defaults.writerLease = '60s';
    await store.save();
    expect(store.conflict).toBe(true);
    expect(store.saveError).toBe(false);
    expect(store.dirty).toBe(true);
  });
});

function useTuttiFleetStore() {
  return useTuttiProxyFleetConfigStore();
}
