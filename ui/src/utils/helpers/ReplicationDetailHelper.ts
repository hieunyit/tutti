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

import type { LocationQueryRaw, LocationQuery } from 'vue-router';
import type { TuttiReplicationId } from '@/utils/types/tuttiCore';

/** Deep-link query of the replication detail page: the replication id fields. */
export abstract class ReplicationDetailHelper {
  static toQuery(id: TuttiReplicationId): LocationQueryRaw {
    const query: LocationQueryRaw = {
      from: id.fromStorage,
      to: id.toStorage,
    };

    if (id.user) query.user = id.user;

    if (id.fromBucket) query.fromBucket = id.fromBucket;

    if (id.toBucket) query.toBucket = id.toBucket;

    return query;
  }

  /** null when the required fields are missing */
  static fromQuery(query: LocationQuery): TuttiReplicationId | null {
    const pick = (key: string): string => {
      const value = query[key];
      const first = Array.isArray(value) ? value[0] : value;

      return typeof first === 'string' ? first : '';
    };
    const fromStorage = pick('from');
    const toStorage = pick('to');

    if (!fromStorage || !toStorage) return null;

    const id: TuttiReplicationId = { fromStorage, toStorage };
    const user = pick('user');
    const fromBucket = pick('fromBucket');
    const toBucket = pick('toBucket');

    if (user) id.user = user;

    if (fromBucket) id.fromBucket = fromBucket;

    if (toBucket) id.toBucket = toBucket;

    return id;
  }
}
