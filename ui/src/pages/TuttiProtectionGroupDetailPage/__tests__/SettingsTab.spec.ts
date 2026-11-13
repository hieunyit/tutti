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
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, describe, expect, it, vi } from 'vitest';
import SettingsTab from '../SettingsTab.vue';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';
import { useAuthStore } from '@/stores/authStore';
import type { ProtectionGroup } from '@/utils/types/protection';

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
}));

tolerateNaiveStyles();

let wrapper: VueWrapper | undefined;
const group: ProtectionGroup = {
  id: 'pg-prod',
  state: 'ready',
  members: ['a', 'b'],
  primary: 'a',
};

function mountTab(remove = vi.fn().mockResolvedValue(undefined)) {
  const pinia = createPinia();

  setActivePinia(pinia);
  useAuthStore().authEnabled = false;

  wrapper = mount(SettingsTab, {
    attachTo: document.body,
    props: {
      group,
      detail: null,
      actions: { remove },
    },
    global: {
      plugins: [pinia, kitI18n()],
    },
  });

  return { remove };
}

function byAttr(attr: string) {
  return document.body.querySelector<HTMLElement>(`[${attr}]`);
}

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = '';
});

describe('SettingsTab delete', () => {
  it('requires typing the group name before removing', async () => {
    const { remove } = mountTab();

    await wrapper!.find('[data-delete-group]').trigger('click');
    await flushPromises();

    const confirm = byAttr('data-kit-confirm') as HTMLButtonElement;

    expect(confirm.disabled).toBe(true);

    const input = document.body.querySelector<HTMLInputElement>(
      '[data-kit-confirm-input] input, input[data-kit-confirm-input]',
    )!;

    input.value = 'pg-prod';
    input.dispatchEvent(new Event('input'));
    await flushPromises();
    expect(confirm.disabled).toBe(false);
    confirm.click();
    await flushPromises();
    expect(remove).toHaveBeenCalledTimes(1);
    expect(wrapper!.emitted('removed')).toHaveLength(1);
  });
});
