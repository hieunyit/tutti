<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { type DataTableColumns } from 'naive-ui';
  import { computed, h } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nStorages from './i18nStorages';
  import { DataTable, EmptyBlock, StatusBadge } from '@/components/kit';
  import { useNumberFormat } from '@/components/tutti/monitoring/useNumberFormat';
  import {
    formatDateTime,
    formatRelative,
  } from '@/utils/helpers/DateTimeHelper';
  import type {
    TuttiProxyStatus,
    TuttiStorageHealth,
  } from '@/utils/types/tutti';

  interface HealthRow {
    proxy: TuttiProxyStatus;
    health?: TuttiStorageHealth;
    fresh: boolean;
  }

  const props = defineProps<{ rows: HealthRow[]; now: number }>();
  const { t } = useI18n({ messages: i18nStorages });
  const number = useNumberFormat();

  const columns = computed<DataTableColumns<HealthRow>>(() => [
    {
      title: t('name'),
      key: 'proxy',
      render: (row) =>
        h('div', [h('div', row.proxy.appId), h('small', row.proxy.address)]),
    },
    {
      title: t('health'),
      key: 'health',
      render: (row) => {
        const unknown = !row.fresh || row.health?.available === undefined;

        return h(StatusBadge, {
          tone: unknown
            ? 'neutral'
            : row.health?.available
              ? 'success'
              : 'danger',
          label: unknown
            ? t('unknown')
            : row.health?.available
              ? t('available')
              : t('unavailable'),
        });
      },
    },
    {
      title: t('failureCount'),
      key: 'failures',
      render: (row) =>
        row.fresh ? number(row.health?.consecutiveFailures ?? 0) : '–',
    },
    {
      title: t('lastError'),
      key: 'lastError',
      render: (row) => row.health?.lastError || '–',
    },
    {
      title: t('lastChange'),
      key: 'lastChange',
      render: (row) => formatDateTime(row.health?.lastChange),
    },
    {
      title: t('lastReport'),
      key: 'lastReport',
      render: (row) =>
        formatRelative(row.proxy.updatedAt, undefined, props.now),
    },
  ]);
</script>

<template>
  <DataTable
    :columns="columns"
    :rows="rows"
    :loading="false"
    :row-key="(row: HealthRow) => row.proxy.appId"
  >
    <template #empty>
      <EmptyBlock :title="t('noHealth')" />
    </template>
  </DataTable>
</template>
