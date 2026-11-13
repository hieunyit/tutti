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

import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import { AxiosError } from 'axios';
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createMemoryHistory, createRouter } from 'vue-router';
import TuttiProtectionGroupDetailPage from '../TuttiProtectionGroupDetailPage.vue';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';
import { ProtectionGroupService } from '@/services/ProtectionGroupService';
import { RouteName } from '@/utils/types/router';
import { useAuthStore } from '@/stores/authStore';

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
}));
vi.mock('@/utils/helpers/DateTimeHelper', () => ({
  formatDateTime: (value: unknown) => String(value ?? '-'),
}));
vi.mock('@/services/ProtectionGroupService', () => ({
  ProtectionGroupService: {
    get: vi.fn(),
    listWriters: vi.fn().mockResolvedValue({}),
  },
}));

tolerateNaiveStyles();

let wrapper: VueWrapper | undefined;

async function mountPage() {
  const pinia = createPinia();

  setActivePinia(pinia);
  useAuthStore().authEnabled = false;

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/protection-groups',
        name: RouteName.TUTTI_PROTECTION_GROUPS,
        component: { template: '<div />' },
      },
      { path: '/:p(.*)*', component: { template: '<div />' } },
    ],
  });

  wrapper = mount(TuttiProtectionGroupDetailPage, {
    attachTo: document.body,
    props: { name: 'pg-1' },
    global: { plugins: [pinia, router, kitI18n()] },
  });
  await flushPromises();

  return wrapper;
}

function httpError(status: number) {
  return new AxiosError('x', 'E', undefined, undefined, {
    status,
    data: {},
  } as never);
}

beforeEach(() => vi.clearAllMocks());
afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = '';
});

describe('TuttiProtectionGroupDetailPage states', () => {
  it('shows a skeleton while loading', async () => {
    vi.mocked(ProtectionGroupService.get).mockReturnValue(
      new Promise(() => undefined),
    );

    const page = await mountPage();

    expect(page.find('[data-kit-state="loading"]').exists()).toBe(true);
  });

  it('shows the group with its four tabs when ready', async () => {
    vi.mocked(ProtectionGroupService.get).mockResolvedValue({
      group: { id: 'pg-1', state: 'ready', members: ['a', 'b'], primary: 'a' },
    } as never);

    const page = await mountPage();

    expect(page.get('h1').text()).toBe('pg-1');
    expect(page.findAll('[role="tab"]')).toHaveLength(4);
  });

  it('shows the error block with a working retry', async () => {
    vi.mocked(ProtectionGroupService.get).mockRejectedValue(httpError(500));

    const page = await mountPage();

    expect(page.find('[data-kit-state="error"]').exists()).toBe(true);
    vi.mocked(ProtectionGroupService.get).mockClear();
    await page
      .get('[data-kit-retry], [data-kit-state="error"] button')
      .trigger('click');
    expect(ProtectionGroupService.get).toHaveBeenCalledWith('pg-1');
  });

  it('shows the empty state when the group does not exist', async () => {
    vi.mocked(ProtectionGroupService.get).mockRejectedValue(httpError(404));

    const page = await mountPage();

    expect(page.find('[data-kit-state="empty"]').exists()).toBe(true);
  });
});
