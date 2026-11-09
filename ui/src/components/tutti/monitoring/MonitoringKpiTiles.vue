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
    hint?: string;
    tone?: 'warning' | 'error';
  }

  interface KpiGroup {
    key: string;
    title: string;
    kpis: Kpi[];
  }

  const groups = computed<KpiGroup[]>(() => {
    const r = props.rates;
    const none = t('kpiNoData');
    const percent = (v: number) => `${v.toFixed(v < 10 ? 2 : 1)} %`;
    const rate = (v: number) => TrafficStatsHelper.formatRate(v);
    const share = (kind: 'responses4xx' | 'responses5xx') =>
      r
        ? t('kpiShareHint', {
            value: percent(TrafficStatsHelper.errorPercent(r, kind)),
          })
        : '';
    const e4 = r ? TrafficStatsHelper.errorPercent(r, 'responses4xx') : 0;
    const e5 = r ? TrafficStatsHelper.errorPercent(r, 'responses5xx') : 0;

    return [
      {
        key: 'traffic',
        title: t('groupTraffic'),
        kpis: [
          {
            key: 'requests',
            label: t('kpiRequests'),
            value: r ? rate(r.requests) : none,
          },
          {
            key: 'in',
            label: t('kpiThroughputIn'),
            value: r
              ? TrafficStatsHelper.formatBytesPerSecond(r.bytesIn)
              : none,
          },
          {
            key: 'out',
            label: t('kpiThroughputOut'),
            value: r
              ? TrafficStatsHelper.formatBytesPerSecond(r.bytesOut)
              : none,
          },
        ],
      },
      {
        key: 'latency',
        title: t('groupLatency'),
        kpis: [
          {
            key: 'avg',
            label: t('kpiLatencyAvg'),
            value: r
              ? TrafficStatsHelper.formatLatency(
                  TrafficStatsHelper.averageLatency(r),
                )
              : none,
          },
          {
            key: 'p95',
            label: t('kpiLatencyP95'),
            value: r
              ? TrafficStatsHelper.formatLatency(
                  TrafficStatsHelper.p95Latency(r),
                )
              : none,
          },
        ],
      },
      {
        key: 'errors',
        title: t('groupErrors'),
        kpis: [
          {
            key: 'e4',
            label: t('kpi4xxRate'),
            value: r ? rate(r.responses4xx) : none,
            hint: share('responses4xx'),
            tone: e4 > 10 ? 'warning' : undefined,
          },
          {
            key: 'e5',
            label: t('kpi5xxRate'),
            value: r ? rate(r.responses5xx) : none,
            hint: share('responses5xx'),
            tone: e5 > 0 ? 'error' : undefined,
          },
          {
            key: 'ff',
            label: t('kpiFailFast'),
            value: r ? rate(r.failFast) : none,
            tone: r && r.failFast > 0 ? 'warning' : undefined,
          },
          {
            key: 'ad',
            label: t('kpiAccessDenied'),
            value: r ? rate(r.accessDenied) : none,
            tone: r && r.accessDenied > 0 ? 'warning' : undefined,
          },
        ],
      },
    ];
  });
</script>

<template>
  <div class="kpi-groups">
    <section
      v-for="group in groups"
      :key="group.key"
      class="kpi-groups__group"
      :class="`kpi-groups__group--${group.key}`"
      :style="{ '--kpi-count': group.kpis.length }"
      :aria-label="group.title"
    >
      <h2 class="kpi-groups__title">{{ group.title }}</h2>
      <div class="kpi-groups__tiles">
        <div
          v-for="kpi in group.kpis"
          :key="kpi.key"
          class="kpi-groups__tile"
          :class="kpi.tone && `kpi-groups__tile--${kpi.tone}`"
        >
          <span class="kpi-groups__label">{{ kpi.label }}</span>
          <span class="kpi-groups__value">{{ kpi.value }}</span>
          <span class="kpi-groups__hint">{{ kpi.hint }}</span>
        </div>
      </div>
    </section>
  </div>
</template>

<style lang="scss" scoped>
  .kpi-groups {
    container-type: inline-size;
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: 12px;

    @container (min-width: 1000px) {
      grid-template-columns: minmax(0, 3fr) minmax(0, 2fr) minmax(0, 4fr);
    }

    &__group {
      min-width: 0;
      padding: 12px 16px 14px;
      border: 1px solid var(--border-color);
      border-radius: 8px;
      background-color: var(--card-color);
    }

    &__title {
      margin: 0 0 8px;
      font-size: 12px;
      font-weight: 600;
      letter-spacing: 0.5px;
      text-transform: uppercase;
      color: var(--text-color-3);
    }

    &__tiles {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(110px, 1fr));
      gap: 8px 16px;

      @container (min-width: 1000px) {
        grid-template-columns: repeat(var(--kpi-count), minmax(0, 1fr));
        column-gap: 12px;
      }
    }

    &__tile {
      display: flex;
      flex-direction: column;
      gap: 2px;
      min-width: 0;
    }

    &__label {
      font-size: 13px;
      color: var(--text-color-3);
    }

    &__value {
      font-size: 22px;
      line-height: 30px;
      font-weight: 600;
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
      color: var(--text-color-1);
    }

    &__hint {
      min-height: 16px;
      font-size: 12px;
      line-height: 16px;
      color: var(--text-color-3);
    }

    &__tile--warning &__value {
      color: var(--warning-color);
    }

    &__tile--error &__value {
      color: var(--error-color);
    }
  }
</style>
