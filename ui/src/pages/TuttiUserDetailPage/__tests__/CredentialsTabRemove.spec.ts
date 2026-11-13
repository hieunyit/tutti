/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  flushPromises,
  mount,
  RouterLinkStub,
  type VueWrapper,
} from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { AxiosError } from 'axios';
import CredentialsTab from '../CredentialsTab.vue';
import { ConfirmDialog } from '@/components/kit';
import { TuttiService } from '@/services/TuttiService';
import { useAuthStore } from '@/stores/authStore';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('@/services/TuttiService', () => ({
  TuttiService: {
    removeUserCredential: vi.fn(),
    updateUserCredentials: vi.fn(),
    testUser: vi.fn(),
  },
}));

let wrapper: VueWrapper | undefined;

function mountTab() {
  wrapper = mount(CredentialsTab, {
    attachTo: document.body,
    props: {
      userName: 'alice',
      dynamicEnabled: true,
      storages: [
        {
          storage: 'archive',
          configured: true,
          mode: 'override',
          source: 'dynamic',
          accessKeyId: 'AKIA1',
        },
      ],
    },
    global: { plugins: [kitI18n()], stubs: { RouterLink: RouterLinkStub } },
  });

  return wrapper;
}

function button(label: string, root: ParentNode = document.body) {
  return Array.from(root.querySelectorAll<HTMLButtonElement>('button')).find(
    (item) => item.textContent?.trim() === label,
  );
}

function dialogOpen() {
  return wrapper!.findComponent(ConfirmDialog).props('open');
}

async function openRemove() {
  button('Remove override')!.click();
  await flushPromises();
}

beforeEach(() => {
  setActivePinia(createPinia());
  useAuthStore().authEnabled = false;
  vi.mocked(TuttiService.removeUserCredential).mockReset();
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = '';
});

describe('CredentialsTab remove override', () => {
  it('asks for confirmation and does not call the service before it', async () => {
    mountTab();
    await flushPromises();
    expect(dialogOpen()).toBe(false);
    await openRemove();

    expect(dialogOpen()).toBe(true);
    expect(document.body.textContent).toContain('archive');
    expect(TuttiService.removeUserCredential).not.toHaveBeenCalled();
  });

  it('cancel closes the dialog without calling the service', async () => {
    const view = mountTab();

    await flushPromises();
    await openRemove();
    document.body
      .querySelector<HTMLButtonElement>('[data-kit-cancel]')!
      .click();
    await flushPromises();

    expect(dialogOpen()).toBe(false);
    expect(TuttiService.removeUserCredential).not.toHaveBeenCalled();
    expect(view.emitted('changed')).toBeUndefined();
  });

  it('confirming calls the service once and asks the page to refresh', async () => {
    vi.mocked(TuttiService.removeUserCredential).mockResolvedValue(undefined);
    const view = mountTab();

    await flushPromises();
    await openRemove();
    document.body
      .querySelector<HTMLButtonElement>('[data-kit-confirm]')!
      .click();
    await flushPromises();

    expect(TuttiService.removeUserCredential).toHaveBeenCalledTimes(1);
    expect(TuttiService.removeUserCredential).toHaveBeenCalledWith(
      'alice',
      'archive',
    );
    expect(view.emitted('changed')).toHaveLength(1);
    expect(dialogOpen()).toBe(false);
  });

  it('shows the error, keeps the dialog and does not refresh on failure', async () => {
    const failure = new AxiosError('boom');

    failure.response = {
      data: { message: 'storage unreachable' },
    } as AxiosError['response'];
    vi.mocked(TuttiService.removeUserCredential).mockRejectedValue(failure);
    const view = mountTab();

    await flushPromises();
    await openRemove();
    document.body
      .querySelector<HTMLButtonElement>('[data-kit-confirm]')!
      .click();
    await flushPromises();

    expect(TuttiService.removeUserCredential).toHaveBeenCalledTimes(1);
    expect(dialogOpen()).toBe(true);
    expect(document.body.textContent).toContain('storage unreachable');
    expect(view.emitted('changed')).toBeUndefined();
  });
});
