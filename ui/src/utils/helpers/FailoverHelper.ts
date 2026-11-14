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

import axios from 'axios';
import { DashboardHelper } from '@/utils/helpers/DashboardHelper';
import type { TuttiReplication } from '@/utils/types/tuttiCore';
import {
  TuttiPolicyValidationError,
  type TuttiFailoverPolicy,
  type TuttiFailoverPolicyForm,
  type TuttiProxyStatus,
} from '@/utils/types/tutti';

export type FailoverTrigger =
  | { state: 'yes' }
  | { state: 'disabled' }
  | { state: 'no'; reason: 'proxiesOff' | 'replication' };

export type TargetReplicationState = 'none' | 'inSync' | 'lagging' | 'paused';

export interface FailoverProxyOff {
  appId: string;
  address?: string;
  checkInterval?: string;
}

export interface FailoverReadiness {
  /** reporting proxies that publish their failover setting */
  total: number;
  on: number;
  off: FailoverProxyOff[];
  enabledPolicies: number;
  replication: TargetReplicationState;
  /** true when at least one enabled policy exists and a proxy has it off */
  willNotTrigger: boolean;
}

const UNIT_SECONDS: Record<string, number> = { s: 1, m: 60, h: 3600 };

export abstract class FailoverHelper {
  /**
   * Parses "60", "60s", "5m", "1h", "1.5s" into seconds.
   * Returns null for invalid or negative input.
   */
  static parseDurationSeconds(input: string): number | null {
    const match = /^\s*(\d+(?:\.\d+)?)\s*([smh]?)\s*$/.exec(input);

    if (!match) return null;

    return Number(match[1]) * (UNIT_SECONDS[match[2] || 's'] ?? 1);
  }

  /** Formats seconds as protobuf JSON duration string. */
  static toProtoDuration(seconds: number): string {
    return `${seconds}s`;
  }

  /** "60s" -> "60" (for form editing); empty/invalid -> "". */
  static fromProtoDuration(duration: string | undefined): string {
    if (!duration) return '';

    const seconds = FailoverHelper.parseDurationSeconds(duration);

    return seconds === null ? '' : String(seconds);
  }

  /** Storages that can still be added as targets (source excluded, no duplicates). */
  static availableTargets(
    allStorages: string[],
    source: string | null,
    targets: string[],
  ): string[] {
    return allStorages.filter((s) => s !== source && !targets.includes(s));
  }

  static addTarget(
    targets: string[],
    target: string,
    source: string | null,
  ): string[] {
    if (target === source || targets.includes(target)) return targets;

    return [...targets, target];
  }

  static removeTarget(targets: string[], target: string): string[] {
    return targets.filter((item) => item !== target);
  }

  static moveTarget(
    targets: string[],
    index: number,
    direction: -1 | 1,
  ): string[] {
    const newIndex = index + direction;

    if (
      index < 0 ||
      index >= targets.length ||
      newIndex < 0 ||
      newIndex >= targets.length
    ) {
      return targets;
    }

    const result = [...targets];

    const moved = result[index] as string;

    result[index] = result[newIndex] as string;
    result[newIndex] = moved;

    return result;
  }

  /** Mirrors backend validation. */
  static validate(form: TuttiFailoverPolicyForm): TuttiPolicyValidationError[] {
    const errors: TuttiPolicyValidationError[] = [];

    if (!form.source) errors.push(TuttiPolicyValidationError.SOURCE_REQUIRED);

    if (form.enabled && form.targets.length === 0) {
      errors.push(TuttiPolicyValidationError.TARGETS_REQUIRED);
    }

    if (form.source && form.targets.includes(form.source)) {
      errors.push(TuttiPolicyValidationError.TARGET_IS_SOURCE);
    }

    if (new Set(form.targets).size !== form.targets.length) {
      errors.push(TuttiPolicyValidationError.TARGET_DUPLICATE);
    }

    if (
      form.after.trim() !== '' &&
      FailoverHelper.parseDurationSeconds(form.after) === null
    ) {
      errors.push(TuttiPolicyValidationError.AFTER_INVALID);
    }

    if (
      form.maxEventLag !== null &&
      (!Number.isInteger(form.maxEventLag) || form.maxEventLag < 0)
    ) {
      errors.push(TuttiPolicyValidationError.MAX_EVENT_LAG_INVALID);
    }

    return errors;
  }

  static formToPolicy(form: TuttiFailoverPolicyForm): TuttiFailoverPolicy {
    const seconds = FailoverHelper.parseDurationSeconds(form.after);
    const policy: TuttiFailoverPolicy = {
      source: form.source ?? '',
      enabled: form.enabled,
      targets: [...form.targets],
    };

    if (seconds !== null)
      policy.after = FailoverHelper.toProtoDuration(seconds);

    if (form.maxEventLag !== null) policy.maxEventLag = form.maxEventLag;

    return policy;
  }

