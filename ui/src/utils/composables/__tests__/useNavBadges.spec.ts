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
import { computeNavBadges, NAV_BADGE_MAX_AGE_MS } from '../useNavBadges';
import type { TuttiProxyStatus } from '@/utils/types/tutti';

const NOW = Date.parse('2026-01-01T00:10:00Z');
const proxy = (over: Partial<TuttiProxyStatus> = {}): TuttiProxyStatus => ({
  appId: 'p1',
  updatedAt: new Date(NOW - 1000).toISOString(),
  storages: [{ name: 'main', available: false }],
  ...over,
});
const sources = {
  storages: [
    {
      name: 'main',
      isMain: true,
      address: 'x',
      provider: 'S3',
      credentials: [],
    },
  ],
  replications: [],
  proxies: [proxy()],
  events: [{ user: 'a', from: 'main', to: 'b' }],
} as never;

describe('computeNavBadges', () => {
  it('counts attention items and open failover events', () => {
    expect(computeNavBadges(sources, NOW - 1000, NOW)).toEqual({
      attention: 2,
      failover: 1,
    });
  });

  it('hides everything when data is missing or stale', () => {
    expect(computeNavBadges(sources, null, NOW)).toEqual({});
    expect(
      computeNavBadges(sources, NOW - NAV_BADGE_MAX_AGE_MS - 1, NOW),
    ).toEqual({});
  });

  it('omits zero counts', () => {
    expect(
      computeNavBadges(
        { storages: [], replications: [], proxies: [], events: [] },
        NOW,
        NOW,
      ),
    ).toEqual({});
  });
});
