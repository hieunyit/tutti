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
import ReplicationRescanField from '../../add-replication/ReplicationRescanField.vue';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18N_DEFAULT_LOCALE: 'en',
  I18nLocale: { EN: 'en', DE: 'de' },
}));

describe('ReplicationRescanField', () => {
  function mountField(value = 0) {
    return mount(ReplicationRescanField, {
      props: { modelValue: value },
      global: { plugins: [kitI18n()] },
      attachTo: document.body,
    });
  }

  it('is off by default and says when to use it, once', () => {
    const wrapper = mountField();

    expect(wrapper.text()).toContain(
      'Pick up files written directly to the source',
    );
    expect(wrapper.text()).toContain('Off');
    expect(wrapper.text().match(/not through the Tutti proxy/g)).toHaveLength(
      1,
    );
    expect(wrapper.text()).toContain('Files that are deleted at the source');
  });

  it('shows the chosen interval and links the help to the select', () => {
    const wrapper = mountField(60);

    expect(wrapper.text()).toContain('Every hour');
    expect(wrapper.find('p[id^="rescan-help"]').exists()).toBe(true);
  });
});
