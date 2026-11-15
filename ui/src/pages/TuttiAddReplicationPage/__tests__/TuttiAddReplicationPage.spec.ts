/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { createI18n } from 'vue-i18n';
import { createMemoryHistory, createRouter } from 'vue-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import TuttiAddReplicationPage from '../TuttiAddReplicationPage.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import { RouteName } from '@/utils/types/router';

tolerateNaiveStyles();

vi.mock('@/services/TuttiCoreService', () => ({
  TuttiCoreService: {
    getStorages: vi.fn(),
    getBucketsForReplication: vi.fn(),
    addReplication: vi.fn(),
  },
}));
vi.mock('@clyso/clyso-ui-kit', () => ({
  I18N_DEFAULT_LOCALE: 'en',
  I18nLocale: { EN: 'en', DE: 'de' },
  useNotification: () => ({ createNotification: vi.fn() }),
}));

const getStorages = vi.mocked(TuttiCoreService.getStorages);

function mountPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/replication',
        name: RouteName.TUTTI_REPLICATION,
        component: { template: '<div />' },
      },
      { path: '/:p(.*)*', component: { template: '<div />' } },
    ],
  });

  return mount(TuttiAddReplicationPage, {
    global: {
      plugins: [
        createPinia(),
        router,
        createI18n({ legacy: false, locale: 'en', messages: {} }),
      ],
      stubs: { TuttiWizardPage: false },
    },
  });
}

beforeEach(() => {
  setActivePinia(createPinia());
  sessionStorage.clear();
  getStorages.mockReset();
});

describe('TuttiAddReplicationPage', () => {
  it('starts with source and destination on one step, without a user step or Start over', async () => {
    getStorages.mockResolvedValue({
      storages: [
        { name: 'main', isMain: true, credentials: [] },
        { name: 'backup', isMain: false, credentials: [] },
      ],
    } as never);
    const wrapper = mountPage();

    await flushPromises();

    const text = wrapper.text();

    expect(text).toContain('Source storage');
    expect(text).toContain('Destination storage');
    expect(text).toContain('Step 1 of 3');
    expect(text).not.toContain('Start over');
    expect(text).not.toMatch(/Select the user|identity/i);
    wrapper.unmount();
  });

  it('explains what to do when fewer than two storages exist', async () => {
    getStorages.mockResolvedValue({
      storages: [{ name: 'main', isMain: true, credentials: [] }],
    } as never);
    const wrapper = mountPage();

    await flushPromises();

    expect(wrapper.find('[data-kit-state="error"]').exists()).toBe(true);
    expect(wrapper.text()).toContain('needs two storages');
    wrapper.unmount();
  });
});
