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

import { describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import { createMemoryHistory, createRouter } from 'vue-router';
import { createPinia } from 'pinia';
import { ref } from 'vue';
import TuttiDiffReportDetailPage from '../TuttiDiffReportDetailPage.vue';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';

tolerateNaiveStyles();

const init = vi.fn().mockResolvedValue(undefined);
const detailState = {
  isRestartLoading: ref(false),
  hasError: ref(false),
  isNotFound: ref(false),
  locations: ref([]),
  report: ref(null),
  hasFixActivity: ref(false),
};

vi.mock('@/stores/tuttiDiffReportDetailStore', () => ({
  useTuttiDiffReportDetailStore: () => ({
    ...detailState,
    initDiffReportDetailsPage: init,
    $reset: vi.fn(),
  }),
}));
vi.mock('@/stores/tuttiDiffReportEntriesStore', () => ({
  useTuttiDiffReportEntriesStore: () => ({ $reset: vi.fn() }),
}));
vi.mock('@/components/auth/WriteGate.vue', () => ({
  default: { template: '<div><slot /></div>' },
}));
vi.mock('@/utils/composables/useTuttiNotification', () => ({
  useTuttiNotification: () => ({ createNotification: vi.fn() }),
}));
vi.mock(
  '@/components/tutti/diff-report-detail/DiffReportDetailEntries/DiffReportDetailEntries.vue',
  () => ({ default: { template: '<div />' } }),
);
vi.mock(
  '@/components/tutti/diff-report-detail/DiffReportDetailOverview/DiffReportDetailOverview.vue',
  () => ({ default: { template: '<div />' } }),
);
vi.mock(
  '@/components/tutti/diff-report-detail/DiffReportDetailProgress/DiffReportDetailProgress.vue',
  () => ({ default: { template: '<div />' } }),
);

async function mountAt(url: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/replication/diff/:id',
        name: 'diff-detail',
        component: { template: '<div />' },
      },
      { path: '/:p(.*)*', component: { template: '<div />' } },
    ],
  });

  router.push(url);
  await router.isReady();

  const wrapper = mount(TuttiDiffReportDetailPage, {
    global: {
      plugins: [router, createPinia(), kitI18n()],
      stubs: { RouterLink: { template: '<a><slot /></a>' } },
    },
  });

  await flushPromises();

  return { router, wrapper };
}

describe('TuttiDiffReportDetailPage', () => {
  it('keeps the id and fills from/to from it when the query lacks them', async () => {
    init.mockClear();

    const { router } = await mountAt('/replication/diff/s3a:logs~s3b:logs');

    expect(router.currentRoute.value.params.id).toBe('s3a:logs~s3b:logs');
    expect(router.currentRoute.value.query.from).toBe('s3a:logs');
    expect(router.currentRoute.value.query.to).toBe('s3b:logs');
    expect(init).toHaveBeenCalledTimes(1);
  });

  it('loads at once when the old deep link already carries from/to', async () => {
    init.mockClear();

    const { router } = await mountAt(
      '/replication/diff/X?from=s3a:logs&to=s3b:logs',
    );

    expect(router.currentRoute.value.params.id).toBe('X');
    expect(init).toHaveBeenCalledTimes(1);
  });

  it('shows a message instead of a blank page when the id is not a report', async () => {
    init.mockClear();

    const { wrapper } = await mountAt('/replication/diff/garbage');

    expect(init).not.toHaveBeenCalled();
    expect(wrapper.text().length).toBeGreaterThan(0);
  });
});
