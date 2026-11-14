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
import { TuttiCoreService } from '@/services/TuttiCoreService';
import { TuttiService } from '@/services/TuttiService';
import {
  DEFAULT_STATS_RANGE,
  RANGE_MS,
  STATS_RANGES,
  appendPoint,
  historyToPoints,
  normalizeRates,
  sumCounter,
  trimPoints,
  type RatePoint,
  type Rates,
} from '@/utils/helpers/MonitoringHelper';
import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';
import { LocalStorageItem } from '@/utils/types/localStorage';
import type { TuttiReplication } from '@/utils/types/tuttiCore';
import type { TuttiProxyStatus, TuttiStatsRange } from '@/utils/types/tutti';

/** Live values: one small request every 5 s (paused while the tab is hidden). */
export const MONITORING_POLL_INTERVAL_MS = 5_000;
/** History is refetched now and then so the charts stay in sync. */
export const MONITORING_HISTORY_REFRESH_MS = 30_000;
export const MONITORING_REPLICATION_INTERVAL_MS = 30_000;
/** A replication with lag > 0 for this many consecutive polls is flagged. */
export const MONITORING_LAG_WARN_POLLS = 3;
/** A poll still running after this long is abandoned for a fresh one. */
const POLL_STUCK_MS = 20_000;

export interface MonitoringProxy {
  appId: string;
  address?: string;
  startedAt?: string;
  updatedAt?: string;
  /** null for proxies that do not publish rates (older versions) */
  rates: Rates | null;
  /** cumulative requests routed to each storage since proxy start */
  requestsByStorage: Record<string, number>;
  syncQuorumReached: number;
  syncBelowQuorum: number;
  syncSkipped: number;
}

interface MonitoringState {
  /** true until the first history response (success or failure) */
  isLoading: boolean;
  /** user paused the live updates */
  isPaused: boolean;
  /** ms since epoch of the last successful proxies response */
  lastUpdated: number | null;
  /** the last live poll failed */
  hasError: boolean;
  /** the history request failed */
  historyError: boolean;
  range: TuttiStatsRange;
  /** end of the chart window (ms since epoch) */
  windowEnd: number;
  replications: TuttiReplication[];
  replicationsLoaded: boolean;
  replicationsError: boolean;
  /** consecutive polls with eventLag > 0, by replication idStr */
  lagStreaks: Record<string, number>;
}

export function readStoredRange(): TuttiStatsRange {
  try {
    const value = window.localStorage.getItem(
      LocalStorageItem.MONITORING_RANGE,
    );

    if (value && (STATS_RANGES as string[]).includes(value)) {
      return value as TuttiStatsRange;
    }
  } catch {
    // storage unavailable: use the default
  }

  return DEFAULT_STATS_RANGE;
}

function storeRange(range: TuttiStatsRange) {
  try {
    window.localStorage.setItem(LocalStorageItem.MONITORING_RANGE, range);
  } catch {
    // storage unavailable: the choice is just not remembered
  }
}

function getInitialState(): MonitoringState {
  return {
    isLoading: true,
    isPaused: false,
    lastUpdated: null,
    hasError: false,
    historyError: false,
    range: readStoredRange(),
    windowEnd: Date.now(),
    replications: [],
    replicationsLoaded: false,
    replicationsError: false,
    lagStreaks: {},
  };
}

function normalizeProxy(proxy: TuttiProxyStatus): MonitoringProxy {
  const n = TrafficStatsHelper.toNumber;

  return {
    appId: proxy.appId,
    address: proxy.address,
    startedAt: proxy.startedAt,
    updatedAt: proxy.updatedAt,
    rates: proxy.rates ? normalizeRates(proxy.rates) : null,
    requestsByStorage: Object.fromEntries(
      Object.entries(proxy.stats?.requestsByStorage ?? {}).map(([k, v]) => [
        k,
        n(v),
      ]),
    ),
    syncQuorumReached: n(proxy.stats?.syncQuorumReached),
    syncBelowQuorum: n(proxy.stats?.syncBelowQuorum),
    syncSkipped: n(proxy.stats?.syncSkipped),
  };
}

