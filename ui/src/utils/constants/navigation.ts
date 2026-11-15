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

import { RouteName, type NavGroupId } from '@/utils/types/router';

/** Which live counter a sidebar item shows as a badge. */
export type NavBadgeKey = 'attention' | 'failover' | 'protection';

export interface NavItem {
  routeName: RouteName;
  /** SVG path data (24x24 viewBox, stroked) */
  iconPath: string;
  badgeKey?: NavBadgeKey;
  /** hidden for viewers; must match `requiresAdmin` on the route */
  adminOnly?: boolean;
}

export interface NavGroup {
  id: NavGroupId;
  /** no visible group heading (the lone Dashboard item) */
  unlabeled?: boolean;
  items: NavItem[];
}

const ICONS = {
  dashboard: 'M3 3h7v9H3zM14 3h7v5h-7zM14 12h7v9h-7zM3 16h7v5H3z',
  storages:
    'M4 6c0-1.7 3.6-3 8-3s8 1.3 8 3-3.6 3-8 3-8-1.3-8-3zM4 6v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3',
  proxies:
    'M20.8 8.6a5 5 0 0 0-8.8-3.2 5 5 0 0 0-8.8 3.2c0 5.4 8.8 11.4 8.8 11.4s8.8-6 8.8-11.4z',
  users:
    'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM22 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8',
  admin:
    'M12 2l9 4v6c0 5-3.8 9.4-9 10-5.2-.6-9-5-9-10V6zM12 8a2 2 0 1 0 0 4 2 2 0 0 0 0-4zM8.5 16a3.5 3.5 0 0 1 7 0',
  replication:
    'M17 2l4 4-4 4M3 11V9a3 3 0 0 1 3-3h15M7 22l-4-4 4-4M21 13v2a3 3 0 0 1-3 3H3',
  routing:
    'M6 3v12M18 9a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM6 21a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM18 9a9 9 0 0 1-9 9',
  protection: 'M12 2l9 4v6c0 5-3.8 9.4-9 10-5.2-.6-9-5-9-10V6zM8 12h8M12 8v8',
  audit:
    'M9 5H7a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2h-2M9 5a2 2 0 0 0 2 2h2a2 2 0 0 0 2-2M9 5a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2m-6 7h6m-6 4h4',
  key: 'M15 7a4 4 0 1 1-3.9 5H3v3h3v3h3v-3h2.1A4 4 0 0 1 15 7z',
  docs: 'M4 4h11l5 5v11H4zM15 4v5h5M8 13h8M8 17h6',
  api: 'M8 6l-6 6 6 6M16 6l6 6-6 6M14 4l-4 16',
  settings:
    'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zm7.4-2.8l1.8-1.2-1.8-3.1-2.1.7a7.2 7.2 0 0 0-1.7-1L15.3 5h-3.6l-.3 2.6a7.2 7.2 0 0 0-1.7 1l-2.1-.7-1.8 3.1 1.8 1.2a7.4 7.4 0 0 0 0 2l-1.8 1.2 1.8 3.1 2.1-.7a7.2 7.2 0 0 0 1.7 1l.3 2.6h3.6l.3-2.6a7.2 7.2 0 0 0 1.7-1l2.1.7 1.8-3.1-1.8-1.2c.1-.7.1-1.3 0-2z',
} as const;

/**
 * THE sidebar definition (spec 2026-11-15, section 1): 4 groups plus the lone
 * Dashboard, 9 items. Labels come from i18n (group: layout `group.<id>`, item:
 * the route name in i18n/messages/routes.ts). A group is hidden when all its
 * items are hidden for the role. Docs and the API reference open from the
 * topbar Help menu, not here.
 */
export const NAV_GROUPS: NavGroup[] = [
  {
    id: 'overview',
    unlabeled: true,
    items: [
      {
        routeName: RouteName.TUTTI_DASHBOARD,
        iconPath: ICONS.dashboard,
        badgeKey: 'attention',
      },
    ],
  },
  {
    id: 'connect',
    items: [
      { routeName: RouteName.TUTTI_STORAGES, iconPath: ICONS.storages },
      { routeName: RouteName.TUTTI_PROXIES, iconPath: ICONS.proxies },
    ],
  },
  {
    id: 'replicate',
    items: [
      { routeName: RouteName.TUTTI_REPLICATION, iconPath: ICONS.replication },
      {
        routeName: RouteName.TUTTI_PROTECTION_GROUPS,
        iconPath: ICONS.protection,
        badgeKey: 'protection',
      },
      { routeName: RouteName.TUTTI_ROUTING_RULES, iconPath: ICONS.routing },
    ],
  },
  {
    id: 'access',
    items: [
      { routeName: RouteName.TUTTI_USERS, iconPath: ICONS.users },
      { routeName: RouteName.TUTTI_ACCESS_KEYS, iconPath: ICONS.key },
    ],
  },
  {
    id: 'admin',
    items: [
      {
        routeName: RouteName.TUTTI_CONSOLE_USERS,
        iconPath: ICONS.admin,
        adminOnly: true,
      },
      {
        routeName: RouteName.TUTTI_SYSTEM_SETTINGS,
        iconPath: ICONS.settings,
        adminOnly: true,
      },
      {
        routeName: RouteName.TUTTI_AUDIT_LOG,
        iconPath: ICONS.audit,
        adminOnly: true,
      },
    ],
  },
];

/** Groups and items a role may see (viewers lose every `adminOnly` item). */
export function visibleNavGroups(isAdmin: boolean): NavGroup[] {
  return NAV_GROUPS.map((group) => ({
    ...group,
    items: group.items.filter((item) => !item.adminOnly || isAdmin),
  })).filter((group) => group.items.length > 0);
}

/** Topbar Help menu entries (also offered by the quick search). */
export const HELP_ITEMS: NavItem[] = [
  { routeName: RouteName.TUTTI_DOCS, iconPath: ICONS.docs },
  { routeName: RouteName.TUTTI_API_REFERENCE, iconPath: ICONS.api },
];

/** At or below 1024 px the sidebar becomes a drawer opened from the topbar. */
export const SIDEBAR_MOBILE_BREAKPOINT = 1025;
