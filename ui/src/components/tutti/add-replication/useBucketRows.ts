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

import { computed, type Ref } from 'vue';
import { storeToRefs } from 'pinia';
import { useTuttiAddReplicationStore } from '@/stores/tuttiAddReplicationStore';
import { GeneralHelper } from '@/utils/helpers/GeneralHelper';

export interface BucketRowData {
  name: string;
  isReplicated: boolean;
  /** set when a protection group or replication already covers the bucket */
  coveredReason?: string;
}

/**
 * Rows of the bucket picker: available buckets, buckets that are already
 * covered (with the owner as reason) and, when asked for, replicated ones,
 * filtered by the search text and sorted by name.
 */
export function useBucketRows(search: Ref<string>) {
  const { bucketsList, replicatedBucketsList, isReplicatedShown, coveredRows } =
    storeToRefs(useTuttiAddReplicationStore());

  const matches = (name: string) =>
    name.toLowerCase().includes(search.value.trim().toLowerCase());

  return computed<BucketRowData[]>(() => {
    const rows: BucketRowData[] = [
      ...bucketsList.value.map((name) => ({ name, isReplicated: false })),
      ...coveredRows.value.map(({ name, reason }) => ({
        name,
        isReplicated: false,
        coveredReason: reason,
      })),
      ...(isReplicatedShown.value
        ? replicatedBucketsList.value.map((name) => ({
            name,
            isReplicated: true,
          }))
        : []),
    ];

    return GeneralHelper.orderBy(
      rows.filter((row) => matches(row.name)),
      'name',
      'asc',
    );
  });
}
