<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton, NFormItem, NSelect, type DataTableColumns } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, h, onBeforeMount, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { RouterLink } from 'vue-router';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import {
    DataTable,
    EmptyBlock,
    FilterBar,
    PageShell,
    StatTile,
    StatusBadge,
    type FilterChip,
    type StatusTone,
  } from '@/components/kit';
  import { useNumberFormat } from '@/components/tutti/monitoring/useNumberFormat';
  import AddStorageDialog from '@/components/tutti/storages/AddStorageDialog.vue';
  import i18nStorages from '@/components/tutti/storages/i18nStorages';
  import { useTuttiStoragesStore } from '@/stores/tuttiStoragesStore';
  import type { StorageRow } from '@/utils/helpers/StoragesPageHelper';
  import { StorageTestHelper } from '@/utils/helpers/StorageTestHelper';
  import { RouteName } from '@/utils/types/router';

  const { t } = useI18n({ messages: i18nStorages });
  const number = useNumberFormat();
  const store = useTuttiStoragesStore();
  const {
    rows,
    summary,
    isLoading,
    hasError,
    managedStoragesEnabled,
    runtimeStorages,
  } = storeToRefs(store);
  const query = ref('');
  const health = ref('all');
  const source = ref('all');
  const showAdd = ref(false);

  const healthOptions = computed(() =>
    ['all', 'available', 'unavailable', 'unknown'].map((value) => ({
      label: t(value),
      value,
    })),
  );
  const sourceOptions = computed(() =>
    ['all', 'managed', 'config'].map((value) => ({
      label: t(value),
      value,
    })),
  );
  const chips = computed<FilterChip[]>(() => [
    ...(health.value !== 'all'
      ? [{ key: 'health', label: `${t('health')}: ${t(health.value)}` }]
      : []),
    ...(source.value !== 'all'
      ? [{ key: 'source', label: `${t('sourceFilter')}: ${t(source.value)}` }]
      : []),
  ]);

  const runtime = (name: string) =>
    runtimeStorages.value.find((storage) => storage.name === name);
  const filtered = computed(() =>
    rows.value.filter(
      (row) =>
        (health.value === 'all' || row.health.status === health.value) &&
        (source.value === 'all' ||
          (runtime(row.name)?.source ?? 'config') === source.value) &&
        `${row.name} ${row.address}`
          .toLowerCase()
          .includes(query.value.trim().toLowerCase()),
    ),
  );

  const detail = (name: string) => ({
    name: RouteName.CHORUS_STORAGE_DETAILS,
    params: { storageName: name },
  });
  const healthTone = (status: string): StatusTone =>
    status === 'available'
      ? 'success'
      : status === 'unavailable'
        ? 'danger'
        : 'neutral';

  function removeChip(key: string) {
    if (key === 'health') health.value = 'all';
    else source.value = 'all';
  }

  function clearFilters() {
    query.value = '';
    health.value = 'all';
    source.value = 'all';
  }

  function credentialsCell(name: string) {
    const item = runtime(name);
    const failed =
      StorageTestHelper.counts(item?.lastTest).fail > 0 ||
      item?.lastTest?.ok === false;

    return h('div', { class: 'storages-cell' }, [
      h(StatusBadge, {
        tone: item?.legacyCredentials ? 'warning' : 'success',
        label: item?.legacyCredentials ? t('legacyBadge') : t('accountBadge'),
      }),
      failed
        ? h(StatusBadge, { tone: 'danger', label: t('testFailedBadge') })
        : item?.lastTest?.hasWarnings
          ? h(StatusBadge, { tone: 'warning', label: t('testWarnBadge') })
          : null,
    ]);
  }

  const columns = computed<DataTableColumns<StorageRow>>(() => [
    {
      title: t('name'),
      key: 'name',
      sorter: 'default',
      render: (row) =>
        h('div', { class: 'storages-cell' }, [
          h(
            RouterLink,
            { to: detail(row.name), class: 'storages-link' },
            () => row.name,
          ),
          h('small', row.address),
        ]),
    },
    {
      title: t('health'),
      key: 'health',
      render: (row) =>
        h('div', { class: 'storages-cell' }, [
          h(StatusBadge, {
            tone: healthTone(row.health.status),
            label: t(row.health.status),
          }),
          row.health.warning
            ? h(StatusBadge, {
                tone: 'warning',
                label: t('errorBadge'),
              })
            : null,
          h('small', `${row.health.reporting} ${t('reporting')}`),
        ]),
    },
    {
      title: t('breaker'),
      key: 'breaker',
      render: (row) =>
        h('div', { class: 'storages-cell' }, [
          t(row.breaker.state),
          h('small', `${number(row.breaker.failures)} ${t('failureCount')}`),
        ]),
    },
    {
      title: t('requests'),
      key: 'requestsPerSecond',
      sorter: (a, b) => a.requestsPerSecond - b.requestsPerSecond,
      render: (row) => number(row.requestsPerSecond),
    },
    {
      title: t('users'),
      key: 'users',
      render: (row) =>
        h(
          RouterLink,
          {
            to: { ...detail(row.name), query: { tab: 'users' } },
            class: 'storages-link',
          },
          () => number(row.users.count),
        ),
    },
    {
      title: t('replication'),
      key: 'replication',
      render: (row) =>
        h('div', { class: 'storages-cell' }, [
          `${number(row.replication.sourceOf)} → ${number(row.replication.targetOf)}`,
          h('small', `${t('lag')}: ${number(row.replication.lag)}`),
        ]),
    },
    {
      title: t('source'),
      key: 'source',
      render: (row) =>
        runtime(row.name)?.source === 'managed' ? t('managed') : t('config'),
    },
    {
      title: t('credentialsColumn'),
      key: 'credentials',
      render: (row) => credentialsCell(row.name),
    },
  ]);

  // one state for the whole page; a failed refresh keeps the table on screen
  const state = computed(() => {
    if (!store.isLoaded) return hasError.value ? 'error' : 'loading';

    return rows.value.length === 0 ? 'empty' : 'ready';
  });

  onBeforeMount(store.startPolling);
  onBeforeUnmount(store.stopPolling);
