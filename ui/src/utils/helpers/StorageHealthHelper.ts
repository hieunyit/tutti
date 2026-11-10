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
import type { TuttiProxyStatus } from '@/utils/types/tutti';

export type StorageHealthStatus = 'available' | 'unavailable' | 'unknown';

export interface StorageHealthSummary {
  name: string;
  status: StorageHealthStatus;
  /** reporting proxies that know the storage */
  reporting: number;
  /** reporting proxies that report it unavailable */
  unavailableOn: number;
  /** highest consecutive failure count of the proxies that report it down */
  failures: number;
  /** earliest "since" of the unavailable reports, ISO string */
  since?: string;
  lastError?: string;
  /**
   * Error text of a proxy that still reports the storage available, e.g.
   * "authentication: SignatureDoesNotMatch" (account or clock problem).
   */
  warning?: string;
  /** p95 in ms, only when it can be attributed to this storage alone */
  p95Ms: number | null;
}

export abstract class StorageHealthHelper {
  /**
   * Health of one storage over all reporting proxies: unavailable if any
   * reporting proxy says so, available if all that report it say so.
   */
  static summarize(
    name: string,
    proxies: TuttiProxyStatus[] | null,
    now = Date.now(),
  ): StorageHealthSummary {
    const result: StorageHealthSummary = {
      name,
      status: 'unknown',
      reporting: 0,
      unavailableOn: 0,
      failures: 0,
      p95Ms: null,
    };
    let sinceTime = Infinity;

    (proxies ?? []).forEach((proxy) => {
      if (!DashboardHelper.isProxyReporting(proxy, now)) return;

      const health = (proxy.storages ?? []).find((item) => item.name === name);

      if (!health || health.available === undefined) return;

      result.reporting += 1;

      if (health.available === false) {
        result.unavailableOn += 1;
        result.failures = Math.max(
          result.failures,
          health.consecutiveFailures ?? 0,
        );

        const time = Date.parse(health.lastChange ?? '');

        if (Number.isFinite(time) && time < sinceTime) {
          sinceTime = time;
          result.since = health.lastChange;
        }

        if (health.lastError && !result.lastError) {
          result.lastError = health.lastError;
        }
      }

      if (health.available === true && health.lastError && !result.warning) {
        result.warning = health.lastError;
      }

      const rates = proxy.rates;
      const byStorage = rates?.requestsByStorage ?? {};
      const busy = Object.keys(byStorage).filter(
        (key) => (byStorage[key] ?? 0) > 0,
      );

      if (
        health.available === true &&
        rates?.latencyP95Ms !== undefined &&
        rates.latencyP95Ms > 0 &&
        busy.length === 1 &&
        busy[0] === name
      ) {
        result.p95Ms = Math.max(result.p95Ms ?? 0, rates.latencyP95Ms);
      }
    });

    if (result.reporting > 0) {
      result.status = result.unavailableOn > 0 ? 'unavailable' : 'available';
    }

    return result;
  }

  /** Reporting proxies, for the "health reported by N proxies" footer. */
  static reportingCount(
    proxies: TuttiProxyStatus[] | null,
    now = Date.now(),
  ): number {
    return (proxies ?? []).filter((proxy) =>
      DashboardHelper.isProxyReporting(proxy, now),
    ).length;
  }
}
