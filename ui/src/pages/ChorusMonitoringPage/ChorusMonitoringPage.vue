<!--
  - Copyright © 2026 hieunyit
  -
  -  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
  -  you may not use this file except in compliance with the License.
  -  You may obtain a copy of the License at
  -
  -  https://www.gnu.org/licenses/agpl-3.0.html
  -
  -  Unless required by applicable law or agreed to in writing, software
  -  distributed under the License is distributed on an "AS IS" BASIS,
  -  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  -  See the License for the specific language governing permissions and
  -  limitations under the License.
  -->

<script setup lang="ts">
  import { CButton, CSkeleton } from '@clyso/clyso-ui-kit';
  import { storeToRefs } from 'pinia';
  import { computed, onBeforeMount, onBeforeUnmount } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nMonitoring from '@/components/tutti/monitoring/i18nMonitoring';
  import MonitoringBreakdown from '@/components/tutti/monitoring/MonitoringBreakdown.vue';
  import MonitoringKpiTiles from '@/components/tutti/monitoring/MonitoringKpiTiles.vue';
  import MonitoringReplication from '@/components/tutti/monitoring/MonitoringReplication.vue';
  import TrafficLineChart, {
    type TrafficChartLine,
  } from '@/components/tutti/monitoring/TrafficLineChart.vue';
  import ChorusListError from '@/components/chorus/common/ChorusListError/ChorusListError.vue';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import EmptyState from '@/components/ui/EmptyState.vue';
  import { useTuttiMonitoringStore } from '@/stores/tuttiMonitoringStore';
  import { useNow } from '@/utils/composables/useNow';
  import { formatRelative } from '@/utils/helpers/DateTimeHelper';
  import {
    RANGE_BUCKET_MS,
    RANGE_MS,
    STATS_RANGES,
    buildSeries,
    hasValues,
    pickers,
  } from '@/utils/helpers/MonitoringHelper';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';

  const { t } = useI18n({ messages: i18nMonitoring });
  const store = useTuttiMonitoringStore();
  const {
    isLoading,
    isPaused,
    lastUpdated,
    hasError,
    historyError,
    range,
    windowEnd,
    points,
    proxies,
    totalRates,
  } = storeToRefs(store);

  const now = useNow(1000);

  onBeforeMount(() => store.start());
  onBeforeUnmount(() => store.$reset());

  const showError = computed(
    () => historyError.value && points.value.length === 0,
  );

  const liveState = computed<'live' | 'paused' | 'failing'>(() => {
    if (isPaused.value) return 'paused';

    return hasError.value ? 'failing' : 'live';
  });

  const liveText = computed(() => {
    if (liveState.value === 'paused') return t('pausedStatus');

    if (liveState.value === 'live') return t('liveStatus');

    return lastUpdated.value === null
      ? t('failingStatusNever')
      : t('failingStatus', {
          time: formatRelative(lastUpdated.value, undefined, now.value),
        });
  });

  const windowStart = computed(() => windowEnd.value - RANGE_MS[range.value]);
  const bucketMs = computed(() => RANGE_BUCKET_MS[range.value]);
  const rangeLabel = computed(() => t(`rangeText${range.value}`));
  const baseLabel = computed(() => t(`rangeAgo${range.value}`));

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
</script>

<template>
  <div class="monitoring-page">
    <PageHeader
      :title="t('title')"
      :description="t('description')"
    >
      <template #live>
        <span
          class="monitoring-page__dot"
          :class="`monitoring-page__dot--${liveState}`"
          aria-hidden="true"
        />
        {{ liveText }}
      </template>

      <template #actions>
        <div
          class="monitoring-page__range"
          role="group"
          :aria-label="t('rangeLabel')"
        >
          <button
            v-for="r in STATS_RANGES"
            :key="r"
            type="button"
            class="monitoring-page__range-button"
            :aria-pressed="r === range"
            @click="store.setRange(r)"
          >
            {{ t(`range${r}`) }}
          </button>
        </div>
        <CButton @click="isPaused ? store.resume() : store.pause()">
          {{ isPaused ? t('resume') : t('pause') }}
        </CButton>
      </template>
    </PageHeader>

    <ChorusListError
      v-if="showError"
      :title="t('errorTitle')"
      :text="t('errorText')"
      :action-text="t('retry')"
      @retry="store.start()"
    />

    <div
      v-else-if="isLoading"
      class="monitoring-page__sections"
    >
      <div class="monitoring-page__skeleton">
        <CSkeleton
          v-for="n in 3"
          :key="n"
          :height="110"
          :border-radius="12"
        />
      </div>
      <div class="monitoring-page__charts">
        <CSkeleton
          v-for="n in 4"
          :key="n"
          :height="280"
          :border-radius="12"
        />
      </div>
    </div>

    <div
      v-else
      class="monitoring-page__sections"
    >
      <EmptyState
        v-if="proxies.length === 0 && points.length === 0"
        :title="t('noProxiesTitle')"
        :text="t('noProxiesText')"
      />

      <template v-else>
        <MonitoringKpiTiles
          :current="totalRates ?? points[points.length - 1]?.rates ?? null"
          :points="points"
          :base-label="baseLabel"
        />

        <section
          class="monitoring-page__charts"
          :aria-label="t('chartsSection')"
        >
          <TrafficLineChart
            v-for="chart in charts"
            :key="chart.key"
            :title="chart.title"
            :points="points"
            :lines="chart.lines"
            :kind="chart.kind"
            :format-value="chart.format"
            :window-start="windowStart"
            :window-end="windowEnd"
            :bucket-ms="bucketMs"
            :range-label="rangeLabel"
          />
        </section>

        <MonitoringBreakdown />
      </template>

      <MonitoringReplication />
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .monitoring-page {
    max-width: 1600px;

    &__sections {
      display: flex;
      flex-direction: column;
      gap: 20px;
    }

    &__skeleton {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(min(300px, 100%), 1fr));
      gap: 16px;
    }

    &__charts {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 16px;

      @media (width <= 900px) {
        grid-template-columns: minmax(0, 1fr);
      }
    }

    &__dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background-color: var(--tt-success-solid);

      &--paused {
        background-color: var(--tt-neutral-solid);
      }

      &--failing {
        background-color: var(--tt-warning-solid);
      }
    }

    &__range {
      display: flex;
      overflow: hidden;
      font-size: 13px;
      background-color: var(--tt-surface);
      border: 1px solid var(--tt-border-strong);
      border-radius: var(--tt-radius-control);
    }

    &__range-button {
      height: 34px;
      padding: 0 12px;
      font: inherit;
      color: var(--tt-text);
      cursor: pointer;
      background-color: var(--tt-surface);
      border: 0;

      & + & {
        border-left: 1px solid var(--tt-border-strong);
      }

      &:hover {
        background-color: var(--tt-hover);
      }

      &[aria-pressed='true'] {
        font-weight: 600;
        background-color: var(--tt-accent-subtle);
        color: var(--tt-accent);
      }
    }
  }
</style>
