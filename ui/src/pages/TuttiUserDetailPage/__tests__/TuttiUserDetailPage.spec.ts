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

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import TuttiUserDetailPage from '../TuttiUserDetailPage.vue';
import { TuttiService } from '@/services/TuttiService';
import { useTuttiAccessPolicyStore } from '@/stores/tuttiAccessPolicyStore';
import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('vue-router', async (orig) => ({
  ...(await orig<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useRoute: () => ({ query: {} }),
}));
vi.mock('@/components/tutti/accounts/AccessPolicyDrawer.vue', () => ({
  default: { template: '<i />' },
}));
vi.mock('@/components/tutti/accounts/ProxyKeyRotateModal.vue', () => ({
  default: { template: '<i />' },
}));
vi.mock('@/components/tutti/accounts/ProxySecretModal.vue', () => ({
  default: { template: '<i />' },
}));
vi.mock('@/components/tutti/accounts/ProxyAccountKeysTable.vue', () => ({
  default: { template: '<i />' },
}));
vi.mock('@/utils/helpers/DateTimeHelper', () => ({
  formatDateTime: (v?: string) => v ?? '-',
  formatTime: (v?: string) => v ?? '-',
}));
vi.mock('@/services/TuttiService', () => ({
  TuttiService: { getUsers: vi.fn(), deleteUser: vi.fn() },
}));

async function mountPage(name: string) {
  setActivePinia(createPinia());
  useTuttiAccessPolicyStore().init = vi.fn().mockResolvedValue(undefined);
  useTuttiProtectionStore().refresh = vi.fn().mockResolvedValue(undefined);

  const wrapper = mount(TuttiUserDetailPage, {
    props: { name },
    global: { plugins: [kitI18n()], stubs: { RouterLink: RouterLinkStub } },
    attachTo: document.body,
  });

  await flushPromises();

  return wrapper;
}

describe('TuttiUserDetailPage', () => {
  beforeEach(() => {
    vi.mocked(TuttiService.getUsers).mockReset();
  });

  it('shows an error block with retry when loading fails', async () => {
    vi.mocked(TuttiService.getUsers).mockRejectedValue(new Error('down'));

    const wrapper = await mountPage('alice');

    expect(wrapper.find('[data-kit-state="error"]').exists()).toBe(true);
  });

  it('shows a not-found state for an unknown user', async () => {
    vi.mocked(TuttiService.getUsers).mockResolvedValue({ users: [] } as never);

    const wrapper = await mountPage('ghost');

    expect(wrapper.text()).toContain('This user does not exist.');
  });

  it('only deletes after the user name is typed', async () => {
    vi.mocked(TuttiService.getUsers).mockResolvedValue({
      users: [{ name: 'alice', storages: [] }],
    } as never);
    vi.mocked(TuttiService.deleteUser).mockResolvedValue(undefined as never);

    const wrapper = await mountPage('alice');
    const open = wrapper
      .findAll('button')
      .find((b) => b.text() === 'Delete user');

    await open?.trigger('click');
    await flushPromises();

    const confirm = [...document.body.querySelectorAll('button')].find(
      (b) => b.textContent?.trim() === 'Delete user' && b.closest('.n-card'),
    ) as HTMLButtonElement;

    expect(confirm.disabled).toBe(true);
    wrapper.unmount();
  });
});
