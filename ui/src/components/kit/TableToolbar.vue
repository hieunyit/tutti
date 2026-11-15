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
   * The one toolbar of every table: `[search] [Filters (n)] ... [bulk actions]`.
   * Filters (default slot) are collapsed by default, active ones show as
   * removable chips, and `hidden` (empty and unfiltered list) removes the bar.
   * Same props and events as FilterBar, which it wraps.
   */
  import FilterBar from './FilterBar.vue';
  import type { FilterChip } from './types';

  withDefaults(
    defineProps<{
      search?: string;
      searchLabel?: string;
      searchPlaceholder?: string;
      chips?: FilterChip[];
      noSearch?: boolean;
      hidden?: boolean;
      hideClear?: boolean;
    }>(),
    {
      search: '',
      searchLabel: undefined,
      searchPlaceholder: undefined,
      chips: () => [],
      noSearch: false,
      hidden: false,
      hideClear: false,
    },
  );

  const emit = defineEmits<{
    (e: 'update:search', value: string): void;
    (e: 'remove', key: string): void;
    (e: 'clear'): void;
  }>();
</script>

<template>
  <FilterBar
    collapsible
    :search="search"
    :search-label="searchLabel"
    :search-placeholder="searchPlaceholder"
    :chips="chips"
    :no-search="noSearch"
    :hidden="hidden"
    :hide-clear="hideClear"
    @update:search="emit('update:search', $event)"
    @remove="emit('remove', $event)"
    @clear="emit('clear')"
  >
    <template
      v-if="$slots.default"
      #default
    >
      <slot />
    </template>
    <template
      v-if="$slots.bulk"
      #bulk
    >
      <slot name="bulk" />
    </template>
  </FilterBar>
</template>
