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
  export interface SegmentOption {
    value: string;
    label: string;
    count?: number;
  }

  defineProps<{
    modelValue: string;
    options: SegmentOption[];
    /** accessible group name */
    label: string;
  }>();

  const emit = defineEmits<{
    (e: 'update:modelValue', value: string): void;
  }>();
</script>

<template>
  <div
    class="tt-segmented"
    role="group"
    :aria-label="label"
  >
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      class="tt-segmented__item"
      :class="{ 'tt-segmented__item--active': option.value === modelValue }"
      :aria-pressed="option.value === modelValue"
      @click="emit('update:modelValue', option.value)"
    >
      {{ option.label }}
      <span
        v-if="option.count !== undefined"
        class="tt-segmented__count"
        >{{ option.count }}</span
      >
    </button>
  </div>
</template>

<style lang="scss" scoped>
  .tt-segmented {
    display: inline-flex;
    max-width: 100%;
    overflow: hidden;
    font-size: 13px;
    border: 1px solid var(--tt-border-strong);
    border-radius: var(--tt-radius-control);

    &__item {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      height: 34px;
      padding: 0 12px;
      font: inherit;
      color: var(--tt-text);
      background: var(--tt-surface);
      border: 0;
      border-left: 1px solid var(--tt-border-strong);
      cursor: pointer;

      &:first-child {
        border-left: 0;
      }

      &:hover {
        background: var(--tt-subtle);
      }

      &--active {
        font-weight: 600;
        background: var(--tt-disabled-bg);
      }
    }

    &__count {
      font-variant-numeric: tabular-nums;
      color: var(--tt-text-3);
    }
  }
</style>
