/*
 * Copyright © 2026 hieunyit
 *
 *  Licensed under the GNU Affero General Public License, Version 3.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  https://www.gnu.org/licenses/agpl-3.0.html
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createI18n } from 'vue-i18n';
import { createMemoryHistory, createRouter } from 'vue-router';
import TuttiSystemSettingsPage from '../TuttiSystemSettingsPage.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import { TuttiService } from '@/services/TuttiService';
import { useAuthStore } from '@/stores/authStore';
import type { TuttiSystemSettings } from '@/utils/types/tutti';

tolerateNaiveStyles();

vi.mock('@/utils/helpers/DateTimeHelper', () => ({
  formatDateTime: (v: string) => v,
  formatRelative: (v: string) => v,
}));

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
}));

vi.mock('@/services/TuttiService', () => ({
  TuttiService: {
    getSystemSettings: vi.fn(),
    updateSystemSettings: vi.fn(),
  },
}));

const settings: TuttiSystemSettings = {
  version: 3,
  updatedAt: '2026-11-11T20:00:00Z',
  features: {
    tagging: true,
    acl: false,
    lifecycle: true,
    policy: false,
    preserveAclGrants: false,
    versioning: false,
  },
  worker: {
    concurrency: 16,
    shutdownTimeout: '5s',
    queueUpdateInterval: '3s',
    taskCheckInterval: '1s',
    delayedTaskCheckInterval: '3s',
    swiftRetryInterval: '1m',
    pauseRetryInterval: '30s',
    switchRetryInterval: '30s',
    backlogHighWatermark: 10000,
    backlogLowWatermark: 5000,
    bandwidthLimitBytes: '2097152',
    bufferBudgetBytes: '268435456',
  },
  lock: { overlap: '2s' },
  webhook: { enabled: false, baseUrl: '' },
  hardware: {
    cpu: 2,
    cpuFractional: 1.5,
    memoryBytes: '2147483648',
    source: 'cgroup-v2',
    memoryLimited: true,
  },
  recommended: {
    concurrency: 20,
    bufferBudgetBytes: '536870912',
    multipartThreads: 1,
    bulkConcurrencyPercent: 20,
  },
  effective: { multipartThreads: 1, bulkConcurrencyPercent: 20 },
  tuningSources: {
    concurrency: 'explicit',
    bufferBudgetBytes: 'explicit',
    bulkConcurrencyPercent: 'auto',
    multipartThreads: 'auto',
    multipartPartSizeMib: 'auto',
  },
};

async function mountPage(tab?: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/system-settings', component: { template: '<div/>' } }],
  });

  await router.push(tab ? `/system-settings?tab=${tab}` : '/system-settings');
  await router.isReady();

  const wrapper = mount(TuttiSystemSettingsPage, {
    attachTo: document.body,
    global: {
      plugins: [
        router,
        createI18n({ legacy: false, locale: 'en', messages: {} }),
      ],
    },
  });

  await flushPromises();

  return wrapper;
}

function saveButton(wrapper: Awaited<ReturnType<typeof mountPage>>) {
  return wrapper.get('[data-test="settings-save"]');
}

describe('TuttiSystemSettingsPage', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
    setActivePinia(createPinia());
    useAuthStore().authEnabled = false;
    vi.clearAllMocks();
    vi.mocked(TuttiService.getSystemSettings).mockResolvedValue(settings);
  });

  it('disables save until the draft is dirty', async () => {
    const wrapper = await mountPage();

    expect(saveButton(wrapper).attributes('disabled')).toBeDefined();

    await wrapper.get('[role="switch"]').trigger('click');
    await flushPromises();

    expect(saveButton(wrapper).attributes('disabled')).toBeUndefined();
    wrapper.unmount();
  });

  it('labels startup-only fields as needing a worker restart', async () => {
    const wrapper = await mountPage('worker');
    const labels = wrapper.findAll('[data-test="restart-label"]');

    // concurrency + 7 durations
    expect(labels.length).toBeGreaterThanOrEqual(8);
    expect(labels[0]!.text()).toMatch(/restart/i);
    wrapper.unmount();
  });

  it('badges the tuning fields and fills recommendations only on request', async () => {
    const wrapper = await mountPage('worker');

    expect(wrapper.findAll('[data-test="live-label"]')).toHaveLength(2);
    expect(wrapper.find('#settings-bulkConcurrencyPercent').exists()).toBe(
      true,
    );
    expect(saveButton(wrapper).attributes('disabled')).toBeDefined();

    // detected hardware (container limits) and the Auto badge on derived knobs
    expect(wrapper.get('[data-test="detected-hardware"]').text()).toContain(
      'cgroup-v2',
    );
    expect(wrapper.findAll('[data-test="auto-label"]')).toHaveLength(3);

    await wrapper.get('[data-test="fill-multipartThreads"]').trigger('click');
    await flushPromises();

    expect(saveButton(wrapper).attributes('disabled')).toBeUndefined();
    expect(
      (wrapper.get('#settings-multipartThreads').element as HTMLInputElement)
        .value,
    ).toBe('1');
    wrapper.unmount();
  });

  it('shows a reload/overwrite dialog on a version conflict', async () => {
    const wrapper = await mountPage();

    vi.mocked(TuttiService.updateSystemSettings).mockRejectedValueOnce(
      Object.assign(new Error('conflict'), {
        isAxiosError: true,
        response: { status: 409, data: { message: 'stale' } },
      }),
    );

    await wrapper.get('[role="switch"]').trigger('click');
    await flushPromises();
    await saveButton(wrapper).trigger('click');
    await flushPromises();
    // diff summary dialog: confirm the save
    document.body
      .querySelector<HTMLButtonElement>('[data-kit-confirm]')!
      .click();
    await flushPromises();

    const conflict = document.body.querySelector('[data-test="conflict"]');

    expect(conflict).not.toBeNull();
    expect(
      document.body.querySelector('[data-test="conflict-reload"]'),
    ).not.toBeNull();
    expect(
      document.body.querySelector('[data-test="conflict-overwrite"]'),
    ).not.toBeNull();
    wrapper.unmount();
  });

  it('shows an error block with retry when loading fails', async () => {
    vi.mocked(TuttiService.getSystemSettings).mockRejectedValueOnce(
      new Error('down'),
    );

    const wrapper = await mountPage();

    expect(wrapper.find('[role="alert"]').exists()).toBe(true);

    vi.mocked(TuttiService.getSystemSettings).mockResolvedValueOnce(settings);
    await wrapper.get('[role="alert"] button').trigger('click');
    await flushPromises();

    expect(wrapper.find('[role="switch"]').exists()).toBe(true);
    wrapper.unmount();
  });
});
