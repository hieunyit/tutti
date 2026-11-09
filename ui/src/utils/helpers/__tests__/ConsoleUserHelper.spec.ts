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
import { ConsoleUserHelper } from '../ConsoleUserHelper';
import type { ConsoleUser } from '@/utils/types/users';

const user = (patch: Partial<ConsoleUser> = {}): ConsoleUser => ({
  username: 'ops',
  role: 'viewer',
  source: 'managed',
  ...patch,
});

describe('ConsoleUserHelper.status', () => {
  it('derives Active, Invited and Disabled', () => {
    expect(ConsoleUserHelper.status(user())).toBe('active');
    expect(
      ConsoleUserHelper.status(
        user({ mustChangePassword: true, lastLoginAt: null }),
      ),
    ).toBe('invited');
    expect(
      ConsoleUserHelper.status(
        user({ mustChangePassword: true, lastLoginAt: '2026-11-01T10:00:00Z' }),
      ),
    ).toBe('active');
    expect(
      ConsoleUserHelper.status(
        user({ disabled: true, mustChangePassword: true }),
      ),
    ).toBe('disabled');
  });
});

describe('ConsoleUserHelper.actions', () => {
  it('allows every action on other managed users', () => {
    const a = ConsoleUserHelper.actions(user(), 'admin');

    expect(a).toMatchObject({
      isSelf: false,
      canEdit: true,
      canResetPassword: true,
      canToggleDisabled: true,
      canDelete: true,
    });
  });

  it('blocks everything for config users', () => {
    const a = ConsoleUserHelper.actions(user({ source: 'config' }), 'admin');

    expect(a.isConfig).toBe(true);
    expect(
      a.canEdit || a.canDelete || a.canToggleDisabled || a.canResetPassword,
    ).toBe(false);
  });

  it('blocks disable, demote and delete on yourself', () => {
    const a = ConsoleUserHelper.actions(user({ username: 'me' }), 'me');

    expect(a.isSelf).toBe(true);
    expect(a.canEdit || a.canDelete || a.canToggleDisabled).toBe(false);
  });
});

describe('ConsoleUserHelper passwords', () => {
  it('validates length and username', () => {
    expect(ConsoleUserHelper.validatePassword('short')).toBe('tooShort');
    expect(ConsoleUserHelper.validatePassword('x'.repeat(73))).toBe('tooLong');
    expect(
      ConsoleUserHelper.validatePassword('operator-1234', 'operator-1234'),
    ).toBe('sameAsUsername');
    expect(ConsoleUserHelper.validatePassword('a-long-enough-pass')).toBeNull();
  });

  it('rates strength', () => {
    expect(ConsoleUserHelper.passwordStrength('abc')).toBe('weak');
    expect(ConsoleUserHelper.passwordStrength('abcdefghijkl')).toBe('weak');
    expect(ConsoleUserHelper.passwordStrength('Abcdefghijkl')).toBe('fair');
    expect(ConsoleUserHelper.passwordStrength('Abcdefghijkl-1234')).toBe(
      'strong',
    );
  });
});
