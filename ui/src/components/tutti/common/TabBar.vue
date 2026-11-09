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
  export interface TabItem {
    value: string;
    label: string;
    count?: number;
  }

  const props = defineProps<{
    modelValue: string;
    tabs: TabItem[];
    /** accessible name of the tab list */
    label: string;
  }>();

  const emit = defineEmits<{
    (e: 'update:modelValue', value: string): void;
  }>();

  function move(offset: number) {
    const index = props.tabs.findIndex((tab) => tab.value === props.modelValue);
    const next =
      props.tabs[(index + offset + props.tabs.length) % props.tabs.length];

    if (next) emit('update:modelValue', next.value);
  }
</script>

<template>
  <div
    class="tab-bar"
    role="tablist"
    :aria-label="label"
    @keydown.right.prevent="move(1)"
    @keydown.left.prevent="move(-1)"
  >
    <button
      v-for="tab in tabs"
      :id="`tab-${tab.value}`"
      :key="tab.value"
      type="button"
      role="tab"
      class="tab-bar__tab"
      :class="{ 'tab-bar__tab--active': tab.value === modelValue }"
      :aria-selected="tab.value === modelValue"
      :aria-controls="`tabpanel-${tab.value}`"
      :tabindex="tab.value === modelValue ? 0 : -1"
      @click="emit('update:modelValue', tab.value)"
    >
      {{ tab.label }}
      <span
        v-if="tab.count !== undefined"
        class="tab-bar__count"
        >{{ tab.count }}</span
      >
    </button>
  </div>
</template>

<style lang="scss" scoped>
  .tab-bar {
    display: flex;
    gap: 24px;
    overflow-x: auto;
    border-bottom: 1px solid var(--tt-border);

    &__tab {
      padding: 10px 0;
      font: inherit;
      font-size: 14px;
      color: var(--tt-text-2);
      white-space: nowrap;
      background: transparent;
      border: 0;
      cursor: pointer;

      &:hover {
        color: var(--tt-text);
      }

      &--active {
        font-weight: 600;
        color: var(--tt-accent);
        box-shadow: inset 0 -2px 0 var(--tt-accent);
      }
    }

    &__count {
      font-size: 12px;
      font-weight: 400;
      color: var(--tt-text-3);
    }
  }
</style>
