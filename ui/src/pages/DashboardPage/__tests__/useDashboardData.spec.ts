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

import { createPinia, setActivePinia } from 'pinia';
import { defineComponent, nextTick } from 'vue';
import { mount } from '@vue/test-utils';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useDashboardData } from '../useDashboardData';
import { ChorusService } from '@/services/ChorusService';
import { TuttiService } from '@/services/TuttiService';
import { useTuttiDashboardStore } from '@/stores/tuttiDashboardStore';

vi.mock('@/services/ChorusService', () => ({
  ChorusService: { getStorages: vi.fn(), getReplications: vi.fn() },
}));
vi.mock('@/services/TuttiService', () => ({
  TuttiService: {
    getProxies: vi.fn(),
    getFailoverEvents: vi.fn(),
    getFailoverPolicies: vi.fn(),
  },
}));

function mountComposable() {
  let result!: ReturnType<typeof useDashboardData>;
  const wrapper = mount(
    defineComponent({
      setup() {
        result = useDashboardData();

        return () => null;
      },
    }),
  );

  return { wrapper, result };
}

async function settle() {
  await vi.advanceTimersByTimeAsync(0);
  await nextTick();
}

beforeEach(() => {
  setActivePinia(createPinia());
  vi.resetAllMocks();
  vi.useFakeTimers();
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false);
});

afterEach(() => {
  useTuttiDashboardStore().stopPolling();
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe('useDashboardData', () => {
  it('is loading, then ready when the store delivers data', async () => {
    vi.mocked(ChorusService.getStorages).mockResolvedValue({ storages: [] });
    vi.mocked(ChorusService.getReplications).mockResolvedValue({
      replications: [],
    });
    vi.mocked(TuttiService.getProxies).mockResolvedValue({ proxies: [] });
    vi.mocked(TuttiService.getFailoverEvents).mockResolvedValue({ events: [] });
    const { wrapper, result } = mountComposable();

    expect(result.state.value).toBe('loading');
    await settle();
    expect(result.state.value).toBe('ready');
    expect(result.updatedAt.value).toBeInstanceOf(Date);
    wrapper.unmount();
  });

  it('is error when every request rejects, and recovers on refresh', async () => {
    vi.mocked(ChorusService.getStorages).mockRejectedValue(new Error('down'));
    vi.mocked(ChorusService.getReplications).mockRejectedValue(
      new Error('down'),
    );
    vi.mocked(TuttiService.getProxies).mockRejectedValue(new Error('down'));
    vi.mocked(TuttiService.getFailoverEvents).mockRejectedValue(
      new Error('down'),
    );
    const { wrapper, result } = mountComposable();

    await settle();
    expect(result.state.value).toBe('error');

    vi.mocked(ChorusService.getStorages).mockResolvedValue({ storages: [] });
    await result.refresh();
    await settle();
    expect(result.state.value).toBe('ready');
    wrapper.unmount();
  });

  it('stops polling when the component unmounts', async () => {
    vi.mocked(ChorusService.getStorages).mockResolvedValue({ storages: [] });
    vi.mocked(ChorusService.getReplications).mockResolvedValue({
      replications: [],
    });
    vi.mocked(TuttiService.getProxies).mockResolvedValue({ proxies: [] });
    vi.mocked(TuttiService.getFailoverEvents).mockResolvedValue({ events: [] });
    const { wrapper } = mountComposable();

    await settle();
    wrapper.unmount();
    await vi.advanceTimersByTimeAsync(60000);
    expect(ChorusService.getStorages).toHaveBeenCalledTimes(1);
  });
});
