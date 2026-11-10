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
}

export interface NavGroup {
  id: NavGroupId;
  /** hidden for read-only accounts */
  adminOnly?: boolean;
  items: NavItem[];
}

const ICONS = {
  dashboard: 'M3 3h7v9H3zM14 3h7v5h-7zM14 12h7v9h-7zM3 16h7v5H3z',
  monitoring: 'M3 12h4l3-8 4 16 3-8h4',
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
  failover: 'M12 2l9 4v6c0 5-3.8 9.4-9 10-5.2-.6-9-5-9-10V6zM9 12l2 2 4-4',
  protection: 'M12 2l9 4v6c0 5-3.8 9.4-9 10-5.2-.6-9-5-9-10V6zM8 12h8M12 8v8',
  diff: 'M9 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h4M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4M12 3v18',
  key: 'M15 7a4 4 0 1 1-3.9 5H3v3h3v3h3v-3h2.1A4 4 0 0 1 15 7z',
  docs: 'M4 4h11l5 5v11H4zM15 4v5h5M8 13h8M8 17h6',
  api: 'M8 6l-6 6 6 6M16 6l6 6-6 6M14 4l-4 16',
} as const;

/** Sidebar structure; labels come from i18n (group: nav.group.<id>, item: route name). */
export const NAV_GROUPS: NavGroup[] = [
  {
    id: 'overview',
    items: [
      {
        routeName: RouteName.CHORUS_HOME,
        iconPath: ICONS.dashboard,
        badgeKey: 'attention',
      },
      { routeName: RouteName.CHORUS_MONITORING, iconPath: ICONS.monitoring },
    ],
  },
  {
    id: 'storage',
    items: [
      { routeName: RouteName.CHORUS_STORAGES, iconPath: ICONS.storages },
      { routeName: RouteName.TUTTI_PROXIES, iconPath: ICONS.proxies },
    ],
  },
  {
    id: 'dataProtection',
    items: [
      { routeName: RouteName.CHORUS_REPLICATION, iconPath: ICONS.replication },
      {
        routeName: RouteName.CHORUS_ROUTING_POLICIES,
        iconPath: ICONS.routing,
      },
      {
        routeName: RouteName.TUTTI_FAILOVER,
        iconPath: ICONS.failover,
        badgeKey: 'failover',
      },
      {
        routeName: RouteName.TUTTI_PROTECTION_GROUPS,
        iconPath: ICONS.protection,
        badgeKey: 'protection',
      },
      { routeName: RouteName.CHORUS_DIFF_REPORTS, iconPath: ICONS.diff },
    ],
  },
  {
    id: 'access',
    items: [
      { routeName: RouteName.TUTTI_USERS, iconPath: ICONS.users },
      { routeName: RouteName.TUTTI_PROXY_ACCOUNTS, iconPath: ICONS.key },
    ],
  },
  {
    id: 'administration',
    adminOnly: true,
    items: [
      { routeName: RouteName.TUTTI_CONSOLE_USERS, iconPath: ICONS.admin },
    ],
  },
  {
    id: 'help',
    items: [
      { routeName: RouteName.TUTTI_DOCS, iconPath: ICONS.docs },
      { routeName: RouteName.TUTTI_API_REFERENCE, iconPath: ICONS.api },
    ],
  },
];

export const SIDEBAR_MOBILE_BREAKPOINT = 960;
