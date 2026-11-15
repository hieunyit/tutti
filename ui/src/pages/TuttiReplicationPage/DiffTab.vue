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
  import DiffReportsCardList from '@/components/tutti/diff-reports/DiffReportsCardList/DiffReportsCardList.vue';
  import DiffReportsFilters from '@/components/tutti/diff-reports/DiffReportsFilters/DiffReportsFilters.vue';
  import DiffReportsNameCell from '@/components/tutti/diff-reports/DiffReportsNameCell/DiffReportsNameCell.vue';
  import DiffReportsStatusCell from '@/components/tutti/diff-reports/DiffReportsStatusCell/DiffReportsStatusCell.vue';
  import ReplicationRowMenu from '@/components/tutti/replications/ReplicationRowMenu/ReplicationRowMenu.vue';
  import { useNarrowScreen } from '@/components/tutti/storages/useNarrowScreen';
  import { useTuttiDiffReportsStore } from '@/stores/tuttiDiffReportsStore';
  import { useDebouncedSearch } from '@/utils/composables/useDebouncedSearch';
  import { useRowActions } from '@/utils/composables/useRowActions';
  import { useTuttiNotification } from '@/utils/composables/useTuttiNotification';
  import { DiffReportsHelper } from '@/utils/helpers/DiffReportsHelper';
  import {
    DiffReportStatusFilter,
    type DiffReport,
  } from '@/utils/types/tuttiCore';
  import type { AddId } from '@/utils/types/helper';
  import { RouteName } from '@/utils/types/router';

  type Row = AddId<DiffReport>;

  const emit = defineEmits<{
    /** the list has no reports at all (no filter, no search) */
    (e: 'empty', value: boolean): void;
  }>();

  const { t } = useI18n({ messages: i18nReplicationHub });
  const narrow = useNarrowScreen();
  const store = useTuttiDiffReportsStore();
  const {
    isLoading,
    hasError,
    reports,
    computedReports,
    isFiltered,
    filterDirections,
    filterBuckets,
    filterStatuses,
    pageSize,
    page,
  } = storeToRefs(store);
  const { createNotification } = useTuttiNotification();

  const { search, debounced: debouncedSearch } = useDebouncedSearch();
  const selectedKeys = ref<string[]>([]);
  const bulkDeleteOpen = ref(false);
  const bulkRestartOpen = ref(false);

  onBeforeMount(() => {
    // the kit table pages the rows itself, so the store hands over all of them
    pageSize.value = 100000;
    page.value = 1;
    store.initDiffReportPage();
  });

  onUnmounted(() => {
    store.$reset();
  });

  // reports created via CLI (not two locations) stay listed so they can be
  // seen and deleted; they just have no status to show
  const rows = computed<Row[]>(() => {
    const needle = debouncedSearch.value.trim().toLowerCase();
    const all = computedReports.value;

    if (!needle) return all;

    return all.filter((row) =>
      row.locations.some(
        (location) =>
          location.storage.toLowerCase().includes(needle) ||
          location.bucket.toLowerCase().includes(needle),
      ),
    );
  });

  const isEmptyList = computed(
    () =>
      !isLoading.value &&
      !hasError.value &&
      reports.value.length === 0 &&
      !isFiltered.value &&
      search.value === '',
  );

  watch(isEmptyList, (value) => emit('empty', value), { immediate: true });

  const rowKey = (row: Row) => String(row.idStr);
  const isTwo = (row: Row) => DiffReportsHelper.isTwoLocationReport(row);
  const labelOf = (row: Row) =>
    row.locations
      .map((location) => `${location.storage}/${location.bucket}`)
      .join(' → ');

  function report(result: { errorList: unknown[] }) {
    if (result.errorList.length) {
      createNotification({ type: 'error', title: t('actionFailed') });
    } else {
      createNotification({
        type: 'success',
        title: t('actionDone'),
        duration: 4000,
      });
    }
  }

  const { actionsFor, pending, confirmPending, cancelPending } =
    useRowActions<Row>(
      [
        {
          key: 'restart',
          label: t('restart'),
          danger: true,
          run: async (row) => report(await store.restartDiffReports([row])),
          confirm: () => t('restartDiffReportConfirmContent'),
        },
        {
          key: 'delete',
          label: t('deleteReport'),
          danger: true,
          run: async (row) => report(await store.deleteDiffReports([row])),
          confirm: () => t('deleteReportText'),
        },
      ],
      rowKey,
    );

  // CLI reports can only be deleted
  const menuActions = (row: Row) =>
    actionsFor(row).filter((action) => isTwo(row) || action.key === 'delete');

  const columns = computed<DataTableColumns<Row>>(() => [
    {
      title: t('colReport'),
      key: 'report',
      minWidth: 280,
      render: (row) => h(DiffReportsNameCell, { report: row }),
    },
    {
      title: t('colStatus'),
      key: 'status',
      minWidth: 160,
      render: (row) =>
        isTwo(row) ? h(DiffReportsStatusCell, { report: row }) : null,
    },
    {
      title: t('colActions'),
      key: 'actions',
      width: 80,
      render: (row) =>
        h(ReplicationRowMenu, {
          actions: menuActions(row),
          label: labelOf(row),
        }),
    },
  ]);

  const statusLabels: Record<DiffReportStatusFilter, string> = {
    [DiffReportStatusFilter.CHECKING]: 'statusChecking',
    [DiffReportStatusFilter.CONSISTENT]: 'statusConsistent',
    [DiffReportStatusFilter.INCONSISTENT]: 'statusInconsistent',
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

    if (filterDirections.value.length)
      list.push({
        key: 'directions',
        label: t('chipStorages', {
          value: filterDirections.value.join(', '),
        }),
      });

    if (filterBuckets.value.length)
      list.push({
        key: 'buckets',
        label: t('chipBuckets', { value: filterBuckets.value.join(', ') }),
      });

    return list;
  });

  function removeChip(key: string) {
    if (key === 'directions') filterDirections.value = [];
    else if (key === 'buckets') filterBuckets.value = [];
    else if (key === 'statuses') filterStatuses.value = [];
  }

  function clearAll() {
    search.value = '';
    store.clearFilters();
  }

  const selectedRows = computed(() =>
    rows.value.filter((row) => selectedKeys.value.includes(rowKey(row))),
  );

  async function bulkRestart() {
    bulkRestartOpen.value = false;
    report(await store.restartDiffReports(selectedRows.value.filter(isTwo)));
  }

  async function bulkDelete() {
    bulkDeleteOpen.value = false;
    report(await store.deleteDiffReports(selectedRows.value));
  }
