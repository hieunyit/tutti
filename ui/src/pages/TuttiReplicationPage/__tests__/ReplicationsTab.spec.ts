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

import '@/i18n';
import { describe, expect, it, vi } from 'vitest';
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import ReplicationsTab from '../ReplicationsTab.vue';
import { useTuttiReplicationsStore } from '@/stores/tuttiReplicationsStore';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('@/i18n/useStoreI18n', () => ({
  useStoreI18n: () => ({ t: (key: string) => key }),
}));
vi.mock('@/services/TuttiCoreService', () => ({ TuttiCoreService: {} }));
vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
  I18N_DEFAULT_LOCALE: 'en',
  useNotification: () => ({
    createNotification: vi.fn(),
    removeNotification: vi.fn(),
  }),
  CShortList: { template: '<ul />' },
}));
vi.mock('@/components/auth/WriteGate.vue', () => ({
  default: { template: '<div><slot /></div>' },
}));
vi.mock('@/utils/helpers/DateTimeHelper', () => ({
  formatDateTime: (v?: string) => v ?? '-',
  formatTime: (v?: string) => v ?? '-',
}));

const row = (to: string, isPaused: boolean) => ({
  id: {
    user: '',
    fromStorage: 'a',
    toStorage: to,
    fromBucket: '',
    toBucket: '',
  },
  idStr: `:a:${to}`,
  isPaused,
  isArchived: false,
  createdAt: '2026-01-01',
});

function mountTab(rows: ReturnType<typeof row>[]) {
  setActivePinia(createPinia());

  const store = useTuttiReplicationsStore();

  store.initReplicationsPage = vi.fn().mockResolvedValue(undefined);
  store.replications = rows as never;
  store.pauseReplications = vi.fn().mockResolvedValue([]);
  store.resumeReplications = vi.fn().mockResolvedValue([]);

  const wrapper = mount(ReplicationsTab, {
    global: { plugins: [kitI18n()], stubs: { RouterLink: RouterLinkStub } },
  });

  return { wrapper, store };
}

const buttons = (w: ReturnType<typeof mountTab>['wrapper'], text: RegExp) =>
  w.findAll('button').filter((b) => text.test(b.text()));

describe('ReplicationsTab bulk bar', () => {
  it('offers only Resume when every selected row is paused, with its count', async () => {
    const { wrapper } = mountTab([row('b', true), row('c', true)]);

    await flushPromises();
    await wrapper.findAll('.n-checkbox')[0]!.trigger('click');
    await flushPromises();

    expect(buttons(wrapper, /^Pause/)).toHaveLength(0);
    expect(buttons(wrapper, /^Resume 2/)).toHaveLength(1);
  });

  it('counts Pause and Resume by the selected rows they apply to and clears the selection', async () => {
    const { wrapper, store } = mountTab([
      row('b', false),
      row('c', true),
      row('d', false),
    ]);

    await flushPromises();
    await wrapper.findAll('.n-checkbox')[0]!.trigger('click');
    await flushPromises();

    expect(buttons(wrapper, /^Pause 2/)).toHaveLength(1);
    expect(buttons(wrapper, /^Resume 1/)).toHaveLength(1);

    await buttons(wrapper, /^Pause 2/)[0]!.trigger('click');
    await flushPromises();

    expect(store.pauseReplications).toHaveBeenCalledTimes(1);
    expect(vi.mocked(store.pauseReplications).mock.calls[0]![0]).toHaveLength(
      2,
    );
    expect(wrapper.find('.kit-table__bulk').exists()).toBe(false);
  });

  it('keeps only the rows the action failed for selected', async () => {
    const { wrapper, store } = mountTab([row('b', false), row('c', false)]);

    vi.mocked(store.pauseReplications).mockImplementation(((
      rows: ReturnType<typeof row>[],
    ) => Promise.resolve([rows[1]])) as never);

    await flushPromises();
    await wrapper.findAll('.n-checkbox')[0]!.trigger('click');
    await flushPromises();
    await buttons(wrapper, /^Pause 2/)[0]!.trigger('click');
    await flushPromises();

    expect(wrapper.find('.kit-table__bulk').text()).toContain('1');
    expect(buttons(wrapper, /^Pause 1/)).toHaveLength(1);
  });
});

describe('ReplicationsTab filtered to zero', () => {
  it('shows one Clear filters button', async () => {
    const { wrapper, store } = mountTab([row('b', false)]);

    store.filterUsers = ['nobody'];
    await flushPromises();

    expect(buttons(wrapper, /^Clear filters$/)).toHaveLength(1);
  });
});
