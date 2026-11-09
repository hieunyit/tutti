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
  import { CButton, CResult } from '@clyso/clyso-ui-kit';
  import { NAlert, NSpin } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, onBeforeMount, onBeforeUnmount } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nMonitoring from '@/components/tutti/monitoring/i18nMonitoring';
  import MonitoringKpiTiles from '@/components/tutti/monitoring/MonitoringKpiTiles.vue';
  import MonitoringBreakdown from '@/components/tutti/monitoring/MonitoringBreakdown.vue';
  import MonitoringReplication from '@/components/tutti/monitoring/MonitoringReplication.vue';
  import TrafficLineChart, {
    type TrafficChartLine,
  } from '@/components/tutti/monitoring/TrafficLineChart.vue';
  import PageHeader from '@/components/layout/PageHeader.vue';
  import ChorusListError from '@/components/chorus/common/ChorusListError/ChorusListError.vue';
  import { useTuttiMonitoringStore } from '@/stores/tuttiMonitoringStore';
  import { useNow } from '@/utils/composables/useNow';
  import { HAS_PROMETHEUS } from '@/utils/constants/env';
  import {
    formatDateTime,
    formatRelative,
  } from '@/utils/helpers/DateTimeHelper';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nMonitoring });
  const store = useTuttiMonitoringStore();
  const {
    isLoading,
    isPaused,
    lastUpdated,
    hasError,
    proxies,
    proxiesWithoutStats,
    hasStats,
    isWarmingUp,
    current,
    aggregatedSeries,
  } = storeToRefs(store);

  const now = useNow(1000);

  const statusText = computed(() => {
    if (lastUpdated.value === null) return t('waitingStatus');

    const time = formatRelative(lastUpdated.value, undefined, now.value);

    if (hasError.value) return t('staleStatus', { time });

    return isPaused.value
      ? t('pausedStatus', { time })
      : t('liveStatus', { time });
  });

  onBeforeMount(() => store.startPolling());
  onBeforeUnmount(() => store.$reset());

  const classLine = (
    cls: string,
    label: string,
    colorVar: string,
    fallbackColor: string,
  ): TrafficChartLine => ({
    label,
    colorVar,
    fallbackColor,
    pick: (r) => r.byClass[cls] ?? 0,
  });

  const classLines = computed<TrafficChartLine[]>(() => [
    classLine('read', t('seriesRead'), '--info-color', '#2080f0'),
    classLine('write', t('seriesWrite'), '--success-color', '#18a058'),
    classLine('exists', t('seriesExists'), '--primary-color', '#7c5cff'),
    classLine('manage', t('seriesManage'), '--warning-color', '#f0a020'),
    classLine('other', t('seriesOther'), '--text-color-3', '#999999'),
  ]);

  const latencyLines = computed<TrafficChartLine[]>(() => [
    {
      label: t('seriesAvg'),
      colorVar: '--info-color',
      fallbackColor: '#2080f0',
      pick: (r) => {
        const v = TrafficStatsHelper.averageLatency(r);

        return v === null ? null : v;
      },
    },
    {
      label: t('seriesP95'),
      colorVar: '--warning-color',
      fallbackColor: '#f0a020',
      pick: (r) => TrafficStatsHelper.p95Latency(r),
    },
  ]);

  const throughputLines = computed<TrafficChartLine[]>(() => [
    {
      label: t('seriesIn'),
      colorVar: '--success-color',
      fallbackColor: '#18a058',
      pick: (r) => r.bytesIn,
    },
    {
      label: t('seriesOut'),
      colorVar: '--info-color',
      fallbackColor: '#2080f0',
      pick: (r) => r.bytesOut,
    },
  ]);

  const errorLines = computed<TrafficChartLine[]>(() => [
    {
      label: t('series4xx'),
      colorVar: '--warning-color',
      fallbackColor: '#f0a020',
      pick: (r) => r.responses4xx,
    },
    {
      label: t('series5xx'),
      colorVar: '--error-color',
      fallbackColor: '#d03050',
      pick: (r) => r.responses5xx,
    },
    {
      label: t('seriesFailFast'),
      colorVar: '--primary-color',
      fallbackColor: '#7c5cff',
      pick: (r) => r.failFast,
    },
    {
      label: t('seriesAccessDenied'),
      colorVar: '--text-color-3',
      fallbackColor: '#999999',
      pick: (r) => r.accessDenied,
    },
  ]);

  const formatRate = (v: number) => TrafficStatsHelper.formatRate(v);
</script>

