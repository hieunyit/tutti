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

import type {
  TuttiStatsHistory,
  TuttiStatsRange,
  TuttiTrafficRates,
} from '@/utils/types/tutti';

export const STATS_RANGES: TuttiStatsRange[] = ['5m', '15m', '1h', '6h'];
export const DEFAULT_STATS_RANGE: TuttiStatsRange = '15m';

export const RANGE_MS: Record<TuttiStatsRange, number> = {
  '5m': 5 * 60_000,
  '15m': 15 * 60_000,
  '1h': 60 * 60_000,
  '6h': 6 * 60 * 60_000,
};

/** Bucket size of the history endpoint per range. */
export const RANGE_BUCKET_MS: Record<TuttiStatsRange, number> = {
  '5m': 10_000,
  '15m': 10_000,
  '1h': 30_000,
  '6h': 120_000,
};

/** Upper bound of kept points: live points arrive every 2 s between refetches. */
export const MAX_POINTS = 1000;

/** Normalised rates: every field is a finite number. Latencies in ms. */
export interface Rates {
  requests: number;
  byClass: Record<string, number>;
  byStorage: Record<string, number>;
  bytesIn: number;
  bytesOut: number;
  status4xx: number;
  status5xx: number;
  failFast: number;
  accessDenied: number;
  latencyAvg: number;
  latencyP95: number;
}

export interface RatePoint {
  /** ms since epoch */
  t: number;
  rates: Rates;
}

export interface SeriesPoint {
  x: number;
  y: number | null;
}

export type SeriesPicker = (rates: Rates) => number | null;

