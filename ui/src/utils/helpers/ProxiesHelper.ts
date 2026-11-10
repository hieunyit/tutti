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
import type { TuttiInt64, TuttiProxyStatus } from '@/utils/types/tutti';

export interface ConfigSyncStatus {
  state: 'unconfigured' | 'applied' | 'applying';
  targetVersion: number;
  appliedVersion: number;
}

export type DriftField =
  | 'failoverEnabled'
  | 'syncEnabled'
  | 'syncQuorum'
  | 'healthThreshold'
  | 'breakerOpenDuration';

export interface DriftGroup {
  /** comparable value as text ("on", "off", "3") */
  value: string;
  proxies: string[];
}

export interface SettingsDrift {
  field: DriftField;
  groups: DriftGroup[];
}

const FIELDS: {
  field: DriftField;
  read: (p: TuttiProxyStatus) => string | null;
}[] = [
  {
    field: 'failoverEnabled',
    read: (p) => bool(p.settings?.failover?.enabled),
  },
  {
    field: 'syncEnabled',
    read: (p) => bool(p.settings?.replication?.sync?.enabled),
  },
  {
    field: 'syncQuorum',
    read: (p) =>
      p.settings?.replication?.sync?.enabled
        ? String(p.settings.replication.sync.writeQuorum ?? 0)
        : null,
  },
  {
    field: 'healthThreshold',
    read: (p) => num(p.settings?.health?.failureThreshold),
  },
  {
    field: 'breakerOpenDuration',
    read: (p) => p.settings?.health?.openDuration ?? null,
  },
];

function bool(value: boolean | undefined): string | null {
  if (value === undefined) return null;

  return value ? 'on' : 'off';
}

function num(value: number | undefined): string | null {
  return value === undefined ? null : String(value);
}

/** gRPC FAILED_PRECONDITION, returned when the fleet config version changed under the editor */
const GRPC_FAILED_PRECONDITION = 9;

export abstract class ProxiesHelper {
  static isFleetConflict(error: unknown): boolean {
    const data = (
      error as { response?: { data?: { code?: number } } } | null | undefined
    )?.response?.data;

    return data?.code === GRPC_FAILED_PRECONDITION;
  }

  /** Fields on which the reporting proxies disagree (needs two or more). */
  static settingsDrift(
    proxies: TuttiProxyStatus[],
    now = Date.now(),
  ): SettingsDrift[] {
    const reporting = proxies.filter((proxy) =>
      DashboardHelper.isProxyReporting(proxy, now),
    );

    if (reporting.length < 2) return [];

    const drift: SettingsDrift[] = [];

    FIELDS.forEach(({ field, read }) => {
      const groups = new Map<string, string[]>();

      reporting.forEach((proxy) => {
        const value = read(proxy);

        if (value === null) return;

        groups.set(value, [...(groups.get(value) ?? []), proxy.appId]);
      });

      if (groups.size > 1) {
        drift.push({
          field,
          groups: [...groups.entries()]
            .map(([value, ids]) => ({ value, proxies: ids }))
            .sort((a, b) => b.proxies.length - a.proxies.length),
        });
      }
    });

    return drift;
  }

  /** Proxies that deviate from the majority on any drifting field. */
  static outliers(drift: SettingsDrift[]): Set<string> {
    const result = new Set<string>();

    drift.forEach(({ groups }) => {
      groups.slice(1).forEach((group) => {
        group.proxies.forEach((id) => result.add(id));
      });
    });

    return result;
  }

  /** "3 d 4 h", "12 min", "45 s"; null when the start time is unknown. */
  static uptime(
    startedAt: string | undefined,
    now = Date.now(),
  ): string | null {
    const started = startedAt ? Date.parse(startedAt) : NaN;

    if (!Number.isFinite(started)) return null;

    return ProxiesHelper.formatDuration(Math.max(0, (now - started) / 1000));
  }

  static formatDuration(totalSeconds: number): string {
    const seconds = Math.floor(totalSeconds);
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);

    if (days > 0) return `${days} d ${hours} h`;

    if (hours > 0) return `${hours} h ${minutes} min`;

    if (minutes > 0) return `${minutes} min`;

    return `${seconds} s`;
  }

  /** Storages of a proxy that are available vs total. */
  static storageAvailability(proxy: TuttiProxyStatus): {
    available: number;
    total: number;
  } {
    const list = proxy.storages ?? [];

    return {
      available: list.filter((s) => s.available !== false).length,
      total: list.length,
    };
  }

  /** Effective proxy group ("default" when unset). */
  static effectiveGroup(proxy: TuttiProxyStatus): string {
    return proxy.group && proxy.group.trim() ? proxy.group.trim() : 'default';
  }

  /** Evaluates whether a proxy has applied the current cluster fleet config version. */
  static configSyncStatus(
    proxy: TuttiProxyStatus,
    clusterConfigVersion?: TuttiInt64,
  ): ConfigSyncStatus {
    const targetVersion = Number(clusterConfigVersion ?? 0) || 0;
    const appliedVersion = Number(proxy.appliedConfigVersion ?? 0) || 0;

    if (targetVersion <= 0 && appliedVersion <= 0) {
      return { state: 'unconfigured', targetVersion: 0, appliedVersion: 0 };
    }

    if (appliedVersion >= targetVersion) {
      return { state: 'applied', targetVersion, appliedVersion };
    }

    return { state: 'applying', targetVersion, appliedVersion };
  }
}
