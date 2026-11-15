/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { createI18n } from 'vue-i18n';
import { createMemoryHistory, createRouter } from 'vue-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import TuttiAddDiffReportPage from '../TuttiAddDiffReportPage.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import { useTuttiAddDiffReportStore } from '@/stores/tuttiAddDiffReportStore';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import { RouteName } from '@/utils/types/router';

tolerateNaiveStyles();

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18N_DEFAULT_LOCALE: 'en',
  I18nLocale: { EN: 'en', DE: 'de' },
  useNotification: () => ({ createNotification: vi.fn() }),
}));
vi.mock('@/services/TuttiCoreService', () => ({
  TuttiCoreService: { getStorages: vi.fn(), addDiffReport: vi.fn() },
}));

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

  return mount(TuttiAddDiffReportPage, {
    global: {
      plugins: [
        createPinia(),
        router,
        createI18n({ legacy: false, locale: 'en', messages: {} }),
      ],
    },
  });
}

const storage = (name: string, isMain: boolean) => ({
  name,
  isMain,
  credentials: [{ alias: 'alice' }],
});

beforeEach(() => {
  setActivePinia(createPinia());
  sessionStorage.clear();
  vi.mocked(TuttiCoreService.getStorages).mockResolvedValue({
    storages: [storage('main', true), storage('backup', false)],
  } as never);
});

describe('TuttiAddDiffReportPage', () => {
  it('has two steps and starts with source and destination', async () => {
    const wrapper = mountPage();

    await flushPromises();

    const text = wrapper.text();

    expect(text).toContain('Step 1 of 2');
    expect(text).toContain('Source storage');
    expect(text).toContain('Destination bucket');
    expect(text).not.toContain('Start over');
    wrapper.unmount();
  });

  it('preselects the only other storage and the only fitting user', async () => {
    const wrapper = mountPage();

    await flushPromises();

    const store = useTuttiAddDiffReportStore();

    expect(store.fromStorage?.name).toBe('main');
    expect(store.toStorage?.name).toBe('backup');
    expect(store.selectedUser).toBe('alice');
    wrapper.unmount();
  });

  it('explains what to do when fewer than two storages exist', async () => {
    vi.mocked(TuttiCoreService.getStorages).mockResolvedValue({
      storages: [storage('main', true)],
    } as never);
    const wrapper = mountPage();

    await flushPromises();

    expect(wrapper.find('[data-kit-state="error"]').exists()).toBe(true);
    expect(wrapper.text()).toContain('needs two storages');
    wrapper.unmount();
  });
});
