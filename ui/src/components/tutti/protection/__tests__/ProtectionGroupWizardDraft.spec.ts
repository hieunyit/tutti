/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import ProtectionGroupWizard from '../ProtectionGroupWizard.vue';
import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
import { wizardDraftStorageKey } from '@/components/kit/composables';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
  I18N_DEFAULT_LOCALE: 'en',
}));

const KEY = 'protection-group-create';
let wrapper: VueWrapper | undefined;

function mountWizard() {
  wrapper = mount(ProtectionGroupWizard, {
    attachTo: document.body,
    props: { show: true },
    global: { plugins: [kitI18n()] },
  });

  return wrapper;
}

function idInput() {
  return document.body.querySelector<HTMLInputElement>(
    '.n-form-item input',
  ) as HTMLInputElement;
}

async function typeId(value: string) {
  idInput().value = value;
  idInput().dispatchEvent(new Event('input'));
  await flushPromises();
}

function stored() {
  const raw = window.sessionStorage.getItem(wizardDraftStorageKey(KEY));

  return raw === null ? null : (JSON.parse(raw) as Record<string, unknown>);
}

beforeEach(() => {
  window.sessionStorage.clear();
  setActivePinia(createPinia());
  vi.spyOn(useTuttiProtectionStore(), 'loadOptions').mockResolvedValue();
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = '';
});

describe('ProtectionGroupWizard draft', () => {
  it('restores step 1 values after a remount with the same draft key', async () => {
    mountWizard();
    await flushPromises();
    await typeId('gold-group');
    expect(stored()?.id).toBe('gold-group');

    wrapper!.unmount();
    document.body.innerHTML = '';
    mountWizard();
    await flushPromises();

    expect(idInput().value).toBe('gold-group');
  });

  it('start over clears the stored draft and the visible values', async () => {
    mountWizard();
    await flushPromises();
    await typeId('gold-group');

    const startOver = Array.from(
      document.body.querySelectorAll<HTMLButtonElement>('button'),
    ).find((button) => button.textContent?.trim() === 'Start over');

    startOver!.click();
    await flushPromises();

    expect(idInput().value).toBe('');
    // the reset form is re-persisted, but nothing typed survives
    expect(stored()?.id ?? '').toBe('');
    expect(
      window.sessionStorage.getItem(wizardDraftStorageKey(KEY, 'step')),
    ).toBe(null);

    wrapper!.unmount();
    document.body.innerHTML = '';
    mountWizard();
    await flushPromises();
    expect(idInput().value).toBe('');
  });

  it('never writes secrets into the stored draft', async () => {
    mountWizard();
    await flushPromises();
    await typeId('gold-group');

    const dump = JSON.stringify(
      Object.fromEntries(
        Array.from({ length: window.sessionStorage.length }, (_, i) => {
          const key = window.sessionStorage.key(i) as string;

          return [key, window.sessionStorage.getItem(key)];
        }),
      ),
    );

    expect(dump).toContain('gold-group');
    expect(dump).not.toMatch(/secret|password|accessKey|token/i);
  });
});
