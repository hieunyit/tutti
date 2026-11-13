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
import { ref } from 'vue';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useProtectionGroupActions } from '../useProtectionGroupActions';
import { ProtectionGroupService } from '@/services/ProtectionGroupService';
import { useTuttiProtectionStore } from '@/stores/tuttiProtectionStore';

vi.mock('@/services/ProtectionGroupService', () => ({
  ProtectionGroupService: {
    remove: vi.fn().mockResolvedValue(undefined),
    switchover: vi.fn().mockResolvedValue(undefined),
    abortSwitch: vi.fn().mockResolvedValue(undefined),
    clearWriter: vi.fn().mockResolvedValue(undefined),
  },
}));

function setup() {
  setActivePinia(createPinia());

  const store = useTuttiProtectionStore();

  store.refresh = vi.fn().mockResolvedValue(undefined);
  store.loadDetail = vi.fn().mockResolvedValue(undefined);
  store.loadWriters = vi.fn().mockResolvedValue(undefined);

  return { store, actions: useProtectionGroupActions(ref('pg-1')) };
}

describe('useProtectionGroupActions', () => {
  beforeEach(() => vi.clearAllMocks());

  it('remove() calls the service and then reloads the list', async () => {
    const { store, actions } = setup();

    await actions.remove();
    expect(ProtectionGroupService.remove).toHaveBeenCalledWith('pg-1');
    expect(store.refresh).toHaveBeenCalledTimes(1);
  });

  it('switchover() and abortSwitch() reload the detail quietly', async () => {
    const { store, actions } = setup();

    await actions.switchover('s3');
    expect(ProtectionGroupService.switchover).toHaveBeenCalledWith(
      'pg-1',
      's3',
    );
    await actions.abortSwitch();
    expect(ProtectionGroupService.abortSwitch).toHaveBeenCalledWith('pg-1');
    expect(store.loadDetail).toHaveBeenCalledTimes(2);
    expect(store.loadDetail).toHaveBeenCalledWith('pg-1', true);
  });

  it('clearWriter() marks worker tokens and reloads writers', async () => {
    const { store, actions } = setup();

    store.writers = { writers: { a: 'x' }, workers: { w: 'y' } };
    await actions.clearWriter('w');
    expect(ProtectionGroupService.clearWriter).toHaveBeenCalledWith({
      id: 'pg-1',
      token: 'w',
      worker: true,
      externallyFenced: true,
    });
    await actions.clearWriter('a');
    expect(ProtectionGroupService.clearWriter).toHaveBeenLastCalledWith(
      expect.objectContaining({ token: 'a', worker: false }),
    );
    expect(store.loadWriters).toHaveBeenCalledTimes(2);
  });

  it('rejects when the service fails and does not reload', async () => {
    const { store, actions } = setup();

    vi.mocked(ProtectionGroupService.remove).mockRejectedValueOnce(
      new Error('boom'),
    );
    await expect(actions.remove()).rejects.toThrow('boom');
    expect(store.refresh).not.toHaveBeenCalled();
  });
});
