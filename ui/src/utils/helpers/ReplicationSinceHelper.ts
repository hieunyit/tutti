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

export type SinceMode = 'all' | 'recent' | 'date';
export type SinceUnit = 'days' | 'hours';
export type SinceProblem = 'amount' | 'dateMissing' | 'future' | 'tooOld';

export interface SinceChoice {
  mode: SinceMode;
  /** "recent": how many units back */
  amount: number | null;
  unit: SinceUnit;
  /** "date": epoch milliseconds of the picked moment */
  date: number | null;
}

export const DEFAULT_SINCE_AMOUNT = 30;

/** matches the server: nothing earlier than year 2000 */
export const SINCE_MIN_TIME = Date.UTC(2000, 0, 1);

const UNIT_MS: Record<SinceUnit, number> = {
  days: 24 * 60 * 60 * 1000,
  hours: 60 * 60 * 1000,
};

export function defaultSinceChoice(): SinceChoice {
  return {
    mode: 'all',
    amount: DEFAULT_SINCE_AMOUNT,
    unit: 'days',
    date: null,
  };
}

/** The absolute moment the choice stands for, or null (all objects / unusable). */
function momentOf(choice: SinceChoice, now: number): number | null {
  if (choice.mode === 'date') return choice.date;

  if (choice.mode !== 'recent') return null;

  const { amount } = choice;

  if (amount === null || !Number.isInteger(amount) || amount < 1) return null;

  return now - amount * UNIT_MS[choice.unit];
}

/** What is wrong with the choice, or null when it can be sent. */
function getProblem(
  choice: SinceChoice,
  now = Date.now(),
): SinceProblem | null {
  if (choice.mode === 'all') return null;

  if (choice.mode === 'date' && choice.date === null) return 'dateMissing';

  const moment = momentOf(choice, now);

  if (moment === null) return 'amount';

  if (moment > now) return 'future';

  if (moment < SINCE_MIN_TIME) return 'tooOld';

  return null;
}

/**
 * Absolute RFC3339 timestamp (UTC, whole seconds) for the request, or
 * undefined when every object is copied or the choice is invalid. Relative
 * values count back from `now`, so call this when the user presses Create.
 */
function resolve(choice: SinceChoice, now = Date.now()): string | undefined {
  const moment = momentOf(choice, now);

  if (moment === null || getProblem(choice, now)) return undefined;

  return new Date(Math.floor(moment / 1000) * 1000)
    .toISOString()
    .replace(/\.\d{3}Z$/, 'Z');
}

function intlLocale(locale: string): string {
  if (locale === 'de') return 'de-DE';

  if (locale === 'vi') return 'vi-VN';

  return 'en-GB';
}

/** "17 Oct 2026" in the user's locale and time zone. */
function formatDay(value: Date | number | string, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  }).format(new Date(value));
}

/** "17 Oct 2026, 14:03" in the user's locale and time zone, 24h. */
function formatMoment(value: Date | number, locale: string): string {
  return new Intl.DateTimeFormat(intlLocale(locale), {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  }).format(new Date(value));
}

/** The cutoff the server stored, or null when absent, empty or Go zero time. */
function parseStored(since: string | undefined | null): Date | null {
  if (!since) return null;

  const date = new Date(since);

  return Number.isNaN(date.getTime()) || date.getTime() < SINCE_MIN_TIME
    ? null
    : date;
}

export const ReplicationSinceHelper = {
  defaultChoice: defaultSinceChoice,
  getProblem,
  resolve,
  momentOf,
  formatDay,
  formatMoment,
  parseStored,
};
