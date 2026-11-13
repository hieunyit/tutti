<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton, type DataTableColumns } from 'naive-ui';
  import { computed, h } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { RouterLink } from 'vue-router';
  import i18nStorages from './i18nStorages';
  import { DataTable, EmptyBlock } from '@/components/kit';
  import { RouteName } from '@/utils/types/router';
  import type { RuntimeStorage } from '@/utils/types/runtimeStorage';

  type StorageUser = NonNullable<RuntimeStorage['users']>[number];

  defineProps<{ users: StorageUser[] }>();
  const { t } = useI18n({ messages: i18nStorages });

  const columns = computed<DataTableColumns<StorageUser>>(() => [
    {
      title: t('user'),
      key: 'user',
      render: (row) =>
        h(
          RouterLink,
          {
            to: {
              name: RouteName.TUTTI_USER_DETAIL,
              params: { name: row.user },
            },
          },
          () => row.user,
        ),
    },
    { title: t('accessKey'), key: 'accessKeyId' },
    {
      title: t('source'),
      key: 'source',
      render: (row) => (row.source === 'config' ? t('config') : t('managed')),
    },
  ]);
</script>

<template>
  <DataTable
    :columns="columns"
    :rows="users"
    :loading="false"
    :row-key="(row: StorageUser) => row.user"
  >
    <template #empty>
      <EmptyBlock :title="t('searchEmpty')">
        <RouterLink :to="{ name: RouteName.TUTTI_USERS }">
          <NButton>{{ t('manageUsers') }}</NButton>
        </RouterLink>
      </EmptyBlock>
    </template>
  </DataTable>
</template>
