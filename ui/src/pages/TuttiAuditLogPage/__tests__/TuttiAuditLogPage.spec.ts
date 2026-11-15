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
import TuttiAuditLogPage from '../TuttiAuditLogPage.vue';
import { tolerateNaiveStyles } from '@/components/kit/__tests__/kitTestUtils';
import { AuditLogService } from '@/services/AuditLogService';
import type { AuditLogEntry, AuditLogList } from '@/utils/types/audit';

tolerateNaiveStyles();

vi.mock('@/utils/helpers/DateTimeHelper', () => ({
  formatDateTime: (v: string) => v,
  formatRelative: (v: string) => v,
}));

vi.mock('@/services/AuditLogService', () => ({
  AuditLogService: { list: vi.fn() },
}));

const entry = (id: number): AuditLogEntry => ({
  id,
  at: '2026-11-11T20:00:00Z',
  actor: 'admin',
  requestId: `r${id}`,
  action: 'update',
  family: 'storage',
  entityKey: `main-${id}`,
  beforeJson: '{"a":1}',
  afterJson: '{"a":2}',
});

const list = (
  entries: AuditLogEntry[],
  extra: Partial<AuditLogList> = {},
): AuditLogList => ({
  backend: 'postgres',
  readOnly: false,
  entries,
  ...extra,
});

function mountPage() {
  const wrapper = mount(TuttiAuditLogPage, {
    attachTo: document.body,
    global: {
      plugins: [createI18n({ legacy: false, locale: 'en', messages: {} })],
    },
  });

  return wrapper;
}

describe('TuttiAuditLogPage', () => {
  beforeEach(() => {
    document.body.innerHTML = '';
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  it('shows a loading skeleton first, then the entries', async () => {
    vi.mocked(AuditLogService.list).mockResolvedValue(list([entry(2)]));

    const wrapper = mountPage();

    expect(wrapper.find('[data-kit-state="loading"]').exists()).toBe(true);
    await flushPromises();
    expect(wrapper.text()).toContain('main-2');
    wrapper.unmount();
  });

  it('shows the empty state with its reset action when filtered', async () => {
    vi.mocked(AuditLogService.list).mockResolvedValue(list([]));

    const wrapper = mountPage();

    await flushPromises();
    expect(wrapper.text()).toContain('No audit entries yet');
    wrapper.unmount();
  });

  it('shows an error block whose retry reloads', async () => {
    vi.mocked(AuditLogService.list).mockRejectedValueOnce(new Error('down'));

    const wrapper = mountPage();

    await flushPromises();
    expect(wrapper.find('[role="alert"]').exists()).toBe(true);

    vi.mocked(AuditLogService.list).mockResolvedValueOnce(list([entry(1)]));
    await wrapper.get('[role="alert"] button').trigger('click');
    await flushPromises();
    expect(wrapper.text()).toContain('main-1');
    wrapper.unmount();
  });

  it('keeps the Redis and read-only banners', async () => {
    vi.mocked(AuditLogService.list).mockResolvedValue(
      list([entry(1)], { backend: 'redis', readOnly: true }),
    );

    const wrapper = mountPage();

    await flushPromises();
    expect(wrapper.text()).toContain('History is not kept durably');
    expect(wrapper.text()).toContain('Configuration is read-only');
    wrapper.unmount();
  });

  it('has no page-level language switch, refresh button or jargon pills', async () => {
    vi.mocked(AuditLogService.list).mockResolvedValue(list([entry(1)]));

    const wrapper = mountPage();

    await flushPromises();

    const labels = wrapper.findAll('button').map((b) => b.text());

    expect(labels).not.toContain('Tiếng Việt');
    expect(labels).not.toContain('Refresh');
    expect(wrapper.text()).not.toContain('PostgreSQL (transactional)');
    expect(wrapper.text()).not.toContain('Read-write');
    wrapper.unmount();
  });

  it('shows readable labels for raw families and actions', async () => {
    vi.mocked(AuditLogService.list).mockResolvedValue(
      list([
        { ...entry(3), family: 'system_settings', action: 'RENAME' },
        { ...entry(2), family: 'storages', action: 'PAUSE_INTENT' },
      ]),
    );

    const wrapper = mountPage();

    await flushPromises();
    expect(wrapper.text()).toContain('System settings');
    expect(wrapper.text()).toContain('Rename');
    expect(wrapper.text()).toContain('Pause requested');
    expect(wrapper.text()).not.toContain('system_settings');
    expect(wrapper.text()).not.toContain('PAUSE_INTENT');
    wrapper.unmount();
  });

  it('offers View changes only where a snapshot exists', async () => {
    vi.mocked(AuditLogService.list).mockResolvedValue(
      list([
        entry(2),
        { ...entry(1), beforeJson: undefined, afterJson: undefined },
      ]),
    );

    const wrapper = mountPage();

    await flushPromises();
    expect(
      wrapper.findAll('button').filter((b) => b.text() === 'View changes'),
    ).toHaveLength(1);
    wrapper.unmount();
  });

  it('asks for 25 entries by default', async () => {
    vi.mocked(AuditLogService.list).mockResolvedValue(list([entry(1)]));

    const wrapper = mountPage();

    await flushPromises();
    expect(AuditLogService.list).toHaveBeenCalledWith(
      expect.objectContaining({ limit: 25 }),
    );
    wrapper.unmount();
  });

  it('loads older entries with the last id as cursor', async () => {
    const page1 = Array.from({ length: 25 }, (_, i) => entry(100 - i));

    vi.mocked(AuditLogService.list)
      .mockResolvedValueOnce(list(page1))
      .mockResolvedValueOnce(list([entry(50)]));

    const wrapper = mountPage();

    await flushPromises();

    const older = wrapper
      .findAll('button')
      .find((b) => b.text() === 'Load more')!;

    await older.trigger('click');
    await flushPromises();
    expect(AuditLogService.list).toHaveBeenLastCalledWith(
      expect.objectContaining({ beforeId: '76' }),
    );
    expect(wrapper.text()).toContain('main-50');
    wrapper.unmount();
  });
});
