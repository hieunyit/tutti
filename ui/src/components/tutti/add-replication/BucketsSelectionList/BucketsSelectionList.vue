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
  /**
   * Bucket picker of the add replication wizard: search, a switch for already
   * replicated buckets, and a selectable list. Buckets that cannot be picked
   * (replicated, or covered by a protection group or replication) are
   * disabled and say why.
   */
  import { storeToRefs } from 'pinia';
  import {
    CDataTable,
    CInput,
    CSwitch,
    type DataTableBaseColumn,
    type DataTableRowData,
    type DataTableSelectionColumn,
  } from '@clyso/clyso-ui-kit';
  import { NButton } from 'naive-ui';
  import { computed, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nAddReplication from '../i18nAddReplication';
  import { useBucketRows, type BucketRowData } from '../useBucketRows';
  import { EmptyBlock } from '@/components/kit';
  import { useTuttiAddReplicationStore } from '@/stores/tuttiAddReplicationStore';

  const {
    selectedBuckets,
    isBucketsListLoading,
    isReplicatedShown,
    selectedFromStorage,
  } = storeToRefs(useTuttiAddReplicationStore());
  const { t } = useI18n({ messages: i18nAddReplication });

  const search = ref('');
  const rows = useBucketRows(search);

  const columns = computed<
    (DataTableBaseColumn<BucketRowData> | DataTableSelectionColumn)[]
  >(() => [
    {
      type: 'selection',
      width: 44,
      disabled: (row: DataTableRowData) =>
        row.isReplicated || !!row.coveredReason,
    },
    { title: t('columnBucket'), key: 'name', width: '45%', sorter: 'default' },
    { title: t('columnStatus'), key: 'status' },
  ]);

  const statusOf = (row: BucketRowData) =>
    row.coveredReason
      ? t('statusCovered')
      : t(row.isReplicated ? 'statusReplicated' : 'statusAvailable');
  const isLocked = (row: BucketRowData) =>
    row.isReplicated || !!row.coveredReason;
</script>

<template>
  <div class="bucket-picker">
    <div class="bucket-picker__tools">
      <CInput
        v-model:value="search"
        class="bucket-picker__search"
        size="small"
        clearable
        :placeholder="t('searchBuckets')"
        :input-props="{ 'aria-label': t('searchBuckets') }"
      />
      <label class="bucket-picker__switch">
        <CSwitch
          v-model:value="isReplicatedShown"
          size="small"
        />
        <span>{{ t('showReplicated') }}</span>
      </label>
      <span
        v-if="selectedBuckets.length"
        class="bucket-picker__count"
        role="status"
      >
        {{ t('selectedCount', { total: selectedBuckets.length }) }}
      </span>
    </div>

    <div class="bucket-picker__table">
      <CDataTable
        v-model:checked-row-keys="selectedBuckets"
        class="bucket-picker__list"
        :columns="columns"
        :data="rows"
        selectable
        virtual-scroll
        min-height="320px"
        max-height="320px"
        :row-key="(row: BucketRowData) => row.name"
        :is-loading="isBucketsListLoading"
      >
        <template #name="{ rowData }">
          <span :class="{ 'bucket-picker__locked': isLocked(rowData) }">
            {{ rowData.name }}
          </span>
        </template>

        <template #status="{ rowData }">
          <span :class="{ 'bucket-picker__locked': isLocked(rowData) }">
            {{ statusOf(rowData) }}
          </span>
          <small
            v-if="rowData.coveredReason"
            class="bucket-picker__reason"
          >
            {{ rowData.coveredReason }}
          </small>
        </template>

        <template #empty>
          <EmptyBlock
            :title="
              search
                ? t('noMatch')
                : t('noBuckets', { storage: selectedFromStorage?.name })
            "
          >
            <NButton
              v-if="search"
              @click="search = ''"
            >
              {{ t('clearSearch') }}
            </NButton>
          </EmptyBlock>
        </template>
      </CDataTable>
    </div>
  </div>
</template>

<style scoped lang="scss">
  .bucket-picker {
    display: flex;
    flex-direction: column;
    gap: var(--tt-space-3);

    &__tools {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: var(--tt-space-3) var(--tt-space-5);
    }

    &__search {
      flex: 1 1 220px;
      max-width: 320px;
    }

    &__switch {
      display: inline-flex;
      align-items: center;
      gap: var(--tt-space-2);
      cursor: pointer;
    }

    &__count {
      color: var(--tt-text-2);
      font-size: var(--tt-text-sm);
    }

    &__table {
      overflow-x: auto;
    }

    &__list {
      min-width: 480px;
    }

    &__locked {
      color: var(--tt-text-3);
    }

    &__reason {
      margin-left: var(--tt-space-2);
      color: var(--tt-text-3);
    }
  }
</style>
