<!-- Copyright © 2026 hieunyit. Licensed under AGPL-3.0. -->
<script setup lang="ts">
  import { computed, h, onBeforeUnmount, onMounted, ref } from 'vue';
  import { useI18n } from 'vue-i18n';
  import { NButton, type DataTableColumns } from 'naive-ui';
  import {
    Callout,
    ConfirmDialog,
    DataTable,
    EmptyBlock,
    PageShell,
    StatusBadge,
    type StatusTone,
  } from '@/components/kit';
  import ConsoleUserFormModal, {
    type ConsoleUserFormValue,
  } from '@/components/tutti/users/ConsoleUserFormModal.vue';
  import OneTimeSecretModal from '@/components/tutti/users/OneTimeSecretModal.vue';
  import messages from '@/components/tutti/users/i18nAccess';
  import { useTuttiConsoleUsersStore } from '@/stores/tuttiConsoleUsersStore';
  import { useAuthStore } from '@/stores/authStore';
  import { TuttiService } from '@/services/TuttiService';
  import { ConsoleUserHelper } from '@/utils/helpers/ConsoleUserHelper';
  import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
  import { formatDateTime } from '@/utils/helpers/DateTimeHelper';
  import type { ConsoleUser } from '@/utils/types/users';

  type Kind = 'reset' | 'toggle' | 'delete';

  const { t } = useI18n({ messages });
  const store = useTuttiConsoleUsersStore();
  const auth = useAuthStore();
  const formOpen = ref(false);
  const edited = ref<ConsoleUser | null>(null);
  const pending = ref<{ user: ConsoleUser; kind: Kind } | null>(null);
  /** generated password, shown once, kept in memory only */
  const oneTime = ref<string | null>(null);
  const error = ref('');
  const busy = ref(false);
  const loaded = ref(false);

  const enabledAdmins = computed(
    () => store.users.filter((u) => u.role === 'admin' && !u.disabled).length,
  );
  const state = computed(() => {
    if (!auth.isAdmin) return 'ready' as const;

    if (store.hasError) return 'error' as const;

    return loaded.value ? ('ready' as const) : ('loading' as const);
  });

  function protectedAdmin(user: ConsoleUser) {
    return user.role === 'admin' && !user.disabled && enabledAdmins.value <= 1;
  }

  function actions(user: ConsoleUser) {
    return ConsoleUserHelper.actions(user, auth.username);
  }

  function statusTone(user: ConsoleUser): StatusTone {
    const status = ConsoleUserHelper.status(user);

    return status === 'disabled'
      ? 'neutral'
      : status === 'invited'
        ? 'info'
        : 'success';
  }

  function openForm(user: ConsoleUser | null) {
    edited.value = user;
    error.value = '';
    formOpen.value = true;
  }

  async function run(task: () => Promise<void>) {
    error.value = '';
    busy.value = true;

    try {
      await task();
      await store.load(false);

      return true;
    } catch (e) {
      error.value = ProxyAccountHelper.getMessage(e) ?? t('error');

      return false;
    } finally {
      busy.value = false;
    }
  }

  async function save(value: ConsoleUserFormValue) {
    const user = edited.value;
    const ok = await run(async () => {
      if (user) {
        await TuttiService.updateConsoleUser({
          username: user.username,
          role: value.role,
        });

        return;
      }

      const result = await TuttiService.createConsoleUser({
        username: value.username,
        role: value.role,
        generatePassword: value.generate,
        ...(!value.generate ? { password: value.password } : {}),
      });

      if (result.generatedPassword) oneTime.value = result.generatedPassword;
    });

    if (ok) formOpen.value = false;
  }

  async function confirm() {
    const action = pending.value;

    if (!action) return;

    await run(async () => {
      const { user, kind } = action;

      if (kind === 'reset') {
        const result = await TuttiService.resetConsoleUserPassword({
          username: user.username,
          generatePassword: true,
        });

        if (result.generatedPassword) oneTime.value = result.generatedPassword;
      } else if (kind === 'toggle') {
        await TuttiService.updateConsoleUser({
          username: user.username,
          disabled: !user.disabled,
        });
      } else {
        await TuttiService.deleteConsoleUser(user.username);
      }
    });
    pending.value = null;
  }

  function confirmText(action: { user: ConsoleUser; kind: Kind }) {
    const params = { name: action.user.username };

    if (action.kind === 'reset') return t('resetPasswordConfirm', params);

    if (action.kind === 'delete') return t('deleteConsoleConfirm', params);

    return t(action.user.disabled ? 'enableConfirm' : 'disableConfirm', params);
  }

  function confirmLabel(action: { user: ConsoleUser; kind: Kind }) {
    if (action.kind === 'reset') return t('resetPassword');

    if (action.kind === 'delete') return t('deleteLabel');

    return t(action.user.disabled ? 'enable' : 'disable');
  }

  function actionButton(
    user: ConsoleUser,
    id: string,
    label: string,
    disabled: boolean,
    onClick: () => void,
    danger = false,
  ) {
    return h(
      NButton,
      {
        size: 'small',
        secondary: danger,
        type: danger ? 'error' : 'default',
        disabled: busy.value || disabled,
        'data-testid': `console-${id}-${user.username}`,
        onClick,
      },
      () => label,
    );
  }

  const columns = computed<DataTableColumns<ConsoleUser>>(() => [
    {
      title: t('username'),
      key: 'username',
      sorter: 'default',
      render: (user) =>
        h('span', [
          h('strong', user.username),
          actions(user).isSelf ? ` ${t('self')}` : '',
        ]),
    },
    {
      title: t('role'),
      key: 'role',
      render: (user) => t(user.role === 'admin' ? 'admin' : 'viewer'),
    },
    {
      title: t('status'),
      key: 'status',
      render: (user) =>
        h(StatusBadge, {
          tone: statusTone(user),
          label: t(ConsoleUserHelper.status(user)),
        }),
    },
    {
      title: t('source'),
      key: 'source',
      render: (user) => t(user.source === 'config' ? 'config' : 'managed'),
    },
    {
      title: t('lastLogin'),
      key: 'lastLoginAt',
      render: (user) => formatDateTime(user.lastLoginAt ?? undefined),
    },
    {
      title: t('actions'),
      key: 'actions',
      render: (user) => {
        const can = actions(user);

        if (can.isConfig) return h('span', t('editConfig'));

        const guarded = protectedAdmin(user);

        return h('div', { class: 'console-actions' }, [
          actionButton(
            user,
            'role',
            t('changeRole'),
            !can.canEdit || guarded,
            () => openForm(user),
          ),
          actionButton(
            user,
            'reset',
            t('resetPassword'),
            !can.canResetPassword,
            () => (pending.value = { user, kind: 'reset' }),
          ),
          actionButton(
            user,
            'toggle',
            t(user.disabled ? 'enable' : 'disable'),
            !can.canToggleDisabled || guarded,
            () => (pending.value = { user, kind: 'toggle' }),
          ),
          actionButton(
            user,
            'delete',
            t('delete'),
            !can.canDelete || guarded,
            () => (pending.value = { user, kind: 'delete' }),
            true,
          ),
        ]);
      },
    },
  ]);

  onMounted(async () => {
    if (!auth.isAdmin) return;

    await store.load();
    loaded.value = true;
  });
  onBeforeUnmount(() => {
    oneTime.value = null;
  });