<template>
  <div class="monitoring-page">
    <PageHeader
      :title="t('title')"
      :description="t('description')"
    >
      <template
        v-if="!isLoading && !(hasError && !hasStats)"
        #actions
      >
        <span
          class="monitoring-page__status"
          :class="{
            'monitoring-page__status--paused': isPaused,
            'monitoring-page__status--stale': hasError,
          }"
          :title="formatDateTime(lastUpdated)"
          aria-live="off"
        >
          <span
            class="monitoring-page__dot"
            aria-hidden="true"
          />
          {{ statusText }}
        </span>
        <CButton @click="isPaused ? store.resume() : store.pause()">
          {{ isPaused ? t('resume') : t('pause') }}
        </CButton>
      </template>

      <template
        v-if="!isLoading && !(hasError && !hasStats)"
        #extra
      >
        <details class="monitoring-page__about">
          <summary>{{ t('aboutTitle') }}</summary>
          <p>
            {{ t('sinceStartNote') }} {{ t('restartNote') }}
            <template v-if="HAS_PROMETHEUS">
              {{ t('prometheusHint') }}
              <RouterLink :to="{ name: RouteName.CHORUS_HOME }">
                {{ t('prometheusLink') }}
              </RouterLink>
            </template>
          </p>
        </details>
      </template>
    </PageHeader>

    <ChorusListError
      v-if="hasError && !hasStats"
      :title="t('errorTitle')"
      :text="t('errorText')"
      :action-text="t('retry')"
      @retry="store.startPolling()"
    />

    <div
      v-else-if="isLoading"
      class="monitoring-page__loading"
    >
      <NSpin size="large" />
    </div>

    <template v-else>
      <NAlert
        v-if="hasError"
        type="warning"
        class="monitoring-page__note"
      >
        {{ t('errorText') }}
      </NAlert>

      <NAlert
        v-if="proxiesWithoutStats.length > 0"
        type="warning"
        class="monitoring-page__note"
      >
        {{ t('noStatsText', { proxies: proxiesWithoutStats.join(', ') }) }}
      </NAlert>

      <CResult
        v-if="proxies.length === 0"
        type="info"
        size="large"
        :max-width="600"
      >
        <template #title>{{ t('noProxiesTitle') }}</template>
        {{ t('noProxiesText') }}
      </CResult>

      <template v-else>
        <NAlert
          v-if="isWarmingUp"
          type="default"
          class="monitoring-page__note"
        >
          {{ t('warmingUp') }}
        </NAlert>

        <MonitoringKpiTiles
          class="monitoring-page__section"
          :rates="current"
        />

        <div class="monitoring-page__charts monitoring-page__section">
          <TrafficLineChart
            :title="t('chartRequestsByClass')"
            :series="aggregatedSeries"
            :lines="classLines"
            :format-value="formatRate"
          />
          <TrafficLineChart
            :title="t('chartLatency')"
            :series="aggregatedSeries"
            :lines="latencyLines"
            :format-value="TrafficStatsHelper.formatLatency"
          />
          <TrafficLineChart
            :title="t('chartThroughput')"
            :series="aggregatedSeries"
            :lines="throughputLines"
            :format-value="TrafficStatsHelper.formatBytesPerSecond"
          />
          <TrafficLineChart
            :title="t('chartErrors')"
            :series="aggregatedSeries"
            :lines="errorLines"
            :format-value="formatRate"
          />
        </div>

        <MonitoringBreakdown class="monitoring-page__section" />
      </template>

      <MonitoringReplication class="monitoring-page__section" />
    </template>
  </div>
</template>

<style lang="scss" scoped>
  .monitoring-page {
    max-width: 1800px;
    container-type: inline-size;

    &__loading {
      display: flex;
      justify-content: center;
      padding: 64px 0;
    }

    &__note {
      margin-bottom: 16px;
    }

    &__section {
      margin-bottom: 16px;
    }

    &__status {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      font-size: 13px;
      color: var(--text-color-3);
      font-variant-numeric: tabular-nums;
    }

    &__dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background-color: var(--success-color);
      animation: monitoring-pulse 2s ease-in-out infinite;

      @media (prefers-reduced-motion: reduce) {
        animation: none;
      }
    }

    &__status--paused &__dot {
      background-color: var(--text-color-3);
      animation: none;
    }

    &__status--stale &__dot {
      background-color: var(--warning-color);
      animation: none;
    }

    &__about {
      font-size: 13px;
      color: var(--text-color-3);

      summary {
        cursor: pointer;
        width: fit-content;
        color: var(--primary-color);
      }

      p {
        margin: 8px 0 0;
        max-width: 90ch;
        line-height: 20px;
      }
    }

    &__charts {
      display: grid;
      grid-template-columns: minmax(0, 1fr);
      gap: 16px;

      @container (min-width: 860px) {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
    }
  }

  @keyframes monitoring-pulse {
    0%,
    100% {
      opacity: 1;
    }

    50% {
      opacity: 0.35;
    }
  }
</style>
