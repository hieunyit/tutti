/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { createI18n } from 'vue-i18n';
import { createMemoryHistory, createRouter } from 'vue-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import TuttiStoragesPage from '../TuttiStoragesPage.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import { RouteName } from '@/utils/types/router';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';

tolerateNaiveStyles();

vi.mock('@/utils/helpers/DateTimeHelper', () => ({
  EMPTY_DATE_TEXT: '–',
  formatDateTime: () => 'now',
  formatRelative: () => 'now',
}));
vi.mock('@/components/tutti/monitoring/useNumberFormat', () => ({
  useNumberFormat: () => (value: number) => String(value),
}));
vi.mock('@/components/tutti/storages/AddStorageDialog.vue', () => ({
  default: { template: '<div />' },
}));
vi.mock('@/services/RuntimeStorageService', () => ({
  RuntimeStorageService: { list: vi.fn() },
}));
vi.mock('@/services/ChorusService', () => ({
  ChorusService: {
    getReplications: vi.fn().mockResolvedValue({ replications: [] }),
    getRoutingPolicies: vi
      .fn()
      .mockResolvedValue({ userRoutings: [], bucketRoutings: [] }),
  },
}));
vi.mock('@/services/TuttiService', () => ({
  TuttiService: {
    getUsers: vi.fn().mockResolvedValue({ users: [] }),
    getProxies: vi.fn().mockResolvedValue({ proxies: [] }),
    getFailoverPolicies: vi.fn().mockResolvedValue({ policies: [] }),
    getFailoverEvents: vi.fn().mockResolvedValue({ events: [] }),
  },
}));

function mountPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/storages/:storageName',
        name: RouteName.CHORUS_STORAGE_DETAILS,
        component: { template: '<div />' },
      },
      { path: '/:p(.*)*', component: { template: '<div />' } },
    ],
  });

  return mount(TuttiStoragesPage, {
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
  vi.mocked(RuntimeStorageService.list).mockReset();
});

describe('TuttiStoragesPage', () => {
  it('shows a loading skeleton while the request is pending', async () => {
    vi.mocked(RuntimeStorageService.list).mockReturnValue(
      new Promise(() => {}),
    );
    const wrapper = mountPage();

    await flushPromises();
    expect(wrapper.find('[data-kit-state="loading"]').exists()).toBe(true);
    wrapper.unmount();
  });

  it('shows an empty state with an add-storage action', async () => {
    vi.mocked(RuntimeStorageService.list).mockResolvedValue({
      managedStoragesEnabled: true,
      mainStorage: '',
      storages: [],
    });
    const wrapper = mountPage();

    await flushPromises();
    expect(wrapper.find('[data-kit-state="empty"]').exists()).toBe(true);
    expect(wrapper.text()).toContain('Add storage');
    wrapper.unmount();
  });

  it('shows an error with a working retry', async () => {
    vi.mocked(RuntimeStorageService.list).mockRejectedValueOnce(
      new Error('down'),
    );
    const wrapper = mountPage();

    await flushPromises();
    expect(wrapper.find('[data-kit-state="error"]').exists()).toBe(true);

    vi.mocked(RuntimeStorageService.list).mockResolvedValueOnce({
      managedStoragesEnabled: true,
      mainStorage: 'main',
      storages: [
        {
          name: 'main',
          address: 'https://s3.example.com',
          provider: 'Minio',
          defaultRegion: '',
          isSecure: true,
          httpTimeout: '30s',
          healthCheckInterval: '10s',
          source: 'managed',
        },
      ],
    });
    await wrapper.get('[data-kit-state="error"] button').trigger('click');
    await flushPromises();
    expect(wrapper.text()).toContain('main');
    wrapper.unmount();
  });

  it('lists storages with their health when loaded', async () => {
    vi.mocked(RuntimeStorageService.list).mockResolvedValue({
      managedStoragesEnabled: true,
      mainStorage: 'main',
      storages: [
        {
          name: 'archive',
          address: 'https://archive.example.com',
          provider: 'Ceph',
          defaultRegion: '',
          isSecure: true,
          httpTimeout: '30s',
          healthCheckInterval: '10s',
          source: 'config',
        },
      ],
    });
    const wrapper = mountPage();

    await flushPromises();
    expect(wrapper.text()).toContain('archive');
    expect(wrapper.text()).toContain('Unknown');
    wrapper.unmount();
  });
});
