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

import type { TuttiInt64 } from '@/utils/types/tutti';

/** Number parsing and formatting shared by the traffic views. */
export abstract class TrafficStatsHelper {
  /** int64 arrive as strings; missing/garbage becomes 0. */
  static toNumber(value: TuttiInt64 | null | undefined): number {
    if (value === undefined || value === null || value === '') return 0;

    const n = typeof value === 'number' ? value : Number(value);

    return Number.isFinite(n) ? n : 0;
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

  /** Milliseconds to "12 ms" / "1.20 s"; null as an en dash. */
  static formatLatencyMs(ms: number | null): string {
    if (ms === null) return '–';

    if (ms >= 1000) return `${(ms / 1000).toFixed(2)} s`;

    return `${ms < 10 ? ms.toFixed(1) : ms.toFixed(0)} ms`;
  }

  static formatRate(value: number): string {
    if (value === 0) return '0';

    return value < 10 ? value.toFixed(2) : value.toFixed(1);
  }
}
