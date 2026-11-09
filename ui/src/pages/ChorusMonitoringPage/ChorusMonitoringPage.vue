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
  import { CResult } from '@clyso/clyso-ui-kit';
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
  import ChorusListError from '@/components/chorus/common/ChorusListError/ChorusListError.vue';
  import { useTuttiMonitoringStore } from '@/stores/tuttiMonitoringStore';
  import { HAS_PROMETHEUS } from '@/utils/constants/env';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nMonitoring });
  const store = useTuttiMonitoringStore();
  const {
    isLoading,
    hasError,
    proxies,
    proxiesWithoutStats,
    hasStats,
    isWarmingUp,
    current,
    aggregatedSeries,
  } = storeToRefs(store);

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
        type="info"
        class="monitoring-page__note"
      >
        {{ t('sinceStartNote') }} {{ t('restartNote') }}
        <template v-if="HAS_PROMETHEUS">
          {{ t('prometheusHint') }}
          <RouterLink :to="{ name: RouteName.CHORUS_HOME }">
            {{ t('prometheusLink') }}
          </RouterLink>
        </template>
      </NAlert>

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

    &__charts {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 16px;

      @media screen and (max-width: 1000px) {
        grid-template-columns: minmax(0, 1fr);
      }
    }
  }
</style>
