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

import { describe, expect, it, vi } from 'vitest';
import type { RouteRecordRaw } from 'vue-router';
import {
  HELP_ITEMS,
  NAV_GROUPS,
  SIDEBAR_MOBILE_BREAKPOINT,
  visibleNavGroups,
} from '@/utils/constants/navigation';
import { routes } from '@/router/routes';
import messages from '@/i18n/messages/routes';
import { RouteName } from '@/utils/types/router';

// the real kit pulls in a stylesheet that node cannot import
vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
}));

function flatten(records: RouteRecordRaw[]): RouteRecordRaw[] {
  return records.flatMap((record) => [
    record,
    ...flatten(record.children ?? []),
  ]);
}

const allRoutes = flatten(routes);
const byName = new Map(
  allRoutes.filter((r) => r.name).map((r) => [String(r.name), r]),
);
const items = NAV_GROUPS.flatMap((group) => group.items);

describe('sidebar definition (spec 2026-11-15, section 1)', () => {
  it('has the Dashboard alone plus 4 groups and 11 items, without a help group', () => {
    expect(NAV_GROUPS.map((group) => group.id)).toEqual([
      'overview',
      'connect',
      'replicate',
      'access',
      'admin',
    ]);
    expect(NAV_GROUPS.filter((g) => g.unlabeled).map((g) => g.id)).toEqual([
      'overview',
    ]);
    expect(items).toHaveLength(11);
  });

  it('lists the items in the agreed order', () => {
    expect(NAV_GROUPS.map((g) => g.items.map((i) => i.routeName))).toEqual([
      [RouteName.TUTTI_DASHBOARD],
      [RouteName.TUTTI_STORAGES, RouteName.TUTTI_PROXIES],
      [
        RouteName.TUTTI_REPLICATION,
        RouteName.TUTTI_PROTECTION_GROUPS,
        RouteName.TUTTI_ROUTING_RULES,
      ],
      [RouteName.TUTTI_USERS, RouteName.TUTTI_ACCESS_KEYS],
      [
        RouteName.TUTTI_CONSOLE_USERS,
        RouteName.TUTTI_SYSTEM_SETTINGS,
        RouteName.TUTTI_AUDIT_LOG,
      ],
    ]);
  });

  it('has no duplicate route among the items', () => {
    const names = items.map((item) => item.routeName);

    expect(new Set(names).size).toBe(names.length);
  });

  it('points every item at a real, non-redirect route', () => {
    for (const item of items) {
      const record = byName.get(item.routeName);

      expect(record, item.routeName).toBeDefined();
      expect(record?.redirect, item.routeName).toBeUndefined();
      expect(record?.component, item.routeName).toBeDefined();
    }
  });

  it('keeps nav metadata of each route in line with its group and item', () => {
    for (const group of NAV_GROUPS) {
      for (const item of group.items) {
        const meta = byName.get(item.routeName)?.meta;

        expect(meta?.navGroup, item.routeName).toBe(group.id);
        expect(meta?.navSection, item.routeName).toBe(item.routeName);
      }
    }
  });

  it('flags an item admin-only exactly when its route requires admin', () => {
    for (const item of items) {
      const record = byName.get(item.routeName);

      expect(Boolean(item.adminOnly), item.routeName).toBe(
        Boolean(record?.meta?.requiresAdmin),
      );
    }

    expect(
      items
        .filter((item) => item.adminOnly)
        .map((item) => item.routeName)
        .sort(),
    ).toEqual(
      [
        RouteName.TUTTI_CONSOLE_USERS,
        RouteName.TUTTI_SYSTEM_SETTINGS,
        RouteName.TUTTI_AUDIT_LOG,
      ].sort(),
    );
  });

  it('hides the whole Admin group from viewers and shows everything to admins', () => {
    const viewer = visibleNavGroups(false);
    const admin = visibleNavGroups(true);

    expect(viewer.map((g) => g.id)).not.toContain('admin');
    expect(viewer.flatMap((g) => g.items)).toHaveLength(8);
    expect(admin.flatMap((g) => g.items)).toHaveLength(11);
  });

  it('does not mutate the definition when filtering', () => {
    visibleNavGroups(false);
    expect(items).toHaveLength(11);
  });

  it('gives every item a label and an icon in en, de and vi', () => {
    for (const locale of ['en', 'de', 'vi'] as const) {
      const table = (
        messages as unknown as Record<string, Record<string, string>>
      )[locale];

      for (const item of items) {
        expect(
          table?.[item.routeName],
          `${locale} ${item.routeName}`,
        ).toBeTruthy();
        expect(item.iconPath.length).toBeGreaterThan(10);
      }
    }
  });

  it('has the Dashboard badge for attention and Protection groups for failover', () => {
    expect(
      items.filter((i) => i.badgeKey).map((i) => [i.routeName, i.badgeKey]),
    ).toEqual([
      [RouteName.TUTTI_DASHBOARD, 'attention'],
      [RouteName.TUTTI_PROTECTION_GROUPS, 'protection'],
    ]);
  });

  it('moves docs and the API reference to the Help menu', () => {
    expect(HELP_ITEMS.map((item) => item.routeName)).toEqual([
      RouteName.TUTTI_DOCS,
      RouteName.TUTTI_API_REFERENCE,
    ]);
  });

  it('switches to the drawer at 1024 px and below', () => {
    expect(SIDEBAR_MOBILE_BREAKPOINT).toBe(1025);
  });
});

describe('route table', () => {
  it('has no duplicate route names or paths', () => {
    const named = allRoutes.filter((r) => r.name).map((r) => String(r.name));

    expect(new Set(named).size).toBe(named.length);

    const paths = flatten(routes[3]?.children ?? []).map((r) => r.path);

    expect(new Set(paths).size).toBe(paths.length);
  });

  it('gives every named route a title in all three languages', () => {
    for (const locale of ['en', 'de', 'vi'] as const) {
      const table = (
        messages as unknown as Record<string, Record<string, string>>
      )[locale];

      for (const name of Object.values(RouteName)) {
        expect(table?.[name], `${locale} ${name}`).toBeTruthy();
      }
    }
  });

  it('marks Console users, System settings and Audit log requiresAdmin', () => {
    for (const name of [
      RouteName.TUTTI_CONSOLE_USERS,
      RouteName.TUTTI_SYSTEM_SETTINGS,
      RouteName.TUTTI_AUDIT_LOG,
    ]) {
      expect(byName.get(name)?.meta?.requiresAdmin, name).toBe(true);
    }
  });

  it('flags the Traffic tab as admin-only on the dashboard', () => {
    expect(byName.get(RouteName.TUTTI_DASHBOARD)?.meta?.adminTabs).toEqual([
      'traffic',
      'metrics',
    ]);
  });
});
