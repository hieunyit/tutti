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
import { DashboardHelper, DASHBOARD_PROXY_STALE_MS } from '../DashboardHelper';
import { TrafficStatsHelper } from '../TrafficStatsHelper';
import type { ChorusReplication, ChorusStorage } from '@/utils/types/chorus';
import type { TuttiProxyStatus, TuttiTrafficStats } from '@/utils/types/tutti';
import { RouteName } from '@/utils/types/router';

const NOW = Date.parse('2026-01-01T00:10:00Z');

const storage = (name: string, isMain = false) =>
  ({ name, isMain, address: `${name}:9000`, provider: 'S3' }) as ChorusStorage;

const replication = (
  over: Partial<ChorusReplication> & { to?: string } = {},
): ChorusReplication =>
  ({
    id: { user: 'u', fromStorage: 'main', toStorage: over.to ?? 'b' },
    idStr: over.idStr ?? 'r1',
    isPaused: false,
    isInitDone: true,
    isArchived: false,
    eventLag: '0',
    ...over,
  }) as ChorusReplication;

const proxy = (over: Partial<TuttiProxyStatus> = {}): TuttiProxyStatus => ({
  appId: 'p1',
  updatedAt: new Date(NOW - 5000).toISOString(),
  storages: [{ name: 'main', available: true }],
  ...over,
});

