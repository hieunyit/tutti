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
  import { computed, h, onBeforeMount, onUnmounted, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { NButton, type DataTableColumns } from 'naive-ui';
  import i18nReplicationHub from './i18nReplicationHub';
  import {
    ConfirmDialog,
    DataTable,
    EmptyBlock,
    FilterBar,
    type FilterChip,
  } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import ChorusRowActions from '@/components/chorus/common/ChorusRowActions/ChorusRowActions.vue';
  import RowActionsConfirm from '@/components/chorus/common/ChorusRowActions/RowActionsConfirm.vue';
  import ReplicationBucketCell from '@/components/chorus/replications/ReplicationBucketCell/ReplicationBucketCell.vue';
  import ReplicationDirectionCell from '@/components/chorus/replications/ReplicationDirectionCell/ReplicationDirectionCell.vue';
  import ReplicationStatusCell from '@/components/chorus/replications/ReplicationStatusCell/ReplicationStatusCell.vue';
  import ReplicationsFilters from '@/components/chorus/replications/ReplicationsFilters/ReplicationsFilters.vue';
  import { useTuttiReplicationsStore } from '@/stores/tuttiReplicationsStore';
  import { useRowActions } from '@/utils/composables/useRowActions';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import type { ChorusReplication } from '@/utils/types/chorus';
  import type { AddId } from '@/utils/types/helper';
  import { RouteName } from '@/utils/types/router';

  type Row = AddId<ChorusReplication>;

  const { t } = useI18n({ messages: i18nReplicationHub });
  const store = useTuttiReplicationsStore();
  const {
    isLoading,
    hasError,
    computedReplications,
    isFiltered,
    filterUsers,
    filterBuckets,
    filterDirections,
    filterStatuses,
    filterCreatedAtRange,
    filterType,
    pageSize,
    page,
  } = storeToRefs(store);

  const search = ref('');
  const bulkDeleteOpen = ref(false);
  const selectedKeys = ref<string[]>([]);

  onBeforeMount(() => {
    // the kit table pages the rows itself, so the store hands over all of them
    pageSize.value = 100000;
    page.value = 1;
    store.initReplicationsPage();
  });

  onUnmounted(() => {
    store.$reset();
  });

  const rows = computed<Row[]>(() => {
    const needle = search.value.trim().toLowerCase();

    if (!needle) return computedReplications.value;

    return computedReplications.value.filter((row) =>
      [
        row.id.user,
        row.id.fromBucket,
        row.id.toBucket,
        row.id.fromStorage,
        row.id.toStorage,
      ].some((part) => part?.toLowerCase().includes(needle)),
    );
  });

  const rowKey = (row: Row) => row.idStr;
  const targetOf = (row: Row) => row.id.fromBucket || row.id.user;

  const { actionsFor, pending, confirmPending, cancelPending } =
    useRowActions<Row>(
      [
        {
          key: 'pause',
          label: t('pause'),
          run: (row) => store.pauseReplications([row]),
        },
        {
          key: 'resume',
          label: t('resume'),
          run: (row) => store.resumeReplications([row]),
        },
        {
          key: 'delete',
          label: t('deleteReplication'),
          danger: true,
          run: (row) => store.deleteReplications([row]),
          confirm: () => t('deleteReplicationText'),
        },
      ],
      rowKey,
    );

  const columns = computed<DataTableColumns<Row>>(() => [
    {
      title: t('colBucket'),
      key: 'bucket',
      minWidth: 220,
      render: (row) => h(ReplicationBucketCell, { replication: row }),
    },
    {
      title: t('colDirection'),
      key: 'direction',
      minWidth: 190,
      render: (row) => h(ReplicationDirectionCell, { replication: row }),
    },
    {
      title: t('colStatus'),
      key: 'status',
      minWidth: 220,
      render: (row) => h(ReplicationStatusCell, { replication: row }),
    },
    {
      title: t('colCreatedAt'),
      key: 'createdAt',
      width: 170,
      render: (row) => formatDateTime(row.createdAt),
    },
    {
      title: t('colActions'),
      key: 'actions',
      width: 260,
      render: (row) =>
        h(ChorusRowActions, {
          actions: actionsFor(row).filter(
            (action) => action.key !== (row.isPaused ? 'pause' : 'resume'),
          ),
        }),
    },
  ]);

  const chips = computed<FilterChip[]>(() => {
    const list: FilterChip[] = [];

    if (filterUsers.value.length)
      list.push({
        key: 'users',
        label: t('chipUsers', { value: filterUsers.value.join(', ') }),
      });

    if (filterBuckets.value.length)
      list.push({
        key: 'buckets',
        label: t('chipBuckets', { value: filterBuckets.value.join(', ') }),
      });

    if (filterDirections.value.length)
      list.push({
        key: 'directions',
        label: t('chipDirections', {
          value: filterDirections.value.join(', '),
        }),
      });

    if (filterStatuses.value.length)
      list.push({
        key: 'statuses',
        label: t('chipStatuses', { value: filterStatuses.value.join(', ') }),
      });

    if (filterType.value)
      list.push({
        key: 'type',
        label: t('chipType', { value: filterType.value }),
      });

    if (filterCreatedAtRange.value)
      list.push({ key: 'createdAt', label: t('chipCreatedAt') });

    return list;
  });

  function removeChip(key: string) {
    if (key === 'users') filterUsers.value = [];
    else if (key === 'buckets') filterBuckets.value = [];
    else if (key === 'directions') filterDirections.value = [];
    else if (key === 'statuses') filterStatuses.value = [];
    else if (key === 'type') filterType.value = null;
    else if (key === 'createdAt') filterCreatedAtRange.value = null;
  }

  function clearAll() {
    search.value = '';
    store.clearFilters();
  }

  const selectedRows = computed(() =>
    rows.value.filter((row) => selectedKeys.value.includes(rowKey(row))),
  );

  async function bulkDelete() {
    bulkDeleteOpen.value = false;
    await store.deleteReplications(selectedRows.value);
  }
</script>

<template>
  <div class="replications-tab">
    <FilterBar
      v-model:search="search"
      :search-label="t('search')"
      :search-placeholder="t('searchPlaceholder')"
      :chips="chips"
      @remove="removeChip"
      @clear="clearAll"
    >
      <ReplicationsFilters />
    </FilterBar>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="isLoading"
      :error="hasError ? t('loadError') : undefined"
      :row-key="rowKey"
      selectable
      @select="selectedKeys = $event"
      @retry="store.initReplicationsPage()"
    >
      <template #empty>
        <EmptyBlock
          :title="
            isFiltered || search
              ? t('emptyFilteredTitle')
              : t('emptyReplicationsTitle')
          "
          :description="
            isFiltered || search
              ? t('emptyFilteredText')
              : t('emptyReplicationsText')
          "
        >
          <WriteGate v-if="!isFiltered && !search">
            <RouterLink :to="{ name: RouteName.CHORUS_ADD_REPLICATION }">
              <NButton
                type="primary"
                tag="span"
              >
                {{ t('addReplication') }}
              </NButton>
            </RouterLink>
          </WriteGate>
          <NButton
            v-else
            @click="clearAll"
          >
            {{ t('clearFilters') }}
          </NButton>
        </EmptyBlock>
      </template>

      <template #bulk-actions="{ keys }">
        <WriteGate>
          <NButton
            size="small"
            @click="store.pauseReplications(selectedRows)"
          >
            {{ t('pauseSelected', { n: keys.length }) }}
          </NButton>
          <NButton
            size="small"
            @click="store.resumeReplications(selectedRows)"
          >
            {{ t('resumeSelected', { n: keys.length }) }}
          </NButton>
          <NButton
            size="small"
            type="error"
            @click="bulkDeleteOpen = true"
          >
            {{ t('deleteSelected', { n: keys.length }) }}
          </NButton>
        </WriteGate>
      </template>
    </DataTable>

    <ConfirmDialog
      :open="bulkDeleteOpen"
      :title="t('deleteSelected', { n: selectedRows.length })"
      danger
      :confirm-label="t('deleteSelected', { n: selectedRows.length })"
      @confirm="bulkDelete"
      @cancel="bulkDeleteOpen = false"
    >
      <p>{{ t('deleteSelectedText') }}</p>
    </ConfirmDialog>

    <RowActionsConfirm
      :pending="pending"
      :target="targetOf"
      @confirm="confirmPending"
      @cancel="cancelPending"
    />
  </div>
</template>

<style lang="scss" scoped>
  .replications-tab {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
  }
</style>
