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
import {
  TrafficStatsHelper,
  type TrafficSnapshot,
} from '../TrafficStatsHelper';
import type { TuttiTrafficStats } from '@/utils/types/tutti';

const BOUNDS = [0.01, 0.05, 0.1];

function stats(over: Partial<TuttiTrafficStats> = {}): TuttiTrafficStats {
  return {
    collectedAt: '2026-01-01T00:00:00Z',
    startedAt: '2026-01-01T00:00:00Z',
    requestsTotal: '100',
    requestsByClass: { read: '80', write: '20' },
    requestsByStorage: { main: '100' },
    responses2xx: '90',
    responses4xx: '8',
    responses5xx: '2',
    bytesIn: '1000',
    bytesOut: '5000',
    latency: {
      count: '100',
      sumSeconds: 2,
      boundsSeconds: BOUNDS,
      buckets: ['50', '40', '10', '0'],
    },
    ...over,
  };
}

function snap(over: Partial<TuttiTrafficStats> = {}): TrafficSnapshot {
  return TrafficStatsHelper.normalize(stats(over)) as TrafficSnapshot;
}

describe('TrafficStatsHelper', () => {
  describe('normalize / int64', () => {
    it('parses int64 strings and defaults missing counters to 0', () => {
      const s = snap();

      expect(s.requestsTotal).toBe(100);
      expect(s.byClass).toEqual({ read: 80, write: 20 });
      expect(s.responses3xx).toBe(0);
      expect(s.latencyBuckets).toEqual([50, 40, 10, 0]);
    });

    it('handles numbers, empty and garbage', () => {
      expect(TrafficStatsHelper.toNumber('123456789012')).toBe(123456789012);
      expect(TrafficStatsHelper.toNumber(5)).toBe(5);
      expect(TrafficStatsHelper.toNumber('')).toBe(0);
      expect(TrafficStatsHelper.toNumber('x')).toBe(0);
      expect(TrafficStatsHelper.toNumber(undefined)).toBe(0);
    });

    it('returns null for proxies without stats', () => {
      expect(TrafficStatsHelper.normalize(undefined)).toBeNull();
      expect(TrafficStatsHelper.normalize({})).toBeNull();
    });
  });

  describe('computeRates', () => {
    const prev = snap();
    const cur = snap({
      collectedAt: '2026-01-01T00:00:10Z',
      requestsTotal: '200',
      requestsByClass: { read: '150', write: '50' },
      responses2xx: '185',
      responses4xx: '12',
      responses5xx: '3',
      bytesIn: '3000',
      bytesOut: '25000',
      latency: {
        count: '200',
        sumSeconds: 3,
        boundsSeconds: BOUNDS,
        buckets: ['120', '70', '10', '0'],
      },
    });

    it('computes per-second rates from consecutive snapshots', () => {
      const r = TrafficStatsHelper.computeRates(prev, cur)!;

      expect(r.requests).toBe(10);
      expect(r.byClass).toEqual({ read: 7, write: 3 });
      expect(r.bytesIn).toBe(200);
      expect(r.bytesOut).toBe(2000);
      expect(TrafficStatsHelper.errorPercent(r, 'responses4xx')).toBeCloseTo(
        (0.4 / 10) * 100,
      );
      expect(TrafficStatsHelper.errorPercent(r, 'responses5xx')).toBeCloseTo(
        (0.1 / 10) * 100,
      );
      expect(TrafficStatsHelper.averageLatency(r)).toBeCloseTo(0.01);
    });

    it('returns null when startedAt changes (restart)', () => {
      const restarted = snap({
        collectedAt: '2026-01-01T00:00:10Z',
        startedAt: '2026-01-01T00:00:08Z',
        requestsTotal: '5',
      });

      expect(TrafficStatsHelper.computeRates(prev, restarted)).toBeNull();
    });

    it('treats a decreasing counter as a restart', () => {
      const reset = snap({
        collectedAt: '2026-01-01T00:00:10Z',
        requestsTotal: '3',
      });

      expect(TrafficStatsHelper.computeRates(prev, reset)).toBeNull();
    });

    it('returns null for non-increasing time', () => {
      expect(TrafficStatsHelper.computeRates(prev, prev)).toBeNull();
    });

    it('has no average latency without observations', () => {
      const r = TrafficStatsHelper.computeRates(prev, {
        ...prev,
        collectedAt: prev.collectedAt + 10000,
      })!;

      expect(TrafficStatsHelper.averageLatency(r)).toBeNull();
      expect(TrafficStatsHelper.p95Latency(r)).toBeNull();
      expect(TrafficStatsHelper.errorPercent(r, 'responses5xx')).toBe(0);
    });
  });

  describe('percentileFromBuckets', () => {
    it('interpolates inside the matching bucket', () => {
      // 100 obs: 50 <=10ms, 40 in (10,50], 10 in (50,100]
      expect(
        TrafficStatsHelper.percentileFromBuckets(BOUNDS, [50, 40, 10, 0], 0.5),
      ).toBeCloseTo(0.01);
      // p95 target 95 -> 5 into the third bucket (10 obs): 0.05 + 0.5 * 0.05
      expect(
        TrafficStatsHelper.percentileFromBuckets(BOUNDS, [50, 40, 10, 0], 0.95),
      ).toBeCloseTo(0.075);
    });

    it('uses bucket deltas, not lifetime counts', () => {
      const r = TrafficStatsHelper.computeRates(
        snap(),
        snap({
          collectedAt: '2026-01-01T00:00:10Z',
          requestsTotal: '110',
          latency: {
            count: '110',
            sumSeconds: 2.5,
            boundsSeconds: BOUNDS,
            // only 10 new observations, all in the (50,100] ms bucket
            buckets: ['50', '40', '20', '0'],
          },
        }),
      )!;

      const p95 = TrafficStatsHelper.p95Latency(r)!;

      expect(p95).toBeGreaterThan(0.05);
      expect(p95).toBeLessThanOrEqual(0.1);
    });

    it('reports the highest bound for the +Inf bucket', () => {
      expect(
        TrafficStatsHelper.percentileFromBuckets(BOUNDS, [0, 0, 0, 10], 0.95),
      ).toBe(0.1);
    });

    it('returns null without observations', () => {
      expect(
        TrafficStatsHelper.percentileFromBuckets(BOUNDS, [0, 0, 0, 0], 0.95),
      ).toBeNull();
    });
  });

  describe('appendSnapshot (window)', () => {
    it('ignores duplicates, resets on restart and trims old data', () => {
      const a = snap();
      const b = snap({
        collectedAt: '2026-01-01T00:00:10Z',
        requestsTotal: '120',
      });
      let w = TrafficStatsHelper.appendSnapshot([], a);

      w = TrafficStatsHelper.appendSnapshot(w, b);
      expect(w).toHaveLength(2);
      expect(TrafficStatsHelper.appendSnapshot(w, b)).toBe(w);

      const restarted = snap({
        collectedAt: '2026-01-01T00:00:20Z',
        startedAt: '2026-01-01T00:00:15Z',
        requestsTotal: '1',
      });

      w = TrafficStatsHelper.appendSnapshot(w, restarted);
      expect(w).toEqual([restarted]);
      expect(TrafficStatsHelper.computeSeries(w)).toEqual([]);

      const later = snap({
        collectedAt: '2026-01-01T01:00:00Z',
        startedAt: '2026-01-01T00:00:15Z',
        requestsTotal: '500',
      });

      expect(
        TrafficStatsHelper.appendSnapshot(w, later, 30 * 60 * 1000),
      ).toEqual([later]);
    });
  });

  describe('aggregation', () => {
    it('sums rates and merges histograms across proxies', () => {
      const mk = (requests: number, buckets: number[]) => ({
        ...TrafficStatsHelper.computeRates(
          snap(),
          snap({ collectedAt: '2026-01-01T00:00:10Z' }),
        )!,
        requests,
        latencyBuckets: buckets,
        latencyCount: buckets.reduce((x, y) => x + y, 0),
      });
      const total = TrafficStatsHelper.sumRates([
        mk(5, [1, 0, 0, 0]),
        mk(7, [0, 0, 1, 0]),
      ])!;

      expect(total.requests).toBe(12);
      expect(total.latencyBuckets).toEqual([1, 0, 1, 0]);
    });

    it('buckets slightly offset proxy timestamps together', () => {
      const base = TrafficStatsHelper.computeRates(
        snap(),
        snap({ collectedAt: '2026-01-01T00:00:10Z' }),
      )!;
      const series = TrafficStatsHelper.aggregateSeries([
        [{ ...base, timestamp: 10_000, requests: 1 }],
        [{ ...base, timestamp: 11_500, requests: 2 }],
      ]);

      expect(series).toHaveLength(1);
      expect(series[0]!.requests).toBe(3);
    });
  });

  describe('formatting', () => {
    it('formats bytes per second and latency', () => {
      expect(TrafficStatsHelper.formatBytesPerSecond(512)).toBe('512 B/s');
      expect(TrafficStatsHelper.formatBytesPerSecond(1536)).toBe('1.5 KB/s');
      expect(TrafficStatsHelper.formatLatency(0.0123)).toBe('12 ms');
      expect(TrafficStatsHelper.formatLatency(1.234)).toBe('1.23 s');
      expect(TrafficStatsHelper.formatLatency(null)).toBe('-');
    });
  });
});
