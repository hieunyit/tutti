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
import TuttiUsersPage from '../TuttiUsersPage.vue';
import { TuttiService } from '@/services/TuttiService';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('@/services/TuttiService', () => ({
  TuttiService: { getUsers: vi.fn() },
}));

async function mountPage() {
  setActivePinia(createPinia());

  const wrapper = mount(TuttiUsersPage, {
    global: {
      plugins: [kitI18n()],
      stubs: { RouterLink: RouterLinkStub },
    },
  });

  await flushPromises();

  return wrapper;
}

describe('TuttiUsersPage', () => {
  beforeEach(() => {
    vi.mocked(TuttiService.getUsers).mockReset();
  });

  it('shows a skeleton while the first load is pending', async () => {
    vi.mocked(TuttiService.getUsers).mockReturnValue(new Promise(() => {}));

    const wrapper = await mountPage();

    expect(wrapper.find('[data-kit-state="loading"]').exists()).toBe(true);
  });

  it('shows an error block with a working retry, not a blank page', async () => {
    vi.mocked(TuttiService.getUsers).mockRejectedValue(new Error('down'));

    const wrapper = await mountPage();

    expect(wrapper.find('[data-kit-state="error"]').exists()).toBe(true);

    vi.mocked(TuttiService.getUsers).mockResolvedValue({ users: [] } as never);
    await wrapper.get('[data-kit-state="error"] button').trigger('click');
    await flushPromises();
    expect(TuttiService.getUsers).toHaveBeenCalledTimes(2);
    expect(wrapper.find('[data-kit-state="error"]').exists()).toBe(false);
  });

  it('distinguishes no users from a filter with no results', async () => {
    vi.mocked(TuttiService.getUsers).mockResolvedValue({ users: [] } as never);

    const empty = await mountPage();

    expect(empty.text()).toContain('No S3 users yet');

    vi.mocked(TuttiService.getUsers).mockResolvedValue({
      users: [{ name: 'alice', complete: true, storages: [] }],
    } as never);

    const wrapper = await mountPage();

    expect(wrapper.text()).toContain('alice');
    await wrapper.get('input').setValue('zzz');
    expect(wrapper.text()).toContain('No users match this filter');
    expect(wrapper.text()).not.toContain('No S3 users yet');
  });
});
