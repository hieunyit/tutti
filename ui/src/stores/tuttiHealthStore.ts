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
import { defineStore } from 'pinia';
import { computed, reactive, toRefs } from 'vue';
import { TuttiService } from '@/services/TuttiService';
import type { TuttiProxyStatus } from '@/utils/types/tutti';

export const HEALTH_REFRESH_INTERVAL_MS = 10_000;

interface TuttiHealthState {
  isLoading: boolean;
  hasError: boolean;
  proxies: TuttiProxyStatus[];
  timer: ReturnType<typeof setInterval> | null;
}

function getInitialState(): TuttiHealthState {
  return { isLoading: false, hasError: false, proxies: [], timer: null };
}

export const useTuttiHealthStore = defineStore('tuttiHealth', () => {
  const state = reactive<TuttiHealthState>(getInitialState());

  const hasNoData = computed(() => state.proxies.length === 0);

  async function loadProxies(showLoading = false) {
    if (showLoading) state.isLoading = true;

    try {
      const res = await TuttiService.getProxies();

      state.proxies = res.proxies ?? [];
      state.hasError = false;
    } catch {
      state.hasError = true;
    } finally {
      state.isLoading = false;
    }
  }

  function startPolling() {
    stopPolling();
    loadProxies(true);
    state.timer = setInterval(() => loadProxies(), HEALTH_REFRESH_INTERVAL_MS);
  }

  function stopPolling() {
    if (state.timer !== null) {
      clearInterval(state.timer);
      state.timer = null;
    }
  }

  function $reset() {
    stopPolling();
    Object.assign(state, getInitialState());
  }

  return {
    ...toRefs(state),
    hasNoData,
    loadProxies,
    startPolling,
    stopPolling,
    $reset,
  };
});
