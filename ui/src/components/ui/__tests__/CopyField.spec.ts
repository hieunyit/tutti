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
import { createI18n } from 'vue-i18n';
import CopyField from '../CopyField.vue';

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
}));

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en: {} } });
const mountField = (props: {
  value: string;
  label?: string;
  secret?: boolean;
}) => mount(CopyField, { props, global: { plugins: [i18n] } });

describe('CopyField', () => {
  it('shows the value and has no Show button by default', () => {
    const wrapper = mountField({ value: 'abc', label: 'Access key' });

    expect(wrapper.get('code').text()).toBe('abc');
    expect(wrapper.findAll('button')).toHaveLength(1);
  });

  it('masks a secret until Show is pressed and copies the real value', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);

    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText },
      configurable: true,
    });

    const wrapper = mountField({ value: 'topsecret', secret: true });

    expect(wrapper.get('code').text()).not.toContain('topsecret');

    const [toggle, copy] = wrapper.findAll('button');

    await copy!.trigger('click');
    expect(writeText).toHaveBeenCalledWith('topsecret');

    await toggle!.trigger('click');
    expect(wrapper.get('code').text()).toBe('topsecret');
    expect(toggle!.attributes('aria-pressed')).toBe('true');
  });
});