function num(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

function numMap(map: Record<string, number> | undefined) {
  return Object.fromEntries(
    Object.entries(map ?? {}).map(([key, value]) => [key, num(value)]),
  );
}

export function normalizeRates(rates: TuttiTrafficRates | undefined): Rates {
  return {
    requests: num(rates?.requestsPerSec),
    byClass: numMap(rates?.requestsByClass),
    byStorage: numMap(rates?.requestsByStorage),
    bytesIn: num(rates?.bytesInPerSec),
    bytesOut: num(rates?.bytesOutPerSec),
    status4xx: num(rates?.status4xxPerSec),
    status5xx: num(rates?.status5xxPerSec),
    failFast: num(rates?.failFastPerSec),
    accessDenied: num(rates?.accessDeniedPerSec),
    latencyAvg: num(rates?.latencyAvgMs),
    latencyP95: num(rates?.latencyP95Ms),
  };
}

/** History response to ascending points; invalid timestamps are dropped. */
export function historyToPoints(
  history: TuttiStatsHistory | undefined,
): RatePoint[] {
  return (history?.points ?? [])
    .map((p) => ({ t: Date.parse(p.time), rates: normalizeRates(p.rates) }))
    .filter((p) => Number.isFinite(p.t))
    .sort((a, b) => a.t - b.t);
}

/**
 * Appends a live point and drops points outside the window. Points that are
 * not newer than the last one are ignored (the proxies publish every 2 s, a
 * poll may see the same document twice).
 */
export function appendPoint(
  points: RatePoint[],
  point: RatePoint,
  windowMs: number,
  maxPoints = MAX_POINTS,
): RatePoint[] {
  const last = points[points.length - 1];

  if (last && point.t <= last.t) return points;

  const kept = [...points, point].filter((p) => point.t - p.t <= windowMs);

  return kept.length > maxPoints ? kept.slice(kept.length - maxPoints) : kept;
}

/** Drops points older than the window ending at `end`. */
export function trimPoints(
  points: RatePoint[],
  end: number,
  windowMs: number,
): RatePoint[] {
  return points.filter((p) => end - p.t <= windowMs);
}

/**
 * Chart series for one value. A gap longer than 1.5 buckets between two points
 * is bridged by a null so the line breaks (no proxy reported); values for
 * which `pick` returns null (e.g. latency without requests) also break it.
 */
export function buildSeries(
  points: RatePoint[],
  pick: SeriesPicker,
  bucketMs: number,
): SeriesPoint[] {
  const out: SeriesPoint[] = [];
  let prev: RatePoint | null = null;

  points.forEach((p) => {
    if (prev && p.t - prev.t > bucketMs * 1.5) {
      out.push({ x: prev.t + bucketMs, y: null });
    }

    out.push({ x: p.t, y: pick(p.rates) });
    prev = p;
  });

  return out;
}

/** True when the series has at least one non-zero value. */
export function hasValues(series: SeriesPoint[]): boolean {
  return series.some((p) => p.y !== null && p.y !== 0);
}

export interface Delta {
  /** relative change in percent; null when the base is zero */
  percent: number | null;
  direction: 'up' | 'down' | 'flat';
}

/** Change of `current` against the `base` value at the start of the range. */
export function computeDelta(
  current: number | null,
  base: number | null,
): Delta | null {
  if (current === null || base === null) return null;

  if (base === 0) {
    return { percent: null, direction: current === 0 ? 'flat' : 'up' };
  }

  const percent = ((current - base) / base) * 100;

  if (Math.abs(percent) < 0.5) return { percent: 0, direction: 'flat' };

  return { percent, direction: percent > 0 ? 'up' : 'down' };
}

/** First value of the series that is not null (start of the range). */
export function firstValue(
  points: RatePoint[],
  pick: SeriesPicker,
): number | null {
  for (const p of points) {
    const value = pick(p.rates);

    if (value !== null) return value;
  }

  return null;
}

export const pickers = {
  requests: (r: Rates) => r.requests,
  bytesIn: (r: Rates) => r.bytesIn,
  bytesOut: (r: Rates) => r.bytesOut,
  /** latency is meaningless without requests */
  latencyAvg: (r: Rates) => (r.requests > 0 ? r.latencyAvg : null),
  latencyP95: (r: Rates) => (r.requests > 0 ? r.latencyP95 : null),
  status4xx: (r: Rates) => r.status4xx,
  status5xx: (r: Rates) => r.status5xx,
  failFast: (r: Rates) => r.failFast,
  accessDenied: (r: Rates) => r.accessDenied,
  byClass:
    (cls: string): SeriesPicker =>
    (r) =>
      r.byClass[cls] ?? 0,
} as const;

export type AxisKind = 'rate' | 'bytes' | 'latency';

const BYTE_UNITS = ['B/s', 'KB/s', 'MB/s', 'GB/s', 'TB/s'];

function trimNumber(value: number, decimals: number): string {
  return Number(value.toFixed(decimals)).toString();
}

/**
 * Tick formatter with one unit for the whole axis, chosen from its maximum
 * (e.g. every tick in MB/s, not a mix of KB/s and MB/s).
 */
export function axisFormatter(
  kind: AxisKind,
  max: number,
): (value: number) => string {
  if (kind === 'bytes') {
    let unit = 0;
    let scaled = max;

    while (scaled >= 1024 && unit < BYTE_UNITS.length - 1) {
      scaled /= 1024;
      unit += 1;
    }

    const divisor = 1024 ** unit;
    const decimals = scaled < 10 ? 1 : 0;

    return (v) => `${trimNumber(v / divisor, decimals)} ${BYTE_UNITS[unit]}`;
  }

  if (kind === 'latency') {
    if (max >= 1000) return (v) => `${trimNumber(v / 1000, 1)} s`;

    return (v) => `${trimNumber(v, max < 10 ? 1 : 0)} ms`;
  }

  return (v) => trimNumber(v, max < 1 ? 2 : max < 10 ? 1 : 0);
}

const TICK_STEPS_MS = [
  60_000,
  2 * 60_000,
  5 * 60_000,
  10 * 60_000,
  15 * 60_000,
  30 * 60_000,
  60 * 60_000,
  2 * 60 * 60_000,
];

/** Round tick positions (whole minutes/hours) for the time axis. */
export function timeTicks(
  start: number,
  end: number,
  targetCount = 6,
): number[] {
  const span = end - start;
  const step =
    TICK_STEPS_MS.find((s) => span / s <= targetCount) ??
    TICK_STEPS_MS[TICK_STEPS_MS.length - 1]!;
  const ticks: number[] = [];

  for (let t = Math.ceil(start / step) * step; t <= end; t += step) {
    ticks.push(t);
  }

  return ticks;
}

/** Sum of cumulative counters over proxies (sync outcomes table). */
export function sumCounter(values: (number | undefined)[]): number {
  return values.reduce<number>((total, v) => total + num(v), 0);
}
