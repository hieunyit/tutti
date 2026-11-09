/* * Copyright © 2026 hieunyit * * Licensed under the GNU Affero General Public
License, Version 3.0 (the "License"); * you may not use this file except in
compliance with the License. * You may obtain a copy of the License at * *
https://www.gnu.org/licenses/agpl-3.0.html * * Unless required by applicable law
or agreed to in writing, software * distributed under the License is distributed
on an "AS IS" BASIS, * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
express or implied. * See the License for the specific language governing
permissions and * limitations under the License. */

<script setup lang="ts">
  import { storeToRefs } from 'pinia';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nMonitoring from './i18nMonitoring';
  import { useI18nStore } from '@/stores/i18nStore';
  import {
    computeDelta,
    firstValue,
    pickers,
    type RatePoint,
    type Rates,
    type SeriesPicker,
  } from '@/utils/helpers/MonitoringHelper';
  import { TrafficStatsHelper } from '@/utils/helpers/TrafficStatsHelper';

  const props = defineProps<{
    /** newest rates over all proxies */
    current: Rates | null;
    /** points of the selected range, the first one is the delta base */
    points: RatePoint[];
    /** "15 min ago" */
    baseLabel: string;
  }>();

  const { t } = useI18n({ messages: i18nMonitoring });
  const { locale } = storeToRefs(useI18nStore());

  type Tone = 'default' | 'warning' | 'danger';

  interface Kpi {
    key: string;
    label: string;
    value: string;
    sub: string;
    tone: Tone;
  }

  interface KpiGroup {
    key: string;
    title: string;
    kpis: Kpi[];
  }

  const percent = computed(
    () =>
      new Intl.NumberFormat(locale.value === 'de' ? 'de-DE' : 'en-GB', {
        style: 'percent',
        maximumFractionDigits: 0,
        signDisplay: 'exceptZero',
      }),
  );

  function deltaText(pick: SeriesPicker): string {
    const r = props.current;

    if (!r) return '';

    const delta = computeDelta(pick(r), firstValue(props.points, pick));

    if (!delta) return '';

    if (delta.percent === null) {
      // zero before and now: nothing to compare
      return delta.direction === 'flat'
        ? ''
        : t('deltaFromZero', { base: props.baseLabel });
    }

    return t('deltaVs', {
      value: percent.value.format(delta.percent / 100),
      base: props.baseLabel,
    });
  }

  const groups = computed<KpiGroup[]>(() => {
    const r = props.current;
    const none = t('kpiNoData');
    const rate = (v: number) => TrafficStatsHelper.formatRate(v);
    const tone = (v: number | undefined, level: Tone): Tone =>
      v !== undefined && v > 0 ? level : 'default';
    const kpi = (
      key: string,
      label: string,
      value: string | null,
      pick: SeriesPicker,
      level: Tone = 'default',
      raw?: number,
    ): Kpi => ({
      key,
      label,
      value: value ?? none,
      sub: deltaText(pick),
      tone: tone(raw, level),
    });

    return [
      {
        key: 'traffic',
        title: t('groupTraffic'),
        kpis: [
          kpi(
            'requests',
            t('kpiRequests'),
            r && rate(r.requests),
            pickers.requests,
          ),
          kpi(
            'in',
            t('kpiThroughputIn'),
            r && TrafficStatsHelper.formatBytesPerSecond(r.bytesIn),
            pickers.bytesIn,
          ),
          kpi(
            'out',
            t('kpiThroughputOut'),
            r && TrafficStatsHelper.formatBytesPerSecond(r.bytesOut),
            pickers.bytesOut,
          ),
        ],
      },
      {
        key: 'latency',
        title: t('groupLatency'),
        kpis: [
          kpi(
            'avg',
            t('kpiLatencyAvg'),
            r && TrafficStatsHelper.formatLatencyMs(pickers.latencyAvg(r)),
            pickers.latencyAvg,
          ),
          kpi(
            'p95',
            t('kpiLatencyP95'),
            r && TrafficStatsHelper.formatLatencyMs(pickers.latencyP95(r)),
            pickers.latencyP95,
          ),
        ],
      },
      {
        key: 'errors',
        title: t('groupErrors'),
        kpis: [
          kpi('e4', t('kpi4xxRate'), r && rate(r.status4xx), pickers.status4xx),
          kpi(
            'e5',
            t('kpi5xxRate'),
            r && rate(r.status5xx),
            pickers.status5xx,
            'danger',
            r?.status5xx,
          ),
          kpi(
            'ff',
            t('kpiFailFast'),
            r && rate(r.failFast),
            pickers.failFast,
            'warning',
            r?.failFast,
          ),
          kpi(
            'ad',
            t('kpiAccessDenied'),
            r && rate(r.accessDenied),
            pickers.accessDenied,
          ),
        ],
      },
    ];
  });
</script>

<template>
  <section
    class="monitoring-kpis"
    :aria-label="t('kpiSection')"
  >
    <div
      v-for="group in groups"
      :key="group.key"
      class="monitoring-kpis__group"
    >
      <h2 class="monitoring-kpis__title">{{ group.title }}</h2>
      <div class="monitoring-kpis__items">
        <div
          v-for="kpi in group.kpis"
          :key="kpi.key"
          class="monitoring-kpis__item"
        >
          <span class="monitoring-kpis__label">{{ kpi.label }}</span>
          <span
            class="monitoring-kpis__value"
            :class="`monitoring-kpis__value--${kpi.tone}`"
            >{{ kpi.value }}</span
          >
          <span class="monitoring-kpis__sub">{{ kpi.sub }}</span>
        </div>
      </div>
    </div>
  </section>
</template>

<style lang="scss" scoped>
  .monitoring-kpis {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(300px, 100%), 1fr));
    gap: 16px;

    &__group {
      display: flex;
      flex-direction: column;
      gap: 12px;
      min-width: 0;
      padding: var(--tt-card-padding);
      background-color: var(--tt-surface);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-card);
    }

    &__title {
      margin: 0;
      font-size: 13px;
      font-weight: 600;
      color: var(--tt-text-2);
    }

    &__items {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(110px, 1fr));
      gap: 12px;
    }

    &__item {
      display: flex;
      flex-direction: column;
      gap: 2px;
      min-width: 0;
    }

    &__label {
      font-size: 12px;
      color: var(--tt-text-3);
    }

    &__value {
      font-size: 22px;
      line-height: 28px;
      font-weight: 700;
      font-variant-numeric: tabular-nums;
      color: var(--tt-text);
      overflow-wrap: anywhere;

      &--warning {
        color: var(--tt-warning);
      }

      &--danger {
        color: var(--tt-danger);
      }
    }

    &__sub {
      min-height: 16px;
      font-size: 12px;
      color: var(--tt-text-3);
    }
  }
</style>
