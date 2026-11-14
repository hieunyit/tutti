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
   * Object copies running right now, longest running first, capped at
   * ACTIVITY_INFLIGHT_ROW_CAP rows (a note says how many are hidden).
   */
  import { NDataTable, type DataTableColumns } from 'naive-ui';
  import { computed } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nReplicationActivity from './i18nReplicationActivity';
  import { EmptyBlock } from '@/components/kit';
  import {
    ACTIVITY_INFLIGHT_ROW_CAP,
    ActivityLogHelper,
  } from '@/utils/helpers/ActivityLogHelper';
  import { InitProgressHelper } from '@/utils/helpers/InitProgressHelper';
  import type { ReplicationInFlight } from '@/utils/types/replicationActivity';

  const props = defineProps<{
    items: ReplicationInFlight[];
    /** current time in ms, ticking */
    now: number;
  }>();

  const { t } = useI18n({ messages: i18nReplicationActivity });

  const sorted = computed(() =>
    [...props.items].sort(
      (a, b) => Date.parse(a.startedAt) - Date.parse(b.startedAt),
    ),
  );
  const rows = computed(() => sorted.value.slice(0, ACTIVITY_INFLIGHT_ROW_CAP));

  function rowKey(row: ReplicationInFlight): string {
    return `${row.key}|${row.versionId ?? ''}`;
  }

  const columns = computed<DataTableColumns<ReplicationInFlight>>(() => [
    {
      title: t('colObject'),
      key: 'key',
      ellipsis: { tooltip: true },
      render: (row) =>
        row.versionId ? `${row.key} (${row.versionId})` : row.key,
    },
    {
      title: t('colSize'),
      key: 'size',
      width: 110,
      render: (row) => {
        const size = InitProgressHelper.toNumber(row.size);

        return size > 0 ? InitProgressHelper.formatSize(size) : '-';
      },
    },
    {
      title: t('colElapsed'),
      key: 'elapsed',
      width: 110,
      render: (row) =>
        ActivityLogHelper.formatDuration(
          Math.max(0, props.now - Date.parse(row.startedAt)),
        ),
    },
    {
      title: t('colAttempt'),
      key: 'attempt',
      width: 90,
      render: (row) => String(row.attempt ?? 1),
    },
    {
      title: t('colFlow'),
      key: 'flow',
      width: 100,
      render: (row) => (row.flow === 'live' ? t('flowLive') : t('flowInit')),
    },
  ]);
</script>

<template>
  <div class="activity-inflight">
    <EmptyBlock
      v-if="items.length === 0"
      :title="t('inProgressEmpty')"
    />
    <template v-else>
      <NDataTable
        size="small"
        :columns="columns"
        :data="rows"
        :row-key="rowKey"
        :max-height="360"
        :bordered="false"
      />
      <p
        v-if="items.length > rows.length"
        class="activity-inflight__note"
      >
        {{ t('inProgressNote', { max: rows.length, total: items.length }) }}
      </p>
    </template>
  </div>
</template>

<style lang="scss" scoped>
  .activity-inflight {
    min-width: 0;

    &__note {
      margin: var(--tt-space-2) 0 0;
      font-size: 12px;
      color: var(--tt-text-3);
    }
  }
</style>
