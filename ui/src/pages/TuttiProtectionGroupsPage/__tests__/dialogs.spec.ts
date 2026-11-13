import { createPinia, setActivePinia } from 'pinia';
import { mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { defineComponent } from 'vue';
import Page from '../TuttiProtectionGroupsPage.vue';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';
import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
import { useAuthStore } from '@/stores/authStore';
import { AuthRole } from '@/utils/types/auth';

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
  RouterLink: defineComponent({ template: '<a><slot /></a>' }),
}));
vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
  I18N_DEFAULT_LOCALE: 'en',
}));
tolerateNaiveStyles();

beforeEach(() => {
  setActivePinia(createPinia());
  const auth = useAuthStore();

  auth.authEnabled = true;
  auth.role = AuthRole.ADMIN;
  const store = useTuttiProtectionStore();

  store.groups = [];
  store.isLoaded = true;
  store.automaticFrozen = false;
  vi.spyOn(store, 'startPolling').mockImplementation(() => {});
  vi.spyOn(store, 'stopPolling').mockImplementation(() => {});
});

function page() {
  return mount(Page, {
    global: {
      plugins: [kitI18n()],
      stubs: {
        ProtectionGroupWizard: defineComponent({
          props: ['show'],
          template: '<div v-if="show" role="dialog">Create group wizard</div>',
        }),
        ConfirmDialog: defineComponent({
          props: ['open'],
          template:
            '<div v-if="open" role="alertdialog">Confirm failover</div>',
        }),
      },
    },
  });
}

describe('Protection groups actions with an empty list', () => {
  it('opens the create wizard from the header and empty state', async () => {
    const wrapper = page();
    const buttons = wrapper
      .findAll('button')
      .filter((button) => button.text().includes('Create group'));

    expect(buttons.length).toBeGreaterThan(0);
    await buttons[0]!.trigger('click');
    expect(wrapper.get('[role="dialog"]').text()).toBe('Create group wizard');
    wrapper.unmount();
  });

  it('opens automatic failover confirmation even when there are no groups', async () => {
    const wrapper = page();

    await wrapper.get('[role="switch"]').trigger('click');
    expect(wrapper.get('[role="alertdialog"]').text()).toBe('Confirm failover');
    wrapper.unmount();
  });
});
