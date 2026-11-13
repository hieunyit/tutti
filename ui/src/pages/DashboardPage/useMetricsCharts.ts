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

import { computed, type Ref } from 'vue';
import { useI18n } from 'vue-i18n';
import i18nMonitoring from '@/components/tutti/monitoring/i18nMonitoring';
import type { TrafficChartLine } from '@/components/tutti/monitoring/TrafficLineChart.vue';
import {
  buildSeries,
  hasValues,
  pickers,
  type RatePoint,
} from '@/utils/helpers/MonitoringHelper';
import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';

/** Line and chart definitions of the Metrics tab. */
export function useMetricsCharts(
  points: Ref<RatePoint[]>,
  bucketMs: Ref<number>,
) {
  const { t } = useI18n({ messages: i18nMonitoring });

  const line = (
    key: string,
    label: string,
    colorVar: string,
    fallbackColor: string,
    dash: number[],
    pick: TrafficChartLine['pick'],
  ): TrafficChartLine => ({ key, label, colorVar, fallbackColor, dash, pick });

  const SOLID: number[] = [];
  const DASHED = [6, 3];
  const DOTTED = [2, 3];
  const DASH_DOT = [8, 3, 2, 3];

  /** read and write always; other classes only when they have traffic */
  const classLines = computed<TrafficChartLine[]>(() => {
    const all = [
      line(
        'read',
        t('seriesRead'),
        '--tt-chart-1',
        '#2952cc',
        SOLID,
        pickers.byClass('read'),
      ),
      line(
        'write',
        t('seriesWrite'),
        '--tt-chart-2',
        '#e8590c',
        DASHED,
        pickers.byClass('write'),
      ),
      line(
        'exists',
        t('seriesExists'),
        '--tt-chart-3',
        '#0e9384',
        DOTTED,
        pickers.byClass('exists'),
      ),
      line(
        'manage',
        t('seriesManage'),
        '--tt-chart-4',
        '#7a5af8',
        DASH_DOT,
        pickers.byClass('manage'),
      ),
      line(
        'other',
        t('seriesOther'),
        '--tt-chart-5',
        '#667085',
        [1, 3],
        pickers.byClass('other'),
      ),
    ];

    return all.filter(
      (l, i) =>
        i < 2 || hasValues(buildSeries(points.value, l.pick, bucketMs.value)),
    );
  });

  const latencyLines = computed<TrafficChartLine[]>(() => [
    line(
      'avg',
      t('seriesAvg'),
      '--tt-chart-1',
      '#2952cc',
      SOLID,
      pickers.latencyAvg,
    ),
    line(
      'p95',
      t('seriesP95'),
      '--tt-chart-2',
      '#e8590c',
      DASHED,
      pickers.latencyP95,
    ),
  ]);

  const throughputLines = computed<TrafficChartLine[]>(() => [
    line(
      'out',
      t('seriesOut'),
      '--tt-chart-1',
      '#2952cc',
      SOLID,
      pickers.bytesOut,
    ),
    line(
      'in',
      t('seriesIn'),
      '--tt-chart-2',
      '#e8590c',
      DASHED,
      pickers.bytesIn,
    ),
  ]);

  const errorLines = computed<TrafficChartLine[]>(() => [
    line(
      '5xx',
      t('series5xx'),
      '--tt-danger',
      '#b42318',
      SOLID,
      pickers.status5xx,
    ),
    line(
      'ff',
      t('seriesFailFast'),
      '--tt-chart-2',
      '#e8590c',
      DASHED,
      pickers.failFast,
    ),
    line(
      '4xx',
      t('series4xx'),
      '--tt-chart-1',
      '#2952cc',
      DOTTED,
      pickers.status4xx,
    ),
    line(
      'denied',
      t('seriesAccessDenied'),
      '--tt-chart-5',
      '#667085',
      DASH_DOT,
      pickers.accessDenied,
    ),
  ]);

  const charts = computed(() => [
    {
      key: 'requests',
      title: t('chartRequestsByClass'),
      lines: classLines.value,
      kind: 'rate' as const,
      format: TrafficStatsHelper.formatRate,
    },
    {
      key: 'latency',
      title: t('chartLatency'),
      lines: latencyLines.value,
      kind: 'latency' as const,
      format: TrafficStatsHelper.formatLatencyMs,
    },
    {
      key: 'throughput',
      title: t('chartThroughput'),
      lines: throughputLines.value,
      kind: 'bytes' as const,
      format: TrafficStatsHelper.formatBytesPerSecond,
    },
    {
      key: 'errors',
      title: t('chartErrors'),
      lines: errorLines.value,
      kind: 'rate' as const,
      format: TrafficStatsHelper.formatRate,
    },
  ]);

  return { charts };
}
