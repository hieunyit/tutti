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
  import { NDataTable, NTag, type DataTableColumns } from 'naive-ui';
  import { computed, h } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nHealth from './i18nHealth';
  import {
    EMPTY_DATE_TEXT,
    formatDateTime,
    formatRelative,
  } from '@/utils/helpers/DateTimeHelper';
  import type {
    TuttiProxyStatus,
    TuttiStorageHealth,
  } from '@/utils/types/tutti';

  const props = defineProps<{ proxy: TuttiProxyStatus }>();

  const { t } = useI18n({ messages: i18nHealth });
  const relative = (date?: string) => formatRelative(date);
  const absolute = (date?: string) => formatDateTime(date);

  const columns = computed<DataTableColumns<TuttiStorageHealth>>(() => [
    { title: t('columnStorage'), key: 'name' },
    {
      title: t('columnStatus'),
      key: 'available',
      render: (row) =>
        h(
          NTag,
          {
            type: !row.available
              ? 'error'
              : row.lastError
                ? 'warning'
                : 'success',
            size: 'small',
          },
          () =>
            !row.available
              ? t('unavailable')
              : row.lastError
                ? t('availableWarning')
                : t('available'),
        ),
    },
    {
      title: t('columnFailures'),
      key: 'consecutiveFailures',
      render: (row) => String(row.consecutiveFailures ?? 0),
    },
    {
      title: t('columnLastError'),
      key: 'lastError',
      render: (row) => row.lastError || EMPTY_DATE_TEXT,
    },
    {
      title: t('columnLastChange'),
      key: 'lastChange',
      render: (row) =>
        h(
          'span',
          { title: absolute(row.lastChange) },
          relative(row.lastChange),
        ),
    },
  ]);

  function onOff(value?: boolean) {
    return value ? t('on') : t('off');
  }

  const settings = computed(() => {
    const s = props.proxy.settings;
    const sync = s?.replication?.sync;

    return [
      [t('syncReplication'), onOff(sync?.enabled)],
      [t('writeQuorum'), sync?.writeQuorum ?? EMPTY_DATE_TEXT],
      [t('syncTimeout'), sync?.timeout ?? EMPTY_DATE_TEXT],
      [t('onQuorumFailure'), sync?.onQuorumFailure || EMPTY_DATE_TEXT],
      [t('failover'), onOff(s?.failover?.enabled)],
      [t('failureThreshold'), s?.health?.failureThreshold ?? EMPTY_DATE_TEXT],
      [t('openDuration'), s?.health?.openDuration ?? EMPTY_DATE_TEXT],
      [
        t('requireSignedContentType'),
        s?.requireSignedContentType ? t('yes') : t('no'),
      ],
    ];
  });
</script>

<template>
  <CTile class="proxy-health-card">
    <template #title> {{ t('proxy') }}: {{ proxy.appId }} </template>

    <p class="proxy-health-card__meta">
      <span v-if="proxy.address">{{ proxy.address }} · </span>
      {{ t('updatedAt') }}: {{ relative(proxy.updatedAt) }} ·
      {{ t('startedAt') }}: {{ relative(proxy.startedAt) }}
    </p>

    <NDataTable
      :columns="columns"
      :data="proxy.storages ?? []"
      :bordered="false"
      :row-key="(row: TuttiStorageHealth) => row.name"
    />

    <h4 class="proxy-health-card__settings-title">{{ t('settingsTitle') }}</h4>
    <dl class="proxy-health-card__settings">
      <div
        v-for="[label, value] in settings"
        :key="String(label)"
        class="proxy-health-card__setting"
      >
        <dt>{{ label }}</dt>
        <dd>{{ value }}</dd>
      </div>
    </dl>
  </CTile>
</template>

<style lang="scss" scoped>
  @use '@/styles/utils' as utils;

  .proxy-health-card {
    &__meta {
      margin: 0 0 utils.unit(4);
      opacity: 0.7;
    }

    &__settings-title {
      margin: utils.unit(6) 0 utils.unit(2);
    }

    &__settings {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
      gap: utils.unit(3) utils.unit(6);
      margin: 0;
    }

    &__setting {
      dt {
        opacity: 0.7;
        font-size: 12px;
      }

      dd {
        margin: 0;
        font-weight: 500;
      }
    }
  }
</style>