</script>

<template>
  <div class="diff-tab">
    <TableToolbar
      v-model:search="search"
      :search-label="t('search')"
      :search-placeholder="t('searchDiffPlaceholder')"
      :chips="chips"
      :hidden="isEmptyList"
      @remove="removeChip"
      @clear="clearAll"
    >
      <DiffReportsFilters />
    </TableToolbar>

    <DiffReportsCardList
      v-if="narrow && rows.length > 0"
      :rows="rows"
      :actions-for="menuActions"
      :label-of="labelOf"
    />
    <DataTable
      v-else
      :columns="columns"
      :rows="rows"
      :loading="isLoading"
      :error="hasError ? t('loadError') : undefined"
      :row-key="rowKey"
      selectable
      @select="selectedKeys = $event"
      @retry="store.initDiffReportPage()"
    >
      <template #empty>
        <EmptyBlock
          :title="isEmptyList ? t('emptyDiffTitle') : t('emptyFilteredTitle')"
          :description="isEmptyList ? t('emptyDiffText') : undefined"
        >
          <WriteGate v-if="isEmptyList">
            <RouterLink :to="{ name: RouteName.TUTTI_ADD_DIFF_REPORT }">
              <NButton
                type="primary"
                tag="span"
              >
                {{ t('addDiffReport') }}
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
            @click="bulkRestartOpen = true"
          >
            {{ t('restartSelected', { n: keys.length }) }}
          </NButton>
          <NButton
            size="small"
            type="error"
            @click="bulkDeleteOpen = true"
          >
            {{ t('deleteReports', { n: keys.length }) }}
          </NButton>
        </WriteGate>
      </template>
    </DataTable>

    <ConfirmDialog
      :open="bulkRestartOpen"
      :title="t('restartSelected', { n: selectedRows.filter(isTwo).length })"
      danger
      :confirm-label="t('restart')"
      @confirm="bulkRestart"
      @cancel="bulkRestartOpen = false"
    >
      <p>{{ t('restartReportsConfirmContent') }}</p>
    </ConfirmDialog>

    <ConfirmDialog
      :open="bulkDeleteOpen"
      :title="t('deleteReports', { n: selectedRows.length })"
      danger
      :confirm-label="t('deleteReports', { n: selectedRows.length })"
      @confirm="bulkDelete"
      @cancel="bulkDeleteOpen = false"
    >
      <p>{{ t('deleteReportsText') }}</p>
    </ConfirmDialog>

    <RowActionsConfirm
      :pending="pending"
      :target="labelOf"
      @confirm="confirmPending"
      @cancel="cancelPending"
    />
  </div>
</template>

<style lang="scss" scoped>
  .diff-tab {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-4);
  }
</style>
