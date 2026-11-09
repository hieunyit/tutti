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

import { IconName } from '@/utils/types/icon';
import { RouteName, type NavGroupId } from '@/utils/types/router';

export interface NavItem {
  routeName: RouteName;
  icon: IconName;
}

export interface NavGroup {
  id: NavGroupId;
  items: NavItem[];
}

/** Sidebar structure; labels come from i18n (group: nav.group.<id>, item: route name). */
export const NAV_GROUPS: NavGroup[] = [
  {
    id: 'overview',
    items: [
      { routeName: RouteName.CHORUS_HOME, icon: IconName.BASE_BAR_CHART },
      { routeName: RouteName.CHORUS_MONITORING, icon: IconName.BASE_TIMER },
    ],
  },
  {
    id: 'storage',
    items: [
      { routeName: RouteName.CHORUS_STORAGES, icon: IconName.BASE_CYLINDER },
      { routeName: RouteName.TUTTI_HEALTH, icon: IconName.BASE_SERVER },
    ],
  },
  {
    id: 'dataProtection',
    items: [
      {
        routeName: RouteName.CHORUS_REPLICATION,
        icon: IconName.BASE_SWAP_HORIZONTAL,
      },
      {
        routeName: RouteName.CHORUS_ROUTING_POLICIES,
        icon: IconName.BASE_ARROW_FORWARD,
      },
      { routeName: RouteName.TUTTI_FAILOVER, icon: IconName.BASE_REFRESH },
      {
        routeName: RouteName.CHORUS_DIFF_REPORTS,
        icon: IconName.BASE_DOCUMENT,
      },
    ],
  },
  {
    id: 'access',
    items: [
      { routeName: RouteName.TUTTI_PROXY_ACCOUNTS, icon: IconName.BASE_KEY },
    ],
  },
];

export const SIDEBAR_MOBILE_BREAKPOINT = 960;
