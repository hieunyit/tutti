/*
 * Copyright © 2026 Clyso GmbH
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
import { h } from 'vue';
import { mount } from '@vue/test-utils';
import {
  NConfigProvider,
  NDatePicker,
  NInput,
  NInputNumber,
  NSelect,
} from 'naive-ui';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import type { AppLocale } from '@/i18n/locales';
import { NAIVE_LOCALES } from '@/i18n/naiveLocale';

tolerateNaiveStyles();

const APP_LOCALES: AppLocale[] = ['en', 'de', 'vi'];

function render(locale: AppLocale, control: () => ReturnType<typeof h>) {
  const { locale: l, dateLocale } = NAIVE_LOCALES[locale];

  return mount(NConfigProvider, {
    props: { locale: l, dateLocale },
    slots: { default: control },
    attachTo: document.body,
  });
}

describe('Naive UI locale', () => {
  it('has a locale for every app language', () => {
    expect(Object.keys(NAIVE_LOCALES).sort()).toEqual([...APP_LOCALES].sort());
  });

  it.each(APP_LOCALES)(
    '%s: input-like controls have no default placeholder',
    (locale) => {
      const l = NAIVE_LOCALES[locale].locale;

      expect(l.Input?.placeholder).toBe('');
      expect(l.InputNumber?.placeholder).toBe('');
      expect(l.Select?.placeholder).toBe('');
      expect(l.Cascader?.placeholder).toBe('');
    },
  );

  it.each(APP_LOCALES)(
    '%s: NInput, NInputNumber and NSelect render no "Please" text',
    (locale) => {
      const wrapper = render(locale, () =>
        h('div', [h(NInput), h(NInputNumber), h(NSelect, { options: [] })]),
      );

      const html = wrapper.html();

      expect(html).not.toMatch(/Please|Bitte|Vui lòng/);
      expect(wrapper.find('input').attributes('placeholder') ?? '').toBe('');
      wrapper.unmount();
    },
  );

  it('an explicit placeholder still wins', () => {
    const wrapper = render('en', () => h(NInput, { placeholder: 'name@host' }));

    expect(wrapper.find('input').attributes('placeholder')).toBe('name@host');
    wrapper.unmount();
  });

  it('en date picker uses sentence case; de and vi use their own language', () => {
    const en = render('en', () => h(NDatePicker));
    const de = render('de', () => h(NDatePicker));

    expect(en.find('input').attributes('placeholder')).toBe('Select date');
    expect(de.find('input').attributes('placeholder')).not.toBe('Select date');
    en.unmount();
    de.unmount();
    expect(NAIVE_LOCALES.vi.locale.name).toBe('vi-VN');
    expect(NAIVE_LOCALES.en.locale.Empty?.description).toBe('No data');
  });
});
