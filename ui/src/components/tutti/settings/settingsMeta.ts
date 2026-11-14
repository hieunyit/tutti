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

import {
  WORKER_DURATION_FIELDS,
  WORKER_TUNING_KEYS,
} from '@/utils/helpers/SystemSettingsHelper';
import type { TuttiUpdateSystemSettingsRequest } from '@/utils/types/tutti';

export const FEATURE_KEYS = [
  'tagging',
  'acl',
  'lifecycle',
  'policy',
  'preserveAclGrants',
  'versioning',
] as const;
export const WATERMARK_KEYS = [
  'backlogHighWatermark',
  'backlogLowWatermark',
] as const;
export const BYTE_KEYS = ['bandwidthLimitBytes', 'bufferBudgetBytes'] as const;

export const SETTINGS_TABS = [
  'features',
  'bulk',
  'worker',
  'locking',
  'webhook',
] as const;
export type SettingsTab = (typeof SETTINGS_TABS)[number];

/**
 * Fields the worker reads once at startup: a saved change only takes effect
 * after the worker restarts (concurrency, shutdown timeout, intervals, lock
 * overlap).
 */
export const RESTART_FIELDS: ReadonlySet<string> = new Set([
  'concurrency',
  'bulkConcurrencyPercent',
  ...WORKER_DURATION_FIELDS,
  'lockOverlap',
]);

/** Which tab holds the (invalid) field `field`. */
export function tabForField(field: string): SettingsTab {
  if (field === 'lockOverlap') return 'locking';

  if (field === 'webhookBaseUrl') return 'webhook';

  return [...WATERMARK_KEYS, ...BYTE_KEYS].some((key) => key === field)
    ? 'bulk'
    : 'worker';
}

export interface SettingsChange {
  /** i18n key of the field label */
  key: string;
  from: string;
  to: string;
  restart: boolean;
}

type Flat = Record<string, string>;

function flatten(s: TuttiUpdateSystemSettingsRequest | null): Flat {
  const flat: Flat = {};

  if (!s) return flat;

  for (const key of FEATURE_KEYS) flat[key] = String(s.features?.[key]);

  for (const key of ['concurrency', ...WATERMARK_KEYS, ...BYTE_KEYS] as const)
    flat[key] = String(s.worker?.[key]);

  for (const key of WORKER_TUNING_KEYS)
    flat[key] = String(s.worker?.[key] ?? 0);

  for (const key of WORKER_DURATION_FIELDS) flat[key] = String(s.worker?.[key]);

  flat.lockOverlap = String(s.lock?.overlap);
  flat.webhookEnabled = String(s.webhook?.enabled);
  flat.webhookBaseUrl = (s.webhook?.baseUrl ?? '').trim();

  return flat;
}

/** Field-by-field difference between the saved settings and the draft. */
export function diffSettings(
  saved: TuttiUpdateSystemSettingsRequest | null,
  draft: TuttiUpdateSystemSettingsRequest,
): SettingsChange[] {
  const before = flatten(saved);
  const after = flatten(draft);

  return Object.keys(after)
    .filter((key) => before[key] !== after[key])
    .map((key) => ({
      key,
      from: before[key] ?? '',
      to: after[key]!,
      restart: RESTART_FIELDS.has(key),
    }));
}
