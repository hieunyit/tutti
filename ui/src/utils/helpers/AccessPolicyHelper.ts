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

import { FailoverHelper } from '@/utils/helpers/FailoverHelper';
import {
  TuttiAccessPermission,
  TuttiAccessPolicyValidationError,
  type TuttiAccessKey,
  type TuttiAccessPolicy,
  type TuttiAccessPolicyForm,
  type TuttiProxyAccount,
} from '@/utils/types/tutti';

const ALL_BUCKETS = '*';

export abstract class AccessPolicyHelper {
  static readonly ALL_BUCKETS = ALL_BUCKETS;

  /** null if the glob pattern is acceptable. */
  static validatePattern(
    pattern: string,
  ):
    | TuttiAccessPolicyValidationError.PATTERN_EMPTY
    | TuttiAccessPolicyValidationError.PATTERN_WHITESPACE
    | null {
    if (pattern === '') return TuttiAccessPolicyValidationError.PATTERN_EMPTY;

    if (pattern.trim() === '') {
      return TuttiAccessPolicyValidationError.PATTERN_EMPTY;
    }

    if (/\s/.test(pattern)) {
      return TuttiAccessPolicyValidationError.PATTERN_WHITESPACE;
    }

    return null;
  }

  static validate(
    form: TuttiAccessPolicyForm,
  ): TuttiAccessPolicyValidationError[] {
    const errors = new Set<TuttiAccessPolicyValidationError>();

    if (form.user.trim() === '') {
      errors.add(TuttiAccessPolicyValidationError.USER_REQUIRED);
    }

    if (!Object.values(TuttiAccessPermission).includes(form.permission)) {
      errors.add(TuttiAccessPolicyValidationError.PERMISSION_INVALID);
    }

    form.buckets.forEach((pattern) => {
      const error = AccessPolicyHelper.validatePattern(pattern);

      if (error) errors.add(error);
    });

    if (new Set(form.buckets).size !== form.buckets.length) {
      errors.add(TuttiAccessPolicyValidationError.PATTERN_DUPLICATE);
    }

    return [...errors];
  }

  /** Adds a pattern if not present; returns the (possibly unchanged) list. */
  static addPattern(patterns: string[], pattern: string): string[] {
    const trimmed = pattern.trim();

    return trimmed === '' || patterns.includes(trimmed)
      ? patterns
      : [...patterns, trimmed];
  }

  static removePattern(patterns: string[], pattern: string): string[] {
    return patterns.filter((p) => p !== pattern);
  }

  /** An empty list means all buckets. */
  static isAllBuckets(buckets: string[] | undefined): boolean {
    return !buckets || buckets.length === 0 || buckets.includes(ALL_BUCKETS);
  }

  static formToPolicy(form: TuttiAccessPolicyForm): TuttiAccessPolicy {
    return {
      user: form.user.trim(),
      permission: form.permission,
      buckets: form.buckets.length > 0 ? [...form.buckets] : [ALL_BUCKETS],
      description: form.description.trim(),
    };
  }

  static policyToForm(
    user: string,
    policy?: TuttiAccessPolicy,
  ): TuttiAccessPolicyForm {
    const buckets = policy?.buckets ?? [];

    return {
      user,
      permission:
        (policy?.permission as TuttiAccessPermission | undefined) ??
        TuttiAccessPermission.READ_WRITE,
      buckets:
        buckets.length === 1 && buckets[0] === ALL_BUCKETS ? [] : [...buckets],
      description: policy?.description ?? '',
    };
  }

  /** Zips the parallel accessKeyIds/sources arrays of a proxy account. */
  static getAccessKeys(account: TuttiProxyAccount): TuttiAccessKey[] {
    return (account.accessKeyIds ?? []).map((accessKeyId, index) => ({
      accessKeyId,
      source: account.sources?.[index] ?? '',
    }));
  }

  static getValidationMessage(error: unknown): string | null {
    return FailoverHelper.getValidationMessage(error);
  }
}
