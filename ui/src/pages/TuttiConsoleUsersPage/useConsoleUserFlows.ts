// Copyright © 2026 hieunyit. Licensed under AGPL-3.0.
import { computed, ref } from 'vue';
import { isProtectedAdmin } from './consoleUserMenu';
import type { ConsoleUserFormValue } from './ConsoleUserDialog.vue';
import { useTuttiConsoleUsersStore } from '@/stores/tuttiConsoleUsersStore';
import { TuttiService } from '@/services/TuttiService';
import { ProxyAccountHelper } from '@/utils/helpers/ProxyAccountHelper';
import type { ConsoleUser } from '@/utils/types/users';

export type PendingKind = 'reset' | 'toggle' | 'delete';
export interface PendingAction {
  user: ConsoleUser;
  kind: PendingKind;
}

/** Dialog, confirmation and one-time password state of the console users page. */
export function useConsoleUserFlows(
  t: (key: string, params?: Record<string, string>) => string,
) {
  const store = useTuttiConsoleUsersStore();
  const formOpen = ref(false);
  const edited = ref<ConsoleUser | null>(null);
  const pending = ref<PendingAction | null>(null);
  /** generated password, shown once, kept in memory only */
  const oneTime = ref<string | null>(null);
  const error = ref('');
  const busy = ref(false);

  const editedProtected = computed(
    () => !!edited.value && isProtectedAdmin(edited.value, store.users),
  );

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

    const { user, kind } = action;

    await run(async () => {
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

  function confirmText(action: PendingAction) {
    const params = { name: action.user.username };

    if (action.kind === 'reset') return t('resetConfirm', params);

    if (action.kind === 'delete') return t('deleteConfirm', params);

    return t(action.user.disabled ? 'enableConfirm' : 'disableConfirm', params);
  }

  function confirmLabel(action: PendingAction) {
    if (action.kind === 'reset') return t('resetPassword');

    if (action.kind === 'delete') return t('deleteLabel');

    return t(action.user.disabled ? 'enable' : 'disable');
  }

  return {
    formOpen,
    edited,
    editedProtected,
    pending,
    oneTime,
    error,
    busy,
    openForm,
    save,
    confirm,
    confirmText,
    confirmLabel,
  };
}
