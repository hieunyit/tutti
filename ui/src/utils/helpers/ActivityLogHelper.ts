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

import { InitProgressHelper } from '@/utils/helpers/InitProgressHelper';
import type {
  ActivityLevel,
  ActivityType,
  ReplicationActivityEvent,
} from '@/utils/types/replicationActivity';

export type ActivityLevelFilter = 'all' | ActivityLevel;

/** Max events kept in memory by the console. */
export const ACTIVITY_MAX_EVENTS = 2000;

/** Max rows of the in-progress table. */
export const ACTIVITY_INFLIGHT_ROW_CAP = 200;

const pad = (n: number, width = 2) => String(n).padStart(width, '0');

/**
 * Formatting, ordering and filtering of replication activity events for the
 * console panel. Pure functions, no Vue.
 */
export abstract class ActivityLogHelper {
  /** "2026-11-15 10:00:01" in UTC, stable for copy/download */
  static formatTime(iso: string): string {
    const d = new Date(iso);

    if (Number.isNaN(d.getTime())) return iso;

    return (
      `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())} ` +
      `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}`
    );
  }

  /** "850 ms", "1.2 s", "2 min 5 s" */
  static formatDuration(ms: number): string {
    if (ms < 1000) return `${Math.round(ms)} ms`;

    const s = ms / 1000;

    if (s < 60) return `${s.toFixed(1)} s`;

    const min = Math.floor(s / 60);

    return `${min} min ${Math.round(s - min * 60)} s`;
  }

  /** One console line: `2026-11-15 10:00:01 DONE    a/b.jpg 12.0 MB 1.2 s 10.0 MB/s` */
  static formatLine(e: ReplicationActivityEvent): string {
    const parts = [
      ActivityLogHelper.formatTime(e.time),
      e.type.toUpperCase().padEnd(7),
      e.versionId ? `${e.key} (version ${e.versionId})` : e.key,
    ];
    const size = InitProgressHelper.toNumber(e.size);
    const duration = InitProgressHelper.toNumber(e.durationMs);
    const speed = InitProgressHelper.toNumber(e.bytesPerSecond);

    if (size > 0) parts.push(InitProgressHelper.formatSize(size));

    if (e.type === 'done' || duration > 0) {
      parts.push(ActivityLogHelper.formatDuration(duration));
    }

    if (speed > 0) parts.push(`${InitProgressHelper.formatSize(speed)}/s`);

    if ((e.attempt ?? 0) > 1) parts.push(`attempt ${e.attempt}`);

    if (e.error) parts.push(`- ${e.error}`);

    return parts.join(' ');
  }

  /** CSS modifier used for the colour of a line */
  static tone(type: ActivityType): 'ok' | 'warn' | 'error' | 'dim' {
    switch (type) {
      case 'done':
        return 'ok';

      case 'retry':
        return 'warn';

      case 'failed':
        return 'error';

      default:
        return 'dim';
    }
  }

  /** numeric compare of "<ms>-<seq>" cursors; unparsable ones sort first */
  static compareCursor(a: string, b: string): number {
    const parse = (c: string): [number, number] => {
      const [ms, seq] = c.split('-');

      return [Number(ms) || 0, Number(seq) || 0];
    };
    const [am, as] = parse(a);
    const [bm, bs] = parse(b);

    return am - bm || as - bs;
  }

  /**
   * Adds `incoming` to `existing`: no duplicates (by cursor), ascending cursor
   * order, at most `max` newest events kept. Returns `existing` itself when
   * nothing changed.
   */
  static mergeEvents(
    existing: ReplicationActivityEvent[],
    incoming: ReplicationActivityEvent[],
    max = ACTIVITY_MAX_EVENTS,
  ): ReplicationActivityEvent[] {
    if (incoming.length === 0) return existing;

    const seen = new Set(existing.map((e) => e.cursor));
    const fresh = incoming.filter((e) => {
      if (seen.has(e.cursor)) return false;

      seen.add(e.cursor);

      return true;
    });

    if (fresh.length === 0) return existing;

    const merged = [...existing, ...fresh];
    const last = existing[existing.length - 1];
    const inOrder =
      !last ||
      ActivityLogHelper.compareCursor(last.cursor, fresh[0]!.cursor) < 0;

    if (!inOrder || fresh.length > 1) {
      merged.sort((a, b) =>
        ActivityLogHelper.compareCursor(a.cursor, b.cursor),
      );
    }

    return merged.length > max ? merged.slice(merged.length - max) : merged;
  }

  static filter(
    events: ReplicationActivityEvent[],
    level: ActivityLevelFilter,
    text: string,
  ): ReplicationActivityEvent[] {
    const needle = text.trim().toLowerCase();

    if (level === 'all' && !needle) return events;

    return events.filter(
      (e) =>
        (level === 'all' || e.level === level) &&
        (!needle ||
          e.key.toLowerCase().includes(needle) ||
          (e.error ?? '').toLowerCase().includes(needle)),
    );
  }

  static toText(events: ReplicationActivityEvent[]): string {
    return events.map((e) => ActivityLogHelper.formatLine(e)).join('\n');
  }

  /** `<name>-activity-20261115-100001.log`, safe for file systems */
  static fileName(label: string, now = new Date()): string {
    const safe =
      label
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-+|-+$/g, '')
        .slice(0, 60) || 'replication';
    const stamp =
      `${now.getUTCFullYear()}${pad(now.getUTCMonth() + 1)}${pad(now.getUTCDate())}-` +
      `${pad(now.getUTCHours())}${pad(now.getUTCMinutes())}${pad(now.getUTCSeconds())}`;

    return `${safe}-activity-${stamp}.log`;
  }
}
