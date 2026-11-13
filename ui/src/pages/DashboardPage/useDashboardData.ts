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

import { storeToRefs } from 'pinia';
import {
  computed,
  getCurrentInstance,
  onBeforeMount,
  onBeforeUnmount,
} from 'vue';
import type { PageState } from '@/components/kit';
import { useTuttiDashboardStore } from '@/stores/tuttiDashboardStore';

/**
 * Single entry point to the dashboard store. The store refreshes every 30 s
 * and only while the browser tab is visible; this composable starts that when
 * the calling component mounts and stops it when it unmounts.
 */
export function useDashboardData() {
  const store = useTuttiDashboardStore();
  const { isLoading, hasFatalError, lastUpdated } = storeToRefs(store);

  const state = computed<PageState>(() => {
    if (isLoading.value) return 'loading';

    return hasFatalError.value ? 'error' : 'ready';
  });
  const updatedAt = computed(() =>
    lastUpdated.value === null ? null : new Date(lastUpdated.value),
  );

  if (getCurrentInstance()) {
    onBeforeMount(() => store.startPolling());
    onBeforeUnmount(() => store.stopPolling());
  }

  return { state, updatedAt, refresh: () => store.refresh() };
}
