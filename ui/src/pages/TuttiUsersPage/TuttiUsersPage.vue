<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, onMounted, ref } from 'vue';
  import { RouterLink, useRouter } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { NButton, NFormItem, NSelect } from 'naive-ui';
  import {
    Callout,
    DataTable,
    EmptyBlock,
    PageShell,
    TableToolbar,
    type FilterChip,
  } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import UserCardList from '@/components/tutti/users/UserCardList.vue';
  import i18nUsers from '@/components/tutti/users/i18nUsers';
  import {
    detailLink,
    userColumns,
  } from '@/components/tutti/users/userColumns';
  import { useNarrowScreen } from '@/components/tutti/storages/useNarrowScreen';
  import { useTuttiUsersStore } from '@/stores/tuttiUsersStore';
  import { UserHelper } from '@/utils/helpers/UserHelper';
  import { RouteName } from '@/utils/types/router';
  import type { S3User } from '@/utils/types/users';

  const { t } = useI18n({ messages: i18nUsers });
  const store = useTuttiUsersStore();
  const router = useRouter();
  const narrow = useNarrowScreen();
  const search = ref('');
  const status = ref('all');

  const statusOptions = computed(() => [
    { label: t('all'), value: 'all' },
    { label: t('statusIncomplete'), value: 'incomplete' },
  ]);
  const chips = computed<FilterChip[]>(() =>
    status.value === 'all'
      ? []
      : [{ key: 'status', label: `${t('status')}: ${t('statusIncomplete')}` }],
  );
  const visible = computed(() =>
    store.users.filter(
      (user) =>
        user.name.toLowerCase().includes(search.value.trim().toLowerCase()) &&
        (status.value !== 'incomplete' || UserHelper.isIncomplete(user)),
    ),
  );
  const columns = computed(() => userColumns(t));
  const state = computed(() => {
    if (!store.isLoaded) return store.hasError ? 'error' : 'loading';

    return store.users.length === 0 ? 'empty' : 'ready';
  });

  function clearFilters() {
    search.value = '';
    status.value = 'all';
  }

  onMounted(() => store.load());
</script>

<template>
  <PageShell
    :title="t('users')"
    :state="state"
    @retry="store.load()"
  >
    <template
      v-if="state === 'ready'"
      #actions
    >
      <WriteGate>
        <RouterLink :to="{ name: RouteName.TUTTI_USER_CREATE }">
          <NButton type="primary">{{ t('createUser') }}</NButton>
        </RouterLink>
      </WriteGate>
    </template>
    <template #empty>
      <EmptyBlock
        :title="t('emptyTitle')"
        :description="t('emptyText')"
      >
        <WriteGate>
          <RouterLink :to="{ name: RouteName.TUTTI_USER_CREATE }">
            <NButton type="primary">{{ t('createUser') }}</NButton>
          </RouterLink>
        </WriteGate>
      </EmptyBlock>
    </template>

    <div class="users-page">
      <Callout
        v-if="!store.dynamicEnabled"
        tone="warning"
        >{{ t('notConfigured') }}</Callout
      >
      <TableToolbar
        v-model:search="search"
        :search-label="t('searchLabel')"
        :search-placeholder="t('searchLabel')"
        :chips="chips"
        @remove="status = 'all'"
        @clear="clearFilters"
      >
        <NFormItem
          :label="t('status')"
          label-placement="left"
          :show-feedback="false"
        >
          <NSelect
            v-model:value="status"
            :options="statusOptions"
            size="small"
          />
        </NFormItem>
      </TableToolbar>
      <UserCardList
        v-if="narrow && visible.length > 0"
        :users="visible"
      />
      <DataTable
        v-else
        :columns="columns"
        :rows="visible"
        :loading="store.isLoading && !store.isLoaded"
        :row-key="(user: S3User) => user.name"
        row-clickable
        @retry="store.load()"
        @row-click="(user: S3User) => router.push(detailLink(user.name))"
      >
        <template #empty>
          <EmptyBlock :title="t('noMatchesTitle')">
            <NButton @click="clearFilters">{{ t('clearFilters') }}</NButton>
          </EmptyBlock>
        </template>
      </DataTable>
    </div>
  </PageShell>
</template>

<style lang="scss" scoped>
  .users-page {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
    min-width: 0;

    :deep(.users-cell) {
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: var(--tt-space-1);

      small {
        color: var(--tt-text-3);
        font-size: var(--tt-text-xs);
        overflow-wrap: anywhere;
      }
    }
  }
</style>
