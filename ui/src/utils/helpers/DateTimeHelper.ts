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

import { i18n } from '@/i18n';

export type DateTimeInput = string | number | Date | null | undefined;

/** shown for missing, zero (Go zero time) or unparsable values */
export const EMPTY_DATE_TEXT = '–';

const MIN_VALID_TIME = Date.UTC(1971, 0, 1);

function intlLocale(locale?: string): string {
  const current = locale ?? String(i18n.global.locale.value);

  if (current === 'de') return 'de-DE';

  if (current === 'vi') return 'vi-VN';

  return 'en-GB';
}

/** Parses the input; returns null for null/empty/zero/invalid values. */
export function parseDateTime(value: DateTimeInput): Date | null {
  if (value === null || value === undefined || value === '') {
    return null;
  }

  const date = value instanceof Date ? value : new Date(value);
  const time = date.getTime();

  if (Number.isNaN(time) || time < MIN_VALID_TIME) {
    return null;
  }

  return date;
}

/** "9 Nov 2026, 13:15:37" (EN) / "9. Nov. 2026, 13:15:37" (DE), 24h. */
export function formatDateTime(value: DateTimeInput, locale?: string): string {
  const date = parseDateTime(value);

  if (!date) return EMPTY_DATE_TEXT;

  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  }).format(date);
}

/** "13:15:37", 24h. */
export function formatTime(value: DateTimeInput, locale?: string): string {
  const date = parseDateTime(value);

  if (!date) return EMPTY_DATE_TEXT;

  return new Intl.DateTimeFormat(intlLocale(locale), {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  }).format(date);
}

/** "13:15", 24h (chart axis labels). */
export function formatClock(value: DateTimeInput, locale?: string): string {
  const date = parseDateTime(value);

  if (!date) return EMPTY_DATE_TEXT;

  return new Intl.DateTimeFormat(intlLocale(locale), {
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  }).format(date);
}

const RELATIVE_UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
  ['second', 1],
];

/** "3 minutes ago" / "in 2 hours"; `now` is injectable for tests. */
export function formatRelative(
  value: DateTimeInput,
  locale?: string,
  now: number = Date.now(),
): string {
  const date = parseDateTime(value);

  if (!date) return EMPTY_DATE_TEXT;

  const diffSeconds = Math.round((date.getTime() - now) / 1000);
  const absSeconds = Math.abs(diffSeconds);
  const formatter = new Intl.RelativeTimeFormat(intlLocale(locale), {
    numeric: 'auto',
  });

  const [unit, size] =
    RELATIVE_UNITS.find(([, unitSize]) => absSeconds >= unitSize) ??
    RELATIVE_UNITS[RELATIVE_UNITS.length - 1]!;

  return formatter.format(Math.trunc(diffSeconds / size), unit);
}
