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
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import TuttiConsoleUsersPage from '../TuttiConsoleUsersPage.vue';
import { TuttiService } from '@/services/TuttiService';
import { useAuthStore } from '@/stores/authStore';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('@/utils/helpers/DateTimeHelper', () => ({
  formatDateTime: (value?: string) => value ?? '-',
}));

vi.mock('@/services/TuttiService', () => ({
  TuttiService: {
    getConsoleUsers: vi.fn(),
    createConsoleUser: vi.fn(),
  },
}));

const users = [
  { username: 'me', role: 'admin', source: 'managed' },
  { username: 'other', role: 'admin', source: 'managed' },
  { username: 'viewer1', role: 'viewer', source: 'managed' },
];

async function mountPage(role: 'admin' | 'viewer' = 'admin') {
  setActivePinia(createPinia());

  const auth = useAuthStore();

  auth.$patch({ authEnabled: true, username: 'me', role } as never);

  const wrapper = mount(TuttiConsoleUsersPage, {
    global: { plugins: [kitI18n()] },
    attachTo: document.body,
  });

  await flushPromises();

  return wrapper;
}

const exists = (wrapper: Awaited<ReturnType<typeof mountPage>>, id: string) =>
  wrapper.find(`[data-testid="${id}"]`).exists();

describe('TuttiConsoleUsersPage', () => {
  beforeEach(() => {
    vi.mocked(TuttiService.getConsoleUsers)
      .mockReset()
      .mockResolvedValue({ users } as never);
  });

  it('offers one actions menu per other managed user and none for you', async () => {
    const wrapper = await mountPage();

    expect(exists(wrapper, 'console-menu-me')).toBe(false);
    expect(exists(wrapper, 'console-menu-other')).toBe(true);
    expect(exists(wrapper, 'console-menu-viewer1')).toBe(true);
    expect(wrapper.text()).toContain('You');
  });

  it('shows the create button once in the header when users exist', async () => {
    const wrapper = await mountPage();

    expect(
      wrapper
        .findAll('button')
        .filter((b) => b.text() === 'Create console user'),
    ).toHaveLength(1);
  });

  it('shows the create action only in the empty state when there are no users', async () => {
    vi.mocked(TuttiService.getConsoleUsers).mockResolvedValue({
      users: [],
    } as never);

    const wrapper = await mountPage();

    expect(wrapper.text()).toContain('No console users');
    expect(
      wrapper
        .findAll('button')
        .filter((b) => b.text() === 'Create console user'),
    ).toHaveLength(1);
    expect(wrapper.find('[role="search"]').exists()).toBe(false);
  });

  it('filters by username and offers to clear the search', async () => {
    const wrapper = await mountPage();

    await wrapper.get('input[type="search"]').setValue('zzz');
    expect(wrapper.text()).toContain('No console users match');
    expect(exists(wrapper, 'console-menu-other')).toBe(false);
  });

  it('does not load or list console users for a viewer', async () => {
    const wrapper = await mountPage('viewer');

    expect(TuttiService.getConsoleUsers).not.toHaveBeenCalled();
    expect(exists(wrapper, 'console-menu-other')).toBe(false);
    expect(wrapper.text()).toContain('Administrators only');
  });

  it('shows an error block with retry when the list cannot load', async () => {
    vi.mocked(TuttiService.getConsoleUsers).mockRejectedValue(new Error('x'));

    const wrapper = await mountPage();

    expect(wrapper.find('[data-kit-state="error"]').exists()).toBe(true);
  });
});
