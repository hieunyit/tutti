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
import { StorageHealthHelper } from '../StorageHealthHelper';
import type { TuttiProxyStatus } from '@/utils/types/tutti';

const NOW = Date.parse('2026-11-10T10:00:00Z');
const fresh = '2026-11-10T09:59:50Z';

function proxy(
  appId: string,
  available: boolean | undefined,
  over: Partial<TuttiProxyStatus> = {},
  failures = 0,
  lastChange = '2026-11-10T09:00:00Z',
): TuttiProxyStatus {
  return {
    appId,
    updatedAt: fresh,
    storages: [
      {
        name: 'main',
        available,
        consecutiveFailures: failures,
        lastChange,
        lastError: available === false ? 'refused' : '',
      },
    ],
    ...over,
  };
}

describe('StorageHealthHelper.summarize', () => {
  it('is unknown without reporting proxies', () => {
    expect(StorageHealthHelper.summarize('main', [], NOW).status).toBe(
      'unknown',
    );
    expect(StorageHealthHelper.summarize('main', null, NOW).status).toBe(
      'unknown',
    );
  });

  it('is available when every reporting proxy says so', () => {
    const s = StorageHealthHelper.summarize(
      'main',
      [proxy('a', true), proxy('b', true)],
      NOW,
    );

    expect(s.status).toBe('available');
    expect(s.reporting).toBe(2);
    expect(s.unavailableOn).toBe(0);
  });

  it('is unavailable when any proxy says so and counts them', () => {
    const s = StorageHealthHelper.summarize(
      'main',
      [
        proxy('a', true),
        proxy('b', false, {}, 14, '2026-11-10T09:30:00Z'),
        proxy('c', false, {}, 3, '2026-11-10T09:40:00Z'),
      ],
      NOW,
    );

    expect(s.status).toBe('unavailable');
    expect(s.unavailableOn).toBe(2);
    expect(s.reporting).toBe(3);
    expect(s.failures).toBe(14);
    expect(s.since).toBe('2026-11-10T09:30:00Z');
    expect(s.lastError).toBe('refused');
  });

  it('ignores stale proxies', () => {
    const stale = proxy('s', false, { updatedAt: '2026-11-10T09:00:00Z' });
    const s = StorageHealthHelper.summarize(
      'main',
      [proxy('a', true), stale],
      NOW,
    );

    expect(s.status).toBe('available');
    expect(s.reporting).toBe(1);
  });

  it('attributes p95 only when the storage is the only one with traffic', () => {
    const rates = (byStorage: Record<string, number>) => ({
      latencyP95Ms: 9.6,
      requestsByStorage: byStorage,
    });

    expect(
      StorageHealthHelper.summarize(
        'main',
        [proxy('a', true, { rates: rates({ main: 4 }) })],
        NOW,
      ).p95Ms,
    ).toBe(9.6);
    expect(
      StorageHealthHelper.summarize(
        'main',
        [proxy('a', true, { rates: rates({ main: 4, other: 1 }) })],
        NOW,
      ).p95Ms,
    ).toBeNull();
    expect(
      StorageHealthHelper.summarize(
        'main',
        [proxy('a', false, { rates: rates({ main: 4 }) })],
        NOW,
      ).p95Ms,
    ).toBeNull();
  });

  it('counts reporting proxies', () => {
    expect(
      StorageHealthHelper.reportingCount(
        [proxy('a', true), proxy('b', true, { updatedAt: undefined })],
        NOW,
      ),
    ).toBe(1);
  });
});
