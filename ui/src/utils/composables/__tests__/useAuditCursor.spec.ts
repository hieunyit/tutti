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
import { useAuditCursor } from '../useAuditCursor';

interface Item {
  id: string;
}

const page = (ids: string[], next?: string, prev?: string) => ({
  items: ids.map((id) => ({ id })),
  next,
  prev,
});

describe('useAuditCursor', () => {
  it('loadOlder passes the last next cursor and appends', async () => {
    const fetchPage = vi
      .fn<(cursor?: string) => Promise<ReturnType<typeof page>>>()
      .mockResolvedValueOnce(page(['3', '2'], '2'))
      .mockResolvedValueOnce(page(['1'], undefined));
    const cursor = useAuditCursor<Item>(fetchPage, { keyOf: (i) => i.id });

    await cursor.refresh();
    expect(cursor.items.value.map((i) => i.id)).toEqual(['3', '2']);
    expect(cursor.hasOlder.value).toBe(true);

    await cursor.loadOlder();
    expect(fetchPage).toHaveBeenLastCalledWith('2');
    expect(cursor.items.value.map((i) => i.id)).toEqual(['3', '2', '1']);
    expect(cursor.hasOlder.value).toBe(false);
  });

  it('keeps loaded items and sets error when a page fails', async () => {
    const fetchPage = vi
      .fn<(cursor?: string) => Promise<ReturnType<typeof page>>>()
      .mockResolvedValueOnce(page(['3', '2'], '2'))
      .mockRejectedValueOnce(new Error('boom'));
    const cursor = useAuditCursor<Item>(fetchPage);

    await cursor.refresh();
    await cursor.loadOlder();

    expect(cursor.items.value).toHaveLength(2);
    expect(cursor.error.value).toBe('boom');
    expect(cursor.loading.value).toBe(false);
  });

  it('drops old rows when a restart with discardOnError fails', async () => {
    const fetchPage = vi
      .fn<(cursor?: string) => Promise<ReturnType<typeof page>>>()
      .mockResolvedValueOnce(page(['2', '1'], '1'))
      .mockRejectedValueOnce(new Error('boom'));
    const cursor = useAuditCursor<Item>(fetchPage);

    await cursor.refresh();
    await cursor.refresh({ discardOnError: true });

    expect(cursor.items.value).toEqual([]);
    expect(cursor.error.value).toBe('boom');
    expect(cursor.hasOlder.value).toBe(false);
  });

  it('loadNewer prepends only entries not already loaded', async () => {
    const fetchPage = vi
      .fn<(cursor?: string) => Promise<ReturnType<typeof page>>>()
      .mockResolvedValueOnce(page(['3', '2'], '2'))
      .mockResolvedValueOnce(page(['4', '3', '2'], '2'));
    const cursor = useAuditCursor<Item>(fetchPage, { keyOf: (i) => i.id });

    await cursor.refresh();
    await cursor.loadNewer();

    expect(cursor.items.value.map((i) => i.id)).toEqual(['4', '3', '2']);
  });

  it('loadNewer walks pages until the gap to the newest known entry is closed', async () => {
    const fetchPage = vi
      .fn<(cursor?: string) => Promise<ReturnType<typeof page>>>()
      .mockResolvedValueOnce(page(['10', '9'], '9'))
      .mockResolvedValueOnce(page(['15', '14'], '14'))
      .mockResolvedValueOnce(page(['13', '12'], '12'))
      .mockResolvedValueOnce(page(['11', '10'], '10'));
    const cursor = useAuditCursor<Item>(fetchPage, { keyOf: (i) => i.id });

    await cursor.refresh();
    await cursor.loadNewer();

    expect(cursor.items.value.map((i) => i.id)).toEqual([
      '15',
      '14',
      '13',
      '12',
      '11',
      '10',
      '9',
    ]);
    expect(fetchPage.mock.calls.map((c) => c[0])).toEqual([
      undefined,
      undefined,
      '14',
      '12',
    ]);
  });

  it('loadNewer stops after 10 pages and restarts from the newest when the gap stays open', async () => {
    let n = 1000;
    const fetchPage = vi
      .fn<(cursor?: string) => Promise<ReturnType<typeof page>>>()
      .mockResolvedValueOnce(page(['2', '1'], '1'))
      .mockImplementation(async () => {
        n -= 2;

        return page([String(n + 1), String(n)], String(n));
      });
    const cursor = useAuditCursor<Item>(fetchPage, { keyOf: (i) => i.id });

    await cursor.refresh();
    await cursor.loadNewer();

    expect(fetchPage).toHaveBeenCalledTimes(1 + 10);
    expect(cursor.items.value).toHaveLength(20);
    expect(cursor.items.value.map((i) => i.id)).not.toContain('1');
  });

  it('refresh replaces items and ignores a stale response', async () => {
    let resolveFirst: (v: ReturnType<typeof page>) => void = () => {};
    const fetchPage = vi
      .fn<(cursor?: string) => Promise<ReturnType<typeof page>>>()
      .mockImplementationOnce(
        () => new Promise((resolve) => (resolveFirst = resolve)),
      )
      .mockResolvedValueOnce(page(['9']));
    const cursor = useAuditCursor<Item>(fetchPage);
    const first = cursor.refresh();

    await cursor.refresh();
    resolveFirst(page(['1']));
    await first;

    expect(cursor.items.value.map((i) => i.id)).toEqual(['9']);
  });
});
