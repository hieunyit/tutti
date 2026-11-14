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

import { describe, expect, it } from 'vitest';
import { ActivityLogHelper as H } from '../ActivityLogHelper';
import type { ReplicationActivityEvent } from '@/utils/types/replicationActivity';

function ev(
  cursor: string,
  patch: Partial<ReplicationActivityEvent> = {},
): ReplicationActivityEvent {
  return {
    cursor,
    time: '2026-11-15T10:00:01Z',
    level: 'info',
    type: 'started',
    key: `k-${cursor}`,
    ...patch,
  };
}

describe('ActivityLogHelper.formatLine', () => {
  it('formats a done event with size, duration and speed', () => {
    const line = H.formatLine(
      ev('1-0', {
        type: 'done',
        key: 'photos/a.jpg',
        size: '12582912',
        durationMs: '1200',
        bytesPerSecond: '10485760',
      }),
    );

    expect(line).toBe(
      '2026-11-15 10:00:01 DONE    photos/a.jpg 12.0 MB 1.2 s 10.0 MB/s',
    );
  });

  it('adds attempt and the error to failed lines and the version to keys', () => {
    const line = H.formatLine(
      ev('2-0', {
        type: 'failed',
        level: 'error',
        key: 'b',
        versionId: 'v1',
        attempt: 3,
        error: 'boom',
      }),
    );

    expect(line).toBe(
      '2026-11-15 10:00:01 FAILED  b (version v1) attempt 3 - boom',
    );
  });

  it('keeps a started line short', () => {
    expect(H.formatLine(ev('3-0'))).toBe('2026-11-15 10:00:01 STARTED k-3-0');
  });

  it('formats durations', () => {
    expect(H.formatDuration(850)).toBe('850 ms');
    expect(H.formatDuration(1200)).toBe('1.2 s');
    expect(H.formatDuration(125000)).toBe('2 min 5 s');
  });

  it('maps types to tones', () => {
    expect(H.tone('done')).toBe('ok');
    expect(H.tone('retry')).toBe('warn');
    expect(H.tone('failed')).toBe('error');
    expect(H.tone('started')).toBe('dim');
    expect(H.tone('skipped')).toBe('dim');
  });
});

describe('ActivityLogHelper.filter / text', () => {
  const events = [
    ev('1-0', { key: 'a/one.bin' }),
    ev('2-0', {
      key: 'b/two.bin',
      level: 'error',
      type: 'failed',
      error: 'Access Denied',
    }),
    ev('3-0', { key: 'a/three.bin', level: 'warn', type: 'retry' }),
  ];

  it('filters by level and by text in key or error', () => {
    expect(H.filter(events, 'all', '')).toBe(events);
    expect(H.filter(events, 'error', '').map((e) => e.cursor)).toEqual(['2-0']);
    expect(H.filter(events, 'all', 'A/').map((e) => e.cursor)).toEqual([
      '1-0',
      '3-0',
    ]);
    expect(H.filter(events, 'all', 'denied').map((e) => e.cursor)).toEqual([
      '2-0',
    ]);
    expect(H.filter(events, 'warn', 'two')).toEqual([]);
  });

  it('joins lines for copy and download', () => {
    expect(H.toText(events).split('\n')).toHaveLength(3);
  });

  it('builds a safe file name', () => {
    expect(
      H.fileName('a → b · all/buckets', new Date('2026-11-15T10:00:01Z')),
    ).toBe('a-b-all-buckets-activity-20261115-100001.log');
    expect(H.fileName('***')).toMatch(/^replication-activity-/);
  });
});

describe('ActivityLogHelper.mergeEvents', () => {
  it('appends new events in cursor order without duplicates', () => {
    const first = [ev('1-0'), ev('1-1')];
    const merged = H.mergeEvents(first, [ev('1-1'), ev('2-0'), ev('1-2')]);

    expect(merged.map((e) => e.cursor)).toEqual(['1-0', '1-1', '1-2', '2-0']);
  });

  it('compares cursors numerically, not as text', () => {
    const merged = H.mergeEvents([ev('9-0')], [ev('10-0')]);

    expect(merged.map((e) => e.cursor)).toEqual(['9-0', '10-0']);
  });

  it('returns the same array when nothing is new', () => {
    const existing = [ev('1-0')];

    expect(H.mergeEvents(existing, [])).toBe(existing);
    expect(H.mergeEvents(existing, [ev('1-0')])).toBe(existing);
  });

  it('keeps only the newest max events', () => {
    const existing = Array.from({ length: 5 }, (_, i) => ev(`${i + 1}-0`));
    const merged = H.mergeEvents(existing, [ev('6-0'), ev('7-0')], 4);

    expect(merged.map((e) => e.cursor)).toEqual(['4-0', '5-0', '6-0', '7-0']);
  });
});
