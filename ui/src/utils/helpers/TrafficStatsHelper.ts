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

import type { TuttiInt64, TuttiTrafficStats } from '@/utils/types/tutti';

/** Rolling window kept in memory per proxy. */
export const TRAFFIC_WINDOW_MS = 30 * 60 * 1000;

/** Normalised cumulative counters (numbers instead of int64 strings). */
export interface TrafficSnapshot {
  /** ms since epoch, taken from stats.collectedAt */
  collectedAt: number;
  /** stats.startedAt as sent; changes when the proxy restarts */
  startedAt: string;
  requestsTotal: number;
  byClass: Record<string, number>;
  byStorage: Record<string, number>;
  responses2xx: number;
  responses3xx: number;
  responses4xx: number;
  responses5xx: number;
  responsesOther: number;
  failFast: number;
  accessDenied: number;
  eventRecordFailed: number;
  syncQuorumReached: number;
  syncBelowQuorum: number;
  syncSkipped: number;
  chunkedVerifyFailed: number;
  bytesIn: number;
  bytesOut: number;
  latencyCount: number;
  latencySum: number;
  latencyBounds: number[];
  latencyBuckets: number[];
}

/** Per-second rates between two snapshots (or sums of such rates). */
export interface TrafficRates {
  /** ms since epoch of the newer snapshot */
  timestamp: number;
  requests: number;
  byClass: Record<string, number>;
  byStorage: Record<string, number>;
  responses2xx: number;
  responses3xx: number;
  responses4xx: number;
  responses5xx: number;
  responsesOther: number;
  failFast: number;
  accessDenied: number;
  eventRecordFailed: number;
  syncQuorumReached: number;
  syncBelowQuorum: number;
  syncSkipped: number;
  chunkedVerifyFailed: number;
  bytesIn: number;
  bytesOut: number;
  /** observations/s */
  latencyCount: number;
  /** seconds of latency per second */
  latencySum: number;
  latencyBounds: number[];
  /** observations/s per bucket (non-cumulative, last = +Inf) */
  latencyBuckets: number[];
}

const SCALAR_KEYS = [
  'requests',
  'responses2xx',
  'responses3xx',
  'responses4xx',
  'responses5xx',
  'responsesOther',
  'failFast',
  'accessDenied',
  'eventRecordFailed',
  'syncQuorumReached',
  'syncBelowQuorum',
  'syncSkipped',
  'chunkedVerifyFailed',
  'bytesIn',
  'bytesOut',
  'latencyCount',
  'latencySum',
] as const;

export abstract class TrafficStatsHelper {
  /** int64 arrive as strings; missing/garbage becomes 0. */
  static toNumber(value: TuttiInt64 | null | undefined): number {
    if (value === undefined || value === null || value === '') return 0;

    const n = typeof value === 'number' ? value : Number(value);

    return Number.isFinite(n) ? n : 0;
  }

  static normalizeMap(
    map: Record<string, TuttiInt64> | undefined,
  ): Record<string, number> {
    return Object.fromEntries(
      Object.entries(map ?? {}).map(([key, v]) => [
        key,
        TrafficStatsHelper.toNumber(v),
      ]),
    );
  }

  /** null if the proxy publishes no (usable) stats. */
  static normalize(
    stats: TuttiTrafficStats | undefined,
  ): TrafficSnapshot | null {
    if (!stats?.collectedAt || !stats.startedAt) return null;

    const collectedAt = Date.parse(stats.collectedAt);

    if (Number.isNaN(collectedAt)) return null;

    const n = TrafficStatsHelper.toNumber;

    return {
      collectedAt,
      startedAt: stats.startedAt,
      requestsTotal: n(stats.requestsTotal),
      byClass: TrafficStatsHelper.normalizeMap(stats.requestsByClass),
      byStorage: TrafficStatsHelper.normalizeMap(stats.requestsByStorage),
      responses2xx: n(stats.responses2xx),
      responses3xx: n(stats.responses3xx),
      responses4xx: n(stats.responses4xx),
      responses5xx: n(stats.responses5xx),
      responsesOther: n(stats.responsesOther),
      failFast: n(stats.failFast),
      accessDenied: n(stats.accessDenied),
      eventRecordFailed: n(stats.eventRecordFailed),
      syncQuorumReached: n(stats.syncQuorumReached),
      syncBelowQuorum: n(stats.syncBelowQuorum),
      syncSkipped: n(stats.syncSkipped),
      chunkedVerifyFailed: n(stats.chunkedVerifyFailed),
      bytesIn: n(stats.bytesIn),
      bytesOut: n(stats.bytesOut),
      latencyCount: n(stats.latency?.count),
      latencySum: stats.latency?.sumSeconds ?? 0,
      latencyBounds: [...(stats.latency?.boundsSeconds ?? [])],
      latencyBuckets: (stats.latency?.buckets ?? []).map(n),
    };
  }

