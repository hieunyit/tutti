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
   * Key table with every access key for one account (user detail) or all
   * accounts (flat proxy accounts page).
   * Static keys come from the config and are read-only; managed keys can be
   * rotated, disabled/enabled and deleted.
   */
  import { computed, h, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { RouterLink } from 'vue-router';
  import { NButton, type DataTableColumns } from 'naive-ui';
  import i18nAccounts from './i18nAccounts';
  import {
    ConfirmDialog,
    CopyValue,
    DataTable,
    StatusBadge,
  } from '@/components/kit';
  import WriteGate from '@/components/auth/WriteGate.vue';
  import { EMPTY_VALUE } from '@/components/ui/status';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { useTuttiNotification } from '@/utils/composables/useTuttiNotification';
  import { formatDateTime, formatTime } from '@/utils/helpers/DateTimeHelper';
  import {
    ProxyAccountHelper,
    type ProxyAccountKeyRow,
  } from '@/utils/helpers/ProxyAccountHelper';
  import type { TuttiProxyAccount } from '@/utils/types/tutti';
  import { RouteName } from '@/utils/types/router';

  type Row = ProxyAccountKeyRow & { user: string };

  const props = defineProps<{
    account?: TuttiProxyAccount;
    accounts?: TuttiProxyAccount[];
  }>();

  const emit = defineEmits<{
    (e: 'rotate', accessKeyId: string): void;
  }>();

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();
  const { createNotification } = useTuttiNotification();

  const rows = computed<Row[]>(() =>
    (props.accounts ?? (props.account ? [props.account] : [])).flatMap(
      (account) =>
        ProxyAccountHelper.getKeyRows(account).map((row) => ({
          ...row,
          user: account.user,
        })),
    ),
  );
  const deleting = ref<Row | null>(null);
  const isDeleting = ref(false);

  function reportError(error: unknown) {
    createNotification({
      type: 'error',
      title: ProxyAccountHelper.isNotConfigured(error)
        ? t('notConfigured')
        : (ProxyAccountHelper.getMessage(error) ?? t('actionUnknownError')),
      duration: 8000,
    });
  }

  async function toggleStatus(row: Row) {
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

  async function confirmDelete() {
    if (!deleting.value) return;

    isDeleting.value = true;

    try {
      await store.deleteKey(deleting.value.accessKeyId);
      deleting.value = null;
    } catch (error: unknown) {
      reportError(error);
    } finally {
      isDeleting.value = false;
    }
  }

  function sourceLabel(source: string): string {
    if (source === 'dynamic') return t('sourceManaged');

    if (source.startsWith('storage:')) {
      return t('sourceConfigStorage', {
        name: source.slice('storage:'.length),
      });
    }

    return t('sourceConfig');
  }

  function dateCell(value?: string): string {
    return ProxyAccountHelper.hasTime(value)
      ? formatDateTime(value)
      : EMPTY_VALUE;
  }

  /** "old secret valid until HH:MM:SS", only while it is still valid */
  function oldSecretNote(row: Row): string | null {
    const expires = row.dynamicKey?.previousSecretExpiresAt;

    if (!ProxyAccountHelper.hasTime(expires)) return null;

    return Date.parse(expires as string) > Date.now()
      ? t('oldSecretValidUntil', { time: formatTime(expires) })
      : null;
  }

  function rowActions(row: Row) {
    const key = row.dynamicKey;

    if (!key) return h('span', t('managedInConfig'));

    const disabled = ProxyAccountHelper.isDisabled(key);

    return h(WriteGate, null, () =>
      h('span', { class: 'keys-table__buttons' }, [
        h(
          NButton,
          { size: 'small', onClick: () => emit('rotate', row.accessKeyId) },
          () => t('rotate'),
        ),
        h(NButton, { size: 'small', onClick: () => toggleStatus(row) }, () =>
          t(disabled ? 'enableKey' : 'disableKey'),
        ),
        h(
          NButton,
          {
            size: 'small',
            type: 'error',
            secondary: true,
            onClick: () => (deleting.value = row),
          },
          () => t('deleteKey'),
        ),
      ]),
    );
  }

  const columns = computed<DataTableColumns<Row>>(() => [
    ...(props.accounts
      ? [
          {
            title: t('columnUser'),
            key: 'user',
            sorter: 'default' as const,
            render: (row: Row) =>
              h(
                RouterLink,
                {
                  to: {
                    name: RouteName.TUTTI_USER_DETAIL,
                    params: { name: row.user },
                  },
                },
                () => row.user,
              ),
          },
        ]
      : []),
    {
      title: t('columnAccessKeyId'),
      key: 'accessKeyId',
      render: (row) => h(CopyValue, { value: row.accessKeyId }),
    },
    {
      title: t('columnSource'),
      key: 'source',
      render: (row) => sourceLabel(row.source),
    },
    {
      title: t('columnStatus'),
      key: 'status',
      render: (row) => {
        const off =
          !!row.dynamicKey && ProxyAccountHelper.isDisabled(row.dynamicKey);

        return h(StatusBadge, {
          tone: off ? 'neutral' : 'success',
          label: t(off ? 'statusDisabled' : 'statusActive'),
        });
      },
    },
    {
      title: t('columnDescription'),
      key: 'description',
      render: (row) => row.dynamicKey?.description || EMPTY_VALUE,
    },
    {
      title: t('columnCreated'),
      key: 'createdAt',
      render: (row) => dateCell(row.dynamicKey?.createdAt),
    },
    {
      title: t('columnRotated'),
      key: 'rotatedAt',
      render: (row) => {
        const note = oldSecretNote(row);

        return h('span', { class: 'keys-table__stack' }, [
          dateCell(row.dynamicKey?.rotatedAt),
          note ? h('span', { class: 'keys-table__warn' }, note) : null,
        ]);
      },
    },
    {
      title: t('columnActions'),
      key: 'actions',
      render: rowActions,
    },
  ]);
</script>

<template>
  <div class="keys-table">
    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="false"
      :row-key="(row: Row) => `${row.user}:${row.accessKeyId}`"
    >
      <template #empty>
        <p class="keys-table__none">{{ t('noKeys') }}</p>
      </template>
    </DataTable>

    <ConfirmDialog
      :open="deleting !== null"
      :title="t('deleteKeyDialogTitle')"
      :target-name="deleting?.accessKeyId"
      :confirm-label="t('deleteKeyAction')"
      :loading="isDeleting"
      danger
      @cancel="deleting = null"
      @confirm="confirmDelete"
    >
      {{ t('deleteKeyConfirm', { key: deleting?.accessKeyId ?? '' }) }}
    </ConfirmDialog>
  </div>
</template>

<style lang="scss">
  .keys-table {
    &__none {
      margin: 0;
      padding: var(--tt-space-3) var(--tt-space-4);
      font-size: var(--tt-text-sm);
      color: var(--tt-text-3);
    }

    &__buttons {
      display: inline-flex;
      flex-wrap: wrap;
      gap: var(--tt-space-2);
    }

    &__stack {
      display: inline-flex;
      flex-direction: column;
      gap: 2px;
    }

    &__warn {
      font-size: var(--tt-text-xs);
      color: var(--tt-warning);
    }
  }
</style>
