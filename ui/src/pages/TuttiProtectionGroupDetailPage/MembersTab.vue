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
  import type { DataTableColumns } from 'naive-ui';
  import { computed, h } from 'vue';
  import { useI18n } from 'vue-i18n';
  import {
    DataTable,
    EMPTY_VALUE,
    StatusBadge,
    type StatusTone,
  } from '@/components/kit';
  import i18nProtection from '@/components/tutti/protection/i18nProtectionUi';
  import {
    ProtectionGroupHelper,
    type MemberHealth,
    type MemberRow,
  } from '@/utils/helpers/ProtectionGroupHelper';
  import type {
    ProtectionGroup,
    ProtectionGroupStatus,
  } from '@/utils/types/protection';

  const props = defineProps<{
    group: ProtectionGroup;
    detail: ProtectionGroupStatus | null;
  }>();

  const { t } = useI18n({ messages: i18nProtection });

  const TONES: Record<MemberHealth, StatusTone> = {
    healthy: 'success',
    syncing: 'info',
    lagging: 'warning',
    unavailable: 'danger',
  };

  const rows = computed(() =>
    ProtectionGroupHelper.memberRows(props.group, props.detail ?? undefined),
  );

  function syncText(row: MemberRow) {
    if (row.initialized === null) return EMPTY_VALUE;

    return row.initialized ? t('syncDone') : t('syncRunning');
  }

  const columns = computed<DataTableColumns<MemberRow>>(() => [
    {
      title: t('colStorage'),
      key: 'storage',
      render: (row) =>
        h(
          'span',
          { class: 'tt-mono' },
          row.isPrimary ? `${row.storage} (${t('primaryMark')})` : row.storage,
        ),
    },
    {
      title: t('colRole'),
      key: 'role',
      render: (row) =>
        row.isPrimary
          ? h(StatusBadge, { tone: 'info', label: t('rolePrimary') })
          : row.isTarget
            ? h(StatusBadge, { tone: 'warning', label: t('roleTarget') })
            : t('roleReplica'),
    },
    {
      title: t('colHealth'),
      key: 'health',
      render: (row) =>
        h(StatusBadge, {
          tone: TONES[row.health],
          label: t(`health.${row.health}`),
        }),
    },
    {
      title: t('colMemberLag'),
      key: 'lag',
      render: (row) => ProtectionGroupHelper.lagText(row.lag),
    },
    { title: t('colInitialSync'), key: 'sync', render: syncText },
    {
      title: t('colExcluded'),
      key: 'excluded',
      render: (row) =>
        row.excluded
          ? h(StatusBadge, { tone: 'danger', label: t('excludedYes') })
          : EMPTY_VALUE,
    },
  ]);
</script>

<template>
  <DataTable
    :columns="columns"
    :rows="rows"
    :loading="false"
    :row-key="(r: MemberRow) => r.storage"
  />
</template>
