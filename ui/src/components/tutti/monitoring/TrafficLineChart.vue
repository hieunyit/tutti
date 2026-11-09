/* * Copyright © 2026 hieunyit * * Licensed under the GNU Affero General Public
License, Version 3.0 (the "License"); * you may not use this file except in
compliance with the License. * You may obtain a copy of the License at * *
https://www.gnu.org/licenses/agpl-3.0.html * * Unless required by applicable law
or agreed to in writing, software * distributed under the License is distributed
on an "AS IS" BASIS, * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
express or implied. * See the License for the specific language governing
permissions and * limitations under the License. */

<script setup lang="ts">
  import {
    Chart,
    LinearScale,
    LineController,
    LineElement,
    PointElement,
    Tooltip,
    type ChartConfiguration,
  } from 'chart.js';
  import { storeToRefs } from 'pinia';
  import {
    computed,
    nextTick,
    onBeforeUnmount,
    onMounted,
    ref,
    watch,
  } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nMonitoring from './i18nMonitoring';
  import SectionCard from '@/components/ui/SectionCard.vue';
  import { useColorSchemeStore } from '@/stores/colorSchemeStore';
  import { useI18nStore } from '@/stores/i18nStore';
  import { formatClock, formatTime } from '@/utils/helpers/DateTimeHelper';
  import {
    axisFormatter,
    buildSeries,
    timeTicks,
    type AxisKind,
    type RatePoint,
    type SeriesPicker,
  } from '@/utils/helpers/MonitoringHelper';

  Chart.register(
    LinearScale,
    LineController,
    LineElement,
    PointElement,
    Tooltip,
  );

  export interface TrafficChartLine {
    key: string;
    label: string;
    /** token holding the line colour, e.g. --tt-chart-1 */
    colorVar: string;
    fallbackColor: string;
    /** SVG-style dash pattern; keeps series apart without colour */
    dash: number[];
    pick: SeriesPicker;
  }

  const props = defineProps<{
    title: string;
    points: RatePoint[];
    lines: TrafficChartLine[];
    kind: AxisKind;
    /** value formatter of tooltips and the screen reader summary */
    formatValue: (value: number) => string;
    windowStart: number;
    windowEnd: number;
    bucketMs: number;
    /** text of the accessible label, e.g. "last 15 minutes" */
    rangeLabel: string;
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

  const series = computed(() =>
    props.lines.map((line) =>
      buildSeries(props.points, line.pick, props.bucketMs),
    ),
  );

  const hasData = computed(() => props.points.length > 0);

  const summary = computed(() => {
    const last = props.points[props.points.length - 1];

    if (!last)
      return `${props.title}, ${props.rangeLabel}. ${t('chartNoData')}`;

    const values = props.lines
      .map((line) => {
        const v = line.pick(last.rates);

        return `${line.label} ${v === null ? '–' : props.formatValue(v)}`;
      })
      .join(', ');

    return `${props.title}, ${props.rangeLabel}. ${t('chartLatest')}: ${values}.`;
  });

  function isolated(data: { y: number | null }[], i: number): boolean {
    return (
      data[i]?.y != null &&
      (data[i - 1]?.y ?? null) === null &&
      (data[i + 1]?.y ?? null) === null
    );
  }

  function buildConfig(): ChartConfiguration<'line'> {
    const text = cssVar('--tt-text-3', '#667085');
    const grid = cssVar('--tt-chart-grid', '#eef0f3');
    const axis = cssVar('--tt-chart-axis', '#d0d5dd');
    const max = Math.max(
      0,
      ...series.value.flatMap((s) => s.map((p) => p.y ?? 0)),
    );
    const format = axisFormatter(props.kind, max);

    return {
      type: 'line',
      data: {
        datasets: props.lines.map((line, i) => {
          const color = cssVar(line.colorVar, line.fallbackColor);
          const data = series.value[i] ?? [];

          return {
            label: line.label,
            data,
            parsing: false,
            borderColor: color,
            backgroundColor: color,
            borderWidth: 2,
            borderDash: line.dash,
            // a value between two gaps has no line: show it as a dot
            pointRadius: (ctx: { dataIndex: number }) =>
              isolated(data, ctx.dataIndex) ? 3 : 0,
            pointHoverRadius: 4,
            // zero lines sit on the axis: let them overflow the plot area
            clip: 3,
            tension: 0,
            spanGaps: false,
          };
        }),
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        animation: false,
        normalized: true,
        interaction: { mode: 'nearest', axis: 'x', intersect: false },
        scales: {
          x: {
            type: 'linear',
            min: props.windowStart,
            max: props.windowEnd,
            afterBuildTicks: (scale) => {
              scale.ticks = timeTicks(props.windowStart, props.windowEnd).map(
                (value) => ({ value }),
              );
            },
            ticks: {
              color: text,
              autoSkip: false,
              maxRotation: 0,
              font: { size: 11 },
              callback: (value) => formatClock(Number(value), locale.value),
            },
            grid: { color: grid },
            border: { color: axis },
          },
          y: {
            beginAtZero: true,
            ticks: {
              color: text,
              maxTicksLimit: 5,
              font: { size: 11 },
              callback: (value) => format(Number(value)),
            },
            grid: { color: grid },
            border: { display: false },
          },
        },
        plugins: {
          legend: { display: false },
          tooltip: {
            callbacks: {
              title: (items) =>
                items[0] ? formatTime(items[0].parsed.x, locale.value) : '',
              label: (ctx) =>
                `${ctx.dataset.label}: ${
                  ctx.parsed.y === null ? '–' : props.formatValue(ctx.parsed.y)
                }`,
            },
          },
        },
      },
    };
  }

  function render() {
    if (!canvas.value) return;

    const config = buildConfig();

    if (!chart) {
      chart = new Chart(canvas.value, config);

      return;
    }

    chart.data = config.data;
    chart.options = config.options!;
    chart.update('none');
  }

  onMounted(render);

  onBeforeUnmount(() => {
    chart?.destroy();
    chart = null;
  });

  watch(
    () => [props.points, props.lines, props.windowEnd, props.windowStart],
    render,
  );
  // the body class of the theme is applied after the store changed
  watch([isDark, locale], () => nextTick(render));
</script>

<template>
  <SectionCard
    class="traffic-chart"
    :title="title"
  >
    <template #actions>
      <ul
        class="traffic-chart__legend"
        :aria-label="t('legend')"
      >
        <li
          v-for="line in lines"
          :key="line.key"
        >
          <svg
            class="traffic-chart__swatch"
            width="18"
            height="8"
            viewBox="0 0 18 8"
            aria-hidden="true"
          >
            <line
              x1="0"
              y1="4"
              x2="18"
              y2="4"
              :stroke="`var(${line.colorVar})`"
              stroke-width="2.5"
              :stroke-dasharray="line.dash.join(' ')"
            />
          </svg>
          {{ line.label }}
        </li>
      </ul>
    </template>

    <div class="traffic-chart__canvas-wrap">
      <canvas
        ref="canvas"
        role="img"
        :aria-label="summary"
      />
      <p
        v-if="!hasData"
        class="traffic-chart__empty"
      >
        {{ t('chartNoData') }}
      </p>
    </div>
  </SectionCard>
</template>

<style lang="scss" scoped>
  .traffic-chart {
    &__legend {
      display: flex;
      flex-wrap: wrap;
      justify-content: flex-end;
      gap: 4px 12px;
      margin: 0;
      padding: 0;
      list-style: none;
      font-size: 12px;
      color: var(--tt-text-2);

      li {
        display: flex;
        align-items: center;
        gap: 6px;
      }
    }

    &__canvas-wrap {
      position: relative;
      height: 200px;
    }

    &__empty {
      position: absolute;
      inset: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      margin: 0;
      font-size: 13px;
      color: var(--tt-text-3);
    }
  }
</style>
