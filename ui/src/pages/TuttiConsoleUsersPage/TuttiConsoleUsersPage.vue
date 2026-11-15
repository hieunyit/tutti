<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import {
    NButton,
    NDropdown,
    type DataTableColumns,
    type DropdownOption,
  } from 'naive-ui';
  import ConsoleUserDialog from './ConsoleUserDialog.vue';
  import messages from './i18nConsoleUsers';
  import { consoleUserMenu, type ConsoleUserAction } from './consoleUserMenu';
  import { useConsoleUserFlows } from './useConsoleUserFlows';
  import {
    Callout,
    ConfirmDialog,
    DataTable,
    EmptyBlock,
    PageShell,
    StatusBadge,
    TableToolbar,
    TagChip,
    type StatusTone,
  } from '@/components/kit';
  import OneTimeSecretModal from '@/components/tutti/users/OneTimeSecretModal.vue';
  import { useTuttiConsoleUsersStore } from '@/stores/tuttiConsoleUsersStore';
  import { useAuthStore } from '@/stores/authStore';
  import { ConsoleUserHelper } from '@/utils/helpers/ConsoleUserHelper';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import type { ConsoleUser } from '@/utils/types/users';

  const { t } = useI18n({ messages });
  const store = useTuttiConsoleUsersStore();
  const auth = useAuthStore();
  const flows = useConsoleUserFlows(t);
  const loaded = ref(false);
  const search = ref('');

  const state = computed(() => {
    if (!auth.isAdmin) return 'ready' as const;

    if (store.hasError) return 'error' as const;

    return loaded.value ? ('ready' as const) : ('loading' as const);
  });
  const rows = computed(() => {
    const q = search.value.trim().toLowerCase();

    return q
      ? store.users.filter((u) => u.username.toLowerCase().includes(q))
      : store.users;
  });
  const isEmpty = computed(() => loaded.value && store.users.length === 0);

  const TONES: Record<string, StatusTone> = {
    active: 'success',
    invited: 'info',
    disabled: 'neutral',
  };

  function menuOptions(user: ConsoleUser): DropdownOption[] {
    const { enabled } = consoleUserMenu(user, store.users, auth.username);

    return [
      { key: 'role', label: t('changeRole'), disabled: !enabled.role },
      { key: 'reset', label: t('resetPassword'), disabled: !enabled.reset },
      {
        key: 'toggle',
        label: t(user.disabled ? 'enable' : 'disable'),
        disabled: !enabled.toggle,
      },
      { key: 'delete', label: t('delete'), disabled: !enabled.delete },
    ];
  }

  function onMenu(user: ConsoleUser, key: ConsoleUserAction) {
    if (key === 'role') flows.openForm(user);
    else flows.pending.value = { user, kind: key };
  }

  const columns = computed<DataTableColumns<ConsoleUser>>(() => [
    {
      title: t('username'),
      key: 'username',
      sorter: 'default',
      render: (user) => {
        const { isSelf, isConfig } = consoleUserMenu(
          user,
          store.users,
          auth.username,
        );

        return h('span', { class: 'console-name' }, [
          h('strong', user.username),
          isSelf ? h(TagChip, { label: t('you') }) : null,
          isConfig
            ? h(TagChip, { label: t('config'), title: t('configHint') })
            : null,
        ]);
      },
    },
    {
      title: t('role'),
      key: 'role',
      render: (user) => t(user.role === 'admin' ? 'admin' : 'viewer'),
    },
    {
      title: t('status'),
      key: 'status',
      render: (user) => {
        const status = ConsoleUserHelper.status(user);

        return h(StatusBadge, { tone: TONES[status], label: t(status) });
      },
    },
    {
      title: t('lastLogin'),
      key: 'lastLoginAt',
      render: (user) => formatDateTime(user.lastLoginAt ?? undefined),
    },
    {
      title: '',
      key: 'actions',
      width: 56,
      align: 'right',
      render: (user) => {
        if (!consoleUserMenu(user, store.users, auth.username).hasMenu)
          return null;

        return h(
          NDropdown,
          {
            trigger: 'click',
            placement: 'bottom-end',
            options: menuOptions(user),
            onSelect: (key: ConsoleUserAction) => onMenu(user, key),
          },
          () =>
            h(
              NButton,
              {
                size: 'small',
                quaternary: true,
                disabled: flows.busy.value,
                'aria-label': t('rowActions', { name: user.username }),
                'data-testid': `console-menu-${user.username}`,
              },
              () => '⋯',
            ),
        );
      },
    },
  ]);

  onMounted(async () => {
    if (!auth.isAdmin) return;

    await store.load();
    loaded.value = true;
  });
  onBeforeUnmount(() => {
    flows.oneTime.value = null;
  });
</script>

<template>
  <PageShell
    :title="t('title')"
    :state="state"
    @retry="store.load()"
  >
    <template #actions>
      <NButton
        v-if="auth.isAdmin && !isEmpty"
        type="primary"
        :disabled="store.authDisabled || store.hasError"
        @click="flows.openForm(null)"
        >{{ t('create') }}</NButton
      >
    </template>

    <EmptyBlock
      v-if="!auth.isAdmin"
      :title="t('forbiddenTitle')"
      :description="t('forbiddenText')"
    />
    <template v-else>
      <Callout
        v-if="store.authDisabled"
        tone="warning"
        >{{ t('authDisabled') }}</Callout
      >
      <Callout
        v-if="flows.error.value && !flows.formOpen.value"
        tone="danger"
        >{{ flows.error.value }}</Callout
      >
      <TableToolbar
        v-model:search="search"
        :search-label="t('searchLabel')"
        :hidden="isEmpty"
        @clear="search = ''"
      />
      <DataTable
        :columns="columns"
        :rows="rows"
        :loading="store.isLoading"
        :row-key="(user: ConsoleUser) => user.username"
        @retry="store.load()"
      >
        <template #empty>
          <EmptyBlock
            v-if="search.trim()"
            :title="t('noMatchTitle')"
          >
            <NButton @click="search = ''">{{ t('clearSearch') }}</NButton>
          </EmptyBlock>
          <EmptyBlock
            v-else
            :title="t('emptyTitle')"
          >
            <NButton
              type="primary"
              :disabled="store.authDisabled"
              @click="flows.openForm(null)"
              >{{ t('create') }}</NButton
            >
          </EmptyBlock>
        </template>
      </DataTable>
    </template>

    <ConsoleUserDialog
      :show="flows.formOpen.value"
      :user="flows.edited.value"
      :busy="flows.busy.value"
      :error="flows.error.value"
      :protected-admin="flows.editedProtected.value"
      @save="flows.save"
      @close="flows.formOpen.value = false"
    />
    <ConfirmDialog
      :open="flows.pending.value !== null"
      :title="
        flows.pending.value ? flows.confirmLabel(flows.pending.value) : ''
      "
      :target-name="flows.pending.value?.user.username"
      :confirm-label="
        flows.pending.value ? flows.confirmLabel(flows.pending.value) : ''
      "
      :danger="flows.pending.value?.kind === 'delete'"
      :loading="flows.busy.value"
      @confirm="flows.confirm"
      @cancel="flows.pending.value = null"
    >
      {{ flows.pending.value ? flows.confirmText(flows.pending.value) : '' }}
    </ConfirmDialog>
    <OneTimeSecretModal
      :value="flows.oneTime.value"
      :title="t('newPasswordTitle')"
      :label="t('passwordLabel')"
      :close-label="t('close')"
      @close="flows.oneTime.value = null"
    />
  </PageShell>
</template>

<style lang="scss">
  .console-name {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--tt-space-2);
  }
</style>
