import { shallowMount } from '@vue/test-utils';
import { NButton } from 'naive-ui';
import { createI18n } from 'vue-i18n';
import { describe, expect, it, vi } from 'vitest';
import StorageAccountForm from '../StorageAccountForm.vue';
import { StorageTestHelper } from '@/utils/helpers/StorageTestHelper';

vi.mock('@clyso/clyso-ui-kit', () => ({ I18nLocale: { EN: 'en', DE: 'de' } }));

describe('Storage account whitespace', () => {
  it('warns without silently changing credentials, and trims only on request', async () => {
    const wrapper = shallowMount(StorageAccountForm, {
      props: {
        account: { accessKeyId: ' key ', secretAccessKey: ' secret\n' },
        options: StorageTestHelper.emptyOptions(),
        provider: 'Ceph',
      },
      global: {
        plugins: [createI18n({ legacy: false, locale: 'en' })],
        renderStubDefaultSlot: true,
        stubs: {
          NButton: { template: '<button><slot /></button>' },
          Callout: { template: '<div><slot /></div>' },
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    });

    expect(wrapper.text()).toContain('leading/trailing whitespace');
    expect(wrapper.emitted('update:account')).toBeUndefined();
    await wrapper
      .findAllComponents(NButton)
      .find((button) => button.text() === 'Remove surrounding whitespace')!
      .vm.$emit('click');
    expect(wrapper.emitted('update:account')).toEqual([
      [{ accessKeyId: 'key', secretAccessKey: 'secret' }],
    ]);
    wrapper.unmount();
  });
});
