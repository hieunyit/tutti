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

import type { RouteLocationRaw } from 'vue-router';

export type StatusVariant =
  | 'success'
  | 'warning'
  | 'danger'
  | 'info'
  | 'neutral';

export type Tone = 'default' | 'success' | 'warning' | 'danger';

/** Empty values are shown as an en dash everywhere. */
export const EMPTY_VALUE = '–';

const SUCCESS = [
  'healthy',
  'available',
  'active',
  'up',
  'ok',
  'enabled',
  'up_to_date',
  'uptodate',
  'in_sync',
  'synced',
  'running',
];
const WARNING = [
  'paused',
  'behind',
  'degraded',
  'lagging',
  'stale',
  'warning',
  'pending',
];
const DANGER = [
  'unavailable',
  'down',
  'failed',
  'error',
  'blocked',
  'critical',
  'revoked',
];
const INFO = ['syncing', 'initial', 'initial_sync', 'in_progress', 'migrating'];

function normalize(status: string): string {
  return status
    .trim()
    .toLowerCase()
    .replace(/[\s-]+/g, '_');
}

/** Maps a domain status word to the shared colour meaning. */
export function mapStatusToVariant(
  status: string | null | undefined,
): StatusVariant {
  if (!status) return 'neutral';

  const key = normalize(status);

  if (SUCCESS.includes(key)) return 'success';

  if (WARNING.includes(key)) return 'warning';

  if (DANGER.includes(key)) return 'danger';

  if (INFO.includes(key)) return 'info';

  return 'neutral';
}

/** Value or an en dash for null, undefined and empty strings. */
export function orDash(value: string | number | null | undefined): string {
  return value === null || value === undefined || value === ''
    ? EMPTY_VALUE
    : String(value);
}

/** Alert role: only danger interrupts assistive technology. */
export function calloutRole(tone: 'danger' | 'warning' | 'info'): string {
  return tone === 'danger' ? 'alert' : 'status';
}

export type LinkTarget = RouteLocationRaw;
