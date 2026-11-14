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

import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';
import type {
  TuttiWorkerStat,
  TuttiWorkerStatsResponse,
  TuttiWorkerStatsTotals,
} from '@/utils/types/tutti';

const n = TrafficStatsHelper.toNumber;

/** Numeric, defaulted view of the worker-stats response. */
export interface ThroughputTotals {
  inFlight: number;
  concurrency: number;
  objectsPerSec1m: number;
  bytesPerSec1m: number;
  objectsPerSec5m: number;
  bytesPerSec5m: number;
  bufferTotalBytes: number;
  bufferUsedBytes: number;
  bufferWaiters: number;
  copiedObjectsTotal: number;
  copiedBytesTotal: number;
  failedTotal: number;
}

export interface ThroughputWorker extends ThroughputTotals {
  workerId: string;
  startedAt?: string;
  updatedAt?: string;
  bulkConcurrency: number;
  liveConcurrency: number;
  avgCopySeconds1m: number;
  bufferWaitSecondsTotal: number;
}

export function normalizeTotals(
  raw: TuttiWorkerStatsTotals | undefined,
): ThroughputTotals {
  return {
    inFlight: n(raw?.inFlight),
    concurrency: n(raw?.concurrency),
    objectsPerSec1m: n(raw?.objectsPerSec1m),
    bytesPerSec1m: n(raw?.bytesPerSec1m),
    objectsPerSec5m: n(raw?.objectsPerSec5m),
    bytesPerSec5m: n(raw?.bytesPerSec5m),
    bufferTotalBytes: n(raw?.bufferTotalBytes),
    bufferUsedBytes: n(raw?.bufferUsedBytes),
    bufferWaiters: n(raw?.bufferWaiters),
    copiedObjectsTotal: n(raw?.copiedObjectsTotal),
    copiedBytesTotal: n(raw?.copiedBytesTotal),
    failedTotal: n(raw?.failedTotal),
  };
}

export function normalizeWorker(raw: TuttiWorkerStat): ThroughputWorker {
  return {
    ...normalizeTotals(raw),
    workerId: raw.workerId,
    startedAt: raw.startedAt,
    updatedAt: raw.updatedAt,
    bulkConcurrency: n(raw.bulkConcurrency),
    liveConcurrency: n(raw.liveConcurrency),
    avgCopySeconds1m: n(raw.avgCopySeconds1m),
    bufferWaitSecondsTotal: n(raw.bufferWaitSecondsTotal),
  };
}

export function normalizeWorkerStats(raw: TuttiWorkerStatsResponse): {
  workers: ThroughputWorker[];
  totals: ThroughputTotals;
} {
  return {
    workers: (raw.workers ?? []).map(normalizeWorker),
    totals: normalizeTotals(raw.totals),
  };
}

/** Used / total as a whole percent; null when there is no buffer limit. */
export function bufferPercent(totals: {
  bufferTotalBytes: number;
  bufferUsedBytes: number;
}): number | null {
  if (totals.bufferTotalBytes <= 0) return null;

  return Math.round((totals.bufferUsedBytes / totals.bufferTotalBytes) * 100);
}

export function formatBytesPerSec(value: number): string {
  return TrafficStatsHelper.formatBytesPerSecond(value);
}

/** Objects per second with a precision that stays readable for tiny rates. */
export function formatObjectsPerSec(value: number): string {
  const v = Math.max(0, value);

  return v >= 100 ? v.toFixed(0) : v >= 10 ? v.toFixed(1) : v.toFixed(2);
}

export type ThroughputHintId =
  | 'bufferBottleneck'
  | 'workersBusy'
  | 'workersIdle'
  | 'failuresRising';

/** "Low" throughput for the idle hint: under one object per second. */
export const IDLE_OBJECTS_PER_SEC = 1;

/**
 * Plain-language diagnosis of the live numbers. `previousFailedTotal` is the
 * failedTotal of the previous poll (omit on the first one).
 */
export function diagnoseThroughput(
  totals: ThroughputTotals,
  workers: readonly ThroughputWorker[],
  previousFailedTotal?: number | null,
): ThroughputHintId[] {
  const hints: ThroughputHintId[] = [];

  if (workers.length === 0) return hints;

  if (
    totals.bufferWaiters > 0 &&
    totals.bufferTotalBytes > 0 &&
    totals.bufferUsedBytes * 10 >= totals.bufferTotalBytes * 9
  ) {
    hints.push('bufferBottleneck');
  }

  const { concurrency, inFlight } = totals;

  if (concurrency > 0 && inFlight > 0 && inFlight >= concurrency - 1) {
    hints.push('workersBusy');
  } else if (
    concurrency > 0 &&
    inFlight < concurrency / 2 &&
    totals.objectsPerSec1m < IDLE_OBJECTS_PER_SEC
  ) {
    hints.push('workersIdle');
  }

  if (
    previousFailedTotal !== undefined &&
    previousFailedTotal !== null &&
    totals.failedTotal > previousFailedTotal
  ) {
    hints.push('failuresRising');
  }

  return hints;
}
