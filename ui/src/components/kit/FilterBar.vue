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
  /**
   * Search box + filter controls (default slot) + active filters as
   * removable chips + "Clear filters". v-model:search for the text; the page
   * owns the filter state and answers `remove(key)` and `clear`.
   */
  import { computed, useId } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, NInput } from 'naive-ui';
  import i18nKit from './i18nKit';
  import type { FilterChip } from './types';

  const props = withDefaults(
    defineProps<{
      search?: string;
      searchLabel?: string;
      searchPlaceholder?: string;
      chips?: FilterChip[];
      /** hide the search box (filters only) */
      noSearch?: boolean;
    }>(),
    {
      search: '',
      searchLabel: undefined,
      searchPlaceholder: undefined,
      chips: () => [],
      noSearch: false,
    },
  );

  const emit = defineEmits<{
    (e: 'update:search', value: string): void;
    (e: 'remove', key: string): void;
    (e: 'clear'): void;
  }>();

  const { t } = useI18n({ messages: i18nKit });
  const inputId = `kit-filter-${useId()}`;
  const hasFilters = computed(
    () => props.search.trim() !== '' || props.chips.length > 0,
  );

  function clear() {
    if (props.search !== '') emit('update:search', '');

    emit('clear');
  }
</script>

<template>
  <div
    class="kit-filter"
    role="search"
  >
    <div class="kit-filter__controls">
      <div
        v-if="!noSearch"
        class="kit-filter__search"
      >
        <label
          class="tt-sr-only"
          :for="inputId"
          >{{ searchLabel ?? t('search') }}</label
        >
        <NInput
          :value="search"
          clearable
          :placeholder="searchPlaceholder ?? t('search')"
          :input-props="{ id: inputId, type: 'search' }"
          @update:value="emit('update:search', $event ?? '')"
        />
      </div>
      <slot />
      <NButton
        v-if="hasFilters"
        quaternary
        class="kit-filter__clear"
        @click="clear"
      >
        {{ t('clearFilters') }}
      </NButton>
    </div>
    <ul
      v-if="chips.length > 0"
      class="kit-filter__chips"
    >
      <li
        v-for="chip in chips"
        :key="chip.key"
        class="kit-filter__chip"
      >
        {{ chip.label }}
        <button
          type="button"
          class="kit-filter__chip-remove"
          :aria-label="t('removeFilter', { label: chip.label })"
          @click="emit('remove', chip.key)"
        >
          <span aria-hidden="true">×</span>
        </button>
      </li>
    </ul>
  </div>
</template>

<style lang="scss" scoped>
  .kit-filter {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-2);

    &__controls {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
    }

    &__search {
      flex: 1 1 240px;
      max-width: 360px;
    }

    &__clear {
      margin-left: auto;
    }

    &__chips {
      display: flex;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
      margin: 0;
      padding: 0;
      list-style: none;
    }

    &__chip {
      display: inline-flex;
      align-items: center;
      gap: var(--tt-space-1);
      padding: 2px 4px 2px var(--tt-space-2);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-pill);
      background-color: var(--tt-subtle);
      font-size: var(--tt-text-xs);
      line-height: 18px;
      color: var(--tt-text);
    }

    &__chip-remove {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 24px;
      height: 24px;
      border: 0;
      border-radius: 50%;
      background: transparent;
      font-size: var(--tt-text-md);
      color: var(--tt-text-3);
      cursor: pointer;

      &:hover {
        background-color: var(--tt-hover);
        color: var(--tt-text);
      }
    }

    @media screen and (max-width: 639px) {
      &__search {
        max-width: none;
        flex-basis: 100%;
      }
    }
  }
</style>
