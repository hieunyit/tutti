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
  /** Audit entries in a kit DataTable plus cursor paging (older / newer). */
  import { computed, h } from 'vue';
  import { NButton, NTooltip, type DataTableColumns } from 'naive-ui';
  import { actionTone, useAuditI18n } from './useAuditI18n';
  import {
    DataTable,
    EMPTY_VALUE,
    StatusBadge,
    type StatusTone,
  } from '@/components/kit';
  import { useAuditLogStore } from '@/stores/tuttiAuditLogStore';
  import {
    formatDateTime,
    formatRelative,
  } from '@/utils/helpers/DateTimeHelper';
  import type { AuditLogEntry } from '@/utils/types/audit';

  const emitter = defineEmits<{ (e: 'open', entry: AuditLogEntry): void }>();

  const store = useAuditLogStore();
  const { t, familyLabel, actionLabel } = useAuditI18n();

  const columns = computed<DataTableColumns<AuditLogEntry>>(() => [
    {
      key: 'id',
      title: t('colId'),
      width: 90,
      render: (row) => h('span', { class: 'tt-mono' }, `#${row.id}`),
    },
    {
      key: 'at',
      title: t('colTimestamp'),
      width: 190,
      render: (row) =>
        h(NTooltip, null, {
          trigger: () =>
            h('div', [
              h('div', formatDateTime(row.at)),
              h(
                'small',
                { style: 'color: var(--tt-text-3)' },
                formatRelative(row.at),
              ),
            ]),
          default: () => row.at || EMPTY_VALUE,
        }),
    },
    {
      key: 'actor',
      title: t('colActor'),
      render: (row) =>
        h('div', [
          h('strong', row.actor || EMPTY_VALUE),
          row.requestId
            ? h(
                'div',
                { class: 'tt-mono', style: 'font-size: 11px' },
                `${t('requestIdPrefix')} ${row.requestId}`,
              )
            : null,
        ]),
    },
    {
      key: 'action',
      title: t('colAction'),
      render: (row) =>
        h(StatusBadge, {
          tone: actionTone(row.action) as StatusTone,
          label: actionLabel(row.action),
        }),
    },
    {
      key: 'family',
      title: t('colFamily'),
      render: (row) => familyLabel(row.family),
    },
    {
      key: 'entityKey',
      title: t('colEntityKey'),
      render: (row) =>
        h('span', { class: 'tt-mono' }, row.entityKey || EMPTY_VALUE),
    },
    {
      key: 'details',
      title: t('colDetails'),
      align: 'right',
      render: (row) =>
        h(
          NButton,
          { size: 'small', onClick: () => emitOpen(row) },
          { default: () => t('showDiff') },
        ),
    },
  ]);

  function emitOpen(entry: AuditLogEntry) {
    emitter('open', entry);
  }

  const meta = computed(() => t('tableMeta', { count: store.entries.length }));
</script>

<template>
  <div class="audit-table">
    <p
      v-if="store.error"
      class="audit-table__error"
      role="alert"
    >
      {{ t('loadMoreFailed') }}
      <NButton
        text
        type="primary"
        @click="store.loadOlder()"
        >{{ t('retry') }}</NButton
      >
    </p>
    <DataTable
      :columns="columns"
      :rows="store.entries"
      :loading="store.loading && store.entries.length === 0"
      :row-key="(row: AuditLogEntry) => String(row.id)"
      :page-size="200"
    >
      <template #empty>
        <div class="audit-table__empty">
          <strong>{{ t('emptyTitle') }}</strong>
          <p>{{ t('emptyText') }}</p>
          <NButton
            v-if="store.hasActiveFilters"
            @click="store.resetFilters()"
            >{{ t('resetFilters') }}</NButton
          >
        </div>
      </template>
    </DataTable>
    <footer
      v-if="store.entries.length > 0"
      class="audit-table__footer"
    >
      <span class="audit-table__meta">{{
        store.hasOlder ? meta : `${meta} · ${t('allLoaded')}`
      }}</span>
      <div class="audit-table__pager">
        <NButton
          size="small"
          :disabled="store.loading"
          @click="store.loadNewer()"
          >{{ t('loadNewer') }}</NButton
        >
        <NButton
          size="small"
          :disabled="store.loading || !store.hasOlder"
          :loading="store.loading"
          @click="store.loadOlder()"
          >{{ t('loadOlder') }}</NButton
        >
      </div>
    </footer>
  </div>
</template>

<style lang="scss" scoped>
  .audit-table {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);

    &__error {
      margin: 0;
      color: var(--tt-danger);
      font-size: var(--tt-text-sm);
    }

    &__empty {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: var(--tt-space-2);
      padding: var(--tt-space-7) var(--tt-space-4);
      color: var(--tt-text-2);
      text-align: center;

      p {
        margin: 0;
        max-width: var(--tt-measure);
      }
    }

    &__footer {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: space-between;
      gap: var(--tt-space-3);
    }

    &__meta {
      font-size: var(--tt-text-sm);
      color: var(--tt-text-3);
    }

    &__pager {
      display: inline-flex;
      gap: var(--tt-space-2);
    }
  }
</style>
