/* Copyright © 2026 hieunyit. Licensed under the GNU Affero General Public License, Version 3.0. */
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import TuttiUserCreatePage from '../TuttiUserCreatePage.vue';
import { TuttiService } from '@/services/TuttiService';
import { RuntimeStorageService } from '@/services/RuntimeStorageService';
import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

vi.mock('vue-router', async (orig) => ({
  ...(await orig<typeof import('vue-router')>()),
  useRouter: () => ({ push: vi.fn() }),
  useRoute: () => ({ query: {} }),
}));
vi.mock('@/components/tutti/accounts/ProxySecretModal.vue', () => ({
  default: { template: '<i />' },
}));
vi.mock('@/services/TuttiService', () => ({
  TuttiService: { getUsers: vi.fn(), createUser: vi.fn() },
}));
vi.mock('@/services/RuntimeStorageService', () => ({
  RuntimeStorageService: { list: vi.fn() },
}));
vi.mock('@/utils/helpers/BucketDiscoveryHelper', () => ({
  BucketDiscoveryHelper: {
    discoverBuckets: vi.fn().mockResolvedValue([]),
    discoverStorageBuckets: vi.fn().mockResolvedValue([]),
  },
}));

async function mountPage() {
  setActivePinia(createPinia());
  useTuttiProtectionStore().refresh = vi.fn().mockResolvedValue(undefined);

  const wrapper = mount(TuttiUserCreatePage, {
    global: { plugins: [kitI18n()], stubs: { RouterLink: RouterLinkStub } },
  });

  await flushPromises();

  return wrapper;
}

describe('TuttiUserCreatePage', () => {
  beforeEach(() => {
    window.localStorage.clear();
    window.sessionStorage.clear();
    vi.mocked(TuttiService.getUsers).mockResolvedValue({
      users: [],
      dynamicCredentialsEnabled: true,
    } as never);
    vi.mocked(RuntimeStorageService.list).mockResolvedValue({
      storages: [],
    } as never);
    vi.mocked(TuttiService.createUser).mockReset();
  });

  it('is one form with labelled fields, not a wizard', async () => {
    const wrapper = await mountPage();

    expect(wrapper.text()).not.toContain('Next');
    expect(wrapper.text()).toContain('Name');
    expect(wrapper.text()).toContain('Permission');
  });

  it('reports a missing name only after a create attempt, and does not submit', async () => {
    const wrapper = await mountPage();

    expect(wrapper.text()).not.toContain('Enter a name.');
    await wrapper.get('form').trigger('submit');
    await flushPromises();
    expect(wrapper.text()).toContain('Enter a name.');
    expect(TuttiService.createUser).not.toHaveBeenCalled();
  });
});
