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

import { beforeEach, describe, expect, it } from 'vitest';
import { h } from 'vue';
import { mount } from '@vue/test-utils';
import {
  EmptyState,
  PageHeader,
  SectionCard,
  StatusBadge,
  TableToolbar,
  WizardShell,
  clearWizardDraft,
} from '../index';
import { kitI18n, tolerateNaiveStyles } from './kitTestUtils';

tolerateNaiveStyles();

const global = { plugins: [kitI18n()] };

beforeEach(() => {
  window.sessionStorage.clear();
});

describe('PageHeader', () => {
  it('renders the title and a description that adds information', () => {
    const wrapper = mount(PageHeader, {
      props: { title: 'Storages', description: 'S3 backends in use' },
      global,
    });

    expect(wrapper.get('h1').text()).toBe('Storages');
    expect(wrapper.text()).toContain('S3 backends in use');
  });

  it('drops a description that only restates the title', () => {
    const wrapper = mount(PageHeader, {
      props: { title: 'Storages', description: ' storages ' },
      global,
    });

    expect(wrapper.find('.kit-header__description').exists()).toBe(false);
  });

  it('shows the primary action slot and a keyboard reachable More menu', () => {
    const wrapper = mount(PageHeader, {
      props: {
        title: 'Users',
        secondaryActions: [{ key: 'export', label: 'Export' }],
      },
      slots: { actions: '<button class="primary">Create user</button>' },
      global,
    });
    const more = wrapper.get('button.kit-header__more');

    expect(wrapper.find('.primary').exists()).toBe(true);
    expect(more.attributes('aria-haspopup')).toBe('menu');
    expect(more.element.tagName).toBe('BUTTON');
  });

  it('renders no actions area without actions', () => {
    const wrapper = mount(PageHeader, { props: { title: 'Users' }, global });

    expect(wrapper.find('.kit-header__actions').exists()).toBe(false);
  });
});

describe('TableToolbar', () => {
  const filters = '<select class="status"><option>Active</option></select>';

  it('keeps filters collapsed by default and toggles them with a button', async () => {
    const wrapper = mount(TableToolbar, {
      slots: { default: filters },
      global,
    });
    const toggle = wrapper.get('button.kit-filter__toggle');

    expect(toggle.text()).toBe('Filters');
    expect(toggle.attributes('aria-expanded')).toBe('false');
    expect(wrapper.find('.status').exists()).toBe(false);

    await toggle.trigger('click');
    expect(toggle.attributes('aria-expanded')).toBe('true');
    expect(wrapper.find('.status').exists()).toBe(true);
    expect(toggle.attributes('aria-controls')).toBe(
      wrapper.get('.kit-filter__panel').attributes('id'),
    );
  });

  it('shows the active filter count and removable chips', async () => {
    const wrapper = mount(TableToolbar, {
      props: { chips: [{ key: 'status', label: 'Status: Active' }] },
      slots: { default: filters },
      global,
    });

    expect(wrapper.get('button.kit-filter__toggle').text()).toBe('Filters (1)');
    await wrapper.get('.kit-filter__chip-remove').trigger('click');
    expect(wrapper.emitted('remove')?.[0]).toEqual(['status']);
  });

  it('has no Filters button when no filters are given, and hides entirely when empty', () => {
    const bare = mount(TableToolbar, { global });

    expect(bare.find('.kit-filter__toggle').exists()).toBe(false);

    const hidden = mount(TableToolbar, {
      props: { hidden: true },
      slots: { default: filters },
      global,
    });

    expect(hidden.find('.kit-filter').exists()).toBe(false);
  });

  it('renders the bulk actions area and emits search updates', async () => {
    const wrapper = mount(TableToolbar, {
      slots: { bulk: '<button class="bulk">Delete 2</button>' },
      global,
    });

    expect(wrapper.find('.kit-filter__bulk .bulk').exists()).toBe(true);
    await wrapper.get('input').setValue('abc');
    expect(wrapper.emitted('update:search')?.[0]).toEqual(['abc']);
  });
});

