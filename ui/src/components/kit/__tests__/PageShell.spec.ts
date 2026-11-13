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

import { describe, expect, it } from 'vitest';
import { mount } from '@vue/test-utils';
import PageShell from '../PageShell.vue';
import { ErrorBlock } from '../index';
import { kitI18n, tolerateNaiveStyles } from './kitTestUtils';

tolerateNaiveStyles();

type State = 'loading' | 'empty' | 'error' | 'ready';

function mountShell(state: State, extra: Record<string, unknown> = {}) {
  return mount(PageShell, {
    props: { title: 'Storages', state, ...extra },
    slots: {
      default: '<p class="content">content</p>',
      empty: '<p class="empty-slot">nothing yet</p>',
      actions: '<button type="button" class="action">Add</button>',
    },
    global: {
      plugins: [kitI18n()],
    },
  });
}

describe('PageShell', () => {
  it('renders the title, description and actions in every state', () => {
    const wrapper = mountShell('ready', { description: 'All storages' });

    expect(wrapper.get('h1').text()).toBe('Storages');
    expect(wrapper.text()).toContain('All storages');
    expect(wrapper.find('.action').exists()).toBe(true);
  });

  it('shows a skeleton while loading', () => {
    const wrapper = mountShell('loading');

    expect(wrapper.find('[data-kit-state="loading"]').exists()).toBe(true);
    expect(wrapper.find('[aria-busy="true"]').exists()).toBe(true);
    expect(wrapper.find('.content').exists()).toBe(false);
  });

  it('shows the empty slot when empty', () => {
    const wrapper = mountShell('empty');

    expect(wrapper.find('.empty-slot').exists()).toBe(true);
    expect(wrapper.find('.content').exists()).toBe(false);
  });

  it('shows an ErrorBlock whose button emits retry', async () => {
    const wrapper = mountShell('error');
    const block = wrapper.findComponent(ErrorBlock);

    expect(block.exists()).toBe(true);
    expect(wrapper.find('[role="alert"]').exists()).toBe(true);
    await block.get('button').trigger('click');
    expect(wrapper.emitted('retry')).toHaveLength(1);
    expect(wrapper.find('.content').exists()).toBe(false);
  });

  it('renders the default slot when ready', () => {
    const wrapper = mountShell('ready');

    expect(wrapper.find('.content').exists()).toBe(true);
    expect(wrapper.find('[data-kit-state]').exists()).toBe(false);
  });

  it('renders tabs and emits update:activeTab', async () => {
    const wrapper = mountShell('ready', {
      tabs: [
        { key: 'fleet', label: 'Fleet' },
        { key: 'config', label: 'Config', badge: 2 },
      ],
      activeTab: 'fleet',
    });
    const tabs = wrapper.findAll('[role="tab"]');

    expect(tabs).toHaveLength(2);
    expect(tabs[0]!.attributes('aria-selected')).toBe('true');
    expect(tabs[1]!.text()).toContain('2');
    await tabs[1]!.trigger('click');
    expect(wrapper.emitted('update:activeTab')).toEqual([['config']]);
  });
});
