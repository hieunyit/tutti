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

import { RouteName } from '@/utils/types/router';
import type { ChorusReplication, ChorusStorage } from '@/utils/types/chorus';
import type { TuttiFailoverEvent, TuttiProxyStatus } from '@/utils/types/tutti';
import {
  TrafficStatsHelper,
  type TrafficRates,
  type TrafficSnapshot,
} from '@/utils/helpers/TrafficStatsHelper';

/** A proxy that has not reported for longer than this needs attention. */
export const DASHBOARD_PROXY_STALE_MS = 60_000;
/** Rates are only shown when the two newest snapshots are this close. */
export const DASHBOARD_RATE_MAX_GAP_MS = 120_000;

export type DashboardSeverity = 'error' | 'warning';

export type StorageHealthState = 'healthy' | 'unavailable' | 'unknown';

export interface StorageHealthRow {
  name: string;
  isMain: boolean;
  address: string;
  provider: string;
  state: StorageHealthState;
  /** highest consecutive failure count over all proxies */
  failures: number;
}

export interface ReplicationBreakdown {
  total: number;
  active: number;
  paused: number;
  /** initial sync finished and no event lag */
  upToDate: number;
  /** initial sync still running */
  initial: number;
  /** live replication with an event lag */
  behind: number;
  /** sum of the event lag of all replications */
  totalLag: number;
}

export interface ProxyReporting {
  total: number;
  reporting: number;
  stale: TuttiProxyStatus[];
}

export type AttentionKind =
  | 'storageDown'
  | 'failoverEvent'
  | 'replicationPaused'
  | 'replicationBehind'
  | 'proxyStale';

export interface AttentionItem {
  id: string;
  kind: AttentionKind;
  severity: DashboardSeverity;
  /** values for the i18n message of the kind */
  params: Record<string, string | number>;
  route: RouteName;
}

export interface DashboardSources {
  storages: ChorusStorage[] | null;
  replications: ChorusReplication[] | null;
  proxies: TuttiProxyStatus[] | null;
  events: TuttiFailoverEvent[] | null;
}

export interface TrafficSummary {
  /** current requests/s, null while only one snapshot per proxy exists */
  requestsPerSecond: number | null;
  /** total requests since proxy start (sum over proxies) */
  requestsTotal: number;
  /** 5xx share in percent: current rate if known, else since start */
  errorPercent5xx: number;
  /** true when errorPercent5xx is based on the current rate */
  isErrorPercentCurrent: boolean;
}

export abstract class DashboardHelper {
  static isProxyReporting(proxy: TuttiProxyStatus, now: number): boolean {
    const updated = proxy.updatedAt ? Date.parse(proxy.updatedAt) : NaN;

    return (
      Number.isFinite(updated) &&
      now - updated <= DASHBOARD_PROXY_STALE_MS &&
      updated - now <= DASHBOARD_PROXY_STALE_MS
    );
  }

  static replicationLabel(replication: ChorusReplication): string {
    const { user, fromStorage, toStorage, fromBucket } = replication.id;

    return `${user}: ${fromStorage} → ${toStorage}${fromBucket ? ` (${fromBucket})` : ''}`;
  }

  /** Non-archived replications only. */
  static activeReplications(
    replications: ChorusReplication[],
  ): ChorusReplication[] {
    return replications.filter((replication) => !replication.isArchived);
  }

  static eventLag(replication: ChorusReplication): number {
    return Math.max(0, TrafficStatsHelper.toNumber(replication.eventLag));
  }

  /** Health per storage aggregated over all proxies (any proxy down = down). */
  static getStorageHealth(
    storages: ChorusStorage[],
    proxies: TuttiProxyStatus[] | null,
    now = Date.now(),
  ): StorageHealthRow[] {
    return storages.map((storage) => {
      let up = false;
      let down = false;
      let failures = 0;

      (proxies ?? []).forEach((proxy) => {
        if (!DashboardHelper.isProxyReporting(proxy, now)) return;

        (proxy.storages ?? [])
          .filter((health) => health.name === storage.name)
          .forEach((health) => {
            if (health.available === false) down = true;
            else if (health.available === true) up = true;

            failures = Math.max(failures, health.consecutiveFailures ?? 0);
          });
      });

      return {
        name: storage.name,
        isMain: storage.isMain,
        address: storage.address,
        provider: String(storage.provider),
        state: down ? 'unavailable' : up ? 'healthy' : 'unknown',
        failures,
      };
    });
  }

  static getReplicationBreakdown(
    all: ChorusReplication[],
  ): ReplicationBreakdown {
    const replications = DashboardHelper.activeReplications(all);
    const result: ReplicationBreakdown = {
      total: replications.length,
      active: 0,
      paused: 0,
      upToDate: 0,
      initial: 0,
      behind: 0,
      totalLag: 0,
    };

    replications.forEach((replication) => {
      const lag = DashboardHelper.eventLag(replication);

      result.totalLag += lag;

      if (replication.isPaused) {
        result.paused += 1;

        return;
      }

      result.active += 1;

      if (!replication.isInitDone) result.initial += 1;
      else if (lag > 0) result.behind += 1;
      else result.upToDate += 1;
    });

    return result;
  }

