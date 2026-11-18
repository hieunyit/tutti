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

import type { TuttiReplication } from '@/utils/types/tuttiCore';

export const REPLICATION_NAME_MAX_LENGTH = 64;

export type ReplicationNameProblem = 'tooLong' | 'controlChars';

/** C0/C1 control characters, like the server check (unicode.IsControl) */
// oxlint-disable-next-line eslint(no-control-regex)
const CONTROL_CHARS = /[\u0000-\u001f\u007f-\u009f]/;

/**
 * The replication name is a display label only (never part of the id). The
 * helpers mirror the server rules and build the label shown for unnamed rows.
 */
export abstract class ReplicationNameHelper {
  static normalize(name: string | undefined): string {
    return (name ?? '').trim();
  }

  /** null when the (trimmed) name is acceptable; the empty name is acceptable */
  static getProblem(name: string | undefined): ReplicationNameProblem | null {
    const value = this.normalize(name);

    if ([...value].length > REPLICATION_NAME_MAX_LENGTH) return 'tooLong';

    if (CONTROL_CHARS.test(value)) return 'controlChars';

    return null;
  }

  /** `<from> → <to> · <bucket|all buckets>` */
  static getAutoLabel(
    replication: Pick<TuttiReplication, 'id'>,
    allBucketsText: string,
  ): string {
    const { fromStorage, toStorage, fromBucket, toBucket } = replication.id;
    let scope = allBucketsText;

    if (fromBucket) {
      scope =
        toBucket && toBucket !== fromBucket
          ? `${fromBucket} → ${toBucket}`
          : fromBucket;
    }

    return `${fromStorage} → ${toStorage} · ${scope}`;
  }

  /**
   * Readable label for a replication id string as the backend prints it in
   * conflict reasons: `<user>:<from>:<to>:<fromBucket>:<toBucket>` (bucket
   * scope) or `<user>:<from>:<to>` (all buckets). Anything else stays as is.
   */
  static labelFromIdString(id: string, allBucketsText: string): string {
    const parts = id.split(':');
    const n = parts.length;

    if (n === 3 || n === 5) {
      const [fromStorage = '', toStorage = ''] = parts.slice(1, 3);
      const [fromBucket = '', toBucket = ''] = parts.slice(3);

      if (fromStorage && toStorage) {
        return this.getAutoLabel(
          {
            id: {
              user: parts[0] ?? '',
              fromStorage,
              toStorage,
              fromBucket,
              toBucket,
            },
          },
          allBucketsText,
        );
      }
    }

    return id;
  }

  /** the name, or the auto label for an unnamed replication */
  static getDisplayName(
    replication: Pick<TuttiReplication, 'id' | 'name'>,
    allBucketsText: string,
  ): string {
    return (
      this.normalize(replication.name) ||
      this.getAutoLabel(replication, allBucketsText)
    );
  }

  /** from → to / bucket line under a name; empty when the label is the auto one */
  static getSecondaryLabel(
    replication: Pick<TuttiReplication, 'id' | 'name'>,
    allBucketsText: string,
  ): string {
    return this.normalize(replication.name)
      ? this.getAutoLabel(replication, allBucketsText)
      : '';
  }
}
