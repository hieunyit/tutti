import { createPinia, setActivePinia } from 'pinia';
import { mount } from '@vue/test-utils';
import { createI18n } from 'vue-i18n';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import AppQuickSearch from '../AppQuickSearch.vue';
import { useAuthStore } from '@/stores/authStore';
import routes from '@/i18n/messages/routes';
import { AuthRole } from '@/utils/types/auth';

vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }));
// Exercise our search component without importing the UI kit's bundled CSS in Node.
vi.mock('@clyso/clyso-ui-kit', () => ({ I18nLocale: { EN: 'en', DE: 'de' } }));

beforeEach(() => {
  setActivePinia(createPinia());
});

describe('quick search permissions', () => {
  it('hides admin-only items within a shared navigation group for viewers', async () => {
    const auth = useAuthStore();

    auth.authEnabled = true;
    auth.role = AuthRole.VIEWER;
    const wrapper = mount(AppQuickSearch, {
      global: {
        plugins: [
          createI18n({ legacy: false, locale: 'en', messages: routes }),
        ],
      },
    });

    await wrapper.get('input').setValue('Console');
    expect(wrapper.findAll('[role="option"]')).toHaveLength(0);
    auth.role = AuthRole.ADMIN;
    await wrapper.get('input').setValue('Console');
    expect(wrapper.findAll('[role="option"]')).toHaveLength(1);
    wrapper.unmount();
  });
});

describe('quick search entries', () => {
  function mountSearch() {
    return mount(AppQuickSearch, {
      attachTo: document.body,
      global: {
        plugins: [
          createI18n({ legacy: false, locale: 'en', messages: routes }),
        ],
      },
    });
  }

  it('hides System settings and Audit log from viewers', async () => {
    const auth = useAuthStore();

    auth.authEnabled = true;
    auth.role = AuthRole.VIEWER;
    const wrapper = mountSearch();

    for (const query of ['System settings', 'Audit log']) {
      await wrapper.get('input').setValue(query);
      expect(wrapper.findAll('[role="option"]')).toHaveLength(0);
    }

    wrapper.unmount();
  });

  it('offers the docs pages from the Help menu', async () => {
    const wrapper = mountSearch();

    await wrapper.get('input').setValue('API reference');
    expect(wrapper.findAll('[role="option"]')).toHaveLength(1);
    wrapper.unmount();
  });

  it('focuses on Ctrl+K', async () => {
    const wrapper = mountSearch();

    document.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }),
    );
    await new Promise((resolve) => setTimeout(resolve));
    expect(document.activeElement).toBe(wrapper.get('input').element);
    wrapper.unmount();
  });
});
