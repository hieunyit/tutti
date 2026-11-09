/*
 * Copyright © 2026 hieunyit
 *
 *  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  https://www.gnu.org/licenses/agpl-3.0.html
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

import type { ConsoleUser } from '@/utils/types/users';

export type ConsoleUserStatus = 'active' | 'invited' | 'disabled';

export interface ConsoleUserActions {
  isSelf: boolean;
  isConfig: boolean;
  /** change the role */
  canEdit: boolean;
  canResetPassword: boolean;
  canToggleDisabled: boolean;
  canDelete: boolean;
}

export type PasswordStrength = 'weak' | 'fair' | 'strong';

export type PasswordError = 'tooShort' | 'tooLong' | 'sameAsUsername' | null;

export const PASSWORD_MIN_LENGTH = 12;
/** bcrypt limit in bytes */
export const PASSWORD_MAX_BYTES = 72;

export abstract class ConsoleUserHelper {
  /** Disabled wins; invited = must change the password and never signed in. */
  static status(user: ConsoleUser): ConsoleUserStatus {
    if (user.disabled) return 'disabled';

    if (user.mustChangePassword && !user.lastLoginAt) return 'invited';

    return 'active';
  }

  /**
   * Config users are read-only. Nobody can disable, demote, delete or reset
   * themselves here (they use "Change password" in the account menu).
   */
  static actions(
    user: ConsoleUser,
    currentUsername: string | null,
  ): ConsoleUserActions {
    const isSelf =
      currentUsername !== null && user.username === currentUsername;
    const isConfig = user.source === 'config';
    const managed = !isConfig;

    return {
      isSelf,
      isConfig,
      canEdit: managed && !isSelf,
      canResetPassword: managed && !isSelf,
      canToggleDisabled: managed && !isSelf,
      canDelete: managed && !isSelf,
    };
  }

  static adminCount(users: ConsoleUser[]): number {
    return users.filter((user) => user.role === 'admin').length;
  }

  static validatePassword(password: string, username = ''): PasswordError {
    if (password.length < PASSWORD_MIN_LENGTH) return 'tooShort';

    if (new TextEncoder().encode(password).length > PASSWORD_MAX_BYTES) {
      return 'tooLong';
    }

    if (username !== '' && password === username) return 'sameAsUsername';

    return null;
  }

  /** Rough hint only: length and character classes. */
  static passwordStrength(password: string): PasswordStrength {
    const classes = [/[a-z]/, /[A-Z]/, /\d/, /[^A-Za-z0-9]/].filter((re) =>
      re.test(password),
    ).length;

    if (password.length >= 16 && classes >= 3) return 'strong';

    if (password.length >= PASSWORD_MIN_LENGTH && classes >= 2) return 'fair';

    return 'weak';
  }
}
