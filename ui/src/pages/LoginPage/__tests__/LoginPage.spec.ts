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
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createI18n } from 'vue-i18n';
import LoginPage from '../LoginPage.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import { AuthService } from '@/services/AuthService';
import { useAuthStore } from '@/stores/authStore';

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: vi.fn() }),
}));
vi.mock('@/services/AuthService', () => ({
  AuthService: { login: vi.fn(), logout: vi.fn(), me: vi.fn() },
}));

tolerateNaiveStyles();

function mountPage() {
  const pinia = createPinia();

  setActivePinia(pinia);

  return {
    pinia,
    wrapper: mount(LoginPage, {
      global: {
        plugins: [
          pinia,
          createI18n({ legacy: false, locale: 'en', messages: {} }),
        ],
      },
    }),
  };
}

beforeEach(() => vi.resetAllMocks());

describe('LoginPage', () => {
  it('shows the form when the server answers', () => {
    const { wrapper } = mountPage();

    expect(wrapper.find('form').exists()).toBe(true);
    expect(wrapper.find('[data-kit-state="error"]').exists()).toBe(false);
  });

  it('shows an unreachable-server state with a working retry, never a blank page', async () => {
    (AuthService.me as ReturnType<typeof vi.fn>).mockRejectedValue(
      new Error('network'),
    );
    const { wrapper, pinia } = mountPage();

    await useAuthStore(pinia).init();
    await wrapper.vm.$nextTick();

    expect(wrapper.find('[data-kit-state="error"]').exists()).toBe(true);
    await wrapper.find('[data-kit-state="error"] button').trigger('click');
    expect(AuthService.me).toHaveBeenCalledTimes(2);
  });
});
