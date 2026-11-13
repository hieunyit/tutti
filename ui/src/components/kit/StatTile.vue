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
  /** KPI tile: label, value, optional trend and status tone; a link with `to`. */
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NSkeleton } from 'naive-ui';
  import type { RouteLocationRaw } from 'vue-router';
  import i18nKit from './i18nKit';
  import type { StatTrend, StatusTone } from './types';

  const props = withDefaults(
    defineProps<{
      label: string;
      value: string | number;
      /** small line under the value */
      hint?: string;
      trend?: StatTrend;
      tone?: StatusTone;
      to?: RouteLocationRaw;
      loading?: boolean;
    }>(),
    {
      hint: undefined,
      trend: undefined,
      tone: 'neutral',
      to: undefined,
      loading: false,
    },
  );

  const { t } = useI18n({ messages: i18nKit });
  const trendWord = computed(() => {
    if (!props.trend) return '';

    return props.trend.direction === 'up'
      ? t('trendUp')
      : props.trend.direction === 'down'
        ? t('trendDown')
        : t('trendFlat');
  });
  const arrow = computed(() =>
    props.trend?.direction === 'up'
      ? '↑'
      : props.trend?.direction === 'down'
        ? '↓'
        : '→',
  );
</script>

<template>
  <component
    :is="to ? 'RouterLink' : 'div'"
    :to="to"
    class="kit-stat"
    :class="[`kit-stat--${tone}`, { 'kit-stat--link': to }]"
  >
    <span class="kit-stat__label">{{ label }}</span>
    <NSkeleton
      v-if="loading"
      class="kit-stat__skeleton"
      :width="64"
      :height="28"
    />
    <span
      v-else
      class="kit-stat__value"
      >{{ value }}</span
    >
    <span
      v-if="trend && !loading"
      class="kit-stat__trend"
    >
      <span aria-hidden="true">{{ arrow }}</span>
      <span class="tt-sr-only">{{ trendWord }}</span>
      {{ trend.label }}
    </span>
    <span
      v-if="hint"
      class="kit-stat__hint"
      >{{ hint }}</span
    >
  </component>
</template>

<style lang="scss" scoped>
  .kit-stat {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-1);
    min-width: 0;
    padding: var(--tt-space-4);
    border: 1px solid var(--tt-border);
    border-left: 3px solid var(--kit-stat-accent, var(--tt-border));
    border-radius: var(--tt-radius-card);
    background-color: var(--tt-surface);
    color: var(--tt-text);
    text-decoration: none;

    &--link {
      transition: border-color var(--tt-duration-fast) var(--tt-ease);

      &:hover {
        border-color: var(--tt-border-strong);
        border-left-color: var(--kit-stat-accent, var(--tt-border-strong));
      }
    }

    @each $tone in success, warning, danger, info {
      &--#{$tone} {
        --kit-stat-accent: var(--tt-#{$tone}-solid);
        --kit-stat-value: var(--tt-#{$tone});
      }
    }

    &__label {
      font-size: var(--tt-text-sm);
      font-weight: var(--tt-weight-medium);
      color: var(--tt-text-2);
    }

    &__value {
      font-size: var(--tt-text-2xl);
      line-height: 32px;
      font-weight: var(--tt-weight-strong);
      font-variant-numeric: tabular-nums;
      color: var(--kit-stat-value, var(--tt-text));
    }

    &__trend,
    &__hint {
      font-size: var(--tt-text-xs);
      line-height: 16px;
      color: var(--tt-text-3);
    }
  }
</style>
