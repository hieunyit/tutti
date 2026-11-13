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

import { describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import { createMemoryHistory, createRouter } from 'vue-router';
import { createPinia } from 'pinia';
import ReplicationHub from '../ReplicationHub.vue';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('@/components/auth/WriteGate.vue', () => ({
  default: { template: '<div><slot /></div>' },
}));
vi.mock('../ReplicationsTab.vue', () => ({
  default: { template: '<div class="tab-replications" />' },
}));
vi.mock('../RoutingTab.vue', () => ({
  default: { template: '<div class="tab-routing" />' },
}));
vi.mock('../DiffTab.vue', () => ({
  default: { template: '<div class="tab-diff" />' },
}));

async function mountHub(url: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/replication', component: { template: '<div />' } },
      { path: '/:p(.*)*', name: 'other', component: { template: '<div />' } },
    ],
  });

  router.push(url);
  await router.isReady();

  const wrapper = mount(ReplicationHub, {
    global: {
      plugins: [router, createPinia(), kitI18n()],
      stubs: { RouterLink: { template: '<a><slot /></a>' } },
    },
  });

  await flushPromises();

  return wrapper;
}

describe('ReplicationHub', () => {
  it('shows the replications tab by default', async () => {
    const wrapper = await mountHub('/replication');

    expect(wrapper.find('.tab-replications').exists()).toBe(true);
    expect(wrapper.find('.tab-routing').exists()).toBe(false);
  });

  it('selects the routing tab from ?tab=routing', async () => {
    const wrapper = await mountHub('/replication?tab=routing');

    expect(wrapper.find('.tab-routing').exists()).toBe(true);
    expect(wrapper.find('.tab-replications').exists()).toBe(false);
  });

  it('selects the diff tab from ?tab=diff', async () => {
    const wrapper = await mountHub('/replication?tab=diff');

    expect(wrapper.find('.tab-diff').exists()).toBe(true);
  });

  it('falls back to replications for an unknown tab', async () => {
    const wrapper = await mountHub('/replication?tab=nope');

    expect(wrapper.find('.tab-replications').exists()).toBe(true);
  });
});
