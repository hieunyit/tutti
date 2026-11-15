/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { createI18n } from 'vue-i18n';
import { createMemoryHistory, createRouter } from 'vue-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import TuttiAddRoutingPolicyPage from '../TuttiAddRoutingPolicyPage.vue';
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
  TuttiCoreService: { getStorages: vi.fn(), addRoutingPolicy: vi.fn() },
}));
vi.mock('@/services/TuttiService', () => ({
  TuttiService: { getUsers: vi.fn().mockResolvedValue({ users: [] }) },
}));
vi.mock('@/utils/helpers/BucketDiscoveryHelper', () => ({
  BucketDiscoveryHelper: { discoverBuckets: vi.fn().mockResolvedValue([]) },
}));

function mountPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/routing',
        name: RouteName.TUTTI_ROUTING_RULES,
        component: { template: '<div />' },
      },
      { path: '/:p(.*)*', component: { template: '<div />' } },
    ],
  });

  return mount(TuttiAddRoutingPolicyPage, {
    global: {
      plugins: [
        createPinia(),
        router,
        createI18n({ legacy: false, locale: 'en', messages: {} }),
      ],
    },
  });
}

beforeEach(() => {
  setActivePinia(createPinia());
  sessionStorage.clear();
  vi.mocked(TuttiCoreService.getStorages).mockResolvedValue({
    storages: [
      { name: 'main', isMain: true, credentials: [{ alias: 'alice' }] },
    ],
  } as never);
  vi.mocked(TuttiCoreService.addRoutingPolicy).mockReset();
});

describe('TuttiAddRoutingPolicyPage', () => {
  it('is one form with visible labels, not a wizard, and never says policy', async () => {
    const wrapper = mountPage();

    await flushPromises();

    const text = wrapper.text();

    expect(text).toContain('User');
    expect(text).toContain('All buckets, including future ones');
    expect(text).toContain('Send to a storage');
    expect(text).toContain('Create routing rule');
    expect(text).not.toContain('Start over');
    expect(text).not.toMatch(/Step \d of/);
    expect(text).not.toMatch(/policy|policies/i);
    wrapper.unmount();
  });

  it('asks for a user instead of sending an incomplete rule', async () => {
    const wrapper = mountPage();

    await flushPromises();
    await wrapper.get('form').trigger('submit');
    await flushPromises();

    expect(wrapper.text()).toContain('Choose a user.');
    expect(TuttiCoreService.addRoutingPolicy).not.toHaveBeenCalled();
    wrapper.unmount();
  });
});
