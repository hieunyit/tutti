// Copyright © 2026 hieunyit. Licensed under AGPL-3.0.
import { ConsoleUserHelper } from '@/utils/helpers/ConsoleUserHelper';
import type { ConsoleUser } from '@/utils/types/users';

export type ConsoleUserAction = 'role' | 'reset' | 'toggle' | 'delete';

export interface ConsoleUserMenuState {
  isSelf: boolean;
  isConfig: boolean;
  /** which actions are offered and enabled; empty = no menu at all */
  enabled: Record<ConsoleUserAction, boolean>;
  /** the row has a menu (managed users other than yourself) */
  hasMenu: boolean;
}

/** The last enabled administrator can be neither demoted, disabled nor deleted. */
export function isProtectedAdmin(user: ConsoleUser, users: ConsoleUser[]) {
  const enabledAdmins = users.filter(
    (u) => u.role === 'admin' && !u.disabled,
  ).length;

  return user.role === 'admin' && !user.disabled && enabledAdmins <= 1;
}

export function consoleUserMenu(
  user: ConsoleUser,
  users: ConsoleUser[],
  currentUsername: string | null,
): ConsoleUserMenuState {
  const can = ConsoleUserHelper.actions(user, currentUsername);
  const guarded = isProtectedAdmin(user, users);
  const enabled = {
    role: can.canEdit && !guarded,
    reset: can.canResetPassword,
    toggle: can.canToggleDisabled && !guarded,
    delete: can.canDelete && !guarded,
  };

  return {
    isSelf: can.isSelf,
    isConfig: can.isConfig,
    enabled,
    hasMenu: !can.isConfig && !can.isSelf,
  };
}