</script>

<template>
  <PageShell
    :title="t('title')"
    :description="t('description')"
    :state="state"
    :error-text="t('loadError')"
    @retry="store.refresh()"
  >
    <template #actions>
      <NButton
        :loading="isLoading"
        @click="store.refresh()"
        >{{ t('refresh') }}</NButton
      >
      <WriteGate>
        <NButton
          type="primary"
          :disabled="!managedStoragesEnabled"
          @click="showAdd = true"
          >{{ t('add') }}</NButton
        >
      </WriteGate>
    </template>
    <template #empty>
      <EmptyBlock
        :title="t('emptyTitle')"
        :description="t('emptyText')"
      >
        <WriteGate>
          <NButton
            type="primary"
            :disabled="!managedStoragesEnabled"
            @click="showAdd = true"
            >{{ t('add') }}</NButton
          >
        </WriteGate>
      </EmptyBlock>
    </template>

    <div class="storages-page">
      <p
        v-if="hasError"
        class="storages-page__alert"
        role="alert"
      >
        {{ t('loadError') }}
      </p>
      <p
        v-else-if="!managedStoragesEnabled"
        class="storages-page__hint"
      >
        {{ t('disabledHint') }}
      </p>
      <div class="storages-page__tiles">
        <StatTile
          :label="t('available')"
          :value="`${number(summary.available)} / ${number(summary.total)}`"
          :tone="summary.worst ? 'danger' : 'success'"
          :hint="summary.worst?.name"
        />
        <StatTile
          :label="t('requests')"
          :value="number(summary.requestsPerSecond)"
        />
        <StatTile
          :label="t('coverage')"
          :value="`${number(summary.usersComplete)} / ${number(summary.usersTotal)}`"
          :tone="summary.firstIncomplete ? 'warning' : 'success'"
          :hint="summary.firstIncomplete?.name"
        />
        <StatTile
          :label="t('reporting')"
          :value="`${number(summary.proxiesReporting)} / ${number(summary.proxiesTotal)}`"
        />
      </div>
      <FilterBar
        v-model:search="query"
        :search-label="t('filter')"
        :search-placeholder="t('filter')"
        :chips="chips"
        @remove="removeChip"
        @clear="clearFilters"
      >
        <NFormItem
          :label="t('health')"
          label-placement="left"
          :show-feedback="false"
        >
          <NSelect
            v-model:value="health"
            :options="healthOptions"
            size="small"
          />
        </NFormItem>
        <NFormItem
          :label="t('sourceFilter')"
          label-placement="left"
          :show-feedback="false"
        >
          <NSelect
            v-model:value="source"
            :options="sourceOptions"
            size="small"
          />
        </NFormItem>
      </FilterBar>
      <DataTable
        :columns="columns"
        :rows="filtered"
        :loading="isLoading && !store.isLoaded"
        :row-key="(row: StorageRow) => row.name"
        @retry="store.refresh()"
      >
        <template #empty>
          <EmptyBlock :title="t('empty')">
            <NButton @click="clearFilters">{{ t('clearFilters') }}</NButton>
          </EmptyBlock>
        </template>
      </DataTable>
    </div>
  </PageShell>
  <AddStorageDialog
    v-model:show="showAdd"
    @created="store.refresh()"
  />
</template>

<style scoped lang="scss">
  .storages-page {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
    min-width: 0;

    &__tiles {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: var(--tt-space-4);
    }

    &__alert {
      margin: 0;
      color: var(--tt-danger);
      font-size: var(--tt-text-sm);
    }

    &__hint {
      margin: 0;
      max-width: var(--tt-measure);
      color: var(--tt-text-3);
      font-size: var(--tt-text-sm);
    }

    :deep(.storages-cell) {
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

    :deep(.storages-link) {
      color: var(--tt-accent);
      font-weight: var(--tt-weight-strong);
      text-decoration: none;
    }

    @media (max-width: 1000px) {
      &__tiles {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
    }

    @media (max-width: 440px) {
      &__tiles {
        gap: var(--tt-space-2);
      }
    }
  }
</style>
