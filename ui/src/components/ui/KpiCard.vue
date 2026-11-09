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
  import type { RouteLocationRaw } from 'vue-router';
  import type { Tone } from './status';

  withDefaults(
    defineProps<{
      label: string;
      value: string | number;
      sub?: string;
      tone?: Tone;
      /** makes the whole card a link */
      to?: RouteLocationRaw;
    }>(),
    { sub: undefined, tone: 'default', to: undefined },
  );
</script>

<template>
  <component
    :is="to ? 'RouterLink' : 'div'"
    :to="to"
    class="tt-kpi"
    :class="{ 'tt-kpi--link': to }"
  >
    <span class="tt-kpi__label">{{ label }}</span>
    <span
      class="tt-kpi__value"
      :class="`tt-kpi__value--${tone}`"
      >{{ value }}</span
    >
    <span
      v-if="sub"
      class="tt-kpi__sub"
      >{{ sub }}</span
    >
  </component>
</template>

<style lang="scss" scoped>
  .tt-kpi {
    display: flex;
    flex-direction: column;
    gap: 6px;
    min-width: 0;
    padding: var(--tt-card-padding);
    background-color: var(--tt-surface);
    border: 1px solid var(--tt-border);
    border-radius: var(--tt-radius-card);
    color: inherit;
    text-decoration: none;

    &--link:hover {
      border-color: var(--tt-accent);
    }

    &__label {
      font-size: 13px;
      font-weight: 500;
      color: var(--tt-text-2);
    }

    &__value {
      font-size: 26px;
      line-height: 32px;
      font-weight: 700;
      font-variant-numeric: tabular-nums;
      color: var(--tt-text);
      overflow-wrap: anywhere;

      &--success {
        color: var(--tt-success);
      }

      &--warning {
        color: var(--tt-warning);
      }

      &--danger {
        color: var(--tt-danger);
      }
    }

    &__sub {
      font-size: 12px;
      color: var(--tt-text-3);
    }
  }
</style>