  /** The counters were reset: the proxy restarted between the snapshots. */
  static isRestart(prev: TrafficSnapshot, cur: TrafficSnapshot): boolean {
    return (
      prev.startedAt !== cur.startedAt || cur.requestsTotal < prev.requestsTotal
    );
  }

  /**
   * Appends a snapshot to a proxy's window.
   * - the same snapshot twice (equal collectedAt) is ignored
   * - a changed startedAt (restart) or older snapshot resets the series
   * - snapshots older than the window are dropped
   */
  static appendSnapshot(
    window: TrafficSnapshot[],
    snapshot: TrafficSnapshot,
    windowMs = TRAFFIC_WINDOW_MS,
  ): TrafficSnapshot[] {
    const last = window[window.length - 1];

    if (last && last.collectedAt === snapshot.collectedAt) return window;

    if (
      last &&
      (TrafficStatsHelper.isRestart(last, snapshot) ||
        snapshot.collectedAt < last.collectedAt)
    ) {
      return [snapshot];
    }

    return [...window, snapshot].filter(
      (s) => snapshot.collectedAt - s.collectedAt <= windowMs,
    );
  }

  private static mapRate(
    prev: Record<string, number>,
    cur: Record<string, number>,
    dt: number,
  ): Record<string, number> {
    return Object.fromEntries(
      Object.keys({ ...prev, ...cur }).map((key) => [
        key,
        Math.max(0, (cur[key] ?? 0) - (prev[key] ?? 0)) / dt,
      ]),
    );
  }

  /**
   * Rates between two snapshots of the same proxy run.
   * Returns null on restart, non-increasing time or changed histogram layout.
   */
  static computeRates(
    prev: TrafficSnapshot,
    cur: TrafficSnapshot,
  ): TrafficRates | null {
    const dt = (cur.collectedAt - prev.collectedAt) / 1000;

    if (dt <= 0 || TrafficStatsHelper.isRestart(prev, cur)) return null;

    const sameLayout =
      prev.latencyBuckets.length === cur.latencyBuckets.length &&
      prev.latencyBounds.length === cur.latencyBounds.length;
    const rate = (a: number, b: number) => Math.max(0, b - a) / dt;

    return {
      timestamp: cur.collectedAt,
      requests: rate(prev.requestsTotal, cur.requestsTotal),
      byClass: TrafficStatsHelper.mapRate(prev.byClass, cur.byClass, dt),
      byStorage: TrafficStatsHelper.mapRate(prev.byStorage, cur.byStorage, dt),
      responses2xx: rate(prev.responses2xx, cur.responses2xx),
      responses3xx: rate(prev.responses3xx, cur.responses3xx),
      responses4xx: rate(prev.responses4xx, cur.responses4xx),
      responses5xx: rate(prev.responses5xx, cur.responses5xx),
      responsesOther: rate(prev.responsesOther, cur.responsesOther),
      failFast: rate(prev.failFast, cur.failFast),
      accessDenied: rate(prev.accessDenied, cur.accessDenied),
      eventRecordFailed: rate(prev.eventRecordFailed, cur.eventRecordFailed),
      syncQuorumReached: rate(prev.syncQuorumReached, cur.syncQuorumReached),
      syncBelowQuorum: rate(prev.syncBelowQuorum, cur.syncBelowQuorum),
      syncSkipped: rate(prev.syncSkipped, cur.syncSkipped),
      chunkedVerifyFailed: rate(
        prev.chunkedVerifyFailed,
        cur.chunkedVerifyFailed,
      ),
      bytesIn: rate(prev.bytesIn, cur.bytesIn),
      bytesOut: rate(prev.bytesOut, cur.bytesOut),
      latencyCount: rate(prev.latencyCount, cur.latencyCount),
      latencySum: rate(prev.latencySum, cur.latencySum),
      latencyBounds: cur.latencyBounds,
      latencyBuckets: sameLayout
        ? cur.latencyBuckets.map((c, i) => rate(prev.latencyBuckets[i] ?? 0, c))
        : cur.latencyBuckets.map(() => 0),
    };
  }

  /** Rate points of one proxy window (restart gaps are already excluded). */
  static computeSeries(window: TrafficSnapshot[]): TrafficRates[] {
    const series: TrafficRates[] = [];

    for (let i = 1; i < window.length; i += 1) {
      const rates = TrafficStatsHelper.computeRates(
        window[i - 1] as TrafficSnapshot,
        window[i] as TrafficSnapshot,
      );

      if (rates) series.push(rates);
    }

    return series;
  }

