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
import { TuttiCoreService } from '@/services/TuttiCoreService';
import { ProtectionGroupService } from '@/services/ProtectionGroupService';
import { StorageProvider } from '@/utils/types/tuttiCore';
import type { ProtectionGroupListResponse } from '@/utils/types/protection';

vi.mock('@/services/TuttiCoreService', () => ({
  TuttiCoreService: {
    getStorages: vi.fn(),
    getBucketsForReplication: vi.fn(),
  },
}));

vi.mock('@/services/ProtectionGroupService', () => ({
  ProtectionGroupService: {
    list: vi.fn(),
  },
}));

describe('BucketDiscoveryHelper', () => {
  beforeEach(() => {
    vi.clearAllMocks();
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

    vi.mocked(TuttiCoreService.getStorages).mockResolvedValue({
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

    vi.mocked(TuttiCoreService.getBucketsForReplication).mockResolvedValue({
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
    vi.mocked(TuttiCoreService.getStorages).mockResolvedValue({
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

    vi.mocked(TuttiCoreService.getBucketsForReplication).mockImplementation(
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

  it('lists buckets without a user and includes covered buckets', async () => {
    vi.mocked(ProtectionGroupService.list).mockResolvedValue({ groups: [] });
    vi.mocked(TuttiCoreService.getStorages).mockResolvedValue({
      storages: [
        { name: 'st1', isMain: true, address: '', credentials: [] },
        { name: 'st2', isMain: false, address: '', credentials: [] },
      ],
    } as never);
    vi.mocked(TuttiCoreService.getBucketsForReplication).mockResolvedValue({
      buckets: ['b1'],
      replicatedBuckets: [],
      coveredBuckets: [{ bucket: 'b2', ownerKind: 'group', ownerId: 'g1' }],
    });

    const items = await BucketDiscoveryHelper.discoverStorageBuckets(['st1']);

    expect(items).toEqual([
      { bucket: 'b1', storage: 'st1' },
      { bucket: 'b2', storage: 'st1' },
    ]);

    const request = vi.mocked(TuttiCoreService.getBucketsForReplication).mock
      .calls[0]![0];

    expect('user' in request).toBe(false);
  });

  it('passes an explicit user through when the caller has one', async () => {
    vi.mocked(TuttiCoreService.getStorages).mockResolvedValue({
      storages: [{ name: 'st1', isMain: true }],
    } as never);
    vi.mocked(TuttiCoreService.getBucketsForReplication).mockResolvedValue({
      buckets: [],
      replicatedBuckets: [],
    });
    await BucketDiscoveryHelper.discoverStorageBuckets(['st1'], 'u1');
    expect(TuttiCoreService.getBucketsForReplication).toHaveBeenCalledWith(
      expect.objectContaining({ user: 'u1' }),
    );
  });
});
