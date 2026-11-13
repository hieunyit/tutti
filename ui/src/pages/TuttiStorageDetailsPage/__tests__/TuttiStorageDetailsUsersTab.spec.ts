/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  flushPromises,
  mount,
  RouterLinkStub,
  type VueWrapper,
} from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { reactive } from 'vue';
import TuttiStorageDetailsPage from '../TuttiStorageDetailsPage.vue';
import { useTuttiStoragesStore } from '@/stores/tuttiStoragesStore';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';
import type { RuntimeStorage } from '@/utils/types/runtimeStorage';

tolerateNaiveStyles();

const route = vi.hoisted(() => ({ query: {} as Record<string, string> }));
const routeState = vi.hoisted(() => ({ current: null as unknown }));

vi.mock('vue-router', async (orig) => ({
  ...(await orig<typeof import('vue-router')>()),
  useRoute: () => routeState.current,
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));
vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
  I18N_DEFAULT_LOCALE: 'en',
}));
vi.mock('@/components/tutti/storages/StorageSettings.vue', () => ({
  default: { template: '<i />' },
}));
vi.mock('@/components/tutti/storages/StorageOverviewTab.vue', () => ({
  default: { template: '<i />' },
}));
vi.mock('@/components/tutti/storages/StorageHealthTab.vue', () => ({
  default: { template: '<i />' },
}));
vi.mock('@/services/RuntimeStorageService', () => ({
  RuntimeStorageService: { list: vi.fn(), listSummary: vi.fn() },
}));

let wrapper: VueWrapper | undefined;

const archive: RuntimeStorage = {
  name: 'archive',
  address: 'https://s3.example.com',
  provider: 'Minio',
  defaultRegion: '',
  isSecure: true,
  httpTimeout: '60s',
  healthCheckInterval: '10s',
  rateLimit: { enabled: false, rpm: 600, includeMetadataApi: false },
  source: 'managed',
  userCount: 1,
};

function setup() {
  const store = useTuttiStoragesStore();

  store.isLoaded = true;
  store.runtimeStorages = [archive];
  vi.spyOn(store, 'startPolling').mockImplementation(() => {});
  vi.spyOn(store, 'stopPolling').mockImplementation(() => {});
  const spy = vi.spyOn(store, 'loadStorageUsers').mockResolvedValue();

  return { store, spy };
}

function mountPage() {
  wrapper = mount(TuttiStorageDetailsPage, {
    props: { storageName: 'archive' },
    attachTo: document.body,
    global: { plugins: [kitI18n()], stubs: { RouterLink: RouterLinkStub } },
  });

  return wrapper;
}

beforeEach(() => {
  setActivePinia(createPinia());
  route.query = {};
  routeState.current = reactive(route);
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = '';
});

describe('storage detail page users tab', () => {
  it('does not load the full user list while another tab is open', async () => {
    const { spy } = setup();

    mountPage();
    await flushPromises();

    expect(spy).not.toHaveBeenCalled();
  });

  it('loads the users of this storage once when the tab opens', async () => {
    const { spy } = setup();

    mountPage();
    await flushPromises();
    (routeState.current as typeof route).query = { tab: 'users' };
    await flushPromises();

    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy).toHaveBeenCalledWith('archive');
  });

  it('loads at once when the page opens directly on the users tab', async () => {
    route.query = { tab: 'users' };
    const { spy } = setup();

    mountPage();
    await flushPromises();

    expect(spy).toHaveBeenCalledTimes(1);
  });

  it('renders the loaded users', async () => {
    route.query = { tab: 'users' };
    const { store } = setup();

    store.storageUsers = {
      archive: [{ user: 'alice', accessKeyId: 'AKIA1', source: 'managed' }],
    };
    mountPage();
    await flushPromises();

    expect(document.body.textContent).toContain('alice');
    expect(document.body.textContent).toContain('AKIA1');
  });

  it('swallows a failed users load instead of breaking the page', async () => {
    route.query = { tab: 'users' };
    const { spy } = setup();

    spy.mockRejectedValue(new Error('boom'));
    mountPage();
    await flushPromises();

    expect(spy).toHaveBeenCalledTimes(1);
    expect(wrapper!.exists()).toBe(true);
  });
});
