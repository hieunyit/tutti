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
  import { actionTone, hasChanges } from './auditLabels';
  import { useAuditI18n } from './useAuditI18n';
  import {
    DataTable,
    EmptyBlock,
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
      render: (row) => row.actor || EMPTY_VALUE,
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
      key: 'object',
      title: t('colObject'),
      render: (row) =>
        h('div', [
          h('div', familyLabel(row.family)),
          h(
            'small',
            { class: 'tt-mono', style: 'color: var(--tt-text-3)' },
            row.entityKey || EMPTY_VALUE,
          ),
        ]),
    },
    {
      key: 'details',
      title: '',
      align: 'right',
      render: (row) =>
        hasChanges(row)
          ? h(
              NButton,
              {
                size: 'small',
                quaternary: true,
                onClick: () => emitOpen(row),
              },
              { default: () => t('showDiff') },
            )
          : null,
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
        <EmptyBlock
          v-if="store.hasActiveFilters"
          :title="t('noMatchTitle')"
        >
          <NButton @click="store.resetFilters()">{{
            t('clearFilters')
          }}</NButton>
        </EmptyBlock>
        <EmptyBlock
          v-else
          :title="t('emptyTitle')"
        />
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
