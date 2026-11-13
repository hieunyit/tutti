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
import { useRowActions } from '../useRowActions';

interface Row {
  id: string;
}

const row: Row = { id: 'r1' };

function setup() {
  const pause = vi.fn().mockResolvedValue(undefined);
  const remove = vi.fn().mockResolvedValue(undefined);
  const api = useRowActions<Row>(
    [
      { key: 'pause', label: 'Pause', run: pause },
      {
        key: 'delete',
        label: 'Delete',
        danger: true,
        run: remove,
        confirm: (r) => `Delete ${r.id}?`,
      },
    ],
    (r) => r.id,
  );

  return { api, pause, remove };
}

describe('useRowActions', () => {
  it('runs a plain action at once', async () => {
    const { api, pause } = setup();

    api.actionsFor(row)[0]!.invoke();
    await Promise.resolve();

    expect(pause).toHaveBeenCalledWith(row);
    expect(api.pending.value).toBeNull();
  });

  it('parks a danger action until it is confirmed', async () => {
    const { api, remove } = setup();

    api.actionsFor(row)[1]!.invoke();

    expect(remove).not.toHaveBeenCalled();
    expect(api.pending.value?.message).toBe('Delete r1?');

    await api.confirmPending();

    expect(remove).toHaveBeenCalledWith(row);
    expect(api.pending.value).toBeNull();
  });

  it('drops a cancelled danger action', async () => {
    const { api, remove } = setup();

    api.actionsFor(row)[1]!.invoke();
    api.cancelPending();
    await api.confirmPending();

    expect(remove).not.toHaveBeenCalled();
  });

  it('marks the action busy while it runs and clears it afterwards', async () => {
    let finish: () => void = () => {};
    const api = useRowActions<Row>(
      [
        {
          key: 'pause',
          label: 'Pause',
          run: () => new Promise<void>((resolve) => (finish = resolve)),
        },
      ],
      (r) => r.id,
    );

    api.actionsFor(row)[0]!.invoke();

    expect(api.actionsFor(row)[0]!.busy).toBe(true);
    expect(api.running.value.has('pause:r1')).toBe(true);

    finish();
    await Promise.resolve();
    await Promise.resolve();

    expect(api.actionsFor(row)[0]!.busy).toBe(false);
  });

  it('clears busy when the action rejects', async () => {
    const api = useRowActions<Row>(
      [{ key: 'x', label: 'X', run: () => Promise.reject(new Error('no')) }],
      (r) => r.id,
    );

    api.actionsFor(row)[0]!.invoke();
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(api.running.value.size).toBe(0);
  });
});