describe('DashboardHelper', () => {
  describe('getStorageHealth', () => {
    it('marks a storage unavailable when any proxy reports it down', () => {
      const rows = DashboardHelper.getStorageHealth(
        [storage('main'), storage('b'), storage('c')],
        [
          proxy({
            storages: [
              { name: 'main', available: true },
              { name: 'b', available: false, consecutiveFailures: 4 },
            ],
          }),
          proxy({
            appId: 'p2',
            storages: [{ name: 'b', available: true }],
          }),
        ],
        NOW,
      );

      expect(rows.map((r) => r.state)).toEqual([
        'healthy',
        'unavailable',
        'unknown',
      ]);
      expect(rows[1]?.failures).toBe(4);
    });

    it('is unknown for everything without proxy data', () => {
      const rows = DashboardHelper.getStorageHealth(
        [storage('main')],
        null,
        NOW,
      );

      expect(rows[0]?.state).toBe('unknown');
    });

    it('does not present stale proxy observations as current health', () => {
      const rows = DashboardHelper.getStorageHealth(
        [storage('main')],
        [proxy({ updatedAt: new Date(NOW - 120_000).toISOString() })],
        NOW,
      );

      expect(rows[0]?.state).toBe('unknown');
    });
  });

  describe('getReplicationBreakdown', () => {
    it('classifies replications and ignores archived ones', () => {
      const result = DashboardHelper.getReplicationBreakdown([
        replication({ idStr: 'a' }),
        replication({ idStr: 'b', eventLag: '12' }),
        replication({ idStr: 'c', isInitDone: false }),
        replication({ idStr: 'd', isPaused: true, eventLag: '3' }),
        replication({ idStr: 'e', isArchived: true, eventLag: '99' }),
      ]);

      expect(result).toEqual({
        total: 4,
        active: 3,
        paused: 1,
        upToDate: 1,
        initial: 1,
        behind: 1,
        totalLag: 15,
      });
    });

    it('computes percentages safely', () => {
      expect(DashboardHelper.percent(1, 3)).toBe(33);
      expect(DashboardHelper.percent(0, 0)).toBe(0);
    });
  });

  describe('getProxyReporting', () => {
    it('flags proxies silent for more than 60 s', () => {
      const old = new Date(NOW - DASHBOARD_PROXY_STALE_MS - 1000).toISOString();
      const result = DashboardHelper.getProxyReporting(
        [
          proxy(),
          proxy({ appId: 'p2', updatedAt: old }),
          proxy({ appId: 'p3', updatedAt: undefined }),
        ],
        NOW,
      );

      expect(result.total).toBe(3);
      expect(result.reporting).toBe(1);
      expect(result.stale.map((p) => p.appId)).toEqual(['p2', 'p3']);
    });
  });

  describe('getAttentionItems', () => {
    it('is empty when everything is fine', () => {
      expect(
        DashboardHelper.getAttentionItems(
          {
            storages: [storage('main')],
            replications: [replication()],
            proxies: [proxy()],
            events: [],
          },
          NOW,
        ),
      ).toEqual([]);
    });

    it('collects all kinds, errors first, with links', () => {
      const items = DashboardHelper.getAttentionItems(
        {
          storages: [storage('main'), storage('b')],
          replications: [
            replication({ idStr: 'p', isPaused: true }),
            replication({ idStr: 'l', eventLag: '7' }),
          ],
          proxies: [
            proxy({
              storages: [{ name: 'b', available: false }],
            }),
            proxy({
              appId: 'old',
              updatedAt: new Date(NOW - 120_000).toISOString(),
            }),
          ],
          events: [{ user: 'alice', from: 'main', to: 'b' }],
        },
        NOW,
      );

      expect(items.map((i) => i.kind)).toEqual([
        'storageDown',
        'failoverEvent',
        'replicationPaused',
        'replicationBehind',
        'proxyStale',
      ]);
      expect(items.map((i) => i.severity)).toEqual([
        'error',
        'error',
        'warning',
        'warning',
        'warning',
      ]);
      expect(items[1]?.route).toBe(RouteName.TUTTI_FAILOVER);
      expect(items[3]?.params.lag).toBe(7);
    });

    it('tolerates sources that could not be loaded', () => {
      expect(
        DashboardHelper.getAttentionItems(
          { storages: null, replications: null, proxies: null, events: null },
          NOW,
        ),
      ).toEqual([]);
    });
  });

  describe('getTrafficSummary', () => {
    const snapshot = (collectedAt: string, requests: number, e5: number) =>
      TrafficStatsHelper.normalize({
        collectedAt,
        startedAt: '2026-01-01T00:00:00Z',
        requestsTotal: requests,
        responses2xx: requests - e5,
        responses5xx: e5,
      } as TuttiTrafficStats)!;

    it('has no rate with a single snapshot but still reports totals', () => {
      const summary = DashboardHelper.getTrafficSummary(
        {
          p1: [snapshot('2026-01-01T00:01:00Z', 100, 10)],
        },
        Date.parse('2026-01-01T00:01:00Z'),
      );

      expect(summary.requestsPerSecond).toBeNull();
      expect(summary.requestsTotal).toBe(100);
      expect(summary.errorPercent5xx).toBeCloseTo(10);
      expect(summary.isErrorPercentCurrent).toBe(false);
    });

    it('computes the rate from the two newest snapshots', () => {
      const summary = DashboardHelper.getTrafficSummary(
        {
          p1: [
            snapshot('2026-01-01T00:01:00Z', 100, 0),
            snapshot('2026-01-01T00:01:30Z', 400, 30),
          ],
        },
        Date.parse('2026-01-01T00:01:30Z'),
      );

      expect(summary.requestsPerSecond).toBeCloseTo(10);
      expect(summary.errorPercent5xx).toBeCloseTo(10);
      expect(summary.isErrorPercentCurrent).toBe(true);
    });

    it('ignores snapshot pairs that are too far apart', () => {
      const summary = DashboardHelper.getTrafficSummary(
        {
          p1: [
            snapshot('2026-01-01T00:01:00Z', 100, 0),
            snapshot('2026-01-01T00:20:00Z', 400, 0),
          ],
        },
        Date.parse('2026-01-01T00:20:00Z'),
      );

      expect(summary.requestsPerSecond).toBeNull();
    });

    it('expires a formerly current rate after proxy updates stop', () => {
      const summary = DashboardHelper.getTrafficSummary(
        {
          p1: [
            snapshot('2026-01-01T00:01:00Z', 100, 0),
            snapshot('2026-01-01T00:01:30Z', 400, 0),
          ],
        },
        NOW,
      );

      expect(summary.requestsPerSecond).toBeNull();
      expect(summary.requestsTotal).toBe(400);
    });
  });
});
