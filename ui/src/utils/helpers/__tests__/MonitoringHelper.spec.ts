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
  appendPoint,
  axisFormatter,
  buildSeries,
  computeDelta,
  firstValue,
  hasValues,
  historyToPoints,
  normalizeRates,
  pickers,
  timeTicks,
  type RatePoint,
} from '../MonitoringHelper';

const T0 = Date.parse('2026-01-01T12:00:00Z');
const point = (offsetS: number, requests = 1, extra = {}): RatePoint => ({
  t: T0 + offsetS * 1000,
  rates: normalizeRates({ requestsPerSec: requests, ...extra }),
});

describe('MonitoringHelper', () => {
  describe('historyToPoints', () => {
    it('maps the response to sorted points with normalised rates', () => {
      const points = historyToPoints({
        points: [
          {
            time: '2026-01-01T12:00:20Z',
            rates: { requestsPerSec: 4, bytesOutPerSec: 2048 },
          },
          { time: '2026-01-01T12:00:10Z', rates: { requestsPerSec: 2 } },
          { time: 'garbage', rates: { requestsPerSec: 9 } },
        ],
      });

      expect(points.map((p) => p.rates.requests)).toEqual([2, 4]);
      expect(points[1]?.rates.bytesOut).toBe(2048);
      expect(points[0]?.rates.status5xx).toBe(0);
    });

    it('handles an empty history', () => {
      expect(historyToPoints(undefined)).toEqual([]);
      expect(historyToPoints({})).toEqual([]);
    });
  });

  describe('buildSeries', () => {
    it('inserts a gap where buckets are missing', () => {
      const series = buildSeries(
        [point(0, 1), point(10, 2), point(60, 3), point(70, 4)],
        pickers.requests,
        10_000,
      );

      expect(series.map((p) => p.y)).toEqual([1, 2, null, 3, 4]);
      // the null sits one bucket after the last point before the gap
      expect(series[2]?.x).toBe(T0 + 20_000);
    });

    it('does not break the line for the usual bucket spacing', () => {
      const series = buildSeries(
        [point(0), point(10), point(24)],
        pickers.requests,
        10_000,
      );

      expect(series.every((p) => p.y !== null)).toBe(true);
    });

    it('leaves latency empty when there were no requests', () => {
      const series = buildSeries(
        [point(0, 0, { latencyAvgMs: 0 }), point(10, 5, { latencyAvgMs: 3 })],
        pickers.latencyAvg,
        10_000,
      );

      expect(series.map((p) => p.y)).toEqual([null, 3]);
    });

    it('reads classes and reports empty series', () => {
      const points = [
        point(0, 3, { requestsByClass: { read: 2, write: 1 } }),
        point(10, 3, { requestsByClass: { read: 3 } }),
      ];

      expect(
        buildSeries(points, pickers.byClass('write'), 10_000).map((p) => p.y),
      ).toEqual([1, 0]);
      expect(
        hasValues(buildSeries(points, pickers.byClass('manage'), 10_000)),
      ).toBe(false);
    });
  });

  describe('computeDelta', () => {
    it('compares the current value with the start of the range', () => {
      expect(computeDelta(112, 100)).toEqual({ percent: 12, direction: 'up' });
      expect(computeDelta(50, 100)).toEqual({
        percent: -50,
        direction: 'down',
      });
      expect(computeDelta(100.2, 100)).toEqual({
        percent: 0,
        direction: 'flat',
      });
    });

    it('handles a zero base and missing values', () => {
      expect(computeDelta(5, 0)).toEqual({ percent: null, direction: 'up' });
      expect(computeDelta(0, 0)).toEqual({ percent: null, direction: 'flat' });
      expect(computeDelta(null, 1)).toBeNull();
      expect(computeDelta(1, null)).toBeNull();
    });

    it('takes the first non-null value as the base', () => {
      const points = [
        point(0, 0, { latencyAvgMs: 9 }),
        point(10, 4, { latencyAvgMs: 6 }),
      ];

      expect(firstValue(points, pickers.latencyAvg)).toBe(6);
      expect(firstValue([], pickers.requests)).toBeNull();
    });
  });

  describe('appendPoint', () => {
    it('appends newer points and ignores older or equal ones', () => {
      const base = [point(0), point(10)];

      expect(appendPoint(base, point(12), 60_000)).toHaveLength(3);
      expect(appendPoint(base, point(10), 60_000)).toBe(base);
      expect(appendPoint(base, point(5), 60_000)).toBe(base);
    });

    it('drops points outside the window and caps the count', () => {
      const base = [point(0), point(30), point(60)];

      expect(
        appendPoint(base, point(100), 60_000).map((p) => p.t - T0),
      ).toEqual([60_000, 100_000]);

      const many = Array.from({ length: 10 }, (_, i) => point(i));

      expect(appendPoint(many, point(20), 1e9, 5)).toHaveLength(5);
    });
  });

  describe('axes', () => {
    it('uses one unit for the whole bytes axis', () => {
      const f = axisFormatter('bytes', 6 * 1024 * 1024);

      expect(f(0)).toBe('0 MB/s');
      expect(f(3 * 1024 * 1024)).toBe('3 MB/s');
      expect(axisFormatter('bytes', 900)(300)).toBe('300 B/s');
    });

    it('formats latency and rates', () => {
      expect(axisFormatter('latency', 20)(10)).toBe('10 ms');
      expect(axisFormatter('latency', 2500)(1500)).toBe('1.5 s');
      expect(axisFormatter('rate', 0.5)(0.25)).toBe('0.25');
      expect(axisFormatter('rate', 120)(60)).toBe('60');
    });

    it('puts time ticks on whole minutes', () => {
      const ticks = timeTicks(T0 + 7_000, T0 + 15 * 60_000 + 7_000);

      expect(ticks.length).toBeGreaterThan(2);
      expect(ticks.length).toBeLessThanOrEqual(7);
      expect(ticks.every((t) => t % 60_000 === 0)).toBe(true);
    });
  });
});
