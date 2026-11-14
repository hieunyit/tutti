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
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useTuttiProtectionStore } from '../tuttiProtectionStore';
import { ProtectionGroupService } from '@/services/ProtectionGroupService';

vi.mock('@/services/ProtectionGroupService', () => ({
  ProtectionGroupService: {
    list: vi.fn(),
    get: vi.fn(),
    getAutomaticFreeze: vi.fn(),
  },
}));
vi.mock('@/services/TuttiCoreService', () => ({ TuttiCoreService: {} }));
vi.mock('@/services/TuttiService', () => ({ TuttiService: {} }));

const groups = [{ id: 'a' }, { id: 'b' }];
const statuses = groups.map((group) => ({ group, healthyCopies: 2 }));

beforeEach(() => {
  setActivePinia(createPinia());
  vi.mocked(ProtectionGroupService.list).mockReset();
  vi.mocked(ProtectionGroupService.get).mockReset();
  vi.mocked(ProtectionGroupService.getAutomaticFreeze).mockReset();
  vi.mocked(ProtectionGroupService.getAutomaticFreeze).mockResolvedValue(false);
});

describe('tuttiProtectionStore refresh', () => {
  it('uses `statuses` from the list and never calls per-id Get', async () => {
    vi.mocked(ProtectionGroupService.list).mockResolvedValue({
      groups,
      statuses,
    });
    const store = useTuttiProtectionStore();

    await store.refresh();
    await store.refresh();
    await store.refresh();
    expect(ProtectionGroupService.get).not.toHaveBeenCalled();
    expect(Object.keys(store.statuses)).toEqual(['a', 'b']);
    expect(store.statuses['b']?.healthyCopies).toBe(2);
  });

  it('keeps the same reactive objects when a poll is unchanged', async () => {
    vi.mocked(ProtectionGroupService.list).mockImplementation(async () =>
      JSON.parse(JSON.stringify({ groups, statuses })),
    );
    const store = useTuttiProtectionStore();

    await store.refresh();
    const first = store.groups;

    await store.refresh();
    expect(store.groups).toBe(first);
  });

  it('old backend without statuses: per-id Get once, not on every poll', async () => {
    vi.mocked(ProtectionGroupService.list).mockResolvedValue({ groups });
    vi.mocked(ProtectionGroupService.get).mockImplementation(async (id) => ({
      group: { id },
      healthyCopies: 1,
    }));
    const store = useTuttiProtectionStore();

    await store.refresh();
    expect(ProtectionGroupService.get).toHaveBeenCalledTimes(2);
    await store.refresh();
    await store.refresh();
    expect(ProtectionGroupService.get).toHaveBeenCalledTimes(2);
    expect(store.statuses['a']?.healthyCopies).toBe(1);
  });
});
