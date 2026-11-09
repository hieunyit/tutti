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
import type {
  TuttiAccessKey,
  TuttiDynamicKey,
  TuttiProxyAccount,
} from '@/utils/types/tutti';

export const GRPC_CODE_FAILED_PRECONDITION = 9;

export interface TuttiGraceOption {
  /** i18n key suffix */
  id: 'immediate' | 'hour' | 'day' | 'week';
  /** protobuf duration string */
  value: string;
}

/** One row of the nested keys table of a proxy account. */
export interface ProxyAccountKeyRow {
  accessKeyId: string;
  /** Config source when unambiguous, otherwise "config"; "dynamic" for managed keys. */
  source: string;
  /** set for dynamic keys only; static keys are managed in the config */
  dynamicKey?: TuttiDynamicKey;
}

export abstract class ProxyAccountHelper {
  static readonly DEFAULT_GRACE_ID: TuttiGraceOption['id'] = 'hour';

  static readonly GRACE_OPTIONS: TuttiGraceOption[] = [
    { id: 'immediate', value: '0s' },
    { id: 'hour', value: '3600s' },
    { id: 'day', value: '86400s' },
    { id: 'week', value: '604800s' },
  ];

  /** protobuf duration string for a grace option id; falls back to 1 hour */
  static getGraceValue(id: string): string {
    return (
      ProxyAccountHelper.GRACE_OPTIONS.find((o) => o.id === id)?.value ??
      '3600s'
    );
  }

  /** gRPC-gateway error code (e.g. 9 = FailedPrecondition) or null */
  static getErrorCode(error: unknown): number | null {
    if (!axios.isAxiosError(error)) return null;

    const code = (error.response?.data as { code?: number } | undefined)?.code;

    return typeof code === 'number' ? code : null;
  }

  /** true when dynamic credentials / master password are not configured */
  static isNotConfigured(error: unknown): boolean {
    return (
      ProxyAccountHelper.getErrorCode(error) === GRPC_CODE_FAILED_PRECONDITION
    );
  }

  /** backend error message, if any */
  static getMessage(error: unknown): string | null {
    if (!axios.isAxiosError(error)) return null;

    const message = (error.response?.data as { message?: string } | undefined)
      ?.message;

    return message ? message : null;
  }

  /** Static (config) keys only: dynamic ones are rendered separately. */
  static getStaticKeys(account: TuttiProxyAccount): TuttiAccessKey[] {
    const dynamic = new Set(
      (account.dynamicKeys ?? []).map((k) => k.accessKeyId),
    );
    // The API returns independently sorted sets, not parallel arrays per key.
    const sources = (account.sources ?? []).filter(
      (source) => source !== 'dynamic',
    );
    const source = sources.length === 1 ? (sources[0] ?? 'config') : 'config';

    return (account.accessKeyIds ?? [])
      .filter((accessKeyId) => !dynamic.has(accessKeyId))
      .map((accessKeyId) => ({
        accessKeyId,
        source,
      }));
  }

  static isDisabled(key: TuttiDynamicKey): boolean {
    return key.status === 'disabled';
  }

  /** true if the date string is a real, non-zero timestamp */
  static hasTime(value?: string): boolean {
    if (!value) return false;

    const time = Date.parse(value);

    return !Number.isNaN(time) && time > Date.UTC(1971, 0, 1);
  }

  /** Static (config) keys first, then the dynamic ones, as one list. */
  static getKeyRows(account: TuttiProxyAccount): ProxyAccountKeyRow[] {
    return [
      ...ProxyAccountHelper.getStaticKeys(account).map((key) => ({
        accessKeyId: key.accessKeyId,
        source: key.source,
      })),
      ...(account.dynamicKeys ?? []).map((dynamicKey) => ({
        accessKeyId: dynamicKey.accessKeyId,
        source: 'dynamic',
        dynamicKey,
      })),
    ];
  }
}
