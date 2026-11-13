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

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useAuditLogStore } from '../tuttiAuditLogStore';
import { AuditLogService } from '@/services/AuditLogService';

vi.mock('@/services/AuditLogService', () => ({
  AuditLogService: { list: vi.fn() },
}));

const entry = (id: string) => ({ id, at: '2026-01-01T00:00:00Z' });

beforeEach(() => {
  setActivePinia(createPinia());
  vi.mocked(AuditLogService.list).mockReset();
});

describe('tuttiAuditLogStore', () => {
  it('pages with the applied filters, not live edits', async () => {
    vi.mocked(AuditLogService.list)
      .mockResolvedValueOnce({ entries: [entry('2'), entry('1')] } as never)
      .mockResolvedValueOnce({ entries: [] } as never);

    const store = useAuditLogStore();

    store.filters.limit = 2;
    store.filters.actor = 'alice';
    await store.applyFilters();

    store.filters.actor = 'bob'; // typed, not applied yet
    await store.loadOlder();

    expect(AuditLogService.list).toHaveBeenLastCalledWith(
      expect.objectContaining({ actor: 'alice', beforeId: '1' }),
    );
  });

  it('shows the error state instead of old rows when applyFilters fails', async () => {
    vi.mocked(AuditLogService.list)
      .mockResolvedValueOnce({ entries: [entry('1')] } as never)
      .mockRejectedValueOnce(new Error('down'));

    const store = useAuditLogStore();

    await store.applyFilters();
    expect(store.entries).toHaveLength(1);

    store.filters.actor = 'carol';
    await store.applyFilters();

    expect(store.entries).toHaveLength(0);
    expect(store.loadFailed).toBe(true);
  });
});