  /** Sums rates of several proxies (all rates are additive). */
  static sumRates(list: TrafficRates[]): TrafficRates | null {
    const first = list[0];

    if (!first) return null;

    const total: TrafficRates = {
      timestamp: Math.max(...list.map((r) => r.timestamp)),
      requests: 0,
      byClass: {},
      byStorage: {},
      responses2xx: 0,
      responses3xx: 0,
      responses4xx: 0,
      responses5xx: 0,
      responsesOther: 0,
      failFast: 0,
      accessDenied: 0,
      eventRecordFailed: 0,
      syncQuorumReached: 0,
      syncBelowQuorum: 0,
      syncSkipped: 0,
      chunkedVerifyFailed: 0,
      bytesIn: 0,
      bytesOut: 0,
      latencyCount: 0,
      latencySum: 0,
      latencyBounds: first.latencyBounds,
      latencyBuckets: first.latencyBuckets.map(() => 0),
    };

    list.forEach((rates) => {
      SCALAR_KEYS.forEach((key) => {
        total[key] += rates[key];
      });
      (['byClass', 'byStorage'] as const).forEach((key) => {
        Object.entries(rates[key]).forEach(([name, value]) => {
          total[key][name] = (total[key][name] ?? 0) + value;
        });
      });

      // histograms can only be merged when they share the same layout
      if (rates.latencyBuckets.length === total.latencyBuckets.length) {
        rates.latencyBuckets.forEach((value, i) => {
          total.latencyBuckets[i] = (total.latencyBuckets[i] ?? 0) + value;
        });
      }
    });

    return total;
  }

  /**
   * Aggregated series over proxies. Proxies publish at slightly different
   * moments, so points are grouped into `bucketMs` slots before summing.
   */
  static aggregateSeries(
    seriesList: TrafficRates[][],
    bucketMs = 10_000,
  ): TrafficRates[] {
    const slots = new Map<number, TrafficRates[]>();

    seriesList.forEach((series) => {
      series.forEach((rates) => {
        const slot = Math.round(rates.timestamp / bucketMs) * bucketMs;

        slots.set(slot, [...(slots.get(slot) ?? []), rates]);
      });
    });

    return [...slots.entries()]
      .sort(([a], [b]) => a - b)
      .map(([slot, list]) => ({
        ...(TrafficStatsHelper.sumRates(list) as TrafficRates),
        timestamp: slot,
      }));
  }

  /** Total responses/s used as denominator for error rates. */
  static responseRate(rates: TrafficRates): number {
    return (
      rates.responses2xx +
      rates.responses3xx +
      rates.responses4xx +
      rates.responses5xx +
      rates.responsesOther
    );
  }

  /** Percentage (0-100) of responses with the given status class. */
  static errorPercent(
    rates: TrafficRates,
    kind: 'responses4xx' | 'responses5xx',
  ): number {
    const total = TrafficStatsHelper.responseRate(rates);

    return total > 0 ? (rates[kind] / total) * 100 : 0;
  }

  /** Average request duration in seconds, null without observations. */
  static averageLatency(rates: TrafficRates): number | null {
    return rates.latencyCount > 0
      ? rates.latencySum / rates.latencyCount
      : null;
  }

  /**
   * Quantile (0..1) from non-cumulative bucket counts/rates with linear
   * interpolation inside the matching bucket. Observations in the +Inf bucket
   * are reported as the highest finite bound (a lower bound of the real value).
   * Returns null without observations.
   */
  static percentileFromBuckets(
    boundsSeconds: number[],
    buckets: number[],
    quantile: number,
  ): number | null {
    const total = buckets.reduce((sum, v) => sum + v, 0);

    if (total <= 0) return null;

    const target = quantile * total;
    let cumulative = 0;

    for (let i = 0; i < buckets.length; i += 1) {
      const count = buckets[i] ?? 0;

      if (count > 0 && cumulative + count >= target) {
        const upper = boundsSeconds[i];

        if (upper === undefined) {
          return boundsSeconds[boundsSeconds.length - 1] ?? null;
        }

        const lower = i === 0 ? 0 : (boundsSeconds[i - 1] ?? 0);

        return lower + ((target - cumulative) / count) * (upper - lower);
      }

      cumulative += count;
    }

    return boundsSeconds[boundsSeconds.length - 1] ?? null;
  }

  static p95Latency(rates: TrafficRates): number | null {
    return TrafficStatsHelper.percentileFromBuckets(
      rates.latencyBounds,
      rates.latencyBuckets,
      0.95,
    );
  }

  /** "1.5 MB/s" style formatting (1024 based). */
  static formatBytesPerSecond(value: number): string {
    const units = ['B/s', 'KB/s', 'MB/s', 'GB/s', 'TB/s'];
    let v = Math.max(0, value);
    let i = 0;

    while (v >= 1024 && i < units.length - 1) {
      v /= 1024;
      i += 1;
    }

    return `${v >= 100 || i === 0 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`;
  }

  static formatBytes(value: number): string {
    return TrafficStatsHelper.formatBytesPerSecond(value).replace('/s', '');
  }

  /** Seconds to "12 ms" / "1.20 s". */
  static formatLatency(seconds: number | null): string {
    if (seconds === null) return '-';

    return seconds < 1
      ? `${(seconds * 1000).toFixed(seconds < 0.01 ? 1 : 0)} ms`
      : `${seconds.toFixed(2)} s`;
  }

  static formatRate(value: number): string {
    if (value === 0) return '0';

    return value < 10 ? value.toFixed(2) : value.toFixed(1);
  }
}
