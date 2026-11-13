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
import { ChorusService } from '@/services/ChorusService';
import { AddReplicationStepName } from '@/utils/types/chorus';

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

type Mocks = Record<string, ReturnType<typeof vi.fn>>;

const chorus = ChorusService as unknown as Required<Mocks>;
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
  chorus.getStorages!.mockResolvedValue({
    storages: [
      { name: 'main', isMain: true, credentials: [{ alias: 'default' }] },
      { name: 'backup', isMain: false, credentials: [] },
    ],
  });
  chorus.getBucketsForReplication!.mockResolvedValue({
    buckets: ['b1'],
    replicatedBuckets: [],
    coveredBuckets: [],
  });
  chorus.addReplication!.mockResolvedValue({});
});

describe('tuttiAddReplicationStore', () => {
  it('has 3 steps and neither asks for nor discovers a user', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();

    expect(AddReplicationStepName.BUCKETS).toBe(3);
    expect(store.stepsCount).toBe(3);
    expect('selectedUser' in store).toBe(false);
    expect('users' in store).toBe(false);
    expect('eligibleUsers' in store).toBe(false);
    expect(tutti.getUsers).not.toHaveBeenCalled();
  });

  it('lists buckets without a user', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    await store.initBucketsList();

    const request = chorus.getBucketsForReplication!.mock.calls[0]![0];

    expect(request).toEqual({
      fromStorage: 'main',
      toStorage: 'backup',
      showReplicated: true,
    });
    expect('user' in request).toBe(false);
    expect(store.bucketsList).toEqual(['b1']);
  });

  it('exposes covered buckets as disabled rows with the owner as reason', async () => {
    chorus.getBucketsForReplication!.mockResolvedValue({
      buckets: ['b1'],
      replicatedBuckets: [],
      coveredBuckets: [
        { bucket: 'b2', ownerKind: 'group', ownerId: 'g1' },
        { bucket: 'b3', ownerKind: 'replication', ownerId: 'main:backup' },
      ],
    });
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    await store.initBucketsList();

    expect(store.coveredRows).toHaveLength(2);
    expect(store.coveredRows[0]).toMatchObject({ name: 'b2' });
    expect(store.coveredRows[0]!.reason).toContain('group');
    expect(store.coveredRows[0]!.reason).toContain('g1');
    expect(store.coveredRows[1]!.reason).toContain('replication');
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
    chorus.getBucketsForReplication!.mockResolvedValue({
      buckets: [],
      coveredBuckets: [{ bucket: 'b2', ownerKind: 'group', ownerId: 'g1' }],
    });
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    await store.initBucketsList();
    store.isForAllBuckets = true;

    expect(await store.createReplication()).toBe(false);
    expect(chorus.addReplication).not.toHaveBeenCalled();
  });

  it('creates the replication with an id that has no user', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    store.isForAllBuckets = true;
    await store.createReplication();

    const request = chorus.addReplication!.mock.calls[0]![0];

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

    expect(chorus.addReplication).toHaveBeenCalledWith({
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

    chorus.addReplication!.mockRejectedValue(conflictError(reason));
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
    chorus.addReplication!.mockRejectedValue(new Error('boom'));
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
    chorus.addReplication!.mockImplementation(
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

    chorus.addReplication!.mockImplementation(
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
});
