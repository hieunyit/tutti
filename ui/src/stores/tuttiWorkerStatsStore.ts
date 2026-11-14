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
import { reactive, shallowRef, toRefs } from 'vue';
import axios from 'axios';
import { TuttiService } from '@/services/TuttiService';
import { usePolling } from '@/utils/composables/usePolling';
import {
  diagnoseThroughput,
  normalizeWorkerStats,
  type ThroughputHintId,
  type ThroughputTotals,
  type ThroughputWorker,
} from '@/utils/helpers/ThroughputHelper';

export const WORKER_STATS_POLL_INTERVAL_MS = 5_000;

export const useTuttiWorkerStatsStore = defineStore('tuttiWorkerStats', () => {
  const state = reactive({
    /** true until the first response (success or failure) */
    isLoading: true,
    hasError: false,
    errorMessage: '',
    /** backend has no worker-stats route (404/501): hide the panel */
    unavailable: false,
    lastUpdated: null as number | null,
    hints: [] as ThroughputHintId[],
  });
  const workers = shallowRef<ThroughputWorker[]>([]);
  const totals = shallowRef<ThroughputTotals | null>(null);
  const polling = usePolling();
  let previousFailed: number | null = null;

  async function fetchOnce() {
    try {
      const res = normalizeWorkerStats(await TuttiService.getWorkerStats());

      workers.value = res.workers;
      totals.value = res.totals;
      state.hints = diagnoseThroughput(res.totals, res.workers, previousFailed);
      previousFailed = res.totals.failedTotal;
      state.hasError = false;
      state.errorMessage = '';
      state.lastUpdated = Date.now();
    } catch (e) {
      const status = axios.isAxiosError(e) ? e.response?.status : undefined;

      if (status === 404 || status === 501) {
        state.unavailable = true;
        // not awaited: stop() would wait for this very tick
        void polling.stop();
      } else {
        state.hasError = true;
        state.errorMessage =
          e instanceof Error ? e.message : String(e ?? 'error');
      }
    } finally {
      state.isLoading = false;
    }
  }

  /** (Re)starts polling: one request now, then every 5 s (adaptive). */
  async function start() {
    state.unavailable = false;
    state.isLoading = totals.value === null;
    await polling.start(fetchOnce, WORKER_STATS_POLL_INTERVAL_MS);
  }

  async function stop() {
    await polling.stop();
  }

  async function retry() {
    state.hasError = false;
    state.isLoading = true;
    await start();
  }

  return { ...toRefs(state), workers, totals, start, stop, retry };
});
