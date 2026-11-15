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
import type { RouteLocationNormalized } from 'vue-router';
import authGuard from '../auth-guard';
import { AuthService } from '@/services/AuthService';
import { AuthRole } from '@/utils/types/auth';
import { RouteName } from '@/utils/types/router';

vi.mock('@/services/AuthService', () => ({
  AuthService: { login: vi.fn(), logout: vi.fn(), me: vi.fn() },
}));

const me = AuthService.me as unknown as ReturnType<typeof vi.fn>;

function route(
  name: RouteName,
  fullPath: string,
  requiresWrite = false,
  extra: {
    meta?: Record<string, unknown>;
    query?: Record<string, string>;
  } = {},
): RouteLocationNormalized {
  return {
    name,
    fullPath,
    params: {},
    query: extra.query ?? {},
    hash: '',
    meta: { requiresWrite, ...extra.meta },
  } as never;
}

async function signInAs(role: AuthRole) {
  window.sessionStorage.setItem('tutti.auth.token', 't');
  setActivePinia(createPinia());
  me.mockResolvedValue({ authEnabled: true, username: 'u', role });
}

const run = (to: RouteLocationNormalized) =>
  (authGuard as unknown as (to: RouteLocationNormalized) => Promise<unknown>)(
    to,
  );

beforeEach(() => {
  window.sessionStorage.clear();
  window.localStorage.clear();
  setActivePinia(createPinia());
  vi.resetAllMocks();
});

describe('auth guard', () => {
  it('lets everything through when auth is disabled and hides /login', async () => {
    me.mockResolvedValue({ authEnabled: false });
    expect(await run(route(RouteName.TUTTI_REPLICATION, '/replication'))).toBe(
      true,
    );
    expect(await run(route(RouteName.LOGIN, '/login'))).toEqual({
      name: RouteName.TUTTI_HOME,
    });
  });

  it('redirects to login preserving the target when unauthenticated', async () => {
    me.mockRejectedValue({ isAxiosError: true, response: { status: 401 } });
    expect(
      await run(route(RouteName.TUTTI_REPLICATION, '/replication?a=1')),
    ).toEqual({
      name: RouteName.LOGIN,
      query: { redirect: '/replication?a=1' },
    });
    expect(await run(route(RouteName.LOGIN, '/login'))).toBe(true);
  });

  it('blocks write-only routes for viewers', async () => {
    window.sessionStorage.setItem('tutti.auth.token', 't');
    setActivePinia(createPinia());
    me.mockResolvedValue({
      authEnabled: true,
      username: 'v',
      role: AuthRole.VIEWER,
    });
    expect(
      await run(
        route(RouteName.TUTTI_ADD_REPLICATION, '/replication/add', true),
      ),
    ).toEqual({ name: RouteName.TUTTI_HOME });
    expect(await run(route(RouteName.TUTTI_REPLICATION, '/replication'))).toBe(
      true,
    );
  });

  it('sends viewers away from admin-only routes and lets admins in', async () => {
    const adminRoute = route(RouteName.TUTTI_AUDIT_LOG, '/audit-log', false, {
      meta: { requiresAdmin: true },
    });

    await signInAs(AuthRole.VIEWER);
    expect(await run(adminRoute)).toEqual({ name: RouteName.TUTTI_HOME });

    await signInAs(AuthRole.ADMIN);
    expect(await run(adminRoute)).toBe(true);
  });

  it('strips an admin-only tab (Dashboard Traffic) from the URL for viewers', async () => {
    const dashboard = (tab: string) =>
      route(RouteName.TUTTI_DASHBOARD, `/dashboard?tab=${tab}&x=1`, false, {
        meta: { adminTabs: ['traffic', 'metrics'] },
        query: { tab, x: '1' },
      });

    await signInAs(AuthRole.VIEWER);
    expect(await run(dashboard('traffic'))).toEqual({
      name: RouteName.TUTTI_DASHBOARD,
      params: {},
      query: { x: '1' },
      hash: '',
    });
    expect(await run(dashboard('metrics'))).toMatchObject({
      query: { x: '1' },
    });
    expect(await run(dashboard('overview'))).toBe(true);

    await signInAs(AuthRole.ADMIN);
    expect(await run(dashboard('traffic'))).toBe(true);
  });
});
