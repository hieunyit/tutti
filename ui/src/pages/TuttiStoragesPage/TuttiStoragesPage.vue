<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  import { NButton, NFormItem, NSelect } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, onBeforeMount, onBeforeUnmount, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { useRouter } from 'vue-router';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import {
    DataTable,
    EmptyBlock,
    PageShell,
    TableToolbar,
    type FilterChip,
  } from '@/components/kit';
  import { useNumberFormat } from '@/components/tutti/monitoring/useNumberFormat';
  import AddStorageDialog from '@/components/tutti/storages/AddStorageDialog.vue';
  import StorageCardList from '@/components/tutti/storages/StorageCardList.vue';
  import i18nStorages from '@/components/tutti/storages/i18nStorages';
  import {
    storageColumns,
    storageDetailLocation,
  } from '@/components/tutti/storages/storageColumns';
  import { useNarrowScreen } from '@/components/tutti/storages/useNarrowScreen';
  import { useTuttiStoragesStore } from '@/stores/tuttiStoragesStore';
  import type { StorageRow } from '@/utils/helpers/StoragesPageHelper';

  const { t } = useI18n({ messages: i18nStorages });
  const number = useNumberFormat();
  const router = useRouter();
  const narrow = useNarrowScreen();
  const store = useTuttiStoragesStore();
  const { rows, isLoading, hasError, managedStoragesEnabled, runtimeStorages } =
    storeToRefs(store);
  const query = ref('');
  const health = ref('all');
  const origin = ref('all');
  const showAdd = ref(false);

  const options = (values: string[]) =>
    values.map((value) => ({ label: t(value), value }));
  const healthOptions = computed(() =>
    options(['all', 'available', 'unavailable', 'unknown']),
  );
  const originOptions = computed(() => options(['all', 'managed', 'config']));
  const chips = computed<FilterChip[]>(() => [
    ...(health.value !== 'all'
      ? [{ key: 'health', label: `${t('health')}: ${t(health.value)}` }]
      : []),
    ...(origin.value !== 'all'
      ? [{ key: 'origin', label: `${t('origin')}: ${t(origin.value)}` }]
      : []),
  ]);

  const runtime = (name: string) =>
    runtimeStorages.value.find((storage) => storage.name === name);
  const filtered = computed(() =>
    rows.value.filter(
      (row) =>
        (health.value === 'all' || row.health.status === health.value) &&
        (origin.value === 'all' ||
          (runtime(row.name)?.source ?? 'config') === origin.value) &&
        `${row.name} ${row.address}`
          .toLowerCase()
          .includes(query.value.trim().toLowerCase()),
    ),
  );
  const columns = computed(() => storageColumns({ t, number, runtime }));

  function removeChip(key: string) {
    if (key === 'health') health.value = 'all';
    else origin.value = 'all';
  }

  function clearFilters() {
    query.value = '';
    health.value = 'all';
    origin.value = 'all';
  }

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
    :state="state"
    :error-text="t('loadError')"
    @retry="store.refresh()"
  >
    <template
      v-if="state === 'ready'"
      #actions
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
      <TableToolbar
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
          :label="t('origin')"
          label-placement="left"
          :show-feedback="false"
        >
          <NSelect
            v-model:value="origin"
            :options="originOptions"
            size="small"
          />
        </NFormItem>
      </TableToolbar>
      <StorageCardList
        v-if="narrow && filtered.length > 0"
        :rows="filtered"
        :runtime="runtime"
      />
      <DataTable
        v-else
        :columns="columns"
        :rows="filtered"
        :loading="isLoading && !store.isLoaded"
        :row-key="(row: StorageRow) => row.name"
        row-clickable
        @retry="store.refresh()"
        @row-click="
          (row: StorageRow) => router.push(storageDetailLocation(row.name))
        "
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
  }
</style>
