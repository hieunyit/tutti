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
  import { NStatistic } from 'naive-ui';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nMonitoring from './i18nMonitoring';
  import {
    TrafficStatsHelper,
    type TrafficRates,
  } from '@/utils/helpers/TrafficStatsHelper';

  const props = defineProps<{
    rates: TrafficRates | null;
  }>();

  const { t } = useI18n({ messages: i18nMonitoring });

  interface Kpi {
    key: string;
    label: string;
    value: string;
    tone?: 'warning' | 'error';
  }

  const kpis = computed<Kpi[]>(() => {
    const r = props.rates;
    const none = t('kpiNoData');
    const percent = (v: number) => `${v.toFixed(v < 10 ? 2 : 1)} %`;
    const rate = (v: number) => TrafficStatsHelper.formatRate(v);

    if (!r) {
      return [
        'requests',
        'e4',
        'e5',
        'avg',
        'p95',
        'in',
        'out',
        'ff',
        'ad',
      ].map((key) => ({ key, label: '', value: none }));
    }

    const e4 = TrafficStatsHelper.errorPercent(r, 'responses4xx');
    const e5 = TrafficStatsHelper.errorPercent(r, 'responses5xx');

    return [
      { key: 'requests', label: t('kpiRequests'), value: rate(r.requests) },
      {
        key: 'e4',
        label: t('kpiErrors4xx'),
        value: percent(e4),
        tone: e4 > 10 ? 'warning' : undefined,
      },
      {
        key: 'e5',
        label: t('kpiErrors5xx'),
        value: percent(e5),
        tone: e5 > 0 ? 'error' : undefined,
      },
      {
        key: 'avg',
        label: t('kpiLatencyAvg'),
        value: TrafficStatsHelper.formatLatency(
          TrafficStatsHelper.averageLatency(r),
        ),
      },
      {
        key: 'p95',
        label: t('kpiLatencyP95'),
        value: TrafficStatsHelper.formatLatency(
          TrafficStatsHelper.p95Latency(r),
        ),
      },
      {
        key: 'in',
        label: t('kpiThroughputIn'),
        value: TrafficStatsHelper.formatBytesPerSecond(r.bytesIn),
      },
      {
        key: 'out',
        label: t('kpiThroughputOut'),
        value: TrafficStatsHelper.formatBytesPerSecond(r.bytesOut),
      },
      {
        key: 'ff',
        label: t('kpiFailFast'),
        value: rate(r.failFast),
        tone: r.failFast > 0 ? 'warning' : undefined,
      },
      {
        key: 'ad',
        label: t('kpiAccessDenied'),
        value: rate(r.accessDenied),
        tone: r.accessDenied > 0 ? 'warning' : undefined,
      },
    ];
  });

  const labels = computed(() => [
    t('kpiRequests'),
    t('kpiErrors4xx'),
    t('kpiErrors5xx'),
    t('kpiLatencyAvg'),
    t('kpiLatencyP95'),
    t('kpiThroughputIn'),
    t('kpiThroughputOut'),
    t('kpiFailFast'),
    t('kpiAccessDenied'),
  ]);
</script>

<template>
  <div class="kpi-tiles">
    <div
      v-for="(kpi, index) in kpis"
      :key="kpi.key"
      class="kpi-tiles__tile"
      :class="kpi.tone && `kpi-tiles__tile--${kpi.tone}`"
    >
      <NStatistic
        :label="labels[index]"
        :value="kpi.value"
      />
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .kpi-tiles {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(170px, 1fr));
    gap: 12px;

    &__tile {
      padding: 12px 16px;
      border-radius: 8px;
      border: 1px solid var(--divider-color, rgba(128, 128, 128, 0.25));
      border-left-width: 4px;

      &--warning {
        border-left-color: var(--warning-color);
      }

      &--error {
        border-left-color: var(--error-color);
      }
    }
  }
</style>
