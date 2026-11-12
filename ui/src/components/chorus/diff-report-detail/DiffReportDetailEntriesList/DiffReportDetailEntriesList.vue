<!--
  - Copyright © 2026 Clyso GmbH
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
  import {
    CDataTable,
    CTag,
    type DataTableBaseColumn,
    type DataTableSortState,
  } from '@clyso/clyso-ui-kit';
  import { storeToRefs } from 'pinia';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nDiffReportDetail from '@/components/chorus/diff-report-detail/i18nDiffReportDetail';
  import { useTuttiDiffReportEntriesStore } from '@/stores/tuttiDiffReportEntriesStore';
  import { GeneralHelper } from '@/utils/helpers/GeneralHelper';
  import type { DiffReportEntry } from '@/utils/types/chorus';
  import DiffReportDetailEntriesStorageCell from '@/components/chorus/diff-report-detail/DiffReportDetailEntriesStorageCell/DiffReportDetailEntriesStorageCell.vue';
  import { useTuttiDiffReportDetailStore } from '@/stores/tuttiDiffReportDetailStore';

  const { t } = useI18n({ messages: i18nDiffReportDetail });

  const {
    isLoading,
    hasError,
    computedReportObjects,
    pagination,
    sorter,
    page,
    pageSize,
  } = storeToRefs(useTuttiDiffReportEntriesStore());

  const { getReportEntries } = useTuttiDiffReportEntriesStore();

  const { report } = storeToRefs(useTuttiDiffReportDetailStore());

  const columns = computed<DataTableBaseColumn<DiffReportEntry>[]>(() => [
    {
      title: t('columnObject'),
      key: 'object',
      minWidth: 260,
      sorter: true,
    },
    {
      title: t('columnSize'),
      key: 'size',
      width: 130,
      sorter: true,
    },
    {
      title: t('columnEtag'),
      key: 'etag',
      width: 200,
    },
    {
      title: t('columnObjectPlacement'),
      key: 'objectPlacement',
      minWidth: 240,
    },
  ]);

  const handleSortingChange = (newSorter: DataTableSortState | null) => {
    sorter.value = newSorter;
    page.value = 1;
  };

  const handlePageChange = (newPage: number) => {
    page.value = newPage;
  };

  const handlePageSizeUpdate = (newPageSize: number) => {
    pageSize.value = newPageSize;
    page.value = 1;
  };
</script>

<template>
  <div class="diff-report-entries-list">
    <div class="diff-report-entries-list__container">
      <CDataTable
        class="diff-report-entries-list__table"
        :columns="columns"
        :data="computedReportObjects"
        :is-controlled="true"
        :is-loading="!report || !report.ready || isLoading"
        :pagination="pagination"
        :bordered="false"
        :sorter="sorter"
        :has-error="hasError"
        @update:sorter="handleSortingChange"
        @update:page="handlePageChange"
        @update:page-size="handlePageSizeUpdate"
        @retry="getReportEntries"
      >
        <template #object="{ rowData }: { rowData: DiffReportEntry }">
          <div>
            <p>{{ rowData.object }}</p>
            <small class="diff-report-entries-list__version">
              {{ t('versionIdx', { idx: rowData.versionIdx }) }}
            </small>
          </div>
        </template>

        <template #size="{ rowData }: { rowData: DiffReportEntry }">
          {{ GeneralHelper.formatBytes(Number(rowData.size)) }}
        </template>

        <template #etag="{ rowData }: { rowData: DiffReportEntry }">
          <CTag
            :bordered="false"
            round
            size="small"
            type="primary"
          >
            {{ rowData.etag }}
          </CTag>
        </template>

        <template #objectPlacement="{ rowData }: { rowData: DiffReportEntry }">
          <DiffReportDetailEntriesStorageCell
            :storage-entries="rowData.storageEntries"
          />
        </template>
      </CDataTable>
    </div>
  </div>
</template>

<style lang="scss" scoped>
  .diff-report-entries-list {
    min-width: 0;

    &__container {
      max-width: 100%;
      overflow-x: auto;
    }

    &__table {
      min-width: 840px;

      :deep(td) {
        overflow-wrap: anywhere;
      }
    }

    &__version {
      color: var(--text-color-3);
    }
  }
</style>
