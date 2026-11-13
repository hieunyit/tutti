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
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import SessionExpiryDialog from '../SessionExpiryDialog.vue';
import i18nSession from '../i18nSession';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18N_DEFAULT_LOCALE: 'en',
  I18nLocale: { EN: 'en', DE: 'de' },
}));

tolerateNaiveStyles();

let wrapper: VueWrapper | undefined;

function mountDialog(props: Record<string, unknown>) {
  wrapper = mount(SessionExpiryDialog, {
    attachTo: document.body,
    props: {
      warning: 'none',
      remainingIdleMs: 90_000,
      remainingMaxMs: 3600_000,
      ...props,
    },
    global: { plugins: [kitI18n()] },
  });

  return wrapper;
}

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = '';
});

describe('SessionExpiryDialog', () => {
  it('renders nothing without a warning', () => {
    mountDialog({});
    expect(document.body.querySelector('[data-session-continue]')).toBeNull();
    expect(document.body.querySelector('[data-session-max]')).toBeNull();
  });

  it('shows a countdown and emits continue / signOut', async () => {
    const w = mountDialog({ warning: 'idle' });

    await flushPromises();
    expect(
      document.body.querySelector('[data-session-idle-text]')?.textContent,
    ).toContain('1:30');
    document.body
      .querySelector<HTMLElement>('[data-session-continue]')
      ?.click();
    document.body
      .querySelector<HTMLElement>('[data-session-sign-out]')
      ?.click();
    expect(w.emitted('continue')).toHaveLength(1);
    expect(w.emitted('signOut')).toHaveLength(1);
  });

  it('shows the max-lifetime banner with local hh:mm and no extend button', async () => {
    vi.useFakeTimers({ now: new Date(2026, 10, 13, 10, 0, 0) });
    mountDialog({ warning: 'max', remainingMaxMs: 4 * 60_000 });
    await flushPromises();

    const banner = document.body.querySelector('[data-session-max]');

    expect(banner?.textContent).toContain('10:04');
    expect(document.body.querySelector('[data-session-continue]')).toBeNull();
    vi.useRealTimers();
  });
});

describe('session locale keys', () => {
  it('has the same keys in en, de and vi', () => {
    const keys = Object.keys(i18nSession.en ?? {}).sort();

    expect(Object.keys(i18nSession.de ?? {}).sort()).toEqual(keys);
    expect(Object.keys(i18nSession.vi ?? {}).sort()).toEqual(keys);
  });
});
