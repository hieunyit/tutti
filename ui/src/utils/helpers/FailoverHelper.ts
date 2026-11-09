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
import {
  TuttiPolicyValidationError,
  type TuttiFailoverPolicy,
  type TuttiFailoverPolicyForm,
} from '@/utils/types/tutti';

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
