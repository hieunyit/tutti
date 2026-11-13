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
  import { RouterLink } from 'vue-router';
  import { NButton, type DataTableColumns } from 'naive-ui';
  import { diffDetailRoute } from './diffRoutes';
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
  import ChorusDiffReportsProgress from '@/components/chorus/common/ChorusDiffReportProgress/ChorusDiffReportsProgress.vue';
  import DiffReportsBucketCell from '@/components/chorus/diff-reports/DiffReportsBucketCell/DiffReportsBucketCell.vue';
  import DiffReportsDirectionCell from '@/components/chorus/diff-reports/DiffReportsDirectionCell/DiffReportsDirectionCell.vue';
  import DiffReportsFilters from '@/components/chorus/diff-reports/DiffReportsFilters/DiffReportsFilters.vue';
  import DiffReportsStatusCell from '@/components/chorus/diff-reports/DiffReportsStatusCell/DiffReportsStatusCell.vue';
  import { useTuttiDiffReportsStore } from '@/stores/tuttiDiffReportsStore';
  import { useDebouncedSearch } from '@/utils/composables/useDebouncedSearch';
  import { useRowActions } from '@/utils/composables/useRowActions';
  import { useTuttiNotification } from '@/utils/composables/useTuttiNotification';
  import { DiffReportsHelper } from '@/utils/helpers/DiffReportsHelper';
  import type { DiffReport, DiffReportLocation } from '@/utils/types/chorus';
  import type { AddId } from '@/utils/types/helper';
  import { RouteName } from '@/utils/types/router';

  type Row = AddId<DiffReport>;

  const { t } = useI18n({ messages: i18nReplicationHub });
  const store = useTuttiDiffReportsStore();
  const {
    isLoading,
    hasError,
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

  const rows = computed<Row[]>(() => {
    const needle = debouncedSearch.value.trim().toLowerCase();
    // reports created via CLI (not two locations) stay listed so they can be
    // seen and deleted; they just have no direction/bucket to show
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

  const rowKey = (row: Row) => String(row.idStr);
  const isTwo = (row: Row) => DiffReportsHelper.isTwoLocationReport(row);
  const locationsOf = (row: Row) =>
    row.locations as [DiffReportLocation, DiffReportLocation];

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

  const columns = computed<DataTableColumns<Row>>(() => [
    {
      title: t('colStatus'),
      key: 'status',
      minWidth: 150,
      render: (row) =>
        isTwo(row) ? h(DiffReportsStatusCell, { report: row }) : null,
    },
    {
      title: t('colProgress'),
      key: 'progress',
      minWidth: 140,
      render: (row) =>
        isTwo(row) ? h(ChorusDiffReportsProgress, { report: row }) : null,
    },
    {
      title: t('colDirection'),
      key: 'direction',
      minWidth: 180,
      render: (row) =>
        isTwo(row)
          ? h(DiffReportsDirectionCell, { locations: locationsOf(row) })
          : h('span', { class: 'diff-tab__cli-warning' }, t('cliCheckWarning')),
    },
    {
      title: t('colBucket'),
      key: 'bucket',
      minWidth: 220,
      render: (row) =>
        isTwo(row)
          ? h(DiffReportsBucketCell, { locations: locationsOf(row) })
          : null,
    },
    {
      title: t('colActions'),
      key: 'actions',
      width: 300,
      render: (row) => {
        if (!isTwo(row))
          return h(ChorusRowActions, {
            actions: actionsFor(row).filter((a) => a.key === 'delete'),
          });

        const [from, to] = locationsOf(row);

        return h(
          'div',
          {
            style:
              'display:flex;flex-wrap:wrap;align-items:center;gap:var(--tt-space-2)',
          },
          [
            h(
              RouterLink,
              { to: diffDetailRoute(from, to) },
              {
                default: () =>
                  h(
                    NButton,
                    {
                      size: 'tiny',
                      secondary: true,
                      type: 'info',
                      tag: 'span',
                    },
                    () => t('open'),
                  ),
              },
            ),
            h(ChorusRowActions, { actions: actionsFor(row) }),
          ],
        );
      },
    },
  ]);

  const chips = computed<FilterChip[]>(() => {
    const list: FilterChip[] = [];

    if (filterDirections.value.length)
      list.push({
        key: 'directions',
        label: t('chipDirections', {
          value: filterDirections.value.join(', '),
        }),
      });

    if (filterBuckets.value.length)
      list.push({
        key: 'buckets',
        label: t('chipBuckets', { value: filterBuckets.value.join(', ') }),
      });

    if (filterStatuses.value.length)
      list.push({
        key: 'statuses',
        label: t('chipStatuses', { value: filterStatuses.value.join(', ') }),
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
    <FilterBar
      v-model:search="search"
      :search-label="t('search')"
      :search-placeholder="t('searchPlaceholder')"
      :chips="chips"
      @remove="removeChip"
      @clear="clearAll"
    >
      <DiffReportsFilters />
    </FilterBar>

    <DataTable
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
          :title="
            isFiltered || search ? t('emptyFilteredTitle') : t('emptyDiffTitle')
          "
          :description="
            isFiltered || search ? t('emptyFilteredText') : t('emptyDiffText')
          "
        >
          <WriteGate v-if="!isFiltered && !search">
            <RouterLink :to="{ name: RouteName.CHORUS_ADD_DIFF_REPORT }">
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
      :target="
        (row: Row) =>
          row.locations.map((l) => `${l.storage}/${l.bucket}`).join(' → ')
      "
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

  .diff-tab__cli-warning {
    color: var(--tt-warning, inherit);
  }
</style>
