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
  import { CTile } from '@clyso/clyso-ui-kit';
  import {
    NButton,
    NDataTable,
    NPopconfirm,
    type DataTableColumns,
  } from 'naive-ui';
  import { storeToRefs } from 'pinia';
  import { computed, h } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nFailover from './i18nFailover';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { useTuttiFailoverStore } from '@/stores/tuttiFailoverStore';
  import { useChorusNotification } from '@/utils/composables/useChorusNotification';
  import { GeneralHelper } from '@/utils/helpers/GeneralHelper';
  import type { TuttiFailoverEvent } from '@/utils/types/tutti';

  const { t } = useI18n({ messages: i18nFailover });
  const store = useTuttiFailoverStore();
  const { events } = storeToRefs(store);
  const { createNotification } = useChorusNotification();

  function bucketLabel(event: TuttiFailoverEvent): string {
    return event.bucket || t('allBuckets');
  }

  async function acknowledge(event: TuttiFailoverEvent) {
    if (!(await store.acknowledgeEvent(event))) {
      createNotification({
        type: 'error',
        title: t('acknowledgeFailed'),
        duration: 6000,
      });
    }
  }

  const columns = computed<DataTableColumns<TuttiFailoverEvent>>(() => [
    { title: t('columnUser'), key: 'user' },
    {
      title: t('columnBucket'),
      key: 'bucket',
      render: (row) => bucketLabel(row),
    },
    {
      title: t('columnFromTo'),
      key: 'fromTo',
      render: (row) => `${row.from} → ${row.to}`,
    },
    {
      title: t('columnTime'),
      key: 'time',
      render: (row) =>
        row.time ? GeneralHelper.formatDateTime(row.time) : '-',
    },
    {
      title: t('columnEventLag'),
      key: 'eventLag',
      render: (row) => String(row.eventLag ?? 0),
    },
    {
      title: t('columnActions'),
      key: 'actions',
      render: (row) =>
        h(WriteGate, null, () =>
          h(
            NPopconfirm,
            {
              positiveText: t('acknowledge'),
              negativeText: t('cancel'),
              onPositiveClick: () => acknowledge(row),
            },
            {
              trigger: () =>
                h(NButton, { size: 'small' }, () => t('acknowledge')),
              default: () =>
                t('acknowledgeConfirm', {
                  user: row.user,
                  bucket: bucketLabel(row),
                  from: row.from,
                  to: row.to,
                }),
            },
          ),
        ),
    },
  ]);
</script>

<template>
  <CTile class="failover-events-tile">
    <template #title>
      {{ t('eventsTitle') }}
    </template>

    <p class="failover-events-tile__hint">{{ t('eventsHint') }}</p>

    <NDataTable
      :columns="columns"
      :data="events"
      :bordered="false"
      :row-key="(row: TuttiFailoverEvent) => `${row.user}::${row.bucket ?? ''}`"
    />
  </CTile>
</template>

<style lang="scss" scoped>
  .failover-events-tile {
    &__hint {
      margin: 0 0 16px;
      opacity: 0.8;
    }
  }
</style>
