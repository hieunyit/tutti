/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useTuttiStoragesStore } from '../tuttiStoragesStore';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';
import type { RuntimeStorageList } from '@/utils/types/runtimeStorage';

vi.mock('@/services/RuntimeStorageService', () => ({
  RuntimeStorageService: { list: vi.fn(), listSummary: vi.fn() },
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
  vi.mocked(RuntimeStorageService.listSummary).mockReset();
  vi.mocked(RuntimeStorageService.list).mockReset();
});

describe('tuttiStoragesStore', () => {
  it('uses runtime metadata for management availability and the safe list projection', async () => {
    vi.mocked(RuntimeStorageService.listSummary).mockResolvedValue({
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

    vi.mocked(RuntimeStorageService.listSummary).mockReturnValue(
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

  it('requests the summary list and shows userCount, falling back to users.length', async () => {
    const base = {
      address: 'https://s3.example.com',
      provider: 'Minio',
      defaultRegion: '',
      isSecure: true,
      httpTimeout: '60s',
      healthCheckInterval: '10s',
    };

    vi.mocked(RuntimeStorageService.listSummary).mockResolvedValue({
      storages: [
        { ...base, name: 'new', userCount: 7 },
        {
          ...base,
          name: 'old',
          users: [
            { user: 'a', accessKeyId: 'k', source: 'managed' },
            { user: 'b', accessKeyId: 'k', source: 'managed' },
          ],
        },
      ],
    });
    const store = useTuttiStoragesStore();

    await store.refresh();
    expect(RuntimeStorageService.listSummary).toHaveBeenCalled();
    expect(RuntimeStorageService.list).not.toHaveBeenCalled();
    expect(store.storages.map((s) => s.userCount)).toEqual([7, 2]);
    expect(store.rows.map((r) => r.users.count)).toEqual([7, 2]);
  });

  it('loads the users of one storage only on demand', async () => {
    vi.mocked(RuntimeStorageService.list).mockResolvedValue({
      storages: [
        {
          name: 's1',
          address: 'x',
          provider: 'Minio',
          defaultRegion: '',
          isSecure: true,
          httpTimeout: '60s',
          healthCheckInterval: '10s',
          users: [{ user: 'a', accessKeyId: 'k', source: 'managed' }],
        },
      ],
    });
    const store = useTuttiStoragesStore();

    await store.loadStorageUsers('s1');
    expect(store.storageUsers['s1']).toHaveLength(1);
  });
});
