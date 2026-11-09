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
  import { CTile } from '@clyso/clyso-ui-kit';
  import {
    CategoryScale,
    Chart,
    Legend,
    LinearScale,
    LineController,
    LineElement,
    PointElement,
    Tooltip,
    type ChartConfiguration,
  } from 'chart.js';
  import { storeToRefs } from 'pinia';
  import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nMonitoring from './i18nMonitoring';
  import { useColorSchemeStore } from '@/stores/colorSchemeStore';
  import { useI18nStore } from '@/stores/i18nStore';
  import { formatTime } from '@/utils/helpers/DateTimeHelper';
  import type { TrafficRates } from '@/utils/helpers/TrafficStatsHelper';

  Chart.register(
    CategoryScale,
    Legend,
    LinearScale,
    LineController,
    LineElement,
    PointElement,
    Tooltip,
  );

  export interface TrafficChartLine {
    label: string;
    /** css variable of the line colour */
    colorVar: string;
    fallbackColor: string;
    pick: (rates: TrafficRates) => number | null;
  }

  const props = defineProps<{
    title: string;
    series: TrafficRates[];
    lines: TrafficChartLine[];
    formatValue: (value: number) => string;
  }>();

  const { t } = useI18n({ messages: i18nMonitoring });
  const { locale } = storeToRefs(useI18nStore());
  const { isDark } = storeToRefs(useColorSchemeStore());

  const canvas = ref<HTMLCanvasElement | null>(null);
  let chart: Chart | null = null;

  function cssVar(name: string, fallback: string): string {
    const value = getComputedStyle(document.body).getPropertyValue(name).trim();

    return value || fallback;
  }

  const config = computed<ChartConfiguration<'line'>>(() => {
    // re-evaluate colours on theme switch
    const textColor = isDark.value
      ? cssVar('--text-color-1', '#ddd')
      : cssVar('--text-color-1', '#333');
    const gridColor = isDark.value
      ? 'rgba(255,255,255,0.1)'
      : 'rgba(0,0,0,0.08)';

    return {
      type: 'line',
      data: {
        labels: props.series.map((s) => formatTime(s.timestamp, locale.value)),
        datasets: props.lines.map((line) => {
          const color = cssVar(line.colorVar, line.fallbackColor);

          return {
            label: line.label,
            data: props.series.map((s) => line.pick(s)),
            borderColor: color,
            backgroundColor: color,
            borderWidth: 2,
            pointRadius: 0,
            pointHoverRadius: 4,
            tension: 0.25,
            spanGaps: false,
          };
        }),
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        interaction: { mode: 'index', intersect: false },
        scales: {
          x: {
            ticks: { color: textColor, maxTicksLimit: 6 },
            grid: { color: gridColor },
          },
          y: {
            beginAtZero: true,
            ticks: {
              color: textColor,
              callback: (value) => props.formatValue(Number(value)),
            },
            grid: { color: gridColor },
          },
        },
        plugins: {
          legend: { labels: { color: textColor, boxWidth: 12 } },
          tooltip: {
            callbacks: {
              label: (ctx) =>
                `${ctx.dataset.label}: ${props.formatValue(Number(ctx.parsed.y))}`,
            },
          },
        },
      },
    };
  });

  function update() {
    if (!chart) return;

    chart.data = config.value.data;
    chart.options = config.value.options!;
    chart.update('none');
  }

  onMounted(() => {
    if (canvas.value) chart = new Chart(canvas.value, config.value);
  });

  onBeforeUnmount(() => {
    chart?.destroy();
    chart = null;
  });

  watch(config, update);
</script>

<template>
  <CTile class="traffic-chart">
    <template #title>
      {{ title }}
    </template>

    <div class="traffic-chart__canvas-wrap">
      <canvas ref="canvas" />
      <p
        v-if="series.length === 0"
        class="traffic-chart__empty"
      >
        {{ t('chartNoData') }}
      </p>
    </div>
  </CTile>
</template>

<style lang="scss" scoped>
  .traffic-chart {
    &__canvas-wrap {
      position: relative;
      height: 220px;
    }

    &__empty {
      position: absolute;
      inset: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      margin: 0;
      opacity: 0.6;
    }
  }
</style>
