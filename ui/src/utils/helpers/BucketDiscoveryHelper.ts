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

import { ChorusService } from '@/services/ChorusService';
import { ProtectionGroupService } from '@/services/ProtectionGroupService';

export interface StorageBucketEntry {
  bucket: string;
  storage: string;
}

export abstract class BucketDiscoveryHelper {
  /**
   * Discovers unique bucket names across all Protection Groups and reachable storages.
   */
  static async discoverBuckets(userName?: string): Promise<string[]> {
    const bucketsSet = new Set<string>();

    // 1. Gather all buckets from Protection Groups
    try {
      const { groups } = await ProtectionGroupService.list();

      for (const g of groups || []) {
        for (const b of g.buckets || []) {
          if (b && b.trim()) {
            bucketsSet.add(b.trim());
          }
        }
      }
    } catch {
      // Best-effort
    }

    // 2. Query storages via Chorus replication list-buckets API
    try {
      const { storages } = await ChorusService.getStorages();

      if (storages && storages.length > 0) {
        const user =
          userName ||
          storages.flatMap(
            (s) => s.credentials?.map((c) => c.alias) || [],
          )[0] ||
          'user1';

        for (let i = 0; i < storages.length; i++) {
          const from = storages[i]?.name;
          const to = storages[(i + 1) % storages.length]?.name;

          if (!from || !to) continue;

          try {
            const res = await ChorusService.getBucketsForReplication({
              user,
              fromStorage: from,
              toStorage: to,
              showReplicated: true,
            });

            for (const b of res.buckets || []) {
              if (b && b.trim()) bucketsSet.add(b.trim());
            }

            for (const b of res.replicatedBuckets || []) {
              if (b && b.trim()) bucketsSet.add(b.trim());
            }
          } catch {
            // Best-effort per storage pair
          }
        }
      }
    } catch {
      // Best-effort
    }

    return Array.from(bucketsSet).sort();
  }

  /**
   * Discovers buckets on specific storages, returning bucket names paired with their storage name.
   */
  static async discoverStorageBuckets(
    storages: string[],
    userName?: string,
  ): Promise<StorageBucketEntry[]> {
    const results: StorageBucketEntry[] = [];
    const seen = new Set<string>();

    try {
      const { storages: allStorages } = await ChorusService.getStorages();

      if (!allStorages || allStorages.length === 0) return [];

      const user =
        userName ||
        allStorages.flatMap(
          (s) => s.credentials?.map((c) => c.alias) || [],
        )[0] ||
        'user1';

      for (const storageName of storages) {
        const other =
          allStorages.find((s) => s.name !== storageName)?.name || storageName;

        try {
          const res = await ChorusService.getBucketsForReplication({
            user,
            fromStorage: storageName,
            toStorage: other,
            showReplicated: true,
          });
          const all = [
            ...(res.buckets || []),
            ...(res.replicatedBuckets || []),
          ];

          for (const b of all) {
            const key = `${storageName}:${b}`;

            if (b && !seen.has(key)) {
              seen.add(key);
              results.push({ bucket: b, storage: storageName });
            }
          }
        } catch {
          // Best-effort
        }
      }
    } catch {
      // Best-effort
    }

    return results;
  }
}
