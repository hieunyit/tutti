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

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { BucketDiscoveryHelper } from '../BucketDiscoveryHelper';
import { ChorusService } from '@/services/ChorusService';
import { ProtectionGroupService } from '@/services/ProtectionGroupService';
import { TuttiService } from '@/services/TuttiService';
import { StorageProvider } from '@/utils/types/chorus';
import type { ProtectionGroupListResponse } from '@/utils/types/protection';

vi.mock('@/services/ChorusService', () => ({
  ChorusService: {
    getStorages: vi.fn(),
    getBucketsForReplication: vi.fn(),
  },
}));

vi.mock('@/services/ProtectionGroupService', () => ({
  ProtectionGroupService: {
    list: vi.fn(),
  },
}));
vi.mock('@/services/TuttiService', () => ({
  TuttiService: { getUsers: vi.fn() },
}));

describe('BucketDiscoveryHelper', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(TuttiService.getUsers).mockResolvedValue({
      users: [
        {
          name: 'u1',
          storages: [
            { storage: 'st1', configured: true },
            { storage: 'st2', configured: true },
          ],
        },
      ],
    });
  });

  it('discovers buckets across protection groups and storages', async () => {
    const mockGroups: ProtectionGroupListResponse = {
      groups: [
        {
          id: 'pg1',
          user: 'u1',
          buckets: ['bucket-pg1', 'shared-bucket'],
          members: ['st1'],
          copies: 1,
          maxEventLag: 0,
          mode: 'manual',
          after: '1m',
          electorate: [],
          maxSwitchesPerHour: 1,
        },
      ],
    };

    vi.mocked(ProtectionGroupService.list).mockResolvedValue(mockGroups);

    vi.mocked(ChorusService.getStorages).mockResolvedValue({
      storages: [
        {
          name: 'st1',
          isMain: true,
          address: 'http://st1',
          provider: StorageProvider.S3,
          credentials: [{ alias: 'u1', accessKey: 'k', secretKey: '' }],
        },
      ],
    });

    vi.mocked(ChorusService.getBucketsForReplication).mockResolvedValue({
      buckets: ['bucket-st1', 'shared-bucket'],
      replicatedBuckets: ['bucket-repl'],
    });

    const buckets = await BucketDiscoveryHelper.discoverBuckets('u1');

    expect(buckets).toEqual([
      'bucket-pg1',
      'bucket-repl',
      'bucket-st1',
      'shared-bucket',
    ]);
  });

  it('discovers storage-specific buckets', async () => {
    vi.mocked(ChorusService.getStorages).mockResolvedValue({
      storages: [
        {
          name: 'st1',
          isMain: true,
          address: 'http://st1',
          provider: StorageProvider.S3,
          credentials: [{ alias: 'u1', accessKey: 'k', secretKey: '' }],
        },
        {
          name: 'st2',
          isMain: false,
          address: 'http://st2',
          provider: StorageProvider.S3,
          credentials: [{ alias: 'u1', accessKey: 'k', secretKey: '' }],
        },
      ],
    });

    vi.mocked(ChorusService.getBucketsForReplication).mockImplementation(
      async (req) => {
        if (req.fromStorage === 'st1') {
          return { buckets: ['b1'], replicatedBuckets: [] };
        }

        return { buckets: ['b2'], replicatedBuckets: [] };
      },
    );

    const items = await BucketDiscoveryHelper.discoverStorageBuckets([
      'st1',
      'st2',
    ]);

    expect(items).toEqual([
      { bucket: 'b1', storage: 'st1' },
      { bucket: 'b2', storage: 'st2' },
    ]);
  });

  it('does not send a guessed tenant when no user can access the storage', async () => {
    vi.mocked(TuttiService.getUsers).mockResolvedValueOnce({ users: [] });
    await BucketDiscoveryHelper.discoverStorageBuckets(['st1']);
    expect(ChorusService.getBucketsForReplication).not.toHaveBeenCalled();
  });
});