</script>

<template>
  <PageShell
    :title="t('consoleUsers')"
    :description="t('consoleHint')"
    :state="state"
    @retry="store.load()"
  >
    <template #actions>
      <template v-if="auth.isAdmin">
        <NButton
          :loading="store.isLoading"
          @click="store.load()"
          >{{ t('refresh') }}</NButton
        >
        <NButton
          type="primary"
          :disabled="store.authDisabled || store.hasError"
          @click="openForm(null)"
          >{{ t('createConsoleUser') }}</NButton
        >
      </template>
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
        v-if="error && !formOpen"
        tone="danger"
        >{{ error }}</Callout
      >
      <DataTable
        :columns="columns"
        :rows="store.users"
        :loading="store.isLoading"
        :row-key="(user: ConsoleUser) => user.username"
        @retry="store.load()"
      >
        <template #empty>
          <EmptyBlock :title="t('emptyConsoleTitle')">
            <NButton
              type="primary"
              :disabled="store.authDisabled"
              @click="openForm(null)"
              >{{ t('createConsoleUser') }}</NButton
            >
          </EmptyBlock>
        </template>
      </DataTable>
    </template>

    <ConsoleUserFormModal
      :show="formOpen"
      :user="edited"
      :busy="busy"
      :error="error"
      :protected-admin="!!edited && protectedAdmin(edited)"
      @save="save"
      @close="formOpen = false"
    />
    <ConfirmDialog
      :open="pending !== null"
      :title="pending ? confirmLabel(pending) : ''"
      :target-name="pending?.user.username"
      :confirm-label="pending ? confirmLabel(pending) : ''"
      :danger="pending?.kind === 'delete'"
      :loading="busy"
      @confirm="confirm"
      @cancel="pending = null"
    >
      {{ pending ? confirmText(pending) : '' }}
    </ConfirmDialog>
    <OneTimeSecretModal
      :value="oneTime"
      :title="t('newPasswordTitle')"
      :label="t('password')"
      :close-label="t('closeAfterSaving')"
      @close="oneTime = null"
    />
  </PageShell>
</template>

<style lang="scss">
  .console-actions {
    display: flex;
    flex-wrap: wrap;
    gap: var(--tt-space-2);
  }
</style>
