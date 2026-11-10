/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { shallowMount, flushPromises } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import AddStorageDialog from '../AddStorageDialog.vue';
import StorageAccountForm from '../StorageAccountForm.vue';
import StorageConnectionForm from '../StorageConnectionForm.vue';
import StorageTestSteps from '../StorageTestSteps.vue';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';
import { RuntimeStorageHelper } from '@/utils/helpers/RuntimeStorageHelper';
import type { StorageTestResult } from '@/utils/types/runtimeStorage';

vi.mock('@clyso/clyso-ui-kit', () => ({ I18nLocale: { EN: 'en', DE: 'de' } }));

vi.mock('@/utils/helpers/DateTimeHelper', () => ({
  formatDateTime: () => 'now',
}));

vi.mock('@/services/RuntimeStorageService', () => ({
  RuntimeStorageService: { test: vi.fn(), create: vi.fn() },
}));

const passing: StorageTestResult = {
  ok: true,
  steps: [{ id: 'auth', number: 5, name: 'Authentication', status: 'pass' }],
};
const failing: StorageTestResult = {
  ok: false,
  steps: [
    {
      id: 'write',
      number: 9,
      name: 'Write',
      status: 'fail',
      missingPermission: 's3:PutObject',
    },
  ],
};

function mountDialog() {
  return shallowMount(AddStorageDialog, {
    props: { show: false },
    global: {
      renderStubDefaultSlot: true,
      plugins: [createI18n({ legacy: false, locale: 'en' })],
      stubs: {
        Modal: { template: '<div><slot /><slot name="footer" /></div>' },
        Form: { template: '<form><slot /></form>' },
        Button: {
          props: ['disabled'],
          template: '<button :disabled="disabled"><slot /></button>',
        },
        NModal: { template: '<div><slot /><slot name="footer" /></div>' },
        NForm: { template: '<form><slot /></form>' },
        NButton: {
          props: ['disabled'],
          template: '<button :disabled="disabled"><slot /></button>',
        },
      },
    },
  });
}

async function walkToTest(wrapper: ReturnType<typeof mountDialog>) {
  const click = async (label: string) => {
    await wrapper
      .findAll('button')
      .find((button) => button.text() === label)!
      .trigger('click');
    await flushPromises();
  };

  await wrapper.setProps({ show: true });
  wrapper.findComponent(StorageConnectionForm).vm.$emit('update:modelValue', {
    ...RuntimeStorageHelper.empty(),
    name: 'archive',
    address: 'https://s3.example.com',
  });
  await flushPromises();
  await click('Next');
  wrapper.findComponent(StorageAccountForm).vm.$emit('update:account', {
    accessKeyId: 'tutti-svc',
    secretAccessKey: 's3cret',
  });
  await flushPromises();
  await click('Next');

  return click;
}

describe('AddStorageDialog', () => {
  beforeEach(() => vi.clearAllMocks());

  it('tests the unsaved storage with the connection account and saves without force', async () => {
    vi.mocked(RuntimeStorageService.test).mockResolvedValue(passing);
    vi.mocked(RuntimeStorageService.create).mockResolvedValue({
      ...RuntimeStorageHelper.empty(),
      name: 'archive',
    });
    const wrapper = mountDialog();
    const click = await walkToTest(wrapper);

    expect(RuntimeStorageService.test).toHaveBeenCalledWith(
      expect.objectContaining({
        storage: expect.objectContaining({ name: 'archive' }),
        connectionAccount: {
          accessKeyId: 'tutti-svc',
          secretAccessKey: 's3cret',
        },
      }),
    );
    expect(wrapper.findComponent(StorageTestSteps).props('result')).toEqual(
      passing,
    );
    await click('Next');
    await click('Add storage');
    expect(RuntimeStorageService.create).toHaveBeenCalledWith(
      expect.objectContaining({
        force: false,
        connectionAccount: expect.objectContaining({
          accessKeyId: 'tutti-svc',
        }),
      }),
    );
    expect(wrapper.emitted('created')?.[0]).toEqual(['archive']);
  });

  it('blocks saving while steps fail until "save anyway" is confirmed', async () => {
    vi.mocked(RuntimeStorageService.test).mockResolvedValue(failing);
    const wrapper = mountDialog();
    const click = await walkToTest(wrapper);

    await click('Next');

    const add = () =>
      wrapper
        .findAll('button')
        .find((button) => button.text() === 'Add storage')!;

    expect(add().attributes('disabled')).toBeDefined();
    await click('Add storage');
    expect(RuntimeStorageService.create).not.toHaveBeenCalled();
  });

  it('forgets the secret after the dialog closed', async () => {
    vi.mocked(RuntimeStorageService.test).mockResolvedValue(passing);
    const wrapper = mountDialog();

    await walkToTest(wrapper);
    await wrapper.setProps({ show: false });
    await flushPromises();
    await wrapper.setProps({ show: true });
    await flushPromises();
    expect(wrapper.findComponent(StorageConnectionForm).exists()).toBe(true);
    wrapper.unmount();
  });
});
