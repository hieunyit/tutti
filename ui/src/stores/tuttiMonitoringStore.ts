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
import {
  TrafficStatsHelper,
  type TrafficRates,
  type TrafficSnapshot,
} from '@/utils/helpers/TrafficStatsHelper';
import type { ChorusReplication } from '@/utils/types/chorus';
import type { TuttiProxyStatus } from '@/utils/types/tutti';

export const MONITORING_PROXY_INTERVAL_MS = 10_000;
export const MONITORING_REPLICATION_INTERVAL_MS = 30_000;
/** A replication with lag > 0 for this many consecutive polls is flagged. */
export const MONITORING_LAG_WARN_POLLS = 3;
/** Proxies whose newest sample is older than this are not part of "current". */
const STALE_AFTER_MS = 3 * MONITORING_PROXY_INTERVAL_MS;

export interface MonitoringProxy {
  appId: string;
  address?: string;
}

interface MonitoringState {
  /** true until the first proxies response (success or failure) */
  isLoading: boolean;
  hasError: boolean;
  proxies: MonitoringProxy[];
  /** proxies that run without published stats (older versions) */
  proxiesWithoutStats: string[];
  /** rolling window of snapshots per appId (30 min) */
  windows: Record<string, TrafficSnapshot[]>;
  replications: ChorusReplication[];
  replicationsLoaded: boolean;
  replicationsError: boolean;
  /** consecutive polls with eventLag > 0, by replication idStr */
  lagStreaks: Record<string, number>;
  proxyTimer: ReturnType<typeof setInterval> | null;
  replicationTimer: ReturnType<typeof setInterval> | null;
}

function getInitialState(): MonitoringState {
  return {
    isLoading: true,
    hasError: false,
    proxies: [],
    proxiesWithoutStats: [],
    windows: {},
    replications: [],
    replicationsLoaded: false,
    replicationsError: false,
    lagStreaks: {},
    proxyTimer: null,
    replicationTimer: null,
  };
}

export const useTuttiMonitoringStore = defineStore('tuttiMonitoring', () => {
  const state = reactive<MonitoringState>(getInitialState());

  const seriesByProxy = computed<Record<string, TrafficRates[]>>(() =>
    Object.fromEntries(
      Object.entries(state.windows).map(([appId, window]) => [
        appId,
        TrafficStatsHelper.computeSeries(window),
      ]),
    ),
  );

  /** Newest rates per proxy, skipping proxies that stopped reporting. */
  const latestByProxy = computed<Record<string, TrafficRates>>(() => {
    const latest = Object.entries(seriesByProxy.value)
      .map(([appId, series]) => [appId, series[series.length - 1]] as const)
      .filter(
        (entry): entry is [string, TrafficRates] => entry[1] !== undefined,
      );
    const newest = Math.max(0, ...latest.map(([, rates]) => rates.timestamp));

    return Object.fromEntries(
      latest.filter(([, rates]) => newest - rates.timestamp <= STALE_AFTER_MS),
    );
  });

  /** Current rate over all proxies. */
  const current = computed<TrafficRates | null>(() =>
    TrafficStatsHelper.sumRates(Object.values(latestByProxy.value)),
  );

  /** Chart series over all proxies. */
  const aggregatedSeries = computed<TrafficRates[]>(() =>
    TrafficStatsHelper.aggregateSeries(Object.values(seriesByProxy.value)),
  );

  /** Newest cumulative snapshot per proxy (totals since start). */
  const latestSnapshots = computed<Record<string, TrafficSnapshot>>(() =>
    Object.fromEntries(
      Object.entries(state.windows)
        .map(([appId, window]) => [appId, window[window.length - 1]] as const)
        .filter(
          (entry): entry is [string, TrafficSnapshot] => entry[1] !== undefined,
        ),
    ),
  );

  const hasStats = computed(() => Object.keys(state.windows).length > 0);

  /** True while at least one proxy has fewer than two samples (no rate yet). */
  const isWarmingUp = computed(() => hasStats.value && current.value === null);

  function isLagging(replication: ChorusReplication): boolean {
    return (
      (state.lagStreaks[replication.idStr] ?? 0) >= MONITORING_LAG_WARN_POLLS
    );
  }

  function ingestProxies(proxies: TuttiProxyStatus[]) {
    const windows: Record<string, TrafficSnapshot[]> = {};
    const withoutStats: string[] = [];

    proxies.forEach((proxy) => {
      const snapshot = TrafficStatsHelper.normalize(proxy.stats);

      if (!snapshot) {
        withoutStats.push(proxy.appId);

        return;
      }

      windows[proxy.appId] = TrafficStatsHelper.appendSnapshot(
        state.windows[proxy.appId] ?? [],
        snapshot,
      );
    });

    // proxies that disappeared lose their history
    state.windows = windows;
    state.proxies = proxies.map(({ appId, address }) => ({ appId, address }));
    state.proxiesWithoutStats = withoutStats;
  }

  async function loadProxies() {
    try {
      const res = await TuttiService.getProxies();

      ingestProxies(res.proxies ?? []);
      state.hasError = false;
    } catch {
      state.hasError = true;
    } finally {
      state.isLoading = false;
    }
  }

  async function loadReplications() {
    try {
      const res = await ChorusService.getReplications();
      const replications = (res.replications ?? []).filter(
        (replication) => !replication.isArchived,
      );
      const streaks: Record<string, number> = {};

      replications.forEach((replication) => {
        streaks[replication.idStr] =
          TrafficStatsHelper.toNumber(replication.eventLag) > 0
            ? (state.lagStreaks[replication.idStr] ?? 0) + 1
            : 0;
      });

      state.replications = replications;
      state.lagStreaks = streaks;
      state.replicationsError = false;
    } catch {
      state.replicationsError = true;
    } finally {
      state.replicationsLoaded = true;
    }
  }

  function stopPolling() {
    if (state.proxyTimer !== null) clearInterval(state.proxyTimer);

    if (state.replicationTimer !== null) clearInterval(state.replicationTimer);

    state.proxyTimer = null;
    state.replicationTimer = null;
  }

  function startPolling() {
    stopPolling();
    loadProxies();
    loadReplications();
    state.proxyTimer = setInterval(loadProxies, MONITORING_PROXY_INTERVAL_MS);
    state.replicationTimer = setInterval(
      loadReplications,
      MONITORING_REPLICATION_INTERVAL_MS,
    );
  }

  function $reset() {
    stopPolling();
    Object.assign(state, getInitialState());
  }

  return {
    ...toRefs(state),
    seriesByProxy,
    latestByProxy,
    latestSnapshots,
    current,
    aggregatedSeries,
    hasStats,
    isWarmingUp,
    isLagging,
    loadProxies,
    loadReplications,
    startPolling,
    stopPolling,
    $reset,
  };
});