export const useTuttiMonitoringStore = defineStore('tuttiMonitoring', () => {
  const state = reactive<MonitoringState>(getInitialState());
  // large or replaced as a whole: no deep reactivity needed
  const points = shallowRef<RatePoint[]>([]);
  const proxies = shallowRef<MonitoringProxy[]>([]);
  const totalRates = shallowRef<Rates | null>(null);

  let started = false;
  let historyRequest: AbortController | null = null;
  let proxyRequest: AbortController | null = null;
  let proxyRequestStartedAt = 0;
  let replicationRequest: AbortController | null = null;
  let latestLive: RatePoint | null = null;
  let pollTimer: ReturnType<typeof setInterval> | null = null;
  let historyTimer: ReturnType<typeof setInterval> | null = null;
  let replicationTimer: ReturnType<typeof setInterval> | null = null;

  function isLagging(replication: TuttiReplication): boolean {
    return (
      (state.lagStreaks[replication.idStr] ?? 0) >= MONITORING_LAG_WARN_POLLS
    );
  }

  /** Cumulative counters summed over all proxies. */
  function syncTotals() {
    return {
      reached: sumCounter(proxies.value.map((p) => p.syncQuorumReached)),
      below: sumCounter(proxies.value.map((p) => p.syncBelowQuorum)),
      skipped: sumCounter(proxies.value.map((p) => p.syncSkipped)),
    };
  }

  async function loadHistory() {
    historyRequest?.abort();

    const request = new AbortController();
    const { range } = state;

    historyRequest = request;

    try {
      const history = await TuttiService.getStatsHistory(range, request.signal);

      if (historyRequest !== request) return;

      const end = Date.now();
      const loaded = trimPoints(historyToPoints(history), end, RANGE_MS[range]);
      const last = loaded[loaded.length - 1];

      // keep the newest live point when it is newer than the history
      points.value =
        latestLive && (!last || latestLive.t > last.t)
          ? [...loaded, latestLive]
          : loaded;
      state.windowEnd = end;
      state.historyError = false;
    } catch {
      if (historyRequest === request) state.historyError = true;
    } finally {
      if (historyRequest === request) {
        historyRequest = null;
        state.isLoading = false;
      }
    }
  }

  function ingestProxies(list: TuttiProxyStatus[], total: Rates | null) {
    proxies.value = list.map(normalizeProxy);
    totalRates.value = total;

    if (!total || list.length === 0) return;

    const times = list
      .map((p) => Date.parse(p.updatedAt ?? ''))
      .filter((t) => Number.isFinite(t));
    const t = times.length > 0 ? Math.max(...times) : Date.now();
    const point = { t, rates: total };

    latestLive = point;
    points.value = appendPoint(points.value, point, RANGE_MS[state.range]);
    state.windowEnd = Date.now();
  }

  async function loadProxies() {
    if (proxyRequest !== null) {
      if (Date.now() - proxyRequestStartedAt < POLL_STUCK_MS) return;

      proxyRequest.abort();
    }

    const request = new AbortController();

    proxyRequest = request;
    proxyRequestStartedAt = Date.now();

    try {
      const res = await TuttiService.getProxies(request.signal);

      if (proxyRequest !== request) return;

      ingestProxies(
        res.proxies ?? [],
        res.totalRates ? normalizeRates(res.totalRates) : null,
      );
      state.hasError = false;
      state.lastUpdated = Date.now();
    } catch {
      if (proxyRequest === request) state.hasError = true;
    } finally {
      if (proxyRequest === request) proxyRequest = null;
    }
  }

  async function loadReplications() {
    if (replicationRequest !== null) return;

    const request = new AbortController();

    replicationRequest = request;

    try {
      const res = await TuttiCoreService.getReplications(request.signal);

      if (replicationRequest !== request) return;

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
      if (replicationRequest === request) state.replicationsError = true;
    } finally {
      if (replicationRequest === request) {
        replicationRequest = null;
        state.replicationsLoaded = true;
      }
    }
  }

  const isVisible = () => typeof document === 'undefined' || !document.hidden;

  function clearTimers() {
    if (pollTimer !== null) clearInterval(pollTimer);

    if (historyTimer !== null) clearInterval(historyTimer);

    if (replicationTimer !== null) clearInterval(replicationTimer);

    pollTimer = null;
    historyTimer = null;
    replicationTimer = null;
  }

  function abortRequests() {
    historyRequest?.abort();
    proxyRequest?.abort();
    replicationRequest?.abort();
    historyRequest = null;
    proxyRequest = null;
    replicationRequest = null;
  }

  function onVisibilityChange() {
    if (document.hidden || !started || state.isPaused) return;

    // the tab was hidden: catch up at once
    loadHistory();
    loadProxies();
  }

  function schedule() {
    clearTimers();
    pollTimer = setInterval(() => {
      if (isVisible()) loadProxies();
    }, MONITORING_POLL_INTERVAL_MS);
    historyTimer = setInterval(() => {
      if (isVisible()) loadHistory();
    }, MONITORING_HISTORY_REFRESH_MS);
    replicationTimer = setInterval(() => {
      if (isVisible()) loadReplications();
    }, MONITORING_REPLICATION_INTERVAL_MS);
  }

  /** Loads history and live values, then keeps them up to date. */
  function start() {
    stop();
    started = true;
    state.isPaused = false;
    document.addEventListener('visibilitychange', onVisibilityChange);
    loadHistory();
    loadProxies();
    loadReplications();
    schedule();
  }

  function stop() {
    started = false;
    abortRequests();
    clearTimers();

    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', onVisibilityChange);
    }
  }

  /** Stops the live updates but keeps the data on screen. */
  function pause() {
    abortRequests();
    clearTimers();
    state.isPaused = true;
  }

  function resume() {
    state.isPaused = false;
    loadHistory();
    loadProxies();
    loadReplications();
    schedule();
  }

  function setRange(range: TuttiStatsRange) {
    if (range === state.range) return;

    state.range = range;
    storeRange(range);
    // points of the old range are not valid for the new one
    points.value = trimPoints(points.value, Date.now(), RANGE_MS[range]);
    loadHistory();
  }

  function $reset() {
    stop();
    Object.assign(state, getInitialState());
    points.value = [];
    proxies.value = [];
    totalRates.value = null;
    latestLive = null;
  }

  return {
    ...toRefs(state),
    points,
    proxies,
    totalRates,
    isLagging,
    syncTotals,
    loadHistory,
    loadProxies,
    loadReplications,
    start,
    stop,
    pause,
    resume,
    setRange,
    $reset,
  };
});
