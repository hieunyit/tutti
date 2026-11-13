/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { AxiosError } from 'axios';
import StorageSettings from '../StorageSettings.vue';
import { ConfirmDialog } from '@/components/kit';
import { useAuthStore } from '@/stores/authStore';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';
import type { RuntimeStorage } from '@/utils/types/runtimeStorage';

tolerateNaiveStyles();

vi.mock('@/services/RuntimeStorageService', () => ({
  RuntimeStorageService: { remove: vi.fn(), update: vi.fn(), test: vi.fn() },
}));
vi.mock('../StorageAccountPanel.vue', () => ({
  default: { template: '<i />' },
}));
vi.mock('../StorageConnectionForm.vue', () => ({
  default: { template: '<i />' },
}));
vi.mock('../StorageRotateDialog.vue', () => ({
  default: { template: '<i />' },
}));

let wrapper: VueWrapper | undefined;

function storage(extra: Partial<RuntimeStorage> = {}): RuntimeStorage {
  return {
    name: 'archive',
    address: 'https://s3.example.com',
    provider: 'Minio',
    defaultRegion: 'us-east-1',
    isSecure: true,
    httpTimeout: '60s',
    healthCheckInterval: '10s',
    rateLimit: { enabled: false, rpm: 600, includeMetadataApi: false },
    source: 'managed',
    ...extra,
  };
}

function mountSettings(value: RuntimeStorage) {
  wrapper = mount(StorageSettings, {
    props: { storage: value },
    attachTo: document.body,
    global: { plugins: [kitI18n()] },
  });

  return wrapper;
}

function deleteOpener(): HTMLButtonElement {
  const button = Array.from(
    document.body.querySelectorAll<HTMLButtonElement>('button'),
  ).find((item) => item.textContent?.trim() === 'Delete storage');

  if (!button) throw new Error('delete button not found');

  return button;
}

function dialogOpen() {
  return wrapper!.findComponent(ConfirmDialog).props('open');
}

function confirmBtn() {
  return document.body.querySelector<HTMLButtonElement>('[data-kit-confirm]');
}

async function openAndType(name: string) {
  deleteOpener().click();
  await flushPromises();

  const input = document.body.querySelector<HTMLInputElement>(
    '[data-kit-confirm-input] input, input[data-kit-confirm-input]',
  );

  input!.value = name;
  input!.dispatchEvent(new Event('input'));
  await flushPromises();
}

beforeEach(() => {
  setActivePinia(createPinia());
  useAuthStore().authEnabled = false;
  vi.mocked(RuntimeStorageService.remove).mockReset();
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = '';
});

describe('StorageSettings delete flow', () => {
  it('blocks deleting a storage that is in use and names what uses it', async () => {
    mountSettings(storage({ references: ['user:alice', 'replication:r1'] }));
    await flushPromises();

    expect(document.body.textContent).toContain('user:alice');
    expect(document.body.textContent).toContain('replication:r1');
    expect(document.body.textContent).toContain(
      'Remove every reference before deleting this storage.',
    );
    expect(deleteOpener().disabled).toBe(true);
    deleteOpener().click();
    await flushPromises();
    expect(dialogOpen()).toBe(false);
    expect(RuntimeStorageService.remove).not.toHaveBeenCalled();
  });

  it('requires the typed name, then removes once and emits deleted', async () => {
    vi.mocked(RuntimeStorageService.remove).mockResolvedValue(undefined);
    const view = mountSettings(storage());

    await flushPromises();
    await openAndType('wrong');
    expect(confirmBtn()!.disabled).toBe(true);
    await openAndType('archive');
    expect(confirmBtn()!.disabled).toBe(false);
    confirmBtn()!.click();
    await flushPromises();

    expect(RuntimeStorageService.remove).toHaveBeenCalledTimes(1);
    expect(RuntimeStorageService.remove).toHaveBeenCalledWith('archive');
    expect(view.emitted('deleted')).toHaveLength(1);
    expect(dialogOpen()).toBe(false);
  });

  it('does nothing when cancelled', async () => {
    const view = mountSettings(storage());

    await flushPromises();
    await openAndType('archive');
    document.body
      .querySelector<HTMLButtonElement>('[data-kit-cancel]')!
      .click();
    await flushPromises();

    expect(RuntimeStorageService.remove).not.toHaveBeenCalled();
    expect(view.emitted('deleted')).toBeUndefined();
    expect(dialogOpen()).toBe(false);
  });

  it('shows the server error and keeps the dialog open when remove fails', async () => {
    const failure = new AxiosError('boom');

    failure.response = {
      data: { message: 'storage is busy' },
    } as AxiosError['response'];
    vi.mocked(RuntimeStorageService.remove).mockRejectedValue(failure);
    const view = mountSettings(storage());

    await flushPromises();
    await openAndType('archive');
    confirmBtn()!.click();
    await flushPromises();

    expect(RuntimeStorageService.remove).toHaveBeenCalledTimes(1);
    expect(view.emitted('deleted')).toBeUndefined();
    expect(dialogOpen()).toBe(true);
    expect(document.body.textContent).toContain('storage is busy');
  });
});
