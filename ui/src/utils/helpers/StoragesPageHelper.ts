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

import { DashboardHelper } from '@/utils/helpers/DashboardHelper';
import { FailoverHelper } from '@/utils/helpers/FailoverHelper';
import {
  StorageHealthHelper,
  type StorageHealthSummary,
} from '@/utils/helpers/StorageHealthHelper';
import type { ChorusReplication, ChorusStorage } from '@/utils/types/chorus';
import type {
  TuttiFailoverEvent,
  TuttiFailoverPolicy,
  TuttiProxyStatus,
} from '@/utils/types/tutti';
import type { S3User } from '@/utils/types/users';

export interface StoragesSources {
  storages: ChorusStorage[];
  proxies: TuttiProxyStatus[] | null;
  users: S3User[];
  replications: ChorusReplication[];
  policies: TuttiFailoverPolicy[];
  events: TuttiFailoverEvent[];
}

export type BreakerState = 'open' | 'closed' | 'unknown';

export interface StorageRow {
  name: string;
  isMain: boolean;
  address: string;
  provider: string;
  health: StorageHealthSummary;
  breaker: {
    state: BreakerState;
    failures: number;
    /** seconds until the next trial request, only while the breaker is open */
    retryInSeconds: number | null;
  };
  requestsPerSecond: number;
  /** 5xx share in percent, only when this storage carries all the traffic */
  errorPercent: number | null;
  users: { count: number; missing: string[] };
  replication: {
    sourceOf: number;
    targetOf: number;
    lag: number;
    paused: number;
  };
  failover: { target: string; afterSeconds: number | null } | null;
  openFailoverEvents: number;
}

export interface StoragesSummary {
  total: number;
  available: number;
  /** worst storage: the first unavailable one */
  worst: StorageRow | null;
  requestsPerSecond: number;
  usersTotal: number;
  usersComplete: number;
  /** first incomplete user and the storages it misses */
  firstIncomplete: { name: string; missing: string[] } | null;
  proxiesTotal: number;
  proxiesReporting: number;
}

export abstract class StoragesPageHelper {
  static rows(sources: StoragesSources, now = Date.now()): StorageRow[] {
    const { storages, proxies, users, replications, policies, events } =
      sources;
    const active = DashboardHelper.activeReplications(replications);
    const reporting = (proxies ?? []).filter((proxy) =>
      DashboardHelper.isProxyReporting(proxy, now),
    );

    return storages.map((storage) => {
      const name = storage.name;
      const health = StorageHealthHelper.summarize(name, proxies, now);
      const policy = policies.find((item) => item.source === name);
      const openDuration = StoragesPageHelper.openDurationSeconds(reporting);
      const requestsPerSecond = reporting.reduce(
        (sum, proxy) => sum + (proxy.rates?.requestsByStorage?.[name] ?? 0),
        0,
      );
      const bySource = active.filter((r) => r.id.fromStorage === name);
      const byTarget = active.filter((r) => r.id.toStorage === name);
      const configured = users.filter((user) =>
        (user.storages ?? []).some((s) => s.storage === name && s.configured),
      );
      const missing = users.filter((user) =>
        (user.storages ?? []).some((s) => s.storage === name && !s.configured),
      );
      let retry: number | null = null;

      if (health.status === 'unavailable' && health.since && openDuration) {
        const elapsed = (now - Date.parse(health.since)) / 1000;

        retry =
          elapsed < openDuration ? Math.ceil(openDuration - elapsed) : null;
      }

      return {
        name,
        isMain: storage.isMain,
        address: storage.address,
        provider: String(storage.provider),
        health,
        breaker: {
          state:
            health.status === 'unknown'
              ? 'unknown'
              : health.status === 'unavailable'
                ? 'open'
                : 'closed',
          failures: health.failures,
          retryInSeconds: retry,
        },
        requestsPerSecond,
        errorPercent: StoragesPageHelper.errorPercent(name, reporting),
        users: {
          count: configured.length,
          missing: missing.map((u) => u.name),
        },
        replication: {
          sourceOf: bySource.length,
          targetOf: byTarget.length,
          lag: byTarget.reduce(
            (sum, r) => sum + DashboardHelper.eventLag(r),
            0,
          ),
          paused: bySource.filter((r) => r.isPaused).length,
        },
        failover:
          policy?.enabled && (policy.targets ?? []).length > 0
            ? {
                target: policy.targets![0]!,
                afterSeconds: FailoverHelper.parseDurationSeconds(
                  policy.after ?? '',
                ),
              }
            : null,
        openFailoverEvents: events.filter((event) => event.from === name)
          .length,
      };
    });
  }

  /** Breaker open duration (seconds) from the reporting proxies' settings. */
  private static openDurationSeconds(
    reporting: TuttiProxyStatus[],
  ): number | null {
    for (const proxy of reporting) {
      const value = proxy.settings?.health?.openDuration;
      const seconds = value ? FailoverHelper.parseDurationSeconds(value) : null;

      if (seconds) return seconds;
    }

    return null;
  }

  /**
   * Per-storage error rates are not published; the total 5xx share is
   * attributable only when this storage carries all current traffic.
   */
  private static errorPercent(
    name: string,
    reporting: TuttiProxyStatus[],
  ): number | null {
    let requests = 0;
    let errors = 0;
    let other = 0;

    reporting.forEach((proxy) => {
      const rates = proxy.rates;

      if (!rates) return;

      const mine = rates.requestsByStorage?.[name] ?? 0;
      const total = Object.values(rates.requestsByStorage ?? {}).reduce(
        (a, b) => a + b,
        0,
      );

      requests += mine;
      other += total - mine;
      errors += rates.status5xxPerSec ?? 0;
    });

    if (requests <= 0 || other > 0) return null;

    return Math.min(100, (errors / requests) * 100);
  }

  static summary(
    rows: StorageRow[],
    sources: Pick<StoragesSources, 'users' | 'proxies'>,
    now = Date.now(),
  ): StoragesSummary {
    const incomplete = sources.users.find((user) => user.complete === false);
    const proxies = sources.proxies ?? [];

    return {
      total: rows.length,
      available: rows.filter((row) => row.health.status === 'available').length,
      worst: rows.find((row) => row.health.status === 'unavailable') ?? null,
      requestsPerSecond: rows.reduce(
        (sum, row) => sum + row.requestsPerSecond,
        0,
      ),
      usersTotal: sources.users.length,
      usersComplete: sources.users.filter((user) => user.complete !== false)
        .length,
      firstIncomplete: incomplete
        ? {
            name: incomplete.name,
            missing: (incomplete.storages ?? [])
              .filter((s) => !s.configured)
              .map((s) => s.storage),
          }
        : null,
      proxiesTotal: proxies.length,
      proxiesReporting: StorageHealthHelper.reportingCount(proxies, now),
    };
  }
}
