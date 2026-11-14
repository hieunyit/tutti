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

import { flushPromises, mount, type VueWrapper } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createMemoryHistory, createRouter } from 'vue-router';
import TuttiReplicationDetailPage from '../TuttiReplicationDetailPage.vue';
import {
  kitI18n,
  tolerateNaiveStyles,
} from '@/components/kit/__tests__/kitTestUtils';
import { TuttiCoreService } from '@/services/TuttiCoreService';
import { useAuthStore } from '@/stores/authStore';
import { RouteName } from '@/utils/types/router';

vi.mock('@clyso/clyso-ui-kit', () => ({
  I18nLocale: { EN: 'en', DE: 'de' },
}));
vi.mock('@/utils/helpers/DateTimeHelper', () => ({
  formatDateTime: (value: unknown) => String(value ?? '-'),
}));
vi.mock('@/utils/helpers/GeneralHelper', () => ({
  GeneralHelper: { formatDurationSeconds: (value: unknown) => String(value) },
}));
vi.mock('@/services/TuttiCoreService', () => ({
  TuttiCoreService: {
    getReplication: vi.fn(),
    getReplicationActivity: vi.fn(),
    renameReplication: vi.fn(),
    pauseBucketReplication: vi.fn(),
    resumeBucketReplication: vi.fn(),
  },
}));

tolerateNaiveStyles();

const replication = {
  id: { fromStorage: 'a', toStorage: 'b', fromBucket: 'x', toBucket: 'x' },
  name: 'prod to dr',
  isPaused: false,
  isInitDone: false,
  initObjListed: '10',
  initObjDone: '4',
  initBytesTotal: '1000',
  initBytesDone: '400',
  initBytesPerSecond: '50',
  events: '0',
  eventsDone: '0',
  createdAt: '2026-11-15T09:00:00Z',
};

const activity = {
  inFlight: [
    {
      key: 'big/file.bin',
      size: '52428800',
      bytesDone: '26214400',
      startedAt: '2026-11-15T10:00:00Z',
      flow: 'init',
      attempt: 1,
    },
  ],
  events: [
    {
      cursor: '1-0',
      time: '2026-11-15T10:00:01Z',
      level: 'info',
      type: 'done',
      key: 'small/a.txt',
      size: '10',
      durationMs: '20',
      bytesPerSecond: '500',
    },
    {
      cursor: '2-0',
      time: '2026-11-15T10:00:02Z',
      level: 'error',
      type: 'failed',
      key: 'bad key',
      error: 'Access Denied',
    },
  ],
  nextCursor: '2-0',
  serverTime: '2026-11-15T10:00:03Z',
};

let wrapper: VueWrapper | undefined;

async function mountPage(
  query: Record<string, string> = {
    from: 'a',
    to: 'b',
    fromBucket: 'x',
    toBucket: 'x',
  },
) {
  const pinia = createPinia();

  setActivePinia(pinia);
  useAuthStore().authEnabled = false;

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/replication',
        name: RouteName.TUTTI_REPLICATION,
        component: { template: '<div />' },
      },
      {
        path: '/replication/detail',
        name: RouteName.TUTTI_REPLICATION_DETAIL,
        component: TuttiReplicationDetailPage,
      },
    ],
  });

  await router.push({ name: RouteName.TUTTI_REPLICATION_DETAIL, query });
  wrapper = mount(TuttiReplicationDetailPage, {
    attachTo: document.body,
    global: {
      plugins: [pinia, router, kitI18n()],
      // these render icons of the (mocked) clyso kit
      stubs: { ReplicationDirectionCell: true, ReplicationBucketCell: true },
    },
  });
  await flushPromises();

  return wrapper;
}

describe('TuttiReplicationDetailPage', () => {
  beforeEach(() => {
    vi.mocked(TuttiCoreService.getReplication).mockResolvedValue(
      replication as never,
    );
    vi.mocked(TuttiCoreService.getReplicationActivity).mockResolvedValue(
      activity as never,
    );
  });
  afterEach(() => {
    wrapper?.unmount();
    wrapper = undefined;
    vi.clearAllMocks();
  });

  it('shows the name, the running copies and the console lines', async () => {
    const page = await mountPage();

    expect(page.text()).toContain('prod to dr');
    expect(page.text()).toContain('big/file.bin');
    expect(page.text()).toContain('50.0 MB');
    expect(page.text()).toContain('25.0 MB / 50.0 MB (50%)');

    const lines = page.findAll('.activity-console__line');

    expect(lines).toHaveLength(2);
    expect(lines[0]!.classes()).toContain('activity-console__line--ok');
    expect(lines[0]!.text()).toContain('DONE    small/a.txt');
    expect(lines[1]!.classes()).toContain('activity-console__line--error');
    expect(lines[1]!.text()).toContain('Access Denied');
    expect(TuttiCoreService.getReplicationActivity).toHaveBeenCalledWith(
      expect.objectContaining({
        id: expect.objectContaining({ fromStorage: 'a', toStorage: 'b' }),
      }),
    );
  });

  it('filters by level and clears the view', async () => {
    const page = await mountPage();

    const errors = page
      .findAll('.tt-segmented__item')
      .find((b) => b.text() === 'Errors');

    await errors!.trigger('click');
    expect(page.findAll('.activity-console__line')).toHaveLength(1);

    await errors!.trigger('click');
    const clear = page.findAll('button').find((b) => b.text() === 'Clear view');

    await clear!.trigger('click');
    expect(page.text()).toContain('No activity yet');
  });

  it('shows an empty state when nothing happened yet', async () => {
    vi.mocked(TuttiCoreService.getReplicationActivity).mockResolvedValue({
      inFlight: [],
      events: [],
      nextCursor: '',
    } as never);

    const page = await mountPage();

    expect(page.text()).toContain('No activity yet');
    expect(page.text()).toContain('Nothing is being copied right now.');
  });

  it('shows an error banner with retry when the activity cannot be loaded', async () => {
    vi.mocked(TuttiCoreService.getReplicationActivity).mockRejectedValue(
      new Error('boom'),
    );

    const page = await mountPage();

    expect(page.text()).toContain('Unable to load the activity log');

    vi.mocked(TuttiCoreService.getReplicationActivity).mockResolvedValue(
      activity as never,
    );
    await page
      .findAll('button')
      .find((b) => b.text() === 'Try again')!
      .trigger('click');
    await flushPromises();

    expect(page.text()).not.toContain('Unable to load the activity log');
    expect(page.findAll('.activity-console__line')).toHaveLength(2);
  });

  it('shows not found for a query without a replication id', async () => {
    const page = await mountPage({});

    expect(page.text()).toContain('Replication not found');
    expect(TuttiCoreService.getReplicationActivity).not.toHaveBeenCalled();
  });
});
