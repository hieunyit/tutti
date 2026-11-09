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
  defineProps<{
    modelValue: string;
    placeholder: string;
    /** accessible name */
    label: string;
  }>();

  const emit = defineEmits<{
    (e: 'update:modelValue', value: string): void;
  }>();
</script>

<template>
  <label class="tt-search">
    <svg
      aria-hidden="true"
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
    >
      <circle
        cx="11"
        cy="11"
        r="7"
      />
      <path d="M20 20l-3.5-3.5" />
    </svg>
    <input
      type="search"
      class="tt-search__input"
      :value="modelValue"
      :placeholder="placeholder"
      :aria-label="label"
      @input="
        emit('update:modelValue', ($event.target as HTMLInputElement).value)
      "
    />
  </label>
</template>

<style lang="scss" scoped>
  .tt-search {
    display: flex;
    align-items: center;
    gap: 8px;
    box-sizing: border-box;
    width: 280px;
    max-width: 100%;
    height: 36px;
    padding: 0 10px;
    font-size: 13px;
    color: var(--tt-text-3);
    background-color: var(--tt-surface);
    border: 1px solid var(--tt-border-strong);
    border-radius: var(--tt-radius-control);

    &:focus-within {
      border-color: var(--tt-accent);
      outline: 2px solid var(--tt-accent-subtle);
    }

    &__input {
      width: 100%;
      min-width: 0;
      font: inherit;
      color: var(--tt-text);
      background: transparent;
      border: 0;
      outline: 0;

      &::placeholder {
        color: var(--tt-text-3);
      }
    }
  }
</style>
