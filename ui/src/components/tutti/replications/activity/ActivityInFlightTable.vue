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
  import { computed, h } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nReplicationActivity from './i18nReplicationActivity';
  import { EmptyBlock } from '@/components/kit';
  import { useNarrowScreen } from '@/components/tutti/storages/useNarrowScreen';
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
  // <= 640px the wide table would scroll sideways: one compact card per object instead
  const narrow = useNarrowScreen();

  const sorted = computed(() =>
    [...props.items].sort(
      (a, b) => Date.parse(a.startedAt) - Date.parse(b.startedAt),
    ),
  );
  const rows = computed(() => sorted.value.slice(0, ACTIVITY_INFLIGHT_ROW_CAP));

  function rowKey(row: ReplicationInFlight): string {
    return `${row.key}|${row.versionId ?? ''}`;
  }

  const objectName = (row: ReplicationInFlight) =>
    row.versionId ? `${row.key} (${row.versionId})` : row.key;
  const sizeText = (row: ReplicationInFlight) => {
    const size = InitProgressHelper.toNumber(row.size);

    return size > 0 ? InitProgressHelper.formatSize(size) : '-';
  };
  const progressOf = (row: ReplicationInFlight) =>
    InitProgressHelper.copyProgress(row.size, row.bytesDone);
  const elapsedText = (row: ReplicationInFlight) =>
    ActivityLogHelper.formatDuration(
      Math.max(0, props.now - Date.parse(row.startedAt)),
    );

  // the attempt number only matters once a copy was retried
  const retried = computed(() =>
    props.items.some((item) => (item.attempt ?? 1) > 1),
  );

  function flowText(row: { flow?: string }) {
    if (row.flow === 'live') return t('flowLive');

    return row.flow === 'rescan' ? t('flowRescan') : t('flowInit');
  }

  const columns = computed<DataTableColumns<ReplicationInFlight>>(() => {
    const all: DataTableColumns<ReplicationInFlight> = [
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
        title: t('colProgress'),
        key: 'progress',
        width: 200,
        render: (row) => {
          const p = InitProgressHelper.copyProgress(row.size, row.bytesDone);

          if (!p) return '-';

          return h('div', { class: 'activity-inflight__progress' }, [
            h(
              'span',
              `${InitProgressHelper.formatSize(p.done)} / ${InitProgressHelper.formatSize(p.size)} (${p.percent}%)`,
            ),
            h('div', { class: 'activity-inflight__bar' }, [
              h('div', {
                class: 'activity-inflight__bar-fill',
                style: { width: `${p.percent}%` },
              }),
            ]),
          ]);
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
        render: (row) => flowText(row),
      },
    ];

    return all.filter(
      (column) =>
        !('key' in column) || column.key !== 'attempt' || retried.value,
    );
  });
</script>

<template>
  <div class="activity-inflight">
    <EmptyBlock
      v-if="items.length === 0"
      :title="t('inProgressEmpty')"
    />
    <template v-else>
      <ul
        v-if="narrow"
        class="activity-inflight__list"
      >
        <li
          v-for="row in rows"
          :key="rowKey(row)"
          class="activity-inflight__card"
        >
          <span
            class="activity-inflight__name"
            :title="objectName(row)"
            >{{ objectName(row) }}</span
          >
          <span class="activity-inflight__meta">
            <span>{{ sizeText(row) }}</span>
            <span>{{ elapsedText(row) }}</span>
            <span>{{ flowText(row) }}</span>
          </span>
          <span
            v-if="progressOf(row)"
            class="activity-inflight__progress"
          >
            <span
              >{{ InitProgressHelper.formatSize(progressOf(row)!.done) }} /
              {{ InitProgressHelper.formatSize(progressOf(row)!.size) }} ({{
                progressOf(row)!.percent
              }}%)</span
            >
            <span class="activity-inflight__bar">
              <span
                class="activity-inflight__bar-fill"
                :style="{ width: `${progressOf(row)!.percent}%` }"
              />
            </span>
          </span>
        </li>
      </ul>
      <NDataTable
        v-else
        size="small"
        :columns="columns"
        :data="rows"
        :row-key="rowKey"
        :max-height="360"
        :scroll-x="640"
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

    :deep(.activity-inflight__progress) {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }

    :deep(.activity-inflight__bar) {
      height: 3px;
      border-radius: 2px;
      background: var(--tt-border);
      overflow: hidden;
    }

    :deep(.activity-inflight__bar-fill) {
      height: 100%;
      background: var(--tt-accent);
    }

    &__list {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-2);
      margin: 0;
      padding: 0;
      list-style: none;
    }

    &__card {
      display: flex;
      flex-direction: column;
      gap: var(--tt-space-1);
      min-width: 0;
      padding: var(--tt-space-2) var(--tt-space-3);
      border: 1px solid var(--tt-border);
      border-radius: var(--tt-radius-md, 6px);
      font-size: 13px;
    }

    &__name {
      overflow-wrap: anywhere;
      font-weight: 500;
    }

    &__meta {
      display: flex;
      gap: var(--tt-space-3);
      color: var(--tt-text-3);
    }

    &__card &__progress {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }

    &__card &__bar {
      display: block;
    }

    &__note {
      margin: var(--tt-space-2) 0 0;
      font-size: 12px;
      color: var(--tt-text-3);
    }
  }
</style>
