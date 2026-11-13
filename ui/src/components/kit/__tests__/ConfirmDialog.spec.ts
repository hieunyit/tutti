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

import { afterEach, describe, expect, it } from 'vitest';
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import ConfirmDialog from '../ConfirmDialog.vue';
import { kitI18n, tolerateNaiveStyles } from './kitTestUtils';

tolerateNaiveStyles();

let wrapper: VueWrapper | undefined;

function mountDialog(props: Record<string, unknown>) {
  wrapper = mount(ConfirmDialog, {
    attachTo: document.body,
    props: {
      open: true,
      title: 'Delete storage',
      confirmLabel: 'Delete storage',
      ...props,
    },
    global: {
      plugins: [kitI18n()],
    },
  });

  return wrapper;
}

function confirmButton(): HTMLButtonElement {
  const button =
    document.body.querySelector<HTMLButtonElement>('[data-kit-confirm]');

  if (!button) throw new Error('confirm button not rendered');

  return button;
}

function typeInto(value: string) {
  const input = document.body.querySelector<HTMLInputElement>(
    '[data-kit-confirm-input] input, input[data-kit-confirm-input]',
  );

  if (!input) throw new Error('confirm input not rendered');

  input.value = value;
  input.dispatchEvent(new Event('input'));
}

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = '';
});

describe('ConfirmDialog', () => {
  it('names the target and confirms directly when typing is not required', async () => {
    const dialog = mountDialog({ targetName: 'bucket-a', danger: true });

    await flushPromises();
    expect(document.body.textContent).toContain('bucket-a');
    expect(confirmButton().disabled).toBe(false);
    confirmButton().click();
    expect(dialog.emitted('confirm')).toHaveLength(1);
  });

  it('keeps confirm disabled until the target name is typed', async () => {
    const dialog = mountDialog({
      targetName: 'bucket-a',
      requireTyping: true,
      danger: true,
    });

    await flushPromises();
    expect(confirmButton().disabled).toBe(true);

    typeInto('bucket-');
    await flushPromises();
    expect(confirmButton().disabled).toBe(true);
    confirmButton().click();
    expect(dialog.emitted('confirm')).toBeUndefined();

    typeInto('bucket-a');
    await flushPromises();
    expect(confirmButton().disabled).toBe(false);
    confirmButton().click();
    expect(dialog.emitted('confirm')).toHaveLength(1);
  });

  it('emits cancel from the cancel button', async () => {
    const dialog = mountDialog({});

    await flushPromises();
    document.body
      .querySelector<HTMLButtonElement>('[data-kit-cancel]')
      ?.click();
    expect(dialog.emitted('cancel')).toHaveLength(1);
  });
});
