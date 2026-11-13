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

import { flushPromises, mount } from '@vue/test-utils';
import { describe, expect, it, vi } from 'vitest';
import { createMemoryHistory, createRouter } from 'vue-router';
import { createI18n } from 'vue-i18n';
import DashboardPage from '../DashboardPage.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('../DashboardOverviewTab.vue', () => ({
  default: { template: '<div class="overview-tab" />' },
}));
vi.mock('../DashboardMetricsTab.vue', () => ({
  default: { template: '<div class="metrics-tab" />' },
}));
vi.mock('../useDashboardData', async () => {
  const { ref } = await import('vue');

  return {
    useDashboardData: () => ({
      state: ref('ready'),
      updatedAt: ref(null),
      refresh: vi.fn(),
    }),
  };
});

async function mountAt(url: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/dashboard', component: { template: '<div />' } }],
  });

  router.push(url);
  await router.isReady();

  const wrapper = mount(DashboardPage, {
    global: {
      plugins: [
        router,
        createI18n({ legacy: false, locale: 'en', messages: {} }),
      ],
    },
  });

  return { wrapper, router };
}

describe('DashboardPage', () => {
  it('renders the overview and metrics tabs, overview selected by default', async () => {
    const { wrapper } = await mountAt('/dashboard');
    const tabs = wrapper.findAll('[role="tab"]');

    expect(tabs.map((tab) => tab.text())).toEqual(['Overview', 'Metrics']);
    expect(tabs[0]?.attributes('aria-selected')).toBe('true');
    expect(wrapper.find('.overview-tab').exists()).toBe(true);
    expect(wrapper.find('.metrics-tab').exists()).toBe(false);
  });

  it('selects the metrics tab from ?tab=metrics', async () => {
    const { wrapper } = await mountAt('/dashboard?tab=metrics');
    const tabs = wrapper.findAll('[role="tab"]');

    expect(tabs[1]?.attributes('aria-selected')).toBe('true');
    expect(wrapper.find('.metrics-tab').exists()).toBe(true);
  });

  it('writes the tab to the URL when clicking Metrics', async () => {
    const { wrapper, router } = await mountAt('/dashboard');

    await wrapper.findAll('[role="tab"]')[1]?.trigger('click');
    await flushPromises();
    expect(router.currentRoute.value.query.tab).toBe('metrics');
  });
});
