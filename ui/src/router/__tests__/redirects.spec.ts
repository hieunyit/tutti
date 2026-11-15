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
import { defineComponent } from 'vue';
import {
  createMemoryHistory,
  createRouter,
  type RouteRecordRaw,
} from 'vue-router';
import authGuard from '../guards/auth-guard';
import { routes } from '../routes';
import { AuthService } from '@/services/AuthService';
import { AuthRole } from '@/utils/types/auth';
import { RouteName } from '@/utils/types/router';

vi.mock('@/services/AuthService', () => ({
  AuthService: { login: vi.fn(), logout: vi.fn(), me: vi.fn() },
}));

const Stub = defineComponent({ render: () => null });

// Swap every (lazy) page for a stub so the test does not load real pages.
function stubbed(records: RouteRecordRaw[]): RouteRecordRaw[] {
  return records.map((record) => {
    const copy = { ...record } as RouteRecordRaw & {
      component?: unknown;
      children?: RouteRecordRaw[];
    };

    if (copy.component) copy.component = Stub;

    if (copy.children) copy.children = stubbed(copy.children);

    return copy as RouteRecordRaw;
  });
}

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: stubbed(routes),
  });
}

async function land(
  target: Parameters<ReturnType<typeof makeRouter>['push']>[0],
) {
  const router = makeRouter();

  await router.push(target);

  return router.currentRoute.value;
}

describe('old URLs redirect to the new routes (spec section 2)', () => {
  it.each([
    ['/home', '/dashboard', {}],
    ['/monitoring', '/dashboard', { tab: 'traffic' }],
    ['/health', '/proxies', {}],
    ['/routing-policies', '/routing', {}],
    ['/routing-policies/add', '/routing/add', {}],
    ['/replication/routing/add', '/routing/add', {}],
    ['/diff-reports', '/replication', { tab: 'diff' }],
    ['/diff-reports/add', '/replication/diff/add', {}],
    ['/failover', '/protection-groups', {}],
    ['/proxy-accounts', '/access-keys', {}],
    ['/', '/dashboard', {}],
  ])('%s -> %s keeps the query', async (from, to, extra) => {
    const route = await land(`${from}?keep=1&other=x`);

    expect(route.path).toBe(to);
    expect(route.query).toEqual({ keep: '1', other: 'x', ...extra });
  });

  it('/diff-reports/detail?id=X lands on /replication/diff/X with the query', async () => {
    const route = await land('/diff-reports/detail?id=X&from=s1:b&to=s2:b');

    expect(route.path).toBe('/replication/diff/X');
    expect(route.params.id).toBe('X');
    expect(route.query).toEqual({ id: 'X', from: 's1:b', to: 's2:b' });
  });

  it('/diff-reports/detail without an id derives one from from/to', async () => {
    const route = await land('/diff-reports/detail?from=s1:b&to=s2:b');

    expect(route.name).toBe(RouteName.TUTTI_REPLICATION_DIFF_DETAIL);
    expect(route.params.id).toBe('s1:b~s2:b');
    expect(route.query).toEqual({ from: 's1:b', to: 's2:b' });
  });

  it('moved tabs keep their deep links: ?tab=routing and ?tab=config', async () => {
    const routing = await land('/replication?tab=routing&x=1');

    expect(routing.name).toBe(RouteName.TUTTI_ROUTING_RULES);
    expect(routing.path).toBe('/routing');
    expect(routing.query).toEqual({ x: '1' });

    const config = await land('/proxies?tab=config&x=1');

    expect(config.name).toBe(RouteName.TUTTI_SYSTEM_SETTINGS);
    expect(config.query).toEqual({ tab: 'proxies', x: '1' });
    expect((await land('/proxies')).name).toBe(RouteName.TUTTI_PROXIES);
    expect((await land('/replication?tab=diff')).name).toBe(
      RouteName.TUTTI_REPLICATION,
    );
  });

  it('the replication detail deep link is unchanged', async () => {
    const route = await land('/replication/detail?from=s1:b&to=s2:b');

    expect(route.name).toBe(RouteName.TUTTI_REPLICATION_DETAIL);
    expect(route.query).toEqual({ from: 's1:b', to: 's2:b' });
  });

  it('unknown paths land on the dashboard', async () => {
    const route = await land('/no/such/page');

    expect(route.name).toBe(RouteName.TUTTI_DASHBOARD);
  });
});

