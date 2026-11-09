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
   * Nested table (expanded row) with every access key of one proxy account.
   * Static keys come from the config and are read-only; dynamic keys can be
   * rotated, disabled/enabled and deleted.
   */
  import {
    NDataTable,
    NPopconfirm,
    NTag,
    type DataTableColumns,
  } from 'naive-ui';
  import { computed, h } from 'vue';
  import { useI18n } from 'vue-i18n';
  import i18nAccounts from './i18nAccounts';
  import CopyButton from '@/components/chorus/common/CopyButton/CopyButton.vue';
  import RowActionButton from '@/components/tutti/common/RowActionButton.vue';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { useChorusNotification } from '@/utils/composables/useChorusNotification';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import {
    ProxyAccountHelper,
    type ProxyAccountKeyRow,
  } from '@/utils/helpers/ProxyAccountHelper';
  import { IconName } from '@/utils/types/icon';
  import type { TuttiProxyAccount } from '@/utils/types/tutti';

  const props = defineProps<{
    account: TuttiProxyAccount;
  }>();

  const emit = defineEmits<{
    (e: 'rotate', accessKeyId: string): void;
  }>();

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();
  const { createNotification } = useChorusNotification();

  const rows = computed(() => ProxyAccountHelper.getKeyRows(props.account));

  function reportError(error: unknown) {
    createNotification({
      type: 'error',
      title: ProxyAccountHelper.isNotConfigured(error)
        ? t('notConfigured')
        : (ProxyAccountHelper.getMessage(error) ?? t('actionUnknownError')),
      duration: 8000,
    });
  }

  async function toggleStatus(row: ProxyAccountKeyRow) {
    if (!row.dynamicKey) return;

    try {
      await store.setKeyStatus(
        row.accessKeyId,
        ProxyAccountHelper.isDisabled(row.dynamicKey) ? 'active' : 'disabled',
      );
    } catch (error: unknown) {
      reportError(error);
    }
  }

  async function removeKey(row: ProxyAccountKeyRow) {
    try {
      await store.deleteKey(row.accessKeyId);
    } catch (error: unknown) {
      reportError(error);
    }
  }

  function sourceLabel(source: string): string {
    if (source === 'custom' || source === 'config') return t('badgeConfig');

    if (source === 'dynamic') return t('sourceDynamic');

    return source;
  }

  const muted = (text: string) =>
    h('span', { class: 'keys-table__muted' }, text);

  const dateCell = (value?: string) =>
    ProxyAccountHelper.hasTime(value)
      ? formatDateTime(value)
      : muted(formatDateTime(undefined));

  const columns = computed<DataTableColumns<ProxyAccountKeyRow>>(() => [
    {
      title: t('columnAccessKeyId'),
      key: 'accessKeyId',
      render: (row) =>
        h('span', { class: 'keys-table__key-id' }, [
          h('code', row.accessKeyId),
          h(CopyButton, { text: row.accessKeyId }),
        ]),
    },
    {
      title: t('columnSource'),
      key: 'source',
      render: (row) =>
        h(
          NTag,
          {
            size: 'small',
            round: true,
            type: row.dynamicKey ? 'info' : 'default',
          },
          () => sourceLabel(row.source),
        ),
    },
    {
      title: t('columnStatus'),
      key: 'status',
      render: (row) => {
        const disabled =
          row.dynamicKey !== undefined &&
          ProxyAccountHelper.isDisabled(row.dynamicKey);

        return h(
          NTag,
          {
            size: 'small',
            round: true,
            type: disabled ? 'warning' : 'success',
          },
          () => (disabled ? t('statusDisabled') : t('statusActive')),
        );
      },
    },
    {
      title: t('columnDescription'),
      key: 'description',
      render: (row) =>
        row.dynamicKey?.description
          ? row.dynamicKey.description
          : muted(formatDateTime(undefined)),
    },
    {
      title: t('columnCreated'),
      key: 'createdAt',
      render: (row) => dateCell(row.dynamicKey?.createdAt),
    },
    {
      title: t('columnRotated'),
      key: 'rotatedAt',
      render: (row) => dateCell(row.dynamicKey?.rotatedAt),
    },
    {
      title: t('columnPreviousValid'),
      key: 'previousSecretExpiresAt',
      render: (row) => dateCell(row.dynamicKey?.previousSecretExpiresAt),
    },
    {
      title: t('columnActions'),
      key: 'actions',
      render: (row) => {
        if (!row.dynamicKey) return muted(t('managedInConfig'));

        const disabled = ProxyAccountHelper.isDisabled(row.dynamicKey);

        return h(WriteGate, null, () =>
          h('span', { class: 'keys-table__actions' }, [
            h(RowActionButton, {
              label: t('rotateSecret'),
              icon: IconName.BASE_REFRESH,
              onClick: () => emit('rotate', row.accessKeyId),
            }),
            h(RowActionButton, {
              label: disabled ? t('enableKey') : t('disableKey'),
              icon: disabled ? IconName.BASE_PLAY : IconName.BASE_PAUSE,
              type: disabled ? 'success' : 'warning',
              onClick: () => toggleStatus(row),
            }),
            h(
              NPopconfirm,
              {
                positiveText: t('deleteKeyAction'),
                negativeText: t('cancel'),
                positiveButtonProps: { type: 'error' },
                onPositiveClick: () => removeKey(row),
              },
              {
                trigger: () =>
                  h(RowActionButton, {
                    label: t('deleteKey'),
                    icon: IconName.BASE_TRASH,
                    type: 'error',
                  }),
                default: () => t('deleteKeyConfirm', { key: row.accessKeyId }),
              },
            ),
          ]),
        );
      },
    },
  ]);
</script>

<template>
  <div class="keys-table">
    <NDataTable
      :columns="columns"
      :data="rows"
      :bordered="false"
      size="small"
      :scroll-x="1040"
      :row-key="(row: ProxyAccountKeyRow) => row.accessKeyId"
    >
      <template #empty>{{ t('noKeys') }}</template>
    </NDataTable>
  </div>
</template>

<style lang="scss" scoped>
  .keys-table {
    padding: 4px 0 8px;

    :deep(.keys-table__key-id) {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      white-space: nowrap;

      code {
        font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      }
    }

    :deep(.keys-table__actions) {
      display: inline-flex;
      align-items: center;
      gap: 4px;
    }

    :deep(.keys-table__muted) {
      color: var(--text-color-3);
    }
  }
</style>
