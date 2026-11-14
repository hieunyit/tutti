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
import type { TuttiReplication, TuttiStorage } from '@/utils/types/tuttiCore';
import type {
  TuttiFailoverEvent,
  TuttiFailoverPolicy,
  TuttiProxyStatus,
  TuttiTrafficRates,
} from '@/utils/types/tutti';
import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';

/** A proxy that has not reported for longer than this needs attention. */
export const DASHBOARD_PROXY_STALE_MS = 60_000;

export type DashboardSeverity = 'error' | 'warning';

export type ReplicationState = 'paused' | 'initial' | 'behind' | 'upToDate';

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
  | 'failoverOff'
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
  storages: TuttiStorage[] | null;
  replications: TuttiReplication[] | null;
  proxies: TuttiProxyStatus[] | null;
  events: TuttiFailoverEvent[] | null;
  /** optional: failover policies (not readable by viewers) */
  policies?: TuttiFailoverPolicy[] | null;
}

export interface TrafficSummary {
  /** requests/s over the last 10 s, all proxies */
  requestsPerSecond: number;
  /** share of 5xx responses in percent (status5xx / requests, last 10 s) */
  errorPercent5xx: number;
  /** p95 latency in ms: the maximum over proxies (an upper bound) */
  p95Ms: number | null;
  /** requests/s by storage name */
  byStorage: Record<string, number>;
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

  static replicationLabel(replication: TuttiReplication): string {
    const { user, fromStorage, toStorage, fromBucket } = replication.id;

    return `${user}: ${fromStorage} → ${toStorage}${fromBucket ? ` (${fromBucket})` : ''}`;
  }

  /** Non-archived replications only. */
  static activeReplications(
    replications: TuttiReplication[],
  ): TuttiReplication[] {
    return replications.filter((replication) => !replication.isArchived);
  }

  static replicationState(replication: TuttiReplication): ReplicationState {
    if (replication.isPaused) return 'paused';

    if (!replication.isInitDone) return 'initial';

    return DashboardHelper.eventLag(replication) > 0 ? 'behind' : 'upToDate';
  }

  /**
   * Active replications for the summary table: paused and lagging ones first
   * (most lag first), then the rest in the given order, at most `limit`.
   */
  static summaryReplications(
    replications: TuttiReplication[],
    limit = 5,
  ): TuttiReplication[] {
    const rank: Record<ReplicationState, number> = {
      paused: 0,
      behind: 1,
      initial: 2,
      upToDate: 3,
    };

    return DashboardHelper.activeReplications(replications)
      .map((replication, index) => ({ replication, index }))
      .sort(
        (a, b) =>
          rank[DashboardHelper.replicationState(a.replication)] -
            rank[DashboardHelper.replicationState(b.replication)] ||
          DashboardHelper.eventLag(b.replication) -
            DashboardHelper.eventLag(a.replication) ||
          a.index - b.index,
      )
      .slice(0, limit)
      .map((entry) => entry.replication);
  }

  static eventLag(replication: TuttiReplication): number {
    return Math.max(0, TrafficStatsHelper.toNumber(replication.eventLag));
  }

  /** Health per storage aggregated over all proxies (any proxy down = down). */
  static getStorageHealth(
    storages: TuttiStorage[],
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
    all: TuttiReplication[],
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

  /** Summary of the proxies' own live rates; null when none are published. */
  static getTrafficSummary(
    totalRates: TuttiTrafficRates | null | undefined,
  ): TrafficSummary | null {
    if (!totalRates) return null;

    const requests = TrafficStatsHelper.toNumber(totalRates.requestsPerSec);
    const e5 = TrafficStatsHelper.toNumber(totalRates.status5xxPerSec);
    const p95 = TrafficStatsHelper.toNumber(totalRates.latencyP95Ms);

    return {
      requestsPerSecond: requests,
      errorPercent5xx: requests > 0 ? Math.min(100, (e5 / requests) * 100) : 0,
      p95Ms: requests > 0 && p95 > 0 ? p95 : null,
      byStorage: totalRates.requestsByStorage ?? {},
    };
  }

  /**
   * Automatic failover is switched off on reporting proxies while enabled
   * failover policies exist: the policies then never take effect.
   */
  static getFailoverOff(
    sources: DashboardSources,
    now: number,
  ): { off: number; total: number } | null {
    const hasPolicies = (sources.policies ?? []).some(
      (policy) => policy.enabled !== false,
    );

    if (!hasPolicies) return null;

    const withSettings = (sources.proxies ?? []).filter(
      (proxy) =>
        DashboardHelper.isProxyReporting(proxy, now) &&
        proxy.settings?.failover?.enabled !== undefined,
    );
    const off = withSettings.filter(
      (proxy) => proxy.settings?.failover?.enabled === false,
    ).length;

    return off > 0 ? { off, total: withSettings.length } : null;
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
            route: RouteName.TUTTI_STORAGES,
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
        route: RouteName.TUTTI_PROTECTION_GROUPS,
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
            route: RouteName.TUTTI_REPLICATION,
          });

          return;
        }

        if (replication.isInitDone && lag > 0) {
          items.push({
            id: `behind:${replication.idStr}`,
            kind: 'replicationBehind',
            severity: 'warning',
            params: { name: label, lag },
            route: RouteName.TUTTI_REPLICATION,
          });
        }
      },
    );

    const failoverOff = DashboardHelper.getFailoverOff(sources, now);

    if (failoverOff) {
      items.push({
        id: 'failover:off',
        kind: 'failoverOff',
        severity: items.some((item) => item.kind === 'storageDown')
          ? 'error'
          : 'warning',
        params: failoverOff,
        route: RouteName.TUTTI_PROTECTION_GROUPS,
      });
    }

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
          route: RouteName.TUTTI_STORAGES,
        }),
    );

    return items.sort(
      (a, b) => Number(b.severity === 'error') - Number(a.severity === 'error'),
    );
  }
}
