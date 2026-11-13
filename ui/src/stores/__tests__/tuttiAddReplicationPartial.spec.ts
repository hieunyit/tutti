/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

import { AxiosError } from 'axios';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useTuttiAddReplicationStore } from '../tuttiAddReplicationStore';
import { ChorusService } from '@/services/ChorusService';

const push = vi.hoisted(() => vi.fn());
const notify = vi.hoisted(() => vi.fn());

vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }));
vi.mock('@/i18n/useStoreI18n', () => ({
  useStoreI18n: () => ({ t: (key: string) => key }),
}));
vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
  useNotification: () => ({ createNotification: notify }),
}));
vi.mock('@/services/ChorusService', () => ({
  ChorusService: {
    getStorages: vi.fn(),
    getBucketsForReplication: vi.fn(),
    addReplication: vi.fn(),
  },
}));
vi.mock('@/services/TuttiService', () => ({
  TuttiService: { getUsers: vi.fn() },
}));

const add = vi.mocked(ChorusService.addReplication);

function rejection(reason: string) {
  return new AxiosError('conflict', 'ERR_BAD_REQUEST', undefined, undefined, {
    status: 409,
    data: {
      code: 6,
      details: [
        {
          '@type': 'type.googleapis.com/google.rpc.ErrorInfo',
          reason,
        },
      ],
    },
  } as never);
}

async function readyStore() {
  const store = useTuttiAddReplicationStore();

  store.selectedFromStorage = { name: 'main' } as never;
  store.selectedToStorage = { name: 'backup' } as never;

  return store;
}

beforeEach(() => {
  setActivePinia(createPinia());
  push.mockReset();
  notify.mockReset();
  add.mockReset();
});

describe('createReplication partial failure', () => {
  it('keeps failed buckets selected, shows their server reasons and does not navigate', async () => {
    add.mockImplementation(async ({ id }) => {
      if (id.fromBucket === 'b2')
        throw rejection('bucket "b2" is already covered by group "g1"');

      if (id.fromBucket === 'b3') throw rejection('b3 is locked');
    });
    const store = await readyStore();

    store.selectedBuckets = ['b1', 'b2', 'b3'];
    const created = await store.createReplication();

    expect(created).toBe(false);
    expect(add).toHaveBeenCalledTimes(3);
    expect(push).not.toHaveBeenCalled();
    expect(store.selectedBuckets).toEqual(['b2', 'b3']);
    expect(store.createError).toContain(
      'b2: bucket "b2" is already covered by group "g1"',
    );
    expect(store.createError).toContain('b3: b3 is locked');
    expect(store.createError).not.toContain('b1');
    expect(store.isSubmitting).toBe(false);
  });

  it('reports the created ones as a success notification and the failure separately', async () => {
    add.mockImplementation(async ({ id }) => {
      if (id.fromBucket === 'b2') throw rejection('nope');
    });
    const store = await readyStore();

    store.selectedBuckets = ['b1', 'b2'];
    await store.createReplication();

    const types = notify.mock.calls.map(([arg]) => arg.type);

    expect(types).toContain('success');
    expect(types).toHaveLength(2);
  });

  it('a retry of the remaining buckets then succeeds and navigates once', async () => {
    add.mockImplementation(async ({ id }) => {
      if (id.fromBucket === 'b2' && add.mock.calls.length <= 2)
        throw rejection('busy');
    });
    const store = await readyStore();

    store.selectedBuckets = ['b1', 'b2'];
    expect(await store.createReplication()).toBe(false);
    expect(store.selectedBuckets).toEqual(['b2']);

    expect(await store.createReplication()).toBe(true);
    expect(store.createError).toBe('');
    expect(push).toHaveBeenCalledTimes(1);
  });
});
