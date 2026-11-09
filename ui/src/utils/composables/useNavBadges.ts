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

import { computed } from 'vue';
import { storeToRefs } from 'pinia';
import { useTuttiDashboardStore } from '@/stores/tuttiDashboardStore';
import {
  DashboardHelper,
  type DashboardSources,
} from '@/utils/helpers/DashboardHelper';
import type { NavBadgeKey } from '@/utils/constants/navigation';
import { useNow } from '@/utils/composables/useNow';

/** Dashboard data older than this is not shown as a live sidebar counter. */
export const NAV_BADGE_MAX_AGE_MS = 90_000;

export type NavBadgeCounts = Partial<Record<NavBadgeKey, number>>;

/**
 * Sidebar counters from the dashboard data. Returns nothing when the data is
 * missing or stale (the dashboard only polls while it is open), so a badge is
 * never wrong, just hidden.
 */
export function computeNavBadges(
  sources: DashboardSources,
  lastUpdated: number | null,
  now: number,
): NavBadgeCounts {
  if (lastUpdated === null || now - lastUpdated > NAV_BADGE_MAX_AGE_MS) {
    return {};
  }

  const counts: NavBadgeCounts = {};
  const attention = DashboardHelper.getAttentionItems(sources, now).length;

  if (attention > 0) counts.attention = attention;

  if (sources.events && sources.events.length > 0) {
    counts.failover = sources.events.length;
  }

  return counts;
}

export function useNavBadges() {
  const store = useTuttiDashboardStore();
  const { storages, replications, proxies, events, policies, lastUpdated } =
    storeToRefs(store);
  const now = useNow(5000);

  return computed(() =>
    computeNavBadges(
      {
        storages: storages.value,
        replications: replications.value,
        proxies: proxies.value,
        events: events.value,
        policies: policies.value,
      },
      lastUpdated.value,
      now.value,
    ),
  );
}
