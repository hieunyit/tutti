/*
 * Copyright © 2026 hieunyit
 *
 *  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  https://www.gnu.org/licenses/agpl-3.0.html
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

import { AxiosError } from 'axios';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useTuttiAddReplicationStore } from '../tuttiAddReplicationStore';
import { TuttiService } from '@/services/TuttiService';
import { TuttiCoreService } from '@/services/TuttiCoreService';

const push = vi.hoisted(() => vi.fn());

vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }));
vi.mock('@/i18n/useStoreI18n', () => ({
  useStoreI18n: () => ({
    t: (key: string, params?: Record<string, string>) =>
      params ? `${key} ${Object.values(params).join(' ')}` : key,
  }),
}));
vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
  useNotification: () => ({ createNotification: vi.fn() }),
}));
vi.mock('@/services/TuttiCoreService', () => ({
  TuttiCoreService: {
    getStorages: vi.fn(),
    getBucketsForReplication: vi.fn(),
    addReplication: vi.fn(),
  },
}));
vi.mock('@/services/TuttiService', () => ({
  TuttiService: { getUsers: vi.fn() },
}));

type Mocks = Record<string, ReturnType<typeof vi.fn>>;

const tuttiCore = TuttiCoreService as unknown as Required<Mocks>;
const tutti = TuttiService as unknown as Required<Mocks>;

function conflictError(reason: string) {
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

beforeEach(() => {
  setActivePinia(createPinia());
  vi.clearAllMocks();
  tuttiCore.getStorages!.mockResolvedValue({
    storages: [
      { name: 'main', isMain: true, credentials: [{ alias: 'default' }] },
      { name: 'backup', isMain: false, credentials: [] },
    ],
  });
  tuttiCore.getBucketsForReplication!.mockResolvedValue({
    buckets: ['b1'],
    replicatedBuckets: [],
    coveredBuckets: [],
  });
  tuttiCore.addReplication!.mockResolvedValue({});
});

describe('tuttiAddReplicationStore', () => {
  it('neither asks for nor discovers a user', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();

    expect('selectedUser' in store).toBe(false);
    expect('users' in store).toBe(false);
    expect('eligibleUsers' in store).toBe(false);
    expect(tutti.getUsers).not.toHaveBeenCalled();
  });

  describe('defaults', () => {
    it('picks the main storage as source and the only other storage as destination', async () => {
      const store = useTuttiAddReplicationStore();

      await store.initAddReplicationPage();

      expect(store.selectedFromStorage?.name).toBe('main');
      expect(store.selectedToStorage?.name).toBe('backup');
      expect(store.hasEnoughStorages).toBe(true);
    });

    it('leaves the destination open when there are several other storages', async () => {
      tuttiCore.getStorages!.mockResolvedValue({
        storages: [
          { name: 'a', isMain: false, credentials: [] },
          { name: 'main', isMain: true, credentials: [] },
          { name: 'c', isMain: false, credentials: [] },
        ],
      });
      const store = useTuttiAddReplicationStore();

      await store.initAddReplicationPage();

      expect(store.selectedFromStorage?.name).toBe('main');
      expect(store.selectedToStorage).toBeNull();
    });

    it('needs two storages', async () => {
      tuttiCore.getStorages!.mockResolvedValue({
        storages: [{ name: 'main', isMain: true, credentials: [] }],
      });
      const store = useTuttiAddReplicationStore();

      await store.initAddReplicationPage();

      expect(store.hasEnoughStorages).toBe(false);
    });
  });

  it('lists buckets without a user', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    await store.initBucketsList();

    const request = tuttiCore.getBucketsForReplication!.mock.calls[0]![0];

    expect(request).toEqual({
      fromStorage: 'main',
      toStorage: 'backup',
      showReplicated: true,
    });
    expect('user' in request).toBe(false);
    expect(store.bucketsList).toEqual(['b1']);
  });

  it('exposes covered buckets as disabled rows with the owner as reason', async () => {
    tuttiCore.getBucketsForReplication!.mockResolvedValue({
      buckets: ['b1'],
      replicatedBuckets: [],
      coveredBuckets: [
        { bucket: 'b2', ownerKind: 'group', ownerId: 'g1' },
        {
          bucket: 'b3',
          ownerKind: 'replication',
          ownerId: ':main:backup:b3:b3',
        },
      ],
    });
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    await store.initBucketsList();

    expect(store.coveredRows).toHaveLength(2);
    expect(store.coveredRows[0]).toMatchObject({ name: 'b2' });
    expect(store.coveredRows[0]!.reason).toContain('coveredByGroup');
    expect(store.coveredRows[0]!.reason).toContain('g1');
    expect(store.coveredRows[1]!.reason).toContain('coveredByReplication');
    expect(store.coveredRows[1]!.reason).toContain('main → backup · b3');
    expect(store.isAllBucketsDisabled).toBe(true);
    expect(store.allBucketsDisabledReason).toContain('g1');
  });

  it('keeps all-buckets mode available when nothing is covered', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    await store.initBucketsList();

    expect(store.isAllBucketsDisabled).toBe(false);
  });

  it('does not create an all-buckets replication when a bucket is covered', async () => {
    tuttiCore.getBucketsForReplication!.mockResolvedValue({
      buckets: [],
      coveredBuckets: [{ bucket: 'b2', ownerKind: 'group', ownerId: 'g1' }],
    });
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    await store.initBucketsList();
    store.isForAllBuckets = true;

    expect(await store.createReplication()).toBe(false);
    expect(tuttiCore.addReplication).not.toHaveBeenCalled();
  });

  it('creates the replication with an id that has no user', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    store.isForAllBuckets = true;
    await store.createReplication();

    const request = tuttiCore.addReplication!.mock.calls[0]![0];

    expect(request).toEqual({
      id: { fromStorage: 'main', toStorage: 'backup' },
    });
    expect('user' in request.id).toBe(false);
  });

  it('creates one bucket-level replication per selected bucket without a user', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    store.selectedBuckets = ['b1'];
    await store.createReplication();

    expect(tuttiCore.addReplication).toHaveBeenCalledWith({
      id: {
        fromStorage: 'main',
        toStorage: 'backup',
        fromBucket: 'b1',
        toBucket: 'b1',
      },
    });
  });

  it('shows the server reason on a 409 and keeps the draft', async () => {
    const reason =
      'bucket "b1" on main->backup is already covered by group "g1"';

    tuttiCore.addReplication!.mockRejectedValue(conflictError(reason));
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    store.selectedBuckets = ['b1'];
    const created = await store.createReplication();

    expect(created).toBe(false);
    expect(push).not.toHaveBeenCalled();
    expect(store.createError).toContain(reason);
    expect(store.selectedBuckets).toEqual(['b1']);
    expect(store.selectedFromStorage?.name).toBe('main');
    expect(store.isSubmitting).toBe(false);
  });

  it('stays on the wizard and reports failure when every request fails', async () => {
    tuttiCore.addReplication!.mockRejectedValue(new Error('boom'));
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    store.isForAllBuckets = true;
    const created = await store.createReplication();

    expect(created).toBe(false);
    expect(push).not.toHaveBeenCalled();
    expect(store.createError).not.toBe('');
    expect(store.isForAllBuckets).toBe(true);
    expect(store.isSubmitting).toBe(false);
  });

  it('keeps only the failed buckets selected after a partial failure', async () => {
    tuttiCore.addReplication!.mockImplementation(
      async ({ id }: { id: { fromBucket?: string } }) => {
        if (id.fromBucket === 'b2') throw new Error('boom');
      },
    );
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    store.selectedBuckets = ['b1', 'b2'];
    const created = await store.createReplication();

    expect(created).toBe(false);
    expect(push).not.toHaveBeenCalled();
    expect(store.selectedBuckets).toEqual(['b2']);
  });

  it('creates the replications one at a time so they do not contend for the storage locks', async () => {
    let running = 0;
    let maxRunning = 0;
    const order: string[] = [];

    tuttiCore.addReplication!.mockImplementation(
      async ({ id }: { id: { fromBucket?: string } }) => {
        running += 1;
        maxRunning = Math.max(maxRunning, running);
        order.push(id.fromBucket ?? '');
        await Promise.resolve();
        await Promise.resolve();
        running -= 1;
      },
    );
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    store.selectedBuckets = ['b1', 'b2', 'b3'];
    await store.createReplication();

    expect(maxRunning).toBe(1);
    expect(order).toEqual(['b1', 'b2', 'b3']);
  });

  it('navigates away and reports success when every request succeeds', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    store.isForAllBuckets = true;
    const created = await store.createReplication();

    expect(created).toBe(true);
    expect(store.createError).toBe('');
    expect(push).toHaveBeenCalledTimes(1);
  });

  describe('replication name', () => {
    async function ready() {
      const store = useTuttiAddReplicationStore();

      await store.initAddReplicationPage();
      store.isForAllBuckets = true;

      return store;
    }

    it('sends the trimmed name', async () => {
      const store = await ready();

      store.name = '  prod to dr ';
      await store.createReplication();

      expect(tuttiCore.addReplication).toHaveBeenCalledWith({
        id: { fromStorage: 'main', toStorage: 'backup' },
        name: 'prod to dr',
      });
    });

    it('omits an empty name', async () => {
      const store = await ready();

      store.name = '   ';
      await store.createReplication();

      expect(tuttiCore.addReplication!.mock.calls[0]![0]).toEqual({
        id: { fromStorage: 'main', toStorage: 'backup' },
      });
    });

    it('does not submit a name over 64 characters', async () => {
      const store = await ready();

      store.name = 'a'.repeat(65);

      expect(await store.createReplication()).toBe(false);
      expect(tuttiCore.addReplication).not.toHaveBeenCalled();
    });
  });
});
