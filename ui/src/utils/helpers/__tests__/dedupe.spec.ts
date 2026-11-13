/*
 * Copyright © 2026 Clyso GmbH
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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { clearDedupeCache, dedupe, invalidate } from '../dedupe';

describe('dedupe', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    clearDedupeCache();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('shares one request between concurrent calls', async () => {
    const fetcher = vi.fn().mockResolvedValue('a');

    const [x, y] = await Promise.all([
      dedupe('k', fetcher),
      dedupe('k', fetcher),
    ]);

    expect(fetcher).toHaveBeenCalledTimes(1);
    expect([x, y]).toEqual(['a', 'a']);
  });

  it('serves from cache within the TTL and refetches after it', async () => {
    const fetcher = vi.fn().mockResolvedValueOnce('a').mockResolvedValue('b');

    expect(await dedupe('k', fetcher, 3000)).toBe('a');
    vi.advanceTimersByTime(2999);
    expect(await dedupe('k', fetcher, 3000)).toBe('a');
    expect(fetcher).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(2);
    expect(await dedupe('k', fetcher, 3000)).toBe('b');
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it('keeps keys independent', async () => {
    const a = vi.fn().mockResolvedValue(1);
    const b = vi.fn().mockResolvedValue(2);

    expect(await dedupe('a', a)).toBe(1);
    expect(await dedupe('b', b)).toBe(2);
  });

  it('invalidate forces a new request', async () => {
    const fetcher = vi.fn().mockResolvedValueOnce('a').mockResolvedValue('b');

    expect(await dedupe('k', fetcher)).toBe('a');
    invalidate('k');
    expect(await dedupe('k', fetcher)).toBe('b');
  });

  it('invalidate during flight does not cache the stale response', async () => {
    let resolveFirst: (v: string) => void = () => undefined;
    const fetcher = vi
      .fn()
      .mockImplementationOnce(
        () => new Promise<string>((resolve) => (resolveFirst = resolve)),
      )
      .mockResolvedValue('fresh');

    const stale = dedupe('k', fetcher);

    invalidate('k');
    expect(await dedupe('k', fetcher)).toBe('fresh');

    resolveFirst('stale');
    expect(await stale).toBe('stale');
    expect(await dedupe('k', fetcher)).toBe('fresh');
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it('does not cache failures', async () => {
    const fetcher = vi
      .fn()
      .mockRejectedValueOnce(new Error('x'))
      .mockResolvedValue('ok');

    await expect(dedupe('k', fetcher)).rejects.toThrow('x');
    expect(await dedupe('k', fetcher)).toBe('ok');
  });
});
