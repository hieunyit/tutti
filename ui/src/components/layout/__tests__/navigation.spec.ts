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
import { HELP_ITEMS, NAV_GROUPS } from '@/utils/constants/navigation';
import { RouteName } from '@/utils/types/router';

describe('sidebar structure (spec section 2)', () => {
  it('has 5 groups and 10 items, without a help group', () => {
    expect(NAV_GROUPS.map((group) => group.id)).toEqual([
      'overview',
      'storage',
      'dataProtection',
      'access',
      'administration',
    ]);
    expect(NAV_GROUPS.flatMap((group) => group.items)).toHaveLength(10);
  });

  it('marks Console users, System settings and Audit log admin-only', () => {
    const adminOnly = NAV_GROUPS.flatMap((group) => group.items)
      .filter((item) => item.adminOnly)
      .map((item) => item.routeName);

    expect(adminOnly.sort()).toEqual(
      [
        RouteName.TUTTI_CONSOLE_USERS,
        RouteName.TUTTI_SYSTEM_SETTINGS,
        RouteName.TUTTI_AUDIT_LOG,
      ].sort(),
    );
  });

  it('moves docs and the API reference to the Help menu', () => {
    expect(HELP_ITEMS.map((item) => item.routeName)).toEqual([
      RouteName.TUTTI_DOCS,
      RouteName.TUTTI_API_REFERENCE,
    ]);
  });
});
