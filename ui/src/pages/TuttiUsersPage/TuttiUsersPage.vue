<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, h, onMounted, ref } from 'vue';
  import { RouterLink } from 'vue-router';
  import { useI18n } from 'vue-i18n';
  import { NButton, type DataTableColumns } from 'naive-ui';
  import {
    Callout,
    DataTable,
    EmptyBlock,
    FilterBar,
    PageShell,
    SegmentedFilter,
    StatusBadge,
  } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import messages from '@/components/tutti/users/i18nAccess';
  import { useTuttiUsersStore } from '@/stores/tuttiUsersStore';
  import { UserHelper } from '@/utils/helpers/UserHelper';
  import { RouteName } from '@/utils/types/router';
  import type { S3User } from '@/utils/types/users';

  const { t } = useI18n({ messages });
  const store = useTuttiUsersStore();
  const search = ref('');
  const segment = ref('all');

  const visible = computed(() =>
    store.users.filter(
      (user) =>
        user.name.toLowerCase().includes(search.value.trim().toLowerCase()) &&
        (segment.value !== 'incomplete' || UserHelper.isIncomplete(user)),
    ),
  );
  const filtered = computed(
    () => search.value.trim() !== '' || segment.value !== 'all',
  );
  const state = computed(() => {
    if (store.hasError) return 'error' as const;

    return store.isLoaded ? ('ready' as const) : ('loading' as const);
  });

  function clearFilters() {
    search.value = '';
    segment.value = 'all';
  }

  function keyCount(user: S3User) {
    return (
      (user.accessKeys?.static ?? 0) +
      (user.accessKeys?.dynamicActive ?? 0) +
      (user.accessKeys?.dynamicDisabled ?? 0)
    );
  }

  const columns = computed<DataTableColumns<S3User>>(() => [
    {
      title: t('user'),
      key: 'name',
      sorter: 'default',
      render: (user) =>
        h(
          RouterLink,
          {
            to: {
              name: RouteName.TUTTI_USER_DETAIL,
              params: { name: user.name },
            },
          },
          () => h('strong', user.name),
        ),
    },
    {
      title: t('status'),
      key: 'status',
      render: (user) =>
        UserHelper.isIncomplete(user)
          ? h(StatusBadge, {
              tone: 'warning',
              label: t('statusIncomplete'),
            })
          : h(StatusBadge, { tone: 'success', label: t('statusComplete') }),
    },
    {
      title: t('credentials'),
      key: 'credentials',
      render: (user) => {
        const parts = UserHelper.credentialParts(user).filter(
          (p) => p.kind !== 'missing',
        );

        return parts.length
          ? parts.map((part) => h('div', UserHelper.credentialText(part, t)))
          : '–';
      },
    },
    {
      title: t('permission'),
      key: 'permission',
      render: (user) =>
        h('div', [
          user.policy?.permission ?? 'read-write',
          h(
            'div',
            { class: 'users-mono' },
            user.policy?.buckets?.join(', ') || '*',
          ),
        ]),
    },
    {
      title: t('keys'),
      key: 'keys',
      align: 'right',
      render: (user) => String(keyCount(user)),
    },
    {
      title: t('source'),
      key: 'source',
      render: (user) =>
        t(
          user.storages?.some((s) => s.source === 'config')
            ? 'config'
            : 'managed',
        ),
    },
  ]);

  onMounted(() => store.load());
</script>

<template>
  <PageShell
    :title="t('users')"
    :description="t('usersHint')"
    :state="state"
    @retry="store.load()"
  >
    <template #actions>
      <NButton
        :loading="store.isLoading"
        @click="store.load()"
        >{{ t('refresh') }}</NButton
      >
      <WriteGate>
        <RouterLink :to="{ name: RouteName.TUTTI_USER_CREATE }">
          <NButton type="primary">{{ t('createUser') }}</NButton>
        </RouterLink>
      </WriteGate>
    </template>

    <Callout
      v-if="!store.dynamicEnabled"
      tone="warning"
      >{{ t('notConfigured') }}</Callout
    >
    <div class="users-toolbar">
      <FilterBar
        v-model:search="search"
        :search-label="t('searchLabel')"
        :search-placeholder="t('search')"
        @clear="clearFilters"
      />
      <SegmentedFilter
        v-model="segment"
        :label="t('users')"
        :options="[
          { value: 'all', label: t('all'), count: store.users.length },
          {
            value: 'incomplete',
            label: t('incomplete'),
            count: store.incompleteUsers.length,
          },
        ]"
      />
    </div>
    <DataTable
      :columns="columns"
      :rows="visible"
      :loading="store.isLoading"
      :row-key="(user: S3User) => user.name"
      @retry="store.load()"
    >
      <template #empty>
        <EmptyBlock
          v-if="filtered"
          :title="t('noMatchesTitle')"
          :description="t('noMatchesHint')"
        >
          <NButton @click="clearFilters">{{ t('clearFilters') }}</NButton>
        </EmptyBlock>
        <EmptyBlock
          v-else
          :title="t('emptyUsersTitle')"
          :description="t('emptyUsers')"
        >
          <WriteGate>
            <RouterLink :to="{ name: RouteName.TUTTI_USER_CREATE }">
              <NButton type="primary">{{ t('createUser') }}</NButton>
            </RouterLink>
          </WriteGate>
        </EmptyBlock>
      </template>
    </DataTable>
  </PageShell>
</template>

<style lang="scss" scoped>
  .users-toolbar {
    display: flex;
    flex-wrap: wrap;
    gap: var(--tt-space-3);
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--tt-space-4);
  }
</style>

<style lang="scss">
  .users-mono {
    font-family: var(--tt-mono);
    font-size: var(--tt-text-xs);
    color: var(--tt-text-3);
  }
</style>
