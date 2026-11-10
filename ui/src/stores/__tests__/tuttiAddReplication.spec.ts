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

import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useTuttiAddReplicationStore } from '../tuttiAddReplicationStore';
import { ChorusService } from '@/services/ChorusService';
import { TuttiService } from '@/services/TuttiService';
import { AddReplicationStepName } from '@/utils/types/chorus';

vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock('@/i18n/useStoreI18n', () => ({
  useStoreI18n: () => ({ t: (key: string) => key }),
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
  });
  chorus.addReplication!.mockResolvedValue({});
  tutti.getUsers!.mockResolvedValue({
    users: [
      {
        name: 'alice',
        storages: [
          { storage: 'main', configured: true, mode: 'override' },
          { storage: 'backup', configured: false, mode: 'connection-account' },
        ],
      },
      {
        name: 'bob',
        storages: [
          { storage: 'main', configured: true, mode: 'override' },
          { storage: 'backup', configured: false, mode: 'missing' },
        ],
      },
    ],
  });
});

describe('chorusAddReplicationStore user step', () => {
  it('has the user step and never picks a user by itself', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();

    expect(AddReplicationStepName.USER).toBe(3);
    expect(store.stepsCount).toBe(4);
    expect(store.users.map((u: { name: string }) => u.name)).toEqual([
      'alice',
      'bob',
    ]);
    expect(store.selectedUser).toBeNull();
  });

  it('reports the credential mode per storage and blocks users without credentials', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();

    expect(store.userModeOn('alice', 'backup')).toBe('connection-account');
    expect(store.userModeOn('nobody', 'backup')).toBe('unknown');

    store.selectedUser = 'alice';
    expect(store.isSelectedUserUsable).toBe(true);
    store.selectedUser = 'bob';
    expect(store.isSelectedUserUsable).toBe(false);
  });

  it('loads buckets for the selected user only', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    await store.getBucketsList();
    expect(chorus.getBucketsForReplication).not.toHaveBeenCalled();

    store.selectedUser = 'alice';
    await store.getBucketsList();
    expect(chorus.getBucketsForReplication).toHaveBeenCalledWith(
      expect.objectContaining({ user: 'alice', fromStorage: 'main' }),
    );
  });

  it('creates the replication for the selected user and nothing without one', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    store.isForAllBuckets = true;
    await store.createReplication();
    expect(chorus.addReplication).not.toHaveBeenCalled();

    store.selectedUser = 'alice';
    await store.createReplication();
    expect(chorus.addReplication).toHaveBeenCalledWith({
      id: { user: 'alice', fromStorage: 'main', toStorage: 'backup' },
    });
  });
});
