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
  /** Colored dot plus text: the label carries the meaning, not the color. */
  import type { StatusTone } from './types';

  defineProps<{
    tone: StatusTone;
    label: string;
  }>();
</script>

<template>
  <span
    class="kit-status"
    :class="`kit-status--${tone}`"
    :data-tone="tone"
  >
    <span
      class="kit-status__dot"
      aria-hidden="true"
    />
    {{ label }}
  </span>
</template>

<style lang="scss" scoped>
  .kit-status {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    max-width: 100%;
    padding: 1px var(--tt-space-2);
    border: 1px solid var(--kit-status-border);
    border-radius: var(--tt-radius-pill);
    background-color: var(--kit-status-bg);
    color: var(--kit-status-text);
    font-size: var(--tt-text-xs);
    line-height: 18px;
    font-weight: var(--tt-weight-medium);
    white-space: nowrap;

    &__dot {
      flex: none;
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background-color: var(--kit-status-dot);
    }

    @each $tone in success, warning, danger, info, neutral {
      &--#{$tone} {
        --kit-status-text: var(--tt-#{$tone});
        --kit-status-bg: var(--tt-#{$tone}-bg);
        --kit-status-border: var(--tt-#{$tone}-border);
        --kit-status-dot: var(--tt-#{$tone}-solid);
      }
    }
  }
</style>
