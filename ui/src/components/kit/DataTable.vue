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

<script setup lang="ts" generic="T extends DataTableRowData">
  /**
   * NDataTable with the kit states built in: skeleton on first load, the
   * `empty` slot (or a default EmptyBlock) for zero rows, ErrorBlock with
   * retry when `error` is set, multi-select with a bulk action bar (slot
   * `bulk-actions`, slot props `{ keys, clear }`), client paging above
   * `pageSize` rows and row density (prop, else the saved user choice).
   */
  import { computed, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import {
    NButton,
    NDataTable,
    type DataTableColumns,
    type DataTableRowData,
  } from 'naive-ui';
  import { EmptyBlock, ErrorBlock, LoadingBlock } from './stateBlocks';
  import { useTableDensity } from './composables';
  import i18nKit from './i18nKit';

  const props = withDefaults(
    defineProps<{
      columns: DataTableColumns<T>;
      rows: T[];
      loading: boolean;
      error?: string;
      rowKey: (r: T) => string;
      selectable?: boolean;
      density?: 'comfortable' | 'compact';
      /** rows per page; paging only shows when there are more rows */
      pageSize?: number;
      /** when true, clicking a row emits `rowClick` (and shows a pointer) */
      rowClickable?: boolean;
    }>(),
    {
      error: undefined,
      selectable: false,
      density: undefined,
      pageSize: 20,
      rowClickable: false,
    },
  );

  const emit = defineEmits<{
    (e: 'retry'): void;
    (e: 'select', keys: string[]): void;
    (e: 'rowClick', row: T): void;
  }>();

  const { t } = useI18n({ messages: i18nKit });
  const savedDensity = useTableDensity();
  const checked = ref<string[]>([]);

  const effectiveDensity = computed(() => props.density ?? savedDensity.value);
  const allColumns = computed<DataTableColumns<T>>(() =>
    props.selectable
      ? [{ type: 'selection' }, ...props.columns]
      : props.columns,
  );
  const pagination = computed(() =>
    props.rows.length > props.pageSize
      ? { pageSize: props.pageSize }
      : (false as const),
  );
  const isFirstLoad = computed(() => props.loading && props.rows.length === 0);
  const isEmpty = computed(
    () => !props.loading && !props.error && props.rows.length === 0,
  );

  // drop selections whose rows are gone (deleted, filtered out)
  watch(
    () => props.rows,
    (rows) => {
      if (checked.value.length === 0) return;

      const present = new Set(rows.map((row) => props.rowKey(row)));
      const next = checked.value.filter((key) => present.has(key));

      if (next.length !== checked.value.length) setChecked(next);
    },
  );

  function setChecked(keys: (string | number)[]) {
    checked.value = keys.map(String);
    emit('select', checked.value);
  }

  function rowProps(row: T) {
    if (!props.rowClickable) return {};

    return {
      class: 'kit-table__row--clickable',
      onClick: (ev: MouseEvent) => {
        // ignore clicks on interactive children (buttons, links, checkboxes, menus)
        const target = ev.target as HTMLElement | null;

        if (target?.closest('a,button,input,label,.n-dropdown,.n-checkbox')) {
          return;
        }

        emit('rowClick', row);
      },
    };
  }

  function clear() {
    setChecked([]);
  }
</script>

<template>
  <div
    class="kit-table"
    :class="`kit-table--${effectiveDensity}`"
  >
    <ErrorBlock
      v-if="error"
      :description="error"
      @retry="emit('retry')"
    />
    <LoadingBlock
      v-else-if="isFirstLoad"
      :rows="5"
    />
    <template v-else-if="isEmpty">
      <slot name="empty">
        <EmptyBlock />
      </slot>
    </template>
    <template v-else>
      <div
        v-if="selectable && checked.length > 0"
        class="kit-table__bulk"
        role="region"
        :aria-label="t('selected', { n: checked.length })"
      >
        <span class="kit-table__bulk-count">{{
          t('selected', { n: checked.length })
        }}</span>
        <slot
          name="bulk-actions"
          :keys="checked"
          :clear="clear"
        />
        <NButton
          quaternary
          size="small"
          class="kit-table__bulk-clear"
          @click="clear"
        >
          {{ t('clearSelection') }}
        </NButton>
      </div>
      <div class="kit-table__scroll">
        <NDataTable
          :columns="allColumns"
          :data="rows"
          :loading="loading"
          :row-key="rowKey"
          :checked-row-keys="checked"
          :pagination="pagination"
          :size="effectiveDensity === 'compact' ? 'small' : 'medium'"
          :row-props="rowProps"
          :bordered="false"
          :single-line="true"
          @update:checked-row-keys="setChecked"
        />
      </div>
    </template>
  </div>
</template>

<style lang="scss" scoped>
  .kit-table {
    min-width: 0;
    border: 1px solid var(--tt-border);
    border-radius: var(--tt-radius-card);
    background-color: var(--tt-surface);
    overflow: hidden;

    &__bulk {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
      padding: var(--tt-space-2) var(--tt-space-4);
      border-bottom: 1px solid var(--tt-border);
      background-color: var(--tt-accent-subtle);
    }

    &__bulk-count {
      font-size: var(--tt-text-sm);
      font-weight: var(--tt-weight-strong);
      color: var(--tt-text);
    }

    &__bulk-clear {
      margin-left: auto;
    }

    &__scroll {
      overflow-x: auto;
      max-width: 100%;
    }

    :deep(.kit-table__row--clickable) {
      cursor: pointer;
    }

    &--comfortable :deep(.n-data-table-td) {
      height: var(--tt-row-comfortable);
    }

    &--compact :deep(.n-data-table-td) {
      height: var(--tt-row-compact);
    }
  }
</style>
