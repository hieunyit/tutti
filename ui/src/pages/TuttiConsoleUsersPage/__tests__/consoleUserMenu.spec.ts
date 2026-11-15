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
import { consoleUserMenu, isProtectedAdmin } from '../consoleUserMenu';
import type { ConsoleUser } from '@/utils/types/users';

const user = (over: Partial<ConsoleUser>): ConsoleUser =>
  ({
    username: 'u',
    role: 'viewer',
    source: 'managed',
    ...over,
  }) as ConsoleUser;

describe('consoleUserMenu', () => {
  const admin = user({ username: 'a', role: 'admin' });
  const other = user({ username: 'b', role: 'admin' });

  it('has no menu for yourself or for config users', () => {
    expect(consoleUserMenu(admin, [admin, other], 'a').hasMenu).toBe(false);
    expect(
      consoleUserMenu(user({ source: 'config' }), [admin], 'a').hasMenu,
    ).toBe(false);
  });

  it('allows every action on another managed administrator', () => {
    const menu = consoleUserMenu(other, [admin, other], 'a');

    expect(menu.hasMenu).toBe(true);
    expect(Object.values(menu.enabled).every(Boolean)).toBe(true);
  });

  it('protects the last enabled administrator but still allows a reset', () => {
    const menu = consoleUserMenu(other, [other, user({ username: 'v' })], 'v');

    expect(isProtectedAdmin(other, [other])).toBe(true);
    expect(menu.enabled).toEqual({
      role: false,
      reset: true,
      toggle: false,
      delete: false,
    });
  });
});
