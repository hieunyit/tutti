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
  import type { StatusVariant } from './status';

  const props = withDefaults(
    defineProps<{
      variant?: StatusVariant;
      /** text; the default slot wins when both are given */
      label?: string;
      /** hide the dot (the text still carries the meaning) */
      noDot?: boolean;
    }>(),
    { variant: 'neutral', label: undefined, noDot: false },
  );

  const classes = computed(() => `tt-status tt-status--${props.variant}`);
</script>

<template>
  <span :class="classes">
    <span
      v-if="!noDot"
      class="tt-status__dot"
      aria-hidden="true"
    />
    <slot>{{ label }}</slot>
  </span>
</template>

<style lang="scss" scoped>
  .tt-status {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 2px 8px;
    border-radius: var(--tt-radius-pill);
    font-size: 12px;
    line-height: 16px;
    font-weight: 600;
    white-space: nowrap;

    &__dot {
      flex: none;
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background-color: currentcolor;
    }

    @each $name in (success, warning, danger, info, neutral) {
      &--#{$name} {
        color: var(--tt-#{$name});
        background-color: var(--tt-#{$name}-bg);
      }
    }
  }
</style>