  static policyToForm(policy: TuttiFailoverPolicy): TuttiFailoverPolicyForm {
    return {
      source: policy.source,
      enabled: policy.enabled ?? false,
      targets: [...(policy.targets ?? [])],
      after: FailoverHelper.fromProtoDuration(policy.after),
      maxEventLag: policy.maxEventLag ?? null,
    };
  }

  private static reportingProxies(
    proxies: TuttiProxyStatus[] | null,
    now: number,
  ): TuttiProxyStatus[] {
    return (proxies ?? []).filter(
      (proxy) =>
        DashboardHelper.isProxyReporting(proxy, now) &&
        proxy.settings?.failover?.enabled !== undefined,
    );
  }

  /** Effective max event lag: the policy value, else the largest proxy default. */
  static effectiveMaxLag(
    policy: TuttiFailoverPolicy,
    proxies: TuttiProxyStatus[] | null,
  ): number {
    if (policy.maxEventLag !== undefined && policy.maxEventLag !== null) {
      return policy.maxEventLag;
    }

    return Math.max(
      0,
      ...(proxies ?? []).map(
        (proxy) => proxy.settings?.failover?.maxEventLag ?? 0,
      ),
    );
  }

  /** Replications source -> target (not archived). */
  private static replicationsTowards(
    source: string,
    target: string,
    replications: TuttiReplication[],
  ): TuttiReplication[] {
    return DashboardHelper.activeReplications(replications).filter(
      (replication) =>
        replication.id.fromStorage === source &&
        replication.id.toStorage === target,
    );
  }

  /** State of the replications from the source towards one target. */
  static targetReplicationState(
    source: string,
    target: string,
    replications: TuttiReplication[],
    maxLag: number,
  ): TargetReplicationState {
    const list = FailoverHelper.replicationsTowards(
      source,
      target,
      replications,
    );

    if (list.length === 0) return 'none';

    if (list.some((replication) => replication.isPaused)) return 'paused';

    if (
      list.some((replication) => DashboardHelper.eventLag(replication) > maxLag)
    ) {
      return 'lagging';
    }

    return 'inSync';
  }

  /**
   * Will this policy actually trigger? No when a reporting proxy has failover
   * off, or when no target is replicated, running and within the lag limit.
   * Unknown replications (null) are not held against the policy.
   */
  static evaluatePolicy(
    policy: TuttiFailoverPolicy,
    proxies: TuttiProxyStatus[] | null,
    replications: TuttiReplication[] | null,
    now = Date.now(),
  ): FailoverTrigger {
    if (policy.enabled === false) return { state: 'disabled' };

    if (
      FailoverHelper.reportingProxies(proxies, now).some(
        (proxy) => proxy.settings?.failover?.enabled === false,
      )
    ) {
      return { state: 'no', reason: 'proxiesOff' };
    }

    if (replications === null) return { state: 'yes' };

    const maxLag = FailoverHelper.effectiveMaxLag(policy, proxies);
    const ready = (policy.targets ?? []).some(
      (target) =>
        FailoverHelper.targetReplicationState(
          policy.source,
          target,
          replications,
          maxLag,
        ) === 'inSync',
    );

    return ready ? { state: 'yes' } : { state: 'no', reason: 'replication' };
  }

  static getReadiness(
    policies: TuttiFailoverPolicy[],
    proxies: TuttiProxyStatus[] | null,
    replications: TuttiReplication[] | null,
    now = Date.now(),
  ): FailoverReadiness {
    const reporting = FailoverHelper.reportingProxies(proxies, now);
    const off = reporting
      .filter((proxy) => proxy.settings?.failover?.enabled === false)
      .map((proxy) => ({
        appId: proxy.appId,
        address: proxy.address,
        checkInterval: proxy.settings?.failover?.checkInterval,
      }));
    const enabled = policies.filter((policy) => policy.enabled !== false);
    let replication: TargetReplicationState = 'none';

    if (replications !== null) {
      const states = enabled.flatMap((policy) => {
        const maxLag = FailoverHelper.effectiveMaxLag(policy, proxies);

        return (policy.targets ?? []).map((target) =>
          FailoverHelper.targetReplicationState(
            policy.source,
            target,
            replications,
            maxLag,
          ),
        );
      });

      if (states.includes('paused')) replication = 'paused';
      else if (states.includes('lagging')) replication = 'lagging';
      else if (states.includes('inSync')) replication = 'inSync';
    }

    return {
      total: reporting.length,
      on: reporting.length - off.length,
      off,
      enabledPolicies: enabled.length,
      replication,
      willNotTrigger: enabled.length > 0 && off.length > 0,
    };
  }

  /** Backend (gRPC gateway) message for InvalidArgument (code 3) errors. */
  static getValidationMessage(error: unknown): string | null {
    if (!axios.isAxiosError(error)) return null;

    const data = error.response?.data as
      | { code?: number; message?: string }
      | undefined;

    if (data?.code === 3 && data.message) return data.message;

    return null;
  }
}
