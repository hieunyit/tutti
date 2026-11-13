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
import { usePolling } from '../usePolling';

function setHidden(value: boolean) {
  Object.defineProperty(document, 'hidden', {
    configurable: true,
    get: () => value,
  });
  document.dispatchEvent(new Event('visibilitychange'));
}

describe('usePolling', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    setHidden(false);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('runs immediately and then every interval', async () => {
    const fn = vi.fn().mockResolvedValue(undefined);
    const polling = usePolling();

    await polling.start(fn, 1000);
    expect(fn).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(1000);
    expect(fn).toHaveBeenCalledTimes(2);

    await vi.advanceTimersByTimeAsync(2000);
    expect(fn).toHaveBeenCalledTimes(4);
    await polling.stop();
  });

  it('rejects start() on first failure but keeps polling', async () => {
    const fn = vi
      .fn()
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValue(undefined);
    const polling = usePolling();

    await expect(polling.start(fn, 1000)).rejects.toThrow('boom');
    await vi.advanceTimersByTimeAsync(1000);
    expect(fn).toHaveBeenCalledTimes(2);
    await polling.stop();
  });

  it('does not overlap ticks while a request is slow', async () => {
    let resolveSlow: () => void = () => undefined;
    const fn = vi
      .fn()
      .mockResolvedValueOnce(undefined)
      .mockImplementationOnce(
        () => new Promise<void>((resolve) => (resolveSlow = resolve)),
      )
      .mockResolvedValue(undefined);
    const polling = usePolling();

    await polling.start(fn, 1000);
    await vi.advanceTimersByTimeAsync(1000);
    expect(fn).toHaveBeenCalledTimes(2);

    // slow tick still pending: nothing else is started
    await vi.advanceTimersByTimeAsync(10_000);
    expect(fn).toHaveBeenCalledTimes(2);

    resolveSlow();
    await vi.advanceTimersByTimeAsync(1000);
    expect(fn).toHaveBeenCalledTimes(3);
    await polling.stop();
  });

  it('pauses while hidden and refreshes once when visible again', async () => {
    const fn = vi.fn().mockResolvedValue(undefined);
    const polling = usePolling();

    await polling.start(fn, 1000);
    setHidden(true);

    await vi.advanceTimersByTimeAsync(10_000);
    expect(fn).toHaveBeenCalledTimes(1);

    setHidden(false);
    await vi.advanceTimersByTimeAsync(0);
    expect(fn).toHaveBeenCalledTimes(2);

    await vi.advanceTimersByTimeAsync(1000);
    expect(fn).toHaveBeenCalledTimes(3);
    await polling.stop();
  });

  it('stop() clears the timer and ignores visibility changes', async () => {
    const fn = vi.fn().mockResolvedValue(undefined);
    const polling = usePolling();

    await polling.start(fn, 1000);
    await polling.stop();

    await vi.advanceTimersByTimeAsync(5000);
    setHidden(true);
    setHidden(false);
    await vi.advanceTimersByTimeAsync(5000);

    expect(fn).toHaveBeenCalledTimes(1);
    expect(polling.isRunning()).toBe(false);
  });

  it('restarting replaces the previous loop', async () => {
    const first = vi.fn().mockResolvedValue(undefined);
    const second = vi.fn().mockResolvedValue(undefined);
    const polling = usePolling();

    await polling.start(first, 1000);
    await polling.start(second, 1000);
    await vi.advanceTimersByTimeAsync(3000);

    expect(first).toHaveBeenCalledTimes(1);
    expect(second).toHaveBeenCalledTimes(4);
    await polling.stop();
  });
});
