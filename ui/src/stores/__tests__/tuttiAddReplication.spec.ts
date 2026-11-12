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

describe('tuttiAddReplicationStore', () => {
  it('has 3 storage-to-storage steps without manual user step', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();

    expect(AddReplicationStepName.FROM_STORAGE).toBe(1);
    expect(AddReplicationStepName.TO_STORAGE).toBe(2);
    expect(AddReplicationStepName.BUCKETS).toBe(3);
    expect(store.stepsCount).toBe(3);
    expect(store.selectedUser).toBe('alice');
  });

  it('loads buckets using background tenant user', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    await store.getBucketsList();
    expect(chorus.getBucketsForReplication).toHaveBeenCalledWith(
      expect.objectContaining({ user: 'alice', fromStorage: 'main', toStorage: 'backup' }),
    );
  });

  it('creates the replication for the background tenant user', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    store.isForAllBuckets = true;
    await store.createReplication();
    expect(chorus.addReplication).toHaveBeenCalledWith({
      id: { user: 'alice', fromStorage: 'main', toStorage: 'backup' },
    });
  });

  it('allows operator to explicitly change the replication tenant user', async () => {
    const store = useTuttiAddReplicationStore();

    await store.initAddReplicationPage();
    expect(store.selectedUser).toBe('alice');

    await store.selectUser('bob');
    expect(store.selectedUser).toBe('bob');
    store.isForAllBuckets = true;
    await store.createReplication();
    expect(chorus.addReplication).toHaveBeenCalledWith({
      id: { user: 'bob', fromStorage: 'main', toStorage: 'backup' },
    });
  });
});