describe('old route names still resolve (names are never removed)', () => {
  it.each([
    [RouteName.TUTTI_HOME, '/dashboard', {}],
    [RouteName.TUTTI_MONITORING, '/dashboard', { tab: 'traffic' }],
    [RouteName.TUTTI_PROXY_ACCOUNTS, '/access-keys', {}],
    [RouteName.TUTTI_ROUTING_POLICIES, '/routing', {}],
    [RouteName.TUTTI_DIFF_REPORTS, '/replication', { tab: 'diff' }],
    [RouteName.TUTTI_ADD_ROUTING_POLICY, '/routing/add', {}],
    [RouteName.TUTTI_ADD_DIFF_REPORT, '/replication/diff/add', {}],
  ])('%s -> %s', async (name, path, query) => {
    const route = await land({ name });

    expect(route.path).toBe(path);
    expect(route.query).toEqual(query);
  });

  it('TUTTI_DIFF_REPORT_DETAIL with from/to lands on the new detail route', async () => {
    const route = await land({
      name: RouteName.TUTTI_DIFF_REPORT_DETAIL,
      query: { from: 's1:b', to: 's2:b' },
    });

    expect(route.name).toBe(RouteName.TUTTI_REPLICATION_DIFF_DETAIL);
    expect(route.query).toEqual({ from: 's1:b', to: 's2:b' });
  });
});

describe('new routes', () => {
  it('serves the dashboard and replication sub-pages', async () => {
    expect((await land('/dashboard')).name).toBe(RouteName.TUTTI_DASHBOARD);
    expect((await land('/replication?tab=diff')).name).toBe(
      RouteName.TUTTI_REPLICATION,
    );
    expect((await land('/replication/diff/abc')).name).toBe(
      RouteName.TUTTI_REPLICATION_DIFF_DETAIL,
    );
  });

  it('keeps admin-only routes flagged', () => {
    const router = makeRouter();

    for (const name of [
      RouteName.TUTTI_CONSOLE_USERS,
      RouteName.TUTTI_SYSTEM_SETTINGS,
      RouteName.TUTTI_AUDIT_LOG,
    ]) {
      expect(router.resolve({ name }).meta.requiresAdmin).toBe(true);
    }
  });
});

describe('legacy credentials URL', () => {
  it('/storages/s1/credentials/alice opens the user page', async () => {
    const route = await land('/storages/s1/credentials/alice');

    expect(route.name).toBe(RouteName.TUTTI_USER_DETAIL);
    expect(route.params.name).toBe('alice');
  });

  it('/storages/s1/credentials opens the storage users tab', async () => {
    const route = await land('/storages/s1/credentials');

    expect(route.name).toBe(RouteName.TUTTI_STORAGE_DETAILS);
    expect(route.params.storageName).toBe('s1');
    expect(route.query).toEqual({ tab: 'users' });
  });
});

describe('viewer cannot reach admin pages by URL', () => {
  const me = AuthService.me as unknown as ReturnType<typeof vi.fn>;

  async function landAs(role: AuthRole, path: string) {
    window.sessionStorage.setItem('tutti.auth.token', 't');
    setActivePinia(createPinia());
    me.mockResolvedValue({ authEnabled: true, username: 'u', role });

    const router = makeRouter();

    router.beforeEach(authGuard);
    await router.push(path);

    return router.currentRoute.value;
  }

  beforeEach(() => {
    window.sessionStorage.clear();
    vi.resetAllMocks();
  });

  it.each(['/console-users', '/system-settings', '/audit-log'])(
    'redirects a viewer away from %s',
    async (path) => {
      const route = await landAs(AuthRole.VIEWER, path);

      expect(route.path).toBe('/dashboard');
    },
  );

  it.each(['/console-users', '/system-settings', '/audit-log'])(
    'lets an admin open %s',
    async (path) => {
      const route = await landAs(AuthRole.ADMIN, path);

      expect(route.path).toBe(path);
    },
  );

  it('strips the Traffic tab (and its old names) for viewers, keeps it for admins', async () => {
    for (const path of [
      '/dashboard?tab=traffic',
      '/dashboard?tab=metrics',
      '/monitoring',
    ]) {
      const viewer = await landAs(AuthRole.VIEWER, path);

      expect(viewer.path).toBe('/dashboard');
      expect(viewer.query.tab).toBeUndefined();
    }

    const admin = await landAs(AuthRole.ADMIN, '/monitoring');

    expect(admin.query.tab).toBe('traffic');
  });

  it('a viewer following the old Proxies config link ends on the dashboard', async () => {
    const route = await landAs(AuthRole.VIEWER, '/proxies?tab=config');

    expect(route.path).toBe('/dashboard');
  });

  it.each(['/dashboard', '/storages', '/replication', '/routing'])(
    'lets a viewer read %s',
    async (path) => {
      expect((await landAs(AuthRole.VIEWER, path)).path).toBe(path);
    },
  );
});