describe('EmptyState', () => {
  it('shows a title, one sentence and the primary action', () => {
    const wrapper = mount(EmptyState, {
      props: { title: 'No users yet', description: 'Users sign in to S3.' },
      slots: {
        default: '<button class="cta">Create user</button>',
        icon: '<svg class="ico" />',
      },
      global,
    });

    expect(wrapper.text()).toContain('No users yet');
    expect(wrapper.text()).toContain('Users sign in to S3.');
    expect(wrapper.find('.cta').exists()).toBe(true);
    expect(wrapper.find('.ico').exists()).toBe(true);
  });

  it('drops a body that stutters the title', () => {
    const wrapper = mount(EmptyState, {
      props: {
        title: 'No users yet',
        description: 'No users yet. Create one to start.',
      },
      global,
    });

    expect(wrapper.find('.kit-state__text').exists()).toBe(false);
  });
});

describe('WizardShell pattern', () => {
  function mountWizard() {
    return mount(WizardShell, {
      props: {
        draftKey: 'pattern-wizard',
        steps: [
          { key: 'one', label: 'One', valid: true },
          { key: 'two', label: 'Two', valid: true },
        ],
      },
      slots: {
        one: () => h('p', 'first'),
        two: () => h('p', 'second'),
        summary: () => h('p', { class: 'sum' }, 'summary'),
      },
      global,
    });
  }

  it('shows progress, a summary aside and no Start over on step 1', () => {
    const wrapper = mountWizard();
    const bar = wrapper.get('[role="progressbar"]');

    expect(bar.attributes('aria-valuenow')).toBe('1');
    expect(bar.attributes('aria-valuemax')).toBe('2');
    expect(wrapper.text()).toContain('Step 1 of 2');
    expect(wrapper.find('aside .sum').exists()).toBe(true);
    expect(wrapper.text()).not.toContain('Start over');
    wrapper.unmount();
    clearWizardDraft('pattern-wizard');
  });

  it('offers Back and Start over after step 1', async () => {
    const wrapper = mountWizard();
    const next = wrapper
      .findAll('button')
      .find((b) => b.classes().includes('n-button--primary-type'))!;

    await next.trigger('click');
    expect(wrapper.text()).toContain('Step 2 of 2');
    expect(wrapper.text()).toContain('Start over');
    expect(wrapper.text()).toContain('Back');
    wrapper.unmount();
    clearWizardDraft('pattern-wizard');
  });
});

describe('StatusBadge', () => {
  it('maps a domain status word to a tone and can hide the dot', () => {
    const wrapper = mount(StatusBadge, {
      props: { status: 'Degraded', label: 'Degraded', noDot: true },
    });

    expect(wrapper.attributes('data-tone')).toBe('warning');
    expect(wrapper.find('.kit-status__dot').exists()).toBe(false);
    expect(wrapper.text()).toBe('Degraded');
  });

  it('prefers an explicit tone and falls back to neutral', () => {
    expect(
      mount(StatusBadge, {
        props: { tone: 'danger', status: 'ok', label: 'x' },
      }).attributes('data-tone'),
    ).toBe('danger');
    expect(
      mount(StatusBadge, { props: { label: 'x' } }).attributes('data-tone'),
    ).toBe('neutral');
  });
});

describe('SectionCard variants', () => {
  it('is a bordered card by default and a plain section on request', () => {
    const stubs = { global: { stubs: { RouterLink: true } } };
    const card = mount(SectionCard, { props: { title: 'Members' }, ...stubs });
    const plain = mount(SectionCard, {
      props: { title: 'Members', variant: 'section' },
      ...stubs,
    });

    expect(card.classes()).toContain('tt-section--card');
    expect(plain.classes()).toContain('tt-section--section');
    expect(plain.find('h2').text()).toBe('Members');
  });
});
