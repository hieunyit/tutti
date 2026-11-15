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

import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import ActivityInFlightTable from '../ActivityInFlightTable.vue';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
  I18N_DEFAULT_LOCALE: 'en',
}));

const items = [
  {
    key: 'photos/2026/a-very-long-object-name.jpg',
    size: '2048',
    bytesDone: '1024',
    startedAt: '2026-01-01T00:00:00Z',
    flow: 'live',
  },
  {
    key: 'b.txt',
    size: '10',
    bytesDone: '10',
    startedAt: '2026-01-01T00:00:05Z',
    flow: 'live',
  },
] as never;

function setNarrow(narrow: boolean) {
  vi.stubGlobal('matchMedia', () => ({
    matches: narrow,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }));
}

function render() {
  return mount(ActivityInFlightTable, {
    props: { items, now: Date.parse('2026-01-01T00:01:00Z') },
    global: { plugins: [kitI18n()] },
  });
}

afterEach(() => vi.unstubAllGlobals());

describe('ActivityInFlightTable', () => {
  it('shows a wide table on a wide screen', () => {
    setNarrow(false);

    const wrapper = render();

    expect(wrapper.find('.n-data-table').exists()).toBe(true);
    expect(wrapper.find('.activity-inflight__list').exists()).toBe(false);
  });

  it('shows one compact card per object (key, size, progress, elapsed) when narrow', async () => {
    setNarrow(true);

    const wrapper = render();

    await flushPromises();
    const cards = wrapper.findAll('.activity-inflight__card');

    expect(wrapper.find('.n-data-table').exists()).toBe(false);
    expect(cards).toHaveLength(2);
    expect(cards[0]!.text()).toContain('a-very-long-object-name.jpg');
    expect(cards[0]!.text()).toContain('50%');
    expect(cards[0]!.find('.activity-inflight__bar-fill').exists()).toBe(true);
  });
});
