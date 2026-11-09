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
import { ChorusService } from '@/services/ChorusService';
import { TuttiService } from '@/services/TuttiService';
import { ErrorHelper } from '@/utils/helpers/ErrorHelper';
import {
  TrafficStatsHelper,
  type TrafficSnapshot,
} from '@/utils/helpers/TrafficStatsHelper';
import type { ChorusReplication, ChorusStorage } from '@/utils/types/chorus';
import type { TuttiFailoverEvent, TuttiProxyStatus } from '@/utils/types/tutti';

export const DASHBOARD_REFRESH_INTERVAL_MS = 30_000;

type SourceKey = 'storages' | 'replications' | 'proxies' | 'events';

interface DashboardState {
  /** true until the first refresh has finished */
  isLoading: boolean;
  isRefreshing: boolean;
  /** ms since epoch of the last refresh that delivered data */
  lastUpdated: number | null;
  storages: ChorusStorage[] | null;
  replications: ChorusReplication[] | null;
  proxies: TuttiProxyStatus[] | null;
  events: TuttiFailoverEvent[] | null;
  /** sources whose last request failed (403 is not an error) */
  failed: Record<SourceKey, boolean>;
  /** rolling snapshots per proxy, kept across page visits for rates */
  windows: Record<string, TrafficSnapshot[]>;
  timer: ReturnType<typeof setInterval> | null;
}

function getInitialState(): DashboardState {
  return {
    isLoading: true,
    isRefreshing: false,
    lastUpdated: null,
    storages: null,
    replications: null,
    proxies: null,
    events: null,
    failed: {
      storages: false,
      replications: false,
      proxies: false,
      events: false,
    },
    windows: {},
    timer: null,
  };
}

export const useTuttiDashboardStore = defineStore('tuttiDashboard', () => {
  const state = reactive<DashboardState>(getInitialState());
  let generation = 0;

  const failedSources = computed(() =>
    (Object.keys(state.failed) as SourceKey[]).filter(
      (key) => state.failed[key],
    ),
  );
  /** nothing could be loaded at all */
  const hasFatalError = computed(
    () =>
      state.storages === null &&
      state.replications === null &&
      state.proxies === null &&
      failedSources.value.some((source) => source !== 'events'),
  );

  function ingestProxies(proxies: TuttiProxyStatus[]) {
    const windows: Record<string, TrafficSnapshot[]> = {};

    proxies.forEach((proxy) => {
      const snapshot = TrafficStatsHelper.normalize(proxy.stats);

      if (!snapshot) return;

      windows[proxy.appId] = TrafficStatsHelper.appendSnapshot(
        state.windows[proxy.appId] ?? [],
        snapshot,
      );
    });

    state.windows = windows;
  }

  async function refresh() {
    if (state.isRefreshing) return;

    state.isRefreshing = true;
    const requestGeneration = generation;

    const [storages, replications, proxies, events] = await Promise.allSettled([
      ChorusService.getStorages(),
      ChorusService.getReplications(),
      TuttiService.getProxies(),
      TuttiService.getFailoverEvents(),
    ]);

    // Leaving the page or resetting the session invalidates pending responses.
    if (requestGeneration !== generation) return;

    let delivered = false;

    if (storages.status === 'fulfilled') {
      state.storages = storages.value.storages ?? [];
      delivered = true;
    }

    if (replications.status === 'fulfilled') {
      state.replications = replications.value.replications ?? [];
      delivered = true;
    }

    if (proxies.status === 'fulfilled') {
      state.proxies = proxies.value.proxies ?? [];
      ingestProxies(state.proxies);
      delivered = true;
    }

    if (events.status === 'fulfilled') {
      state.events = events.value.events ?? [];
      delivered = true;
    }

    state.failed = {
      storages: storages.status === 'rejected',
      replications: replications.status === 'rejected',
      proxies: proxies.status === 'rejected',
      // a viewer may not be allowed to read failover events
      events:
        events.status === 'rejected' &&
        ErrorHelper.getStatusCode(events.reason) !== 403,
    };

    if (events.status === 'rejected') state.events = null;

    if (delivered) state.lastUpdated = Date.now();

    state.isLoading = false;
    state.isRefreshing = false;
  }

  function onVisibilityChange() {
    if (
      !document.hidden &&
      (state.lastUpdated === null ||
        Date.now() - state.lastUpdated >= DASHBOARD_REFRESH_INTERVAL_MS)
    ) {
      refresh();
    }
  }

  function stopPolling() {
    generation += 1;
    state.isRefreshing = false;

    if (state.timer !== null) {
      clearInterval(state.timer);
      state.timer = null;
    }

    document.removeEventListener('visibilitychange', onVisibilityChange);
  }

  /** Refreshes now and every 30 s while the tab is visible. */
  function startPolling() {
    stopPolling();

    if (!document.hidden) refresh();

    state.timer = setInterval(() => {
      if (!document.hidden) refresh();
    }, DASHBOARD_REFRESH_INTERVAL_MS);
    document.addEventListener('visibilitychange', onVisibilityChange);
  }

  function $reset() {
    stopPolling();
    Object.assign(state, getInitialState());
  }

  return {
    ...toRefs(state),
    failedSources,
    hasFatalError,
    refresh,
    startPolling,
    stopPolling,
    $reset,
  };
});
