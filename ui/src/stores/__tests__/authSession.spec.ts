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

import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useAuthStore } from '../authStore';
import { AuthService } from '@/services/AuthService';
import { AuthRole } from '@/utils/types/auth';

const push = vi.fn();

vi.mock('@/router', () => ({
  default: {
    currentRoute: { value: { name: 'TUTTI_HOME', fullPath: '/storages?x=1' } },
    push: (...a: unknown[]) => push(...a),
  },
}));
vi.mock('@/services/AuthService', () => ({
  AuthService: {
    login: vi.fn(),
    logout: vi.fn(),
    me: vi.fn(),
    refresh: vi.fn(),
  },
}));

const svc = AuthService as unknown as Record<
  'login' | 'logout' | 'me' | 'refresh',
  ReturnType<typeof vi.fn>
>;

beforeEach(() => {
  window.sessionStorage.clear();
  window.localStorage.clear();
  setActivePinia(createPinia());
  vi.resetAllMocks();
});

describe('authStore session clock', () => {
  it('derives the server offset and deadlines from login', async () => {
    const server = Date.now() + 10 * 60_000;

    svc.login.mockResolvedValue({
      token: 't',
      username: 'a',
      role: AuthRole.ADMIN,
      expiresAt: new Date(server + 30 * 60_000).toISOString(),
      maxExpiresAt: new Date(server + 24 * 3600_000).toISOString(),
      serverTime: new Date(server).toISOString(),
    });
    svc.me.mockResolvedValue({ authEnabled: true });
    const store = useAuthStore();

    await store.login('a', 'p', false);

    expect(Math.abs(store.serverOffsetMs - 10 * 60_000)).toBeLessThan(1000);
    expect(store.expiresAt).toBe(server + 30 * 60_000);
    expect(store.maxExpiresAt).toBe(server + 24 * 3600_000);
  });

  it('refreshSession updates deadlines and clears them on expire', async () => {
    const now = Date.now();

    svc.refresh.mockResolvedValue({
      expiresAt: new Date(now + 30 * 60_000).toISOString(),
      maxExpiresAt: new Date(now + 3600_000).toISOString(),
      serverTime: new Date(now).toISOString(),
    });
    const store = useAuthStore();

    store.token = 't';
    await store.refreshSession();
    expect(store.expiresAt).toBe(now + 30 * 60_000);

    await store.expire('max');
    expect(store.token).toBeNull();
    expect(store.expiresAt).toBeNull();
    expect(push).toHaveBeenCalledWith({
      name: 'LOGIN',
      query: { redirect: '/storages?x=1', reason: 'max' },
    });
  });

  it('routes to login with reason=idle', async () => {
    const store = useAuthStore();

    await store.expire('idle');
    expect(push.mock.calls[0]?.[0].query).toEqual({
      redirect: '/storages?x=1',
      reason: 'idle',
    });
  });
});
