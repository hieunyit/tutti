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
  token: string | null;
  username: string | null;
  role: AuthRole | null;
  isLoggingIn: boolean;
  loginError: AuthLoginErrorKind | null;
  /** seconds from Retry-After when rate limited */
  retryAfterSeconds: number | null;
}

function getInitialState(): AuthState {
  return {
    isInitialized: false,
    authEnabled: null,
    token: AuthTokenHelper.get(),
    username: null,
    role: null,
    isLoggingIn: false,
    loginError: null,
    retryAfterSeconds: null,
  };
}

export const useAuthStore = defineStore('auth', () => {
  const state = reactive<AuthState>(getInitialState());

  /** With auth disabled everybody has full access (legacy behaviour). */
  const isAuthenticated = computed(
    () => state.authEnabled === false || state.token !== null,
  );
  /** Single source of truth for every mutating UI action. */
  const canWrite = computed(
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
  }

  function clearSession() {
    AuthTokenHelper.clear();
    state.token = null;
    state.username = null;
    state.role = null;
  }

  /** Asks the backend who we are. Idempotent; call from the router guard. */
  async function init(force = false): Promise<void> {
    if (state.isInitialized && !force) return;

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
        // backend unreachable: keep going, API calls will surface the error;
        // a stored token keeps the session assumption.
        state.authEnabled = state.token !== null ? true : false;
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

    try {
      const res = await AuthService.login({ username, password });

      AuthTokenHelper.set(res.token, keepSignedIn);
      state.token = res.token;
      state.username = res.username;
      state.role = res.role;
      state.authEnabled = true;
      state.isInitialized = true;

      return true;
    } catch (e) {
      const status = axios.isAxiosError(e) ? e.response?.status : undefined;

      if (status === 401) {
        state.loginError = 'invalidCredentials';
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
    isViewer,
    showUserMenu,
    init,
    login,
    logout,
    handleUnauthorized,
    $reset,
  };
});
