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
import { ErrorHelper } from '@/utils/helpers/ErrorHelper';
import type { TuttiReplication } from '@/utils/types/tuttiCore';

/** the choices of the periodic rescan, in minutes; 0 = off */
export const RESCAN_MINUTES = [0, 15, 30, 60, 180, 360, 720, 1440] as const;

/** shown as "Recommended" when files may be written past the proxy */
export const RESCAN_SUGGESTED_MINUTES = 60;

const MIN_SECONDS = 15 * 60;
const MAX_SECONDS = 24 * 60 * 60;

export type RescanBlock = 'paused' | 'initial' | 'running';

/** minutes to the request's Duration string; undefined when the rescan is off */
function toDuration(minutes: number): string | undefined {
  return minutes > 0 ? `${minutes * 60}s` : undefined;
}

/** "3600s" to seconds; 0 when missing, off or unreadable */
function parseSeconds(duration: string | undefined | null): number {
  const match = /^(\d+(?:\.\d+)?)s$/.exec(duration ?? '');
  const seconds = match ? Math.round(Number(match[1])) : 0;

  return seconds >= MIN_SECONDS && seconds <= MAX_SECONDS ? seconds : 0;
}

/** seconds as short text: "15 min", "1 h", "90 min" */
function formatSeconds(seconds: number, unitMin: string, unitHour: string) {
  const minutes = Math.round(seconds / 60);

  return minutes % 60 === 0
    ? `${minutes / 60} ${unitHour}`
    : `${minutes} ${unitMin}`;
}

/** why "Sync now" is unavailable, or null when it can start */
function blockedBy(replication: TuttiReplication): RescanBlock | null {
  if (replication.isPaused) return 'paused';

  if (!replication.isInitDone) return 'initial';

  return replication.rescanRunning ? 'running' : null;
}

/** time only for today, else day and time: "06:10" / "9 Nov, 06:10" */
function formatShort(
  value: string | undefined,
  locale: string,
  now = new Date(),
): string {
  const date = value ? new Date(value) : null;

  if (!date || Number.isNaN(date.getTime()) || date.getFullYear() < 2000) {
    return '';
  }

  const tag = locale === 'de' ? 'de-DE' : locale === 'vi' ? 'vi-VN' : 'en-GB';
  const sameDay = date.toDateString() === now.toDateString();

  return new Intl.DateTimeFormat(tag, {
    ...(sameDay ? {} : { day: 'numeric', month: 'short' }),
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  }).format(date);
}

/** the server's readable message, never the raw gRPC code */
function errorText(error: unknown, fallback: string): string {
  if (axios.isAxiosError(error)) {
    // the backend puts the readable reason in the ErrorInfo detail and only a
    // bare code name ("FailedPrecondition") in `message`
    const data = error.response?.data as
      | { message?: unknown; details?: unknown }
      | undefined;
    const reason = Array.isArray(data?.details)
      ? ErrorHelper.getReason(error)
      : '';

    if (reason) return reason;

    const message = data?.message;

    if (typeof message === 'string' && message) {
      const text = ErrorHelper.stripCode(message);

      if (text !== message || /\s/.test(text)) return text;
    }
  }

  return fallback;
}

export const ReplicationRescanHelper = {
  toDuration,
  parseSeconds,
  formatSeconds,
  blockedBy,
  formatShort,
  errorText,
};
