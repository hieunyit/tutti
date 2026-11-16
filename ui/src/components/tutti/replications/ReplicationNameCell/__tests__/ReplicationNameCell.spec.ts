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

import { describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import ReplicationNameCell from '../ReplicationNameCell.vue';
import ReplicationNameField from '../../ReplicationNameField/ReplicationNameField.vue';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
}));

const WriteGate = { template: '<slot />' };

const unnamed = { id: { fromStorage: 'a', toStorage: 'b' } } as never;
const named = {
  id: { fromStorage: 'a', toStorage: 'b', fromBucket: 'x', toBucket: 'x' },
  name: 'prod to dr',
} as never;

function mountCell(
  replication: unknown,
  rename: (name: string) => Promise<void> = vi
    .fn()
    .mockResolvedValue(undefined),
) {
  return mount(ReplicationNameCell, {
    props: { replication: replication as never, rename },
    global: {
      plugins: [kitI18n()],
      stubs: { WriteGate },
    },
    attachTo: document.body,
  });
}

async function openEdit(wrapper: ReturnType<typeof mountCell>) {
  await wrapper.find('button').trigger('click');
  await flushPromises();

  return wrapper.find('input');
}

describe('ReplicationNameCell', () => {
  it('shows the auto label as the primary text of an unnamed replication', () => {
    const wrapper = mountCell(unnamed);

    expect(wrapper.find('.replication-name-cell__primary').text()).toBe(
      'a → b · all buckets',
    );
    expect(wrapper.find('.replication-name-cell__secondary').exists()).toBe(
      false,
    );
  });

  it('shows the name with from → to and bucket as secondary text', () => {
    const wrapper = mountCell(named);

    expect(wrapper.find('.replication-name-cell__primary').text()).toBe(
      'prod to dr',
    );
    expect(wrapper.find('.replication-name-cell__secondary').text()).toBe(
      'a → b · x',
    );
  });

  it('shows a quiet Since line only when the replication has a cutoff', () => {
    const withSince = mountCell({
      ...(named as object),
      since: '2026-10-17T14:03:00Z',
    });

    expect(withSince.find('.replication-name-cell__since').text()).toBe(
      'Since 17 Oct 2026',
    );
    // the hover text adds the time of day
    expect(
      withSince.find('.replication-name-cell__since').attributes('title'),
    ).toMatch(/^Since 17 Oct 2026, \d{2}:\d{2}$/);
    expect(
      mountCell(named).find('.replication-name-cell__since').exists(),
    ).toBe(false);
    expect(
      mountCell({ ...(unnamed as object), since: '0001-01-01T00:00:00Z' })
        .find('.replication-name-cell__since')
        .exists(),
    ).toBe(false);
  });

  it('saves the trimmed name on Enter', async () => {
    const rename = vi.fn().mockResolvedValue(undefined);
    const wrapper = mountCell(named, rename);
    const input = await openEdit(wrapper);

    await input.setValue('  new name ');
    await input.trigger('keydown', { key: 'Enter' });
    await flushPromises();

    expect(rename).toHaveBeenCalledWith('new name');
    expect(wrapper.find('input').exists()).toBe(false);
  });

  it('cancels on Escape without calling the server', async () => {
    const rename = vi.fn();
    const wrapper = mountCell(named, rename);
    const input = await openEdit(wrapper);

    await input.setValue('other');
    await input.trigger('keydown', { key: 'Escape' });

    expect(rename).not.toHaveBeenCalled();
    expect(wrapper.find('input').exists()).toBe(false);
    expect(wrapper.find('.replication-name-cell__primary').text()).toBe(
      'prod to dr',
    );
  });

  it('allows clearing the name with an empty value', async () => {
    const rename = vi.fn().mockResolvedValue(undefined);
    const wrapper = mountCell(named, rename);
    const input = await openEdit(wrapper);

    await input.setValue('');
    await input.trigger('keydown', { key: 'Enter' });
    await flushPromises();

    expect(rename).toHaveBeenCalledWith('');
  });

  it('keeps the edit open and shows an error when the server refuses', async () => {
    const rename = vi.fn().mockRejectedValue(new Error('boom'));
    const wrapper = mountCell(named, rename);
    const input = await openEdit(wrapper);

    await input.setValue('new name');
    await input.trigger('keydown', { key: 'Enter' });
    await flushPromises();

    expect(wrapper.find('input').exists()).toBe(true);
    expect(wrapper.find('[role="alert"]').text()).toContain('Could not rename');
  });

  it('does not submit an over-long name', async () => {
    const rename = vi.fn();
    const wrapper = mountCell(named, rename);
    const input = await openEdit(wrapper);

    (input.element as HTMLInputElement).value = 'a'.repeat(65);
    await input.trigger('input');
    await input.trigger('keydown', { key: 'Enter' });

    expect(rename).not.toHaveBeenCalled();
  });
});

describe('ReplicationNameField', () => {
  it('has a visible label and the character counter', () => {
    const wrapper = mount(ReplicationNameField, {
      props: { modelValue: 'abc' },
      global: {
        plugins: [createI18n({ legacy: false, locale: 'en', messages: {} })],
      },
    });

    expect(wrapper.find('label').text()).toBe('Name (optional)');
    expect(wrapper.text()).toContain('3 / 64');
  });

  it('shows the error after blur for an over-long value', async () => {
    const wrapper = mount(ReplicationNameField, {
      props: { modelValue: 'a'.repeat(65) },
      global: { plugins: [kitI18n()] },
    });

    expect(wrapper.find('[role="alert"]').exists()).toBe(false);
    await wrapper.find('input').trigger('blur');

    expect(wrapper.find('[role="alert"]').text()).toContain('64');
  });
});
