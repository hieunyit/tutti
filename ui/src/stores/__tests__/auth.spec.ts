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

import { AxiosError, AxiosHeaders } from 'axios';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useAuthStore } from '../authStore';
import { AuthService } from '@/services/AuthService';
import { AuthTokenHelper } from '@/utils/helpers/AuthTokenHelper';
import { AuthRole } from '@/utils/types/auth';

vi.mock('@/services/AuthService', () => ({
  AuthService: { login: vi.fn(), logout: vi.fn(), me: vi.fn() },
}));

type MockFn = ReturnType<typeof vi.fn>;

const svc = AuthService as unknown as Record<'login' | 'logout' | 'me', MockFn>;

function httpError(
  status: number,
  headers: Record<string, string> = {},
  data: unknown = {},
) {
  return new AxiosError('x', String(status), undefined, undefined, {
    status,
    statusText: '',
    data,
    headers,
    config: { headers: new AxiosHeaders() },
  });
}

beforeEach(() => {
  window.sessionStorage.clear();
  window.localStorage.clear();
  setActivePinia(createPinia());
  vi.resetAllMocks();
});

describe('authStore', () => {
  describe('init', () => {
    it('does not grant administrator controls when the server is unreachable', async () => {
      svc.me.mockRejectedValue(new Error('network'));
      const store = useAuthStore();

      await store.init();
      expect(store.authEnabled).toBeNull();
      expect(store.initError).toBe(true);
      expect(store.canWrite).toBe(false);
      expect(store.isAdmin).toBe(false);
      expect(store.isAuthenticated).toBe(false);
    });

    it('retains a stored token for connection retry without trusting a stale role', async () => {
      AuthTokenHelper.set('tok', false);
      svc.me.mockResolvedValueOnce({
        authEnabled: true,
        username: 'admin',
        role: AuthRole.ADMIN,
      });
      const store = useAuthStore();

      await store.init();
      svc.me.mockRejectedValueOnce(new Error('network'));
      await store.init(true);
      expect(store.isAdmin).toBe(false);
      expect(store.isAuthenticated).toBe(false);
      expect(AuthTokenHelper.get()).toBe('tok');
      svc.me.mockResolvedValueOnce({
        authEnabled: true,
        username: 'viewer',
        role: AuthRole.VIEWER,
      });
      await store.init(true);
      expect(store.initError).toBe(false);
      expect(store.isAuthenticated).toBe(true);
      expect(store.canWrite).toBe(false);
    });

    it('with auth disabled grants full access and hides the user menu', async () => {
      svc.me.mockResolvedValue({ authEnabled: false });
      const store = useAuthStore();

      await store.init();
      expect(store.authEnabled).toBe(false);
      expect(store.isAuthenticated).toBe(true);
      expect(store.canWrite).toBe(true);
      expect(store.showUserMenu).toBe(false);
    });

    it('stores identity from /auth/me', async () => {
      AuthTokenHelper.set('tok', false);
      setActivePinia(createPinia());
      svc.me.mockResolvedValue({
        authEnabled: true,
        username: 'viewer',
        role: AuthRole.VIEWER,
      });
      const store = useAuthStore();

      await store.init();
      expect(store.isAuthenticated).toBe(true);
      expect(store.canWrite).toBe(false);
      expect(store.isViewer).toBe(true);
      expect(store.showUserMenu).toBe(true);
    });

    it('treats 401 as enabled-but-unauthenticated and drops the stale token', async () => {
      AuthTokenHelper.set('stale', false);
      setActivePinia(createPinia());
      svc.me.mockRejectedValue(httpError(401));
      const store = useAuthStore();

      await store.init();
      expect(store.authEnabled).toBe(true);
      expect(store.isAuthenticated).toBe(false);
      expect(AuthTokenHelper.get()).toBeNull();
    });

    it('asks the backend only once', async () => {
      svc.me.mockResolvedValue({ authEnabled: false });
      const store = useAuthStore();

      await store.init();
      await store.init();
      expect(svc.me).toHaveBeenCalledTimes(1);
    });
  });

  describe('login', () => {
    it('keeps the token in sessionStorage by default', async () => {
      svc.login.mockResolvedValue({
        token: 't1',
        username: 'admin',
        role: AuthRole.ADMIN,
      });
      const store = useAuthStore();

      expect(await store.login('admin', 'pw', false)).toBe(true);
      expect(window.sessionStorage.getItem('tutti.auth.token')).toBe('t1');
      expect(window.localStorage.getItem('tutti.auth.token')).toBeNull();
      expect(store.token).toBe('t1');
      expect(store.canWrite).toBe(true);
    });

    it('uses localStorage for "keep me signed in"', async () => {
      svc.login.mockResolvedValue({
        token: 't2',
        username: 'admin',
        role: AuthRole.ADMIN,
      });
      const store = useAuthStore();

      await store.login('admin', 'pw', true);
      expect(window.localStorage.getItem('tutti.auth.token')).toBe('t2');
      expect(window.sessionStorage.getItem('tutti.auth.token')).toBeNull();
    });

    it('reports wrong credentials (401)', async () => {
      svc.login.mockRejectedValue(httpError(401));
      const store = useAuthStore();

      expect(await store.login('a', 'b', false)).toBe(false);
      expect(store.loginError).toBe('invalidCredentials');
      expect(store.token).toBeNull();
      expect(store.isLoggingIn).toBe(false);
    });

    it('uses the remaining attempt count from the login response', async () => {
      svc.login.mockRejectedValue(
        httpError(
          401,
          {},
          {
            code: 401,
            message: 'invalid username or password',
            remainingAttempts: 7,
          },
        ),
      );
      const store = useAuthStore();

      await store.login('unknown-or-known', 'wrong', false);

      expect(store.loginError).toBe('invalidCredentials');
      expect(store.remainingAttempts).toBe(7);
    });

    it('reports rate limiting with Retry-After (429)', async () => {
      svc.login.mockRejectedValue(httpError(429, { 'retry-after': '42' }));
      const store = useAuthStore();

      await store.login('a', 'b', false);
      expect(store.loginError).toBe('rateLimited');
      expect(store.retryAfterSeconds).toBe(42);
    });

    it('reports other failures as unknown', async () => {
      svc.login.mockRejectedValue(new Error('network'));
      const store = useAuthStore();

      await store.login('a', 'b', false);
      expect(store.loginError).toBe('unknown');
    });
  });

  describe('logout / 401', () => {
    async function loggedIn() {
      svc.login.mockResolvedValue({
        token: 't',
        username: 'admin',
        role: AuthRole.ADMIN,
      });
      const store = useAuthStore();

      await store.login('admin', 'pw', false);

      return store;
    }

    it('revokes server side and clears local state', async () => {
      const store = await loggedIn();

      svc.logout.mockResolvedValue(undefined);
      await store.logout();
      expect(svc.logout).toHaveBeenCalled();
      expect(store.token).toBeNull();
      expect(store.isAuthenticated).toBe(false);
      expect(AuthTokenHelper.get()).toBeNull();
    });

    it('clears local state even if the server call fails', async () => {
      const store = await loggedIn();

      svc.logout.mockRejectedValue(httpError(401));
      await store.logout();
      expect(store.token).toBeNull();
    });

    it('handleUnauthorized drops the session', async () => {
      const store = await loggedIn();

      store.handleUnauthorized();
      expect(store.token).toBeNull();
      expect(store.role).toBeNull();
      expect(store.canWrite).toBe(false);
      expect(AuthTokenHelper.get()).toBeNull();
    });
  });
});
