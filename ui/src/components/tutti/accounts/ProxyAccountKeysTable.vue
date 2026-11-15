<!-- Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. -->
<script setup lang="ts">
  /**
   * Every access key as a table (cards under 640 px). Config keys are
   * read-only; managed keys can be rotated, disabled or enabled, and deleted
   * after a confirmation that names the key.
   */
  import { computed, h, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { RouterLink } from 'vue-router';
  import { type DataTableColumns } from 'naive-ui';
  import KeyRowMenu from './KeyRowMenu.vue';
  import i18nAccounts from './i18nAccounts';
  import {
    dateCell,
    isDisabled,
    oldSecretNote,
    sourceLabel,
    toKeyRows,
    type KeyRow,
  } from './accessKeyRows';
  import AccessKeyCardList from './AccessKeyCardList.vue';
  import {
    ConfirmDialog,
    CopyValue,
    DataTable,
    StatusBadge,
  } from '@/components/kit';
  import { EMPTY_VALUE } from '@/components/ui/status';
  import { useNarrowScreen } from '@/components/tutti/storages/useNarrowScreen';
  import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
  import { useTuttiNotification } from '@/utils/composables/useTuttiNotification';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import { RouteName } from '@/utils/types/router';
  import type { TuttiProxyAccount } from '@/utils/types/tutti';

  const props = defineProps<{ accounts: TuttiProxyAccount[] }>();
  const emit = defineEmits<{ (e: 'rotate', accessKeyId: string): void }>();

  const { t } = useI18n({ messages: i18nAccounts });
  const store = useTuttiAccessPolicyStore();
  const { createNotification } = useTuttiNotification();
  const narrow = useNarrowScreen();
  const rows = computed(() => toKeyRows(props.accounts));
  const deleting = ref<KeyRow | null>(null);
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

  async function toggle(row: KeyRow) {
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

  const columns = computed<DataTableColumns<KeyRow>>(() => [
    {
      title: t('columnAccessKeyId'),
      key: 'accessKeyId',
      render: (row) =>
        h('div', { class: 'keys-cell' }, [
          h(CopyValue, {
            value: row.accessKeyId,
            label: t('columnAccessKeyId'),
          }),
          h('small', sourceLabel(row.source, t)),
        ]),
    },
    {
      title: t('columnUser'),
      key: 'user',
      sorter: 'default',
      render: (row) =>
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
    {
      title: t('columnStatus'),
      key: 'status',
      render: (row) =>
        h(StatusBadge, {
          tone: isDisabled(row) ? 'neutral' : 'success',
          label: t(isDisabled(row) ? 'statusDisabled' : 'statusActive'),
        }),
    },
    {
      title: t('columnDescription'),
      key: 'description',
      render: (row) => row.dynamicKey?.description || EMPTY_VALUE,
    },
    {
      title: t('columnUpdated'),
      key: 'rotatedAt',
      render: (row) => {
        const note = oldSecretNote(row, t);

        return h('div', { class: 'keys-cell' }, [
          dateCell(row.dynamicKey?.rotatedAt ?? row.dynamicKey?.createdAt),
          note ? h('small', { class: 'keys-cell__warn' }, note) : null,
        ]);
      },
    },
    {
      title: t('columnActions'),
      key: 'actions',
      render: (row) =>
        h(KeyRowMenu, {
          row,
          onRotate: () => emit('rotate', row.accessKeyId),
          onToggle: () => toggle(row),
          onDelete: () => (deleting.value = row),
        }),
    },
  ]);
</script>

<template>
  <div class="keys-table">
    <AccessKeyCardList
      v-if="narrow && rows.length > 0"
      :rows="rows"
      @rotate="emit('rotate', $event)"
      @toggle="toggle"
      @delete="deleting = $event"
    />
    <DataTable
      v-else
      :columns="columns"
      :rows="rows"
      :loading="false"
      :row-key="(row: KeyRow) => `${row.user}:${row.accessKeyId}`"
    />

    <ConfirmDialog
      :open="deleting !== null"
      :title="t('deleteKeyDialogTitle', { key: deleting?.accessKeyId ?? '' })"
      :target-name="deleting?.accessKeyId"
      :confirm-label="t('deleteKey')"
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
  .keys-cell {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--tt-space-1);

    small {
      font-size: var(--tt-text-xs);
      color: var(--tt-text-3);
    }

    &__warn {
      color: var(--tt-warning);
    }
  }
</style>
