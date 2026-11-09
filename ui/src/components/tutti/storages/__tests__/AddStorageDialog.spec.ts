/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { shallowMount, flushPromises } from '@vue/test-utils';
import { NInput } from 'naive-ui';
import { createI18n } from 'vue-i18n';
import { describe, expect, it, vi } from 'vitest';
import AddStorageDialog from '../AddStorageDialog.vue';
import StorageConnectionForm from '../StorageConnectionForm.vue';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';
import { RuntimeStorageHelper } from '@/utils/helpers/RuntimeStorageHelper';

vi.mock('@clyso/clyso-ui-kit', () => ({ I18nLocale: { EN: 'en', DE: 'de' } }));

vi.mock('@/services/RuntimeStorageService', () => ({
  RuntimeStorageService: { test: vi.fn(), create: vi.fn() },
}));

describe('AddStorageDialog', () => {
  it('requires a test of current credentials, invalidates edits and clears secrets on close', async () => {
    vi.mocked(RuntimeStorageService.test).mockResolvedValue({
      ok: true,
      latencyMs: 12,
    });
    const wrapper = shallowMount(AddStorageDialog, {
      props: { show: false, users: ['alice'] },
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
          FormItem: { template: '<div><slot /></div>' },
          NModal: { template: '<div><slot /><slot name="footer" /></div>' },
          NForm: { template: '<form><slot /></form>' },
          NButton: {
            props: ['disabled'],
            template: '<button :disabled="disabled"><slot /></button>',
          },
          NFormItem: { template: '<div><slot /></div>' },
        },
      },
    });

    await wrapper.setProps({ show: true });
    wrapper.findComponent(StorageConnectionForm).vm.$emit('update:modelValue', {
      ...RuntimeStorageHelper.empty(),
      name: 'archive',
      address: 'https://s3.example.com',
    });
    await flushPromises();
    await wrapper
      .findAll('button')
      .find((button) => button.text() === 'Next')!
      .trigger('click');
    const inputs = wrapper.findAllComponents(NInput);

    inputs[0]!.vm.$emit('update:value', 'access');
    inputs[1]!.vm.$emit('update:value', 'secret');
    await flushPromises();
    const addButton = () =>
      wrapper
        .findAll('button')
        .find((button) => button.text() === 'Add storage')!;

    expect(addButton().attributes('disabled')).toBeDefined();
    await wrapper
      .findAll('button')
      .find((button) => button.text() === 'Test connection')!
      .trigger('click');
    await flushPromises();
    expect(addButton().attributes('disabled')).toBeUndefined();
    expect(RuntimeStorageService.test).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'archive' }),
      { user: 'alice', accessKeyId: 'access', secretAccessKey: 'secret' },
    );
    inputs[1]!.vm.$emit('update:value', 'changed');
    await flushPromises();
    expect(addButton().attributes('disabled')).toBeDefined();
    await wrapper.setProps({ show: false });
    await wrapper.setProps({ show: true });
    await wrapper
      .findAll('button')
      .find((button) => button.text() === 'Next')!
      .trigger('click');
    expect(
      wrapper.findAllComponents(NInput).map((input) => input.props('value')),
    ).not.toContain('changed');
    wrapper.unmount();
  });
});