  /** Percentage (0-100, rounded) of part in total; 0 for an empty total. */
  static percent(part: number, total: number): number {
    return total > 0 ? Math.round((part / total) * 100) : 0;
  }

  static getProxyReporting(
    proxies: TuttiProxyStatus[],
    now: number,
  ): ProxyReporting {
    const stale = proxies.filter(
      (proxy) => !DashboardHelper.isProxyReporting(proxy, now),
    );

    return {
      total: proxies.length,
      reporting: proxies.length - stale.length,
      stale,
    };
  }

  /** Newest rates per proxy from the last two snapshots of each window. */
  static getCurrentRates(
    windows: Record<string, TrafficSnapshot[]>,
    maxGapMs = DASHBOARD_RATE_MAX_GAP_MS,
    now = Date.now(),
  ): TrafficRates | null {
    const list: TrafficRates[] = [];

    Object.values(windows).forEach((window) => {
      const prev = window[window.length - 2];
      const last = window[window.length - 1];

      if (
        !prev ||
        !last ||
        last.collectedAt - prev.collectedAt > maxGapMs ||
        now - last.collectedAt > DASHBOARD_PROXY_STALE_MS ||
        last.collectedAt - now > DASHBOARD_PROXY_STALE_MS
      ) {
        return;
      }

      const rates = TrafficStatsHelper.computeRates(prev, last);

      if (rates) list.push(rates);
    });

    return TrafficStatsHelper.sumRates(list);
  }

  static getTrafficSummary(
    windows: Record<string, TrafficSnapshot[]>,
    now = Date.now(),
  ): TrafficSummary {
    const rates = DashboardHelper.getCurrentRates(
      windows,
      DASHBOARD_RATE_MAX_GAP_MS,
      now,
    );
    const latest = Object.values(windows)
      .map((window) => window[window.length - 1])
      .filter((s): s is TrafficSnapshot => s !== undefined);
    const requestsTotal = latest.reduce((n, s) => n + s.requestsTotal, 0);
    const responses = latest.reduce(
      (n, s) =>
        n +
        s.responses2xx +
        s.responses3xx +
        s.responses4xx +
        s.responses5xx +
        s.responsesOther,
      0,
    );
    const total5xx = latest.reduce((n, s) => n + s.responses5xx, 0);

    if (rates && TrafficStatsHelper.responseRate(rates) > 0) {
      return {
        requestsPerSecond: rates.requests,
        requestsTotal,
        errorPercent5xx: TrafficStatsHelper.errorPercent(rates, 'responses5xx'),
        isErrorPercentCurrent: true,
      };
    }

    return {
      requestsPerSecond: rates ? rates.requests : null,
      requestsTotal,
      errorPercent5xx: responses > 0 ? (total5xx / responses) * 100 : 0,
      isErrorPercentCurrent: false,
    };
  }

  /** Everything that needs an operator's attention, errors first. */
  static getAttentionItems(
    sources: DashboardSources,
    now: number,
  ): AttentionItem[] {
    const items: AttentionItem[] = [];

    if (sources.storages && sources.proxies) {
      DashboardHelper.getStorageHealth(sources.storages, sources.proxies, now)
        .filter((row) => row.state === 'unavailable')
        .forEach((row) =>
          items.push({
            id: `storage:${row.name}`,
            kind: 'storageDown',
            severity: 'error',
            params: { name: row.name, failures: row.failures },
            route: RouteName.TUTTI_HEALTH,
          }),
        );
    }

    (sources.events ?? []).forEach((event) =>
      items.push({
        id: `event:${event.user}:${event.bucket ?? ''}`,
        kind: 'failoverEvent',
        severity: 'error',
        params: {
          user: event.user,
          bucket: event.bucket ?? '',
          from: event.from,
          to: event.to,
        },
        route: RouteName.TUTTI_FAILOVER,
      }),
    );

    DashboardHelper.activeReplications(sources.replications ?? []).forEach(
      (replication) => {
        const label = DashboardHelper.replicationLabel(replication);
        const lag = DashboardHelper.eventLag(replication);

        if (replication.isPaused) {
          items.push({
            id: `paused:${replication.idStr}`,
            kind: 'replicationPaused',
            severity: 'warning',
            params: { name: label },
            route: RouteName.CHORUS_REPLICATION,
          });

          return;
        }

        if (replication.isInitDone && lag > 0) {
          items.push({
            id: `behind:${replication.idStr}`,
            kind: 'replicationBehind',
            severity: 'warning',
            params: { name: label, lag },
            route: RouteName.CHORUS_REPLICATION,
          });
        }
      },
    );

    DashboardHelper.getProxyReporting(sources.proxies ?? [], now).stale.forEach(
      (proxy) =>
        items.push({
          id: `proxy:${proxy.appId}`,
          kind: 'proxyStale',
          severity: 'warning',
          params: {
            name: proxy.appId,
            since: proxy.updatedAt ?? '',
          },
          route: RouteName.TUTTI_HEALTH,
        }),
    );

    return items.sort(
      (a, b) => Number(b.severity === 'error') - Number(a.severity === 'error'),
    );
  }
}
