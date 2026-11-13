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
  import { NButton } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, onBeforeMount, onBeforeUnmount } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useMetricsCharts } from './useMetricsCharts';
  import i18nMonitoring from '@/components/tutti/monitoring/i18nMonitoring';
  import MonitoringBreakdown from '@/components/tutti/monitoring/MonitoringBreakdown.vue';
  import MonitoringKpiTiles from '@/components/tutti/monitoring/MonitoringKpiTiles.vue';
  import MonitoringReplication from '@/components/tutti/monitoring/MonitoringReplication.vue';
  import TrafficLineChart from '@/components/tutti/monitoring/TrafficLineChart.vue';
  import { EmptyBlock, ErrorBlock, LoadingBlock } from '@/components/kit';
  import { useTuttiMonitoringStore } from '@/stores/tuttiMonitoringStore';
  import { useNow } from '@/utils/composables/useNow';
  import { formatRelative } from '@/utils/helpers/DateTimeHelper';
  import {
    RANGE_BUCKET_MS,
    RANGE_MS,
    STATS_RANGES,
  } from '@/utils/helpers/MonitoringHelper';

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

  const { charts } = useMetricsCharts(points, bucketMs);
</script>

<template>
  <div class="dashboard-metrics">
    <div class="dashboard-metrics__toolbar">
      <span class="dashboard-metrics__live">
        <span
          class="dashboard-metrics__dot"
          :class="`dashboard-metrics__dot--${liveState}`"
          aria-hidden="true"
        />
        {{ liveText }}
      </span>
      <div class="dashboard-metrics__controls">
        <div
          class="dashboard-metrics__range"
          role="group"
          :aria-label="t('rangeLabel')"
        >
          <button
            v-for="r in STATS_RANGES"
            :key="r"
            type="button"
            class="dashboard-metrics__range-button"
            :aria-pressed="r === range"
            @click="store.setRange(r)"
          >
            {{ t(`range${r}`) }}
          </button>
        </div>
        <NButton @click="isPaused ? store.resume() : store.pause()">
          {{ isPaused ? t('resume') : t('pause') }}
        </NButton>
      </div>
    </div>

    <ErrorBlock
      v-if="showError"
      :title="t('errorTitle')"
      :description="t('errorText')"
      @retry="store.start()"
    />

    <LoadingBlock
      v-else-if="isLoading"
      :rows="6"
    />

    <div
      v-else
      class="dashboard-metrics__sections"
    >
      <EmptyBlock
        v-if="proxies.length === 0 && points.length === 0"
        :title="t('noProxiesTitle')"
        :description="t('noProxiesText')"
      />

      <template v-else>
        <MonitoringKpiTiles
          :current="totalRates ?? points[points.length - 1]?.rates ?? null"
          :points="points"
          :base-label="baseLabel"
        />

        <section
          class="dashboard-metrics__charts"
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
  .dashboard-metrics {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);

    &__toolbar {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: space-between;
      gap: var(--tt-space-3);
    }

    &__live {
      display: inline-flex;
      align-items: center;
      gap: var(--tt-space-2);
      font-size: var(--tt-text-sm);
      color: var(--tt-text-2);
    }

    &__controls {
      display: flex;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
    }

    &__sections {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-5);
    }

    &__charts {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: var(--tt-space-4);

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
      font-size: var(--tt-text-sm);
      background-color: var(--tt-surface);
      border: 1px solid var(--tt-border-strong);
      border-radius: var(--tt-radius-control);
    }

    &__range-button {
      height: 34px;
      padding: 0 var(--tt-space-3);
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
        font-weight: var(--tt-weight-strong);
        background-color: var(--tt-accent-subtle);
        color: var(--tt-accent);
      }
    }
  }
</style>
