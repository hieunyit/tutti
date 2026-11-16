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
  import { computed, h, onBeforeMount, onUnmounted, ref, watch } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { storeToRefs } from 'pinia';
  import { NButton, type DataTableColumns } from 'naive-ui';
  import i18nReplicationHub from './i18nReplicationHub';
  import {
    ConfirmDialog,
    DataTable,
    EmptyBlock,
    TableToolbar,
    type FilterChip,
  } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import RowActionsConfirm from '@/components/tutti/common/TuttiRowActions/RowActionsConfirm.vue';
  import ReplicationCardList from '@/components/tutti/replications/ReplicationCardList/ReplicationCardList.vue';
  import ReplicationNameCell from '@/components/tutti/replications/ReplicationNameCell/ReplicationNameCell.vue';
  import ReplicationRowMenu from '@/components/tutti/replications/ReplicationRowMenu/ReplicationRowMenu.vue';
  import ReplicationStatusCell from '@/components/tutti/replications/ReplicationStatusCell/ReplicationStatusCell.vue';
  import ReplicationsFilters from '@/components/tutti/replications/ReplicationsFilters/ReplicationsFilters.vue';
  import i18nReplicationRescan from '@/components/tutti/replications/i18nReplicationRescan';
  import { useSyncNow } from '@/utils/composables/useSyncNow';
  import i18nReplicationName from '@/components/tutti/replications/i18nReplicationName';
  import i18nReplications from '@/components/tutti/replications/i18nReplications';
  import { useNarrowScreen } from '@/components/tutti/storages/useNarrowScreen';
  import { useTuttiReplicationsStore } from '@/stores/tuttiReplicationsStore';
  import { useDebouncedSearch } from '@/utils/composables/useDebouncedSearch';
  import { useRowActions } from '@/utils/composables/useRowActions';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import { mergeMessages } from '@/utils/helpers/mergeMessages';
  import { ReplicationDetailHelper } from '@/utils/helpers/ReplicationDetailHelper';
  import { ReplicationNameHelper } from '@/utils/helpers/ReplicationNameHelper';
  import { ReplicationsHelper } from '@/utils/helpers/ReplicationsHelper';
  import type { AddId } from '@/utils/types/helper';
  import { RouteName } from '@/utils/types/router';
  import {
    ReplicationStatusFilter,
    ReplicationType,
    type TuttiReplication,
  } from '@/utils/types/tuttiCore';

  type Row = AddId<TuttiReplication>;

  const emit = defineEmits<{
    /** the list has no replications at all (no filter, no search) */
    (e: 'empty', value: boolean): void;
  }>();

  // One local composer only: a second useI18n({ messages }) in the same component returns the first composer and
  // silently drops its messages (the name column title rendered as its raw key).
  const { t } = useI18n({
    messages: mergeMessages(
      i18nReplications,
      i18nReplicationName,
      i18nReplicationHub,
      i18nReplicationRescan,
    ),
  });
  const sync = useSyncNow();
  const narrow = useNarrowScreen();
  const store = useTuttiReplicationsStore();
  const {
    isLoading,
    hasError,
    replications,
    computedReplications,
    isFiltered,
    filterUsers,
    filterBuckets,
    filterDirections,
    filterStatuses,
    filterType,
    pageSize,
    page,
  } = storeToRefs(store);

  const { search, debounced: debouncedSearch } = useDebouncedSearch();
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
    const needle = debouncedSearch.value.trim().toLowerCase();

    if (!needle) return computedReplications.value;

    return computedReplications.value.filter((row) =>
      [
        row.name,
        row.id.user,
        row.id.fromBucket,
        row.id.toBucket,
        row.id.fromStorage,
        row.id.toStorage,
      ].some((part) => part?.toLowerCase().includes(needle)),
    );
  });

  // empty and unfiltered: the empty state is the page, no toolbar, no header action
  const isEmptyList = computed(
    () =>
      !isLoading.value &&
      !hasError.value &&
      replications.value.length === 0 &&
      !isFiltered.value &&
      search.value === '',
  );

  watch(isEmptyList, (value) => emit('empty', value), { immediate: true });

  const rowKey = (row: Row) => row.idStr;
  const nameOf = (row: Row) =>
    ReplicationNameHelper.getDisplayName(row, t('allBuckets'));
  const rename = (row: Row, name: string) => store.renameReplication(row, name);

  const { actionsFor, pending, confirmPending, cancelPending } =
    useRowActions<Row>(
      [
        {
          key: 'pause',
          label: t('pause'),
          run: async (row) => {
            await store.pauseReplications([row]);
          },
          disabled: (row) => row.isPaused,
        },
        {
          key: 'resume',
          label: t('resume'),
          run: async (row) => {
            await store.resumeReplications([row]);
          },
          disabled: (row) => !row.isPaused,
        },
        {
          key: 'sync',
          label: t('syncNow'),
          run: (row) => sync.syncNow(row.id),
          disabled: (row) => sync.blockedReason(row) !== '',
          disabledReason: (row) => sync.blockedReason(row),
        },
        {
          key: 'delete',
          label: t('deleteReplication'),
          danger: true,
          run: async (row) => {
            await store.deleteReplications([row]);
          },
          confirm: (row) =>
            row.replicationType === ReplicationType.USER
              ? t('deleteUserReplicationText')
              : t('deleteBucketReplicationText'),
        },
      ],
      rowKey,
    );

  // pause only while running, resume only while paused
  const menuActions = (row: Row) =>
    actionsFor(row).filter(
      (action) => action.key !== (row.isPaused ? 'pause' : 'resume'),
    );

  const columns = computed<DataTableColumns<Row>>(() => [
    {
      title: t('nameColumn'),
      key: 'name',
      minWidth: 260,
      render: (row) =>
        h(ReplicationNameCell, {
          replication: row,
          rename: (name: string) => rename(row, name),
          detailTo: {
            name: RouteName.TUTTI_REPLICATION_DETAIL,
            query: ReplicationDetailHelper.toQuery(row.id),
          },
        }),
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
      width: 80,
      render: (row) =>
        h(ReplicationRowMenu, {
          actions: menuActions(row),
          label: nameOf(row),
        }),
    },
  ]);

  const statusLabels: Record<ReplicationStatusFilter, string> = {
    [ReplicationStatusFilter.ACTIVE]: 'filterStatusActive',
    [ReplicationStatusFilter.PAUSED]: 'filterStatusPaused',
    [ReplicationStatusFilter.INITIAL_IN_PROGRESS]:
      'filterStatusInitialInProgress',
    [ReplicationStatusFilter.INITIAL_DONE]: 'filterStatusInitialDone',
    [ReplicationStatusFilter.LIVE_BEHIND]: 'filterStatusLiveBehind',
    [ReplicationStatusFilter.LIVE_UP_TO_DATE]: 'filterStatusLiveUpToDate',
  };

  const chips = computed<FilterChip[]>(() => {
    const list: FilterChip[] = [];

    if (filterStatuses.value.length)
      list.push({
        key: 'statuses',
        label: t('chipStatuses', {
          value: filterStatuses.value.map((s) => t(statusLabels[s])).join(', '),
        }),
      });

    if (filterType.value)
      list.push({
        key: 'type',
        label: t('chipType', {
          value: t(
            filterType.value === ReplicationType.USER
              ? 'filterTypeUser'
              : 'filterTypeBucket',
          ),
        }),
      });

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
        label: t('chipStorages', {
          value: filterDirections.value.join(', '),
        }),
      });

    return list;
  });

  function removeChip(key: string) {
    if (key === 'users') filterUsers.value = [];
    else if (key === 'buckets') filterBuckets.value = [];
    else if (key === 'directions') filterDirections.value = [];
    else if (key === 'statuses') filterStatuses.value = [];
    else if (key === 'type') filterType.value = null;
  }

  function clearAll() {
    search.value = '';
    store.clearFilters();
  }

  const selectedRows = computed(() =>
    rows.value.filter((row) => selectedKeys.value.includes(rowKey(row))),
  );

  const table = ref<{ select: (keys: string[]) => void } | null>(null);
  // Pause only applies to running rows, Resume only to paused ones
  const pausableRows = computed(() =>
    selectedRows.value.filter((row) => !row.isPaused),
  );
  const resumableRows = computed(() =>
    selectedRows.value.filter((row) => row.isPaused),
  );

  /** clears the selection afterwards, keeping only the rows the action failed for */
  async function runBulk(
    action: (targets: Row[]) => Promise<Row[]>,
    targets: Row[],
  ) {
    const failed = await action(targets);

    table.value?.select(failed.map(rowKey));
  }

  function bulkDelete() {
    bulkDeleteOpen.value = false;

    return runBulk(store.deleteReplications, selectedRows.value);
  }

  // the empty block carries "Clear filters" when the filters left no row
  const isFilteredToZero = computed(
    () => !isEmptyList.value && !isLoading.value && rows.value.length === 0,
  );

  const targetOf = (row: Row) => ReplicationsHelper.getSubjectLabel(row);
