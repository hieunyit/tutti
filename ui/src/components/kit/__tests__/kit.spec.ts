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
import { defineComponent, h, nextTick } from 'vue';
import { mount } from '@vue/test-utils';
import {
  CopyValue,
  DataTable,
  ErrorBlock,
  StatusBadge,
  WizardShell,
  clearWizardDraft,
  useWizardDraft,
} from '../index';
import { kitI18n, tolerateNaiveStyles } from './kitTestUtils';

tolerateNaiveStyles();

const global = { plugins: [kitI18n()] };

beforeEach(() => {
  window.sessionStorage.clear();
});

describe('StatusBadge', () => {
  it('shows the label next to a tone dot', () => {
    const wrapper = mount(StatusBadge, {
      props: { tone: 'danger', label: 'Unavailable' },
    });

    expect(wrapper.text()).toBe('Unavailable');
    expect(wrapper.attributes('data-tone')).toBe('danger');
  });
});

describe('DataTable', () => {
  const columns = [{ key: 'name', title: 'Name' }];
  const rowKey = (row: Record<string, unknown>) => String(row.name);

  it('shows ErrorBlock with retry when error is set', async () => {
    const wrapper = mount(DataTable, {
      props: { columns, rows: [], loading: false, error: 'boom', rowKey },
      global,
    });

    expect(wrapper.findComponent(ErrorBlock).exists()).toBe(true);
    expect(wrapper.text()).toContain('boom');
    await wrapper.get('button').trigger('click');
    expect(wrapper.emitted('retry')).toHaveLength(1);
  });

  it('shows a skeleton on first load and the empty slot for zero rows', async () => {
    const wrapper = mount(DataTable, {
      props: { columns, rows: [], loading: true, rowKey },
      slots: { empty: '<p class="none">No storages yet</p>' },
      global,
    });

    expect(wrapper.find('[data-kit-state="loading"]').exists()).toBe(true);
    await wrapper.setProps({ loading: false });
    expect(wrapper.find('.none').exists()).toBe(true);
  });

  it('renders rows', () => {
    const wrapper = mount(DataTable, {
      props: {
        columns,
        rows: [{ name: 'alpha' }, { name: 'beta' }],
        loading: false,
        rowKey,
      },
      global,
    });

    expect(wrapper.text()).toContain('alpha');
    expect(wrapper.text()).toContain('beta');
  });
});

describe('CopyValue', () => {
  it('asks for the "saved" tick in one-time mode', async () => {
    const wrapper = mount(CopyValue, {
      props: { value: 'SECRET', oneTime: true },
      global,
    });

    expect(wrapper.text()).toContain('SECRET');
    expect(wrapper.find('[role="note"]').exists()).toBe(true);
    await wrapper.get('.n-checkbox').trigger('click');
    expect(wrapper.emitted('acknowledged')).toEqual([[true]]);
  });

  it('never writes the value to storage', () => {
    mount(CopyValue, { props: { value: 'SECRET', oneTime: true }, global });

    const stored = [
      ...Object.values({ ...window.localStorage }),
      ...Object.values({ ...window.sessionStorage }),
    ].join();

    expect(stored).not.toContain('SECRET');
  });
});

describe('WizardShell and useWizardDraft', () => {
  const Host = defineComponent({
    props: { firstValid: { type: Boolean, default: true } },
    setup(props) {
      const draft = useWizardDraft('test-wizard', () => ({ name: '' }));

      return () =>
        h(
          WizardShell,
          {
            draftKey: 'test-wizard',
            steps: [
              { key: 'one', label: 'One', valid: props.firstValid },
              { key: 'two', label: 'Two', valid: true },
            ],
          },
          {
            one: () =>
              h('input', {
                class: 'name',
                value: draft.value.name,
                onInput: (event: Event) => {
                  draft.value.name = (event.target as HTMLInputElement).value;
                },
              }),
            two: () => h('p', { class: 'step-two' }, draft.value.name),
          },
        );
    },
  });

  function primary(wrapper: ReturnType<typeof mount>) {
    return wrapper
      .findAll('button')
      .find((b) => b.classes().includes('n-button--primary-type'))!;
  }

  it('keeps Next disabled while the step is invalid', () => {
    const wrapper = mount(Host, { props: { firstValid: false }, global });

    expect(primary(wrapper).attributes('disabled')).toBeDefined();
  });

  it('restores the draft and step after leaving, and Start over clears both', async () => {
    const first = mount(Host, { global });

    await first.get('input.name').setValue('my-bucket');
    await primary(first).trigger('click');
    expect(first.find('.step-two').text()).toBe('my-bucket');
    first.unmount();

    const again = mount(Host, { global });

    expect(again.find('.step-two').text()).toBe('my-bucket');

    const startOver = again
      .findAll('button')
      .find((b) => b.text() === 'Start over')!;

    await startOver.trigger('click');
    await nextTick();
    expect((again.get('input.name').element as HTMLInputElement).value).toBe(
      '',
    );
    // back on the first step, with no trace of the old value
    expect(again.find('.step-two').exists()).toBe(false);
    expect(Object.values({ ...window.sessionStorage }).join()).not.toContain(
      'my-bucket',
    );
    again.unmount();
    clearWizardDraft('test-wizard');
  });
});
