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

  defineProps<{
    label: string;
    value: string;
    hint?: string;
    to?: RouteLocationRaw;
    tone?: 'success' | 'warning' | 'error';
  }>();
</script>

<template>
  <component
    :is="to ? 'RouterLink' : 'div'"
    :to="to"
    class="kpi-card"
    :class="[to && 'kpi-card--link', tone && `kpi-card--${tone}`]"
  >
    <span class="kpi-card__label">{{ label }}</span>
    <span class="kpi-card__value">{{ value }}</span>
    <span class="kpi-card__hint">{{ hint }}</span>
  </component>
</template>

<style lang="scss" scoped>
  .kpi-card {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
    padding: 14px 16px;
    border: 1px solid var(--border-color);
    border-radius: 8px;
    background-color: var(--card-color);
    color: var(--text-color-1);
    text-decoration: none;

    &--link {
      transition: border-color 0.15s;

      &:hover,
      &:focus-visible {
        border-color: var(--primary-color);
      }
    }

    &--success {
      .kpi-card__value {
        color: var(--success-color);
      }
    }

    &--warning {
      .kpi-card__value {
        color: var(--warning-color);
      }
    }

    &--error {
      .kpi-card__value {
        color: var(--error-color);
      }
    }

    &__label {
      font-size: 13px;
      line-height: 18px;
      color: var(--text-color-3);
    }

    &__value {
      font-size: 28px;
      line-height: 36px;
      font-weight: 600;
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    &__hint {
      min-height: 16px;
      font-size: 12px;
      line-height: 16px;
      color: var(--text-color-3);
    }
  }
</style>
