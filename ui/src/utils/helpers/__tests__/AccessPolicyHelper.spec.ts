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
import { AccessPolicyHelper } from '../AccessPolicyHelper';
import {
  TuttiAccessPermission,
  TuttiAccessPolicyValidationError as E,
  type TuttiAccessPolicyForm,
} from '@/utils/types/tutti';

function form(
  over: Partial<TuttiAccessPolicyForm> = {},
): TuttiAccessPolicyForm {
  return {
    user: 'user1',
    permission: TuttiAccessPermission.READ_ONLY,
    buckets: ['photos-*', 'logs'],
    description: '',
    ...over,
  };
}

describe('AccessPolicyHelper', () => {
  it('accepts valid patterns including *', () => {
    expect(AccessPolicyHelper.validatePattern('*')).toBeNull();
    expect(AccessPolicyHelper.validatePattern('photos-*')).toBeNull();
    expect(AccessPolicyHelper.validatePattern('logs')).toBeNull();
  });

  it('rejects empty and whitespace patterns', () => {
    expect(AccessPolicyHelper.validatePattern('')).toBe(E.PATTERN_EMPTY);
    expect(AccessPolicyHelper.validatePattern('   ')).toBe(E.PATTERN_EMPTY);
    expect(AccessPolicyHelper.validatePattern('my bucket')).toBe(
      E.PATTERN_WHITESPACE,
    );
    expect(AccessPolicyHelper.validatePattern('a\tb')).toBe(
      E.PATTERN_WHITESPACE,
    );
  });

  it('validates the whole form', () => {
    expect(AccessPolicyHelper.validate(form())).toEqual([]);
    expect(AccessPolicyHelper.validate(form({ user: ' ' }))).toContain(
      E.USER_REQUIRED,
    );
    expect(AccessPolicyHelper.validate(form({ buckets: ['a b', ''] }))).toEqual(
      [E.PATTERN_WHITESPACE, E.PATTERN_EMPTY],
    );
    expect(AccessPolicyHelper.validate(form({ buckets: ['a', 'a'] }))).toEqual([
      E.PATTERN_DUPLICATE,
    ]);
    expect(
      AccessPolicyHelper.validate(
        form({ permission: 'nope' as TuttiAccessPermission }),
      ),
    ).toEqual([E.PERMISSION_INVALID]);
  });

  it('adds patterns trimmed, once, and removes them', () => {
    expect(AccessPolicyHelper.addPattern(['a'], ' b ')).toEqual(['a', 'b']);
    expect(AccessPolicyHelper.addPattern(['a'], 'a')).toEqual(['a']);
    expect(AccessPolicyHelper.addPattern(['a'], '  ')).toEqual(['a']);
    expect(AccessPolicyHelper.removePattern(['a', 'b'], 'a')).toEqual(['b']);
  });

  it('sends * when no bucket is listed', () => {
    expect(AccessPolicyHelper.formToPolicy(form({ buckets: [] }))).toEqual({
      user: 'user1',
      permission: 'read-only',
      buckets: ['*'],
      description: '',
    });
  });

  it('round-trips policy and form, showing "*" as an empty list', () => {
    expect(
      AccessPolicyHelper.policyToForm('u', {
        user: 'u',
        permission: 'write-only',
        buckets: ['*'],
      }),
    ).toMatchObject({ permission: 'write-only', buckets: [] });
    expect(AccessPolicyHelper.policyToForm('u').permission).toBe(
      TuttiAccessPermission.READ_WRITE,
    );
  });

  it('zips parallel access key / source arrays', () => {
    expect(
      AccessPolicyHelper.getAccessKeys({
        user: 'u',
        accessKeyIds: ['k1', 'k2'],
        sources: ['custom'],
      }),
    ).toEqual([
      { accessKeyId: 'k1', source: 'custom' },
      { accessKeyId: 'k2', source: '' },
    ]);
  });

  it('knows when a policy means all buckets', () => {
    expect(AccessPolicyHelper.isAllBuckets(undefined)).toBe(true);
    expect(AccessPolicyHelper.isAllBuckets([])).toBe(true);
    expect(AccessPolicyHelper.isAllBuckets(['*'])).toBe(true);
    expect(AccessPolicyHelper.isAllBuckets(['a'])).toBe(false);
  });
});
