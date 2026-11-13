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
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import TuttiProxyAccountsPage from '../TuttiProxyAccountsPage.vue';
import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('@/components/tutti/accounts/ProxyAccountsTile.vue', () => ({
  default: { template: '<div class="tile-stub" />' },
}));

function mountPage(
  patch: (store: ReturnType<typeof useTuttiAccessPolicyStore>) => void,
) {
  setActivePinia(createPinia());

  const store = useTuttiAccessPolicyStore();

  store.init = vi.fn().mockResolvedValue(undefined);
  patch(store);

  return mount(TuttiProxyAccountsPage, {
    global: {
      plugins: [kitI18n()],
      stubs: { RouterLink: RouterLinkStub },
    },
  });
}

describe('TuttiProxyAccountsPage', () => {
  it('shows an error block whose retry reloads the accounts', async () => {
    const wrapper = mountPage((store) => {
      store.hasError = true;
    });

    await flushPromises();
    expect(wrapper.find('[data-kit-state="error"]').exists()).toBe(true);
    await wrapper.get('[data-kit-state="error"] button').trigger('click');
    expect(
      vi.mocked(useTuttiAccessPolicyStore().init).mock.calls.length,
    ).toBeGreaterThan(1);
  });

  it('explains how to enable dynamic accounts when they are off', async () => {
    const wrapper = mountPage((store) => {
      store.dynamicAvailable = false;
    });

    await flushPromises();
    expect(wrapper.find('[data-kit-state="empty"]').exists()).toBe(true);
    expect(wrapper.find('.tile-stub').exists()).toBe(false);
  });

  it('renders the accounts table when ready', async () => {
    const wrapper = mountPage(() => {});

    await flushPromises();
    expect(wrapper.find('.tile-stub').exists()).toBe(true);
  });
});
