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

import { describe, expect, it } from 'vitest';
import {
  REPLICATION_NAME_MAX_LENGTH,
  ReplicationNameHelper as H,
} from '../ReplicationNameHelper';

const ALL = 'all buckets';

describe('ReplicationNameHelper.getAutoLabel', () => {
  it('shows all buckets for a user-level or storage-level replication', () => {
    expect(
      H.getAutoLabel({ id: { fromStorage: 'a', toStorage: 'b' } }, ALL),
    ).toBe('a → b · all buckets');
  });

  it('shows the bucket, and both names when they differ', () => {
    expect(
      H.getAutoLabel(
        {
          id: {
            fromStorage: 'a',
            toStorage: 'b',
            fromBucket: 'x',
            toBucket: 'x',
          },
        },
        ALL,
      ),
    ).toBe('a → b · x');
    expect(
      H.getAutoLabel(
        {
          id: {
            fromStorage: 'a',
            toStorage: 'b',
            fromBucket: 'x',
            toBucket: 'y',
          },
        },
        ALL,
      ),
    ).toBe('a → b · x → y');
  });
});

describe('ReplicationNameHelper display labels', () => {
  const id = { fromStorage: 'a', toStorage: 'b' };

  it('uses the trimmed name as primary and the auto label as secondary', () => {
    const r = { id, name: '  prod to dr ' };

    expect(H.getDisplayName(r, ALL)).toBe('prod to dr');
    expect(H.getSecondaryLabel(r, ALL)).toBe('a → b · all buckets');
  });

  it('falls back to the auto label when unnamed or blank', () => {
    expect(H.getDisplayName({ id }, ALL)).toBe('a → b · all buckets');
    expect(H.getDisplayName({ id, name: '   ' }, ALL)).toBe(
      'a → b · all buckets',
    );
    expect(H.getSecondaryLabel({ id, name: '' }, ALL)).toBe('');
  });
});

describe('ReplicationNameHelper.getProblem', () => {
  it('accepts empty, blank and 64 characters', () => {
    expect(H.getProblem(undefined)).toBeNull();
    expect(H.getProblem('  ')).toBeNull();
    expect(H.getProblem('é'.repeat(REPLICATION_NAME_MAX_LENGTH))).toBeNull();
  });

  it('counts characters, not UTF-16 units', () => {
    expect(H.getProblem('😀'.repeat(REPLICATION_NAME_MAX_LENGTH))).toBeNull();
    expect(H.getProblem('😀'.repeat(REPLICATION_NAME_MAX_LENGTH + 1))).toBe(
      'tooLong',
    );
  });

  it('rejects control characters', () => {
    expect(H.getProblem('a\nb')).toBe('controlChars');
    expect(H.getProblem('a\u0000b')).toBe('controlChars');
  });
});
