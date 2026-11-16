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
import { mount } from '@vue/test-utils';
import ReplicationSummary from '../ReplicationSummary.vue';
import ReplicationSinceField from '../../add-replication/ReplicationSinceField.vue';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';
import { defaultSinceChoice } from '@/utils/helpers/ReplicationSinceHelper';

tolerateNaiveStyles();

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18N_DEFAULT_LOCALE: 'en',
  I18nLocale: { EN: 'en', DE: 'de' },
}));

const base = {
  id: { fromStorage: 'a', toStorage: 'b' },
  isInitDone: true,
  initObjDone: '3',
  initObjListed: '3',
  eventLag: '',
  createdAt: '2026-10-18T10:00:00Z',
} as never;

function mountSummary(extra: object) {
  return mount(ReplicationSummary, {
    props: { replication: { ...(base as object), ...extra } as never },
    global: { plugins: [kitI18n()] },
  });
}

describe('ReplicationSummary since', () => {
  it('shows the cutoff once when set', () => {
    const wrapper = mountSummary({ since: '2026-10-17T14:03:00Z' });

    expect(wrapper.text()).toContain('Objects copied');
    expect(wrapper.text().match(/Modified since/g)).toHaveLength(1);
  });

  it('shows nothing about it when the whole source is copied', () => {
    expect(mountSummary({}).text()).not.toContain('Modified since');
  });
});

describe('ReplicationSinceField', () => {
  function mountField(value = defaultSinceChoice()) {
    return mount(ReplicationSinceField, {
      props: { modelValue: value },
      global: { plugins: [kitI18n()] },
      attachTo: document.body,
    });
  }

  it('offers all objects first and hides the inputs', () => {
    const wrapper = mountField();

    expect(wrapper.text()).toContain('All objects');
    expect(wrapper.text()).toContain('Only recent objects');
    expect(wrapper.text()).toContain('Since a date');
    expect(wrapper.find('input[type="text"]').exists()).toBe(false);
  });

  it('shows the computed moment for recent objects', () => {
    const wrapper = mountField({ ...defaultSinceChoice(), mode: 'recent' });

    expect(wrapper.text()).toContain('Objects modified since');
  });

  it('shows an inline error after blur for an invalid amount and none before', async () => {
    const wrapper = mountField({
      ...defaultSinceChoice(),
      mode: 'recent',
      amount: null,
    });

    expect(wrapper.find('[role="alert"]').exists()).toBe(false);
    await wrapper.find('input[type="text"]').trigger('blur');

    expect(wrapper.find('[role="alert"]').text()).toBe(
      'Enter a whole number of 1 or more.',
    );
  });

  it('asks for a date when "Since a date" has none yet', async () => {
    const wrapper = mountField({ ...defaultSinceChoice(), mode: 'date' });

    await wrapper.find('input[type="text"]').trigger('blur');

    expect(wrapper.find('[role="alert"]').text()).toBe(
      'Choose a date and time.',
    );
  });
});
