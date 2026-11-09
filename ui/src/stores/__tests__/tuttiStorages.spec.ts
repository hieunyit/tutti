/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useTuttiStoragesStore } from '../tuttiStoragesStore';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';
import type { RuntimeStorageList } from '@/utils/types/runtimeStorage';

vi.mock('@/services/RuntimeStorageService', () => ({
  RuntimeStorageService: { list: vi.fn() },
}));
vi.mock('@/services/ChorusService', () => ({
  ChorusService: {
    getReplications: vi.fn().mockResolvedValue({ replications: [] }),
    getRoutingPolicies: vi
      .fn()
      .mockResolvedValue({ userRoutings: [], bucketRoutings: [] }),
  },
}));
vi.mock('@/services/TuttiService', () => ({
  TuttiService: {
    getUsers: vi.fn().mockResolvedValue({ users: [] }),
    getProxies: vi.fn().mockResolvedValue({ proxies: [] }),
    getFailoverPolicies: vi.fn().mockResolvedValue({ policies: [] }),
    getFailoverEvents: vi.fn().mockResolvedValue({ events: [] }),
  },
}));

beforeEach(() => {
  setActivePinia(createPinia());
  vi.mocked(RuntimeStorageService.list).mockReset();
});

describe('tuttiStoragesStore', () => {
  it('uses runtime metadata for management availability and the safe list projection', async () => {
    vi.mocked(RuntimeStorageService.list).mockResolvedValue({
      managedStoragesEnabled: true,
      mainStorage: 'main',
      storages: [
        {
          name: 'archive',
          address: 'https://s3.example.com',
          provider: 'Minio',
          defaultRegion: '',
          isSecure: true,
          httpTimeout: '60s',
          healthCheckInterval: '10s',
          source: 'managed',
          references: [],
        },
      ],
    });
    const store = useTuttiStoragesStore();

    await store.refresh();
    expect(store.managedStoragesEnabled).toBe(true);
    expect(store.runtimeStorages[0]?.source).toBe('managed');
    expect(store.storages[0]?.credentials).toEqual([]);
    expect(store.mainStorage).toBe('main');
  });

  it('ignores a pending list response after the store is reset', async () => {
    let resolve!: (value: RuntimeStorageList) => void;

    vi.mocked(RuntimeStorageService.list).mockReturnValue(
      new Promise((done) => {
        resolve = done;
      }),
    );
    const store = useTuttiStoragesStore();
    const pending = store.loadSlow();

    store.$reset();
    resolve({ managedStoragesEnabled: true });
    await pending;
    expect(store.managedStoragesEnabled).toBe(false);
    expect(store.runtimeStorages).toEqual([]);
  });
});
