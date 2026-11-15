/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { createI18n } from 'vue-i18n';
import { createMemoryHistory, createRouter } from 'vue-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import TuttiRoutingPage from '../TuttiRoutingPage.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import { RouteName } from '@/utils/types/router';

tolerateNaiveStyles();

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18N_DEFAULT_LOCALE: 'en',
  I18nLocale: { EN: 'en', DE: 'de' },
  useNotification: () => ({ createNotification: vi.fn() }),
}));
vi.mock('@/services/TuttiCoreService', () => ({
  TuttiCoreService: { getRoutingPolicies: vi.fn() },
}));

const getRoutingPolicies = vi.mocked(TuttiCoreService.getRoutingPolicies);

function mountPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/routing/add',
        name: RouteName.TUTTI_ADD_ROUTING_POLICY,
        component: { template: '<div />' },
      },
      { path: '/:p(.*)*', component: { template: '<div />' } },
    ],
  });

  return mount(TuttiRoutingPage, {
    global: {
      plugins: [
        createPinia(),
        router,
        createI18n({
          legacy: false,
          locale: 'en',
          messages: {
            en: { [RouteName.TUTTI_ROUTING_RULES]: 'Routing rules' },
          },
        }),
      ],
    },
  });
}

beforeEach(() => {
  setActivePinia(createPinia());
  getRoutingPolicies.mockReset();
});

describe('TuttiRoutingPage', () => {
  it('shows the add action once, in the empty state, when there are no rules', async () => {
    getRoutingPolicies.mockResolvedValue({
      main: 'main',
      userRoutings: [],
      bucketRoutings: [],
    });
    const wrapper = mountPage();

    await flushPromises();

    expect(wrapper.find('[data-kit-state="empty"]').exists()).toBe(true);
    expect(wrapper.text()).toContain('No routing rules yet');
    expect(wrapper.text().match(/Add routing rule/g)).toHaveLength(1);
    expect(wrapper.find('.kit-filter').exists()).toBe(false);
    wrapper.unmount();
  });

  it('lists rules with readable labels and the add action in the header', async () => {
    getRoutingPolicies.mockResolvedValue({
      main: 'main',
      userRoutings: [{ user: 'alice', toStorage: 'main', isBlocked: false }],
      bucketRoutings: [
        { user: 'bob', bucket: 'photos', toStorage: 'backup', isBlocked: true },
      ],
    });
    const wrapper = mountPage();

    await flushPromises();

    const text = wrapper.text();

    expect(text).toContain('alice');
    expect(text).toContain('All buckets');
    expect(text).toContain('photos');
    expect(text).toContain('Allowed');
    expect(text).toContain('Blocked');
    expect(text).toContain('Add routing rule');
    expect(text).not.toMatch(/policy|policies/i);
    wrapper.unmount();
  });

  it('shows an error with retry when the rules cannot be loaded', async () => {
    getRoutingPolicies.mockRejectedValue(new Error('down'));
    const wrapper = mountPage();

    await flushPromises();

    expect(wrapper.find('[data-kit-state="error"]').exists()).toBe(true);
    wrapper.unmount();
  });
});