</script>

<template>
  <div class="replications-tab">
    <TableToolbar
      v-model:search="search"
      :search-label="t('search')"
      :search-placeholder="t('searchPlaceholder')"
      :chips="chips"
      :hidden="isEmptyList"
      :hide-clear="isFilteredToZero"
      @remove="removeChip"
      @clear="clearAll"
    >
      <ReplicationsFilters />
    </TableToolbar>

    <ReplicationCardList
      v-if="narrow && rows.length > 0"
      :rows="rows"
      :rename="rename"
      :actions-for="menuActions"
      :all-buckets-text="t('allBuckets')"
    />
    <DataTable
      v-else
      ref="table"
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
            isEmptyList ? t('emptyReplicationsTitle') : t('emptyFilteredTitle')
          "
          :description="isEmptyList ? t('emptyReplicationsText') : undefined"
        >
          <WriteGate v-if="isEmptyList">
            <RouterLink :to="{ name: RouteName.TUTTI_ADD_REPLICATION }">
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
            v-if="pausableRows.length > 0"
            size="small"
            @click="runBulk(store.pauseReplications, pausableRows)"
          >
            {{ t('pauseSelected', { n: pausableRows.length }) }}
          </NButton>
          <NButton
            v-if="resumableRows.length > 0"
            size="small"
            @click="runBulk(store.resumeReplications, resumableRows)"
          >
            {{ t('resumeSelected', { n: resumableRows.length }) }}
          </NButton>
          <NButton
            size="small"
            type="error"
            @click="bulkDeleteOpen = true"
          >
            {{ t('deleteSelected', { n: keys.length }, keys.length) }}
          </NButton>
        </WriteGate>
      </template>
    </DataTable>

    <ConfirmDialog
      :open="bulkDeleteOpen"
      :title="
        t('deleteSelected', { n: selectedRows.length }, selectedRows.length)
      "
      danger
      :confirm-label="
        t('deleteSelected', { n: selectedRows.length }, selectedRows.length)
      "
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
