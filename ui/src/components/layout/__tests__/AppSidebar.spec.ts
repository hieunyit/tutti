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

import { mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { describe, expect, it, vi } from 'vitest';
import { defineComponent, ref } from 'vue';
import { createI18n } from 'vue-i18n';
import {
  createMemoryHistory,
  createRouter,
  type RouteRecordRaw,
} from 'vue-router';
import AppSidebar from '../AppSidebar.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import routeMessages from '@/i18n/messages/routes';
import { routes } from '@/router/routes';
import { useAuthStore } from '@/stores/authStore';
import { AuthRole } from '@/utils/types/auth';
import { RouteName } from '@/utils/types/router';

// the real kit pulls in a stylesheet that node cannot import
vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
}));
vi.mock('@/utils/composables/useNavBadges', () => ({
  useNavBadges: () => ref({ attention: 2, failover: 0, protection: 0 }),
}));

tolerateNaiveStyles();

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

async function mountSidebar(
  role: AuthRole,
  props: Record<string, unknown> = {},
  path = '/dashboard',
) {
  const pinia = createPinia();

  setActivePinia(pinia);

  const auth = useAuthStore();

  auth.authEnabled = true;
  auth.role = role;

  const router = createRouter({
    history: createMemoryHistory(),
    routes: stubbed(routes),
  });

  router.push(path);
  await router.isReady();

  const wrapper = mount(AppSidebar, {
    props,
    global: {
      plugins: [
        router,
        pinia,
        createI18n({
          legacy: false,
          locale: 'en',
          messages: routeMessages as never,
        }),
      ],
    },
  });

  return { wrapper, router };
}

describe('AppSidebar', () => {
  it('shows 11 links for an admin with the Admin group last', async () => {
    const { wrapper } = await mountSidebar(AuthRole.ADMIN);
    const labels = wrapper
      .findAll('a.nav-item')
      .map((a) => a.attributes('aria-label'));

    expect(labels).toEqual([
      'Dashboard',
      'Storages',
      'Proxies',
      'Replication',
      'Protection groups',
      'Routing rules',
      'Users',
      'Access keys',
      'Console users',
      'System settings',
      'Audit log',
    ]);
    expect(wrapper.findAll('h2').map((h) => h.text())).toEqual([
      'Connect',
      'Replicate',
      'Access',
      'Admin',
    ]);
  });

  it('hides the Admin group from viewers', async () => {
    const { wrapper } = await mountSidebar(AuthRole.VIEWER);
    const text = wrapper.text();

    expect(text).not.toContain('Console users');
    expect(text).not.toContain('Audit log');
    expect(text).not.toContain('System settings');
    expect(wrapper.findAll('h2').map((h) => h.text())).not.toContain('Admin');
    expect(wrapper.findAll('a.nav-item')).toHaveLength(8);
  });

  it('marks the item of the current section with aria-current, sub-pages included', async () => {
    const { wrapper } = await mountSidebar(
      AuthRole.ADMIN,
      {},
      '/replication/diff/abc',
    );
    const current = wrapper.findAll('a[aria-current="page"]');

    expect(current).toHaveLength(1);
    expect(current[0]?.attributes('aria-label')).toBe('Replication');
  });

  it('shows the attention badge with a screen reader text', async () => {
    const { wrapper } = await mountSidebar(AuthRole.ADMIN);
    const badge = wrapper.get('.nav-item__badge');

    expect(badge.text()).toContain('2');
    expect(badge.find('.tt-sr-only').exists()).toBe(true);
  });

  it('collapses to icon-only links that keep their accessible names', async () => {
    const { wrapper } = await mountSidebar(AuthRole.ADMIN, { collapsed: true });

    expect(wrapper.find('.nav-item__label').exists()).toBe(false);
    expect(wrapper.get('a[href="/storages"]').attributes('aria-label')).toBe(
      'Storages',
    );
    expect(wrapper.find('h2').exists()).toBe(false);
  });

  it('has a keyboard operable collapse toggle that reports its state', async () => {
    const { wrapper } = await mountSidebar(AuthRole.ADMIN);
    const toggle = wrapper.get('button.sidebar-footer__toggle');

    expect(toggle.attributes('aria-expanded')).toBe('true');
    expect(toggle.attributes('aria-controls')).toBe(
      wrapper.get('nav').attributes('id'),
    );
    await toggle.trigger('click');
    expect(wrapper.emitted('toggle')).toHaveLength(1);
  });

  it('has no collapse toggle in the drawer and emits navigate on a click', async () => {
    const { wrapper } = await mountSidebar(AuthRole.ADMIN, { isDrawer: true });

    expect(wrapper.find('button.sidebar-footer__toggle').exists()).toBe(false);
    await wrapper.get('a[href="/storages"]').trigger('click');
    expect(wrapper.emitted('navigate')).toBeTruthy();
    expect(RouteName.TUTTI_STORAGES).toBe('TUTTI_STORAGES');
  });
});
