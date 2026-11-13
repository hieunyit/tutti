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

import axios from 'axios';
import { defineStore } from 'pinia';
import { computed, reactive, toRefs } from 'vue';
import { AuthService } from '@/services/AuthService';
import { AuthTokenHelper } from '@/utils/helpers/AuthTokenHelper';
import { clearDedupeCache } from '@/utils/helpers/dedupe';
import {
  AuthRole,
  type AuthLoginErrorKind,
  type AuthMeResponse,
} from '@/utils/types/auth';

interface AuthState {
  /** false until /auth/me has been answered once */
  isInitialized: boolean;
  /** null = unknown */
  authEnabled: boolean | null;
  initError: boolean;
  token: string | null;
  username: string | null;
  role: AuthRole | null;
  /** 'config' users cannot change their password here */
  source: 'config' | 'managed' | null;
  mustChangePassword: boolean;
  /** operator warning codes from /auth/me */
  warnings: string[];
  isLoggingIn: boolean;
  loginError: AuthLoginErrorKind | null;
  /** seconds from Retry-After when rate limited */
  retryAfterSeconds: number | null;
  /** failed attempts left before a lock, only when the API reports it */
  remainingAttempts: number | null;
}

function getInitialState(): AuthState {
  return {
    isInitialized: false,
    authEnabled: null,
    initError: false,
    token: AuthTokenHelper.get(),
    username: null,
    role: null,
    source: null,
    mustChangePassword: false,
    warnings: [],
    isLoggingIn: false,
    loginError: null,
    retryAfterSeconds: null,
    remainingAttempts: null,
  };
}

export const useAuthStore = defineStore('auth', () => {
  const state = reactive<AuthState>(getInitialState());

  /** With auth disabled everybody has full access (legacy behaviour). */
  const isAuthenticated = computed(
    () =>
      state.authEnabled === false ||
      (state.authEnabled === true && state.token !== null),
  );
  /** Single source of truth for every mutating UI action. */
  const canWrite = computed(
    () => state.authEnabled === false || state.role === AuthRole.ADMIN,
  );
  /** Administrators only; with sign-in disabled everybody is admin. */
  const isAdmin = computed(
    () => state.authEnabled === false || state.role === AuthRole.ADMIN,
  );
  const isViewer = computed(() => !canWrite.value);
  const showUserMenu = computed(
    () => state.authEnabled === true && state.token !== null,
  );

  function applyMe(me: AuthMeResponse) {
    state.authEnabled = me.authEnabled;
    state.username = me.username ?? null;
    state.role = me.role ?? null;
    state.source = me.source ?? null;
    state.mustChangePassword = me.mustChangePassword ?? false;
    state.warnings = me.warnings ?? [];
  }

  function clearSession() {
    AuthTokenHelper.clear();
    state.token = null;
    state.username = null;
    state.role = null;
    state.source = null;
    state.mustChangePassword = false;
  }

  /** Asks the backend who we are. Idempotent; call from the router guard. */
  async function init(force = false): Promise<void> {
    if (state.isInitialized && !force) return;

    state.initError = false;

    try {
      applyMe(await AuthService.me());
    } catch (e) {
      if (axios.isAxiosError(e) && e.response?.status === 401) {
        // auth is enabled and our token is missing/expired
        state.authEnabled = true;
        clearSession();
      } else if (axios.isAxiosError(e) && e.response?.status === 404) {
        state.authEnabled = false;
      } else {
        // A network failure does not mean authentication is disabled.
        // Retain the token for retry, but do not present stale admin controls.
        state.authEnabled = null;
        state.role = null;
        state.initError = true;
      }
    } finally {
      state.isInitialized = true;
    }
  }

  async function login(
    username: string,
    password: string,
    keepSignedIn: boolean,
  ): Promise<boolean> {
    state.isLoggingIn = true;
    state.loginError = null;
    state.retryAfterSeconds = null;
    state.remainingAttempts = null;

    try {
      const res = await AuthService.login({ username, password });

      AuthTokenHelper.set(res.token, keepSignedIn);
      state.token = res.token;
      state.username = res.username;
      state.role = res.role;
      state.authEnabled = true;
      state.initError = false;

      // the login response does not say: /auth/me does
      try {
        applyMe(await AuthService.me());
      } catch {
        // keep the login data
      }
      state.isInitialized = true;

      return true;
    } catch (e) {
      const status = axios.isAxiosError(e) ? e.response?.status : undefined;

      if (status === 401) {
        state.loginError = 'invalidCredentials';

        const data = axios.isAxiosError(e) ? e.response?.data : undefined;
        const remaining = Number(
          (data as { remainingAttempts?: unknown } | undefined)
            ?.remainingAttempts ??
            (axios.isAxiosError(e)
              ? e.response?.headers?.['x-remaining-attempts']
              : undefined),
        );

        state.remainingAttempts =
          Number.isInteger(remaining) && remaining >= 0 ? remaining : null;
      } else if (status === 429) {
        state.loginError = 'rateLimited';

        const retryAfter = Number(
          axios.isAxiosError(e) ? e.response?.headers?.['retry-after'] : NaN,
        );

        state.retryAfterSeconds =
          Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter : null;
      } else {
        state.loginError = 'unknown';
      }

      return false;
    } finally {
      state.isLoggingIn = false;
    }
  }

  /** Best-effort server-side revoke, always clears local state. */
  async function logout(): Promise<void> {
    try {
      await AuthService.logout();
    } catch {
      // token may already be invalid; local logout must still succeed
    }

    clearSession();
    clearDedupeCache();
  }

  /** Called by the HTTP interceptor on 403 PASSWORD_CHANGE_REQUIRED. */
  function handlePasswordChangeRequired() {
    state.mustChangePassword = true;
  }

  /** Sets a new password; the calling session stays valid. */
  async function changePassword(
    currentPassword: string,
    newPassword: string,
  ): Promise<void> {
    await AuthService.changePassword(currentPassword, newPassword);
    state.mustChangePassword = false;
  }

  /** Called by the HTTP interceptor on any 401. */
  function handleUnauthorized() {
    clearSession();
  }

  function $reset() {
    Object.assign(state, getInitialState());
  }

  return {
    ...toRefs(state),
    isAuthenticated,
    canWrite,
    isAdmin,
    isViewer,
    showUserMenu,
    init,
    login,
    logout,
    handleUnauthorized,
    handlePasswordChangeRequired,
    changePassword,
    $reset,
  };
});
