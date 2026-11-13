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

import { AxiosError, AxiosHeaders } from 'axios';
import { createPinia, setActivePinia } from 'pinia';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useSessionKeeper } from '../useSessionKeeper';
import { useAuthStore } from '@/stores/authStore';
import { AuthService } from '@/services/AuthService';
import type {
  SessionChannelLike,
  SessionMessage,
} from '@/utils/helpers/SessionChannel';

vi.mock('@/services/AuthService', () => ({
  AuthService: {
    login: vi.fn(),
    logout: vi.fn(),
    me: vi.fn(),
    refresh: vi.fn(),
  },
}));

const refresh = AuthService.refresh as unknown as ReturnType<typeof vi.fn>;
const MIN = 60_000;

function httpError(status: number) {
  return new AxiosError('x', String(status), undefined, undefined, {
    status,
    statusText: '',
    data: {},
    headers: {},
    config: { headers: new AxiosHeaders() },
  });
}

function times(idleMin: number, maxMin: number, skewMs = 0) {
  const server = Date.now() + skewMs;

  return {
    expiresAt: new Date(server + idleMin * MIN).toISOString(),
    maxExpiresAt: new Date(server + maxMin * MIN).toISOString(),
    serverTime: new Date(server).toISOString(),
  };
}

function setup(idleMin = 30, maxMin = 24 * 60, skewMs = 0) {
  const store = useAuthStore();
  const expire = vi.spyOn(store, 'expire').mockResolvedValue();
  const listeners: ((m: SessionMessage) => void)[] = [];
  const posted: SessionMessage[] = [];
  const channel: SessionChannelLike = {
    post: (m) => posted.push(m),
    subscribe: (l) => {
      listeners.push(l);

      return () => undefined;
    },
  };

  store.token = 't';
  store.$patch({ token: 't' });
  // applyTimes is internal: feed it through a refresh response
  refresh.mockResolvedValueOnce(times(idleMin, maxMin, skewMs));

  return { store, expire, channel, listeners, posted };
}

async function start(idleMin = 30, maxMin = 24 * 60, skewMs = 0) {
  const ctx = setup(idleMin, maxMin, skewMs);

  await ctx.store.refreshSession();
  refresh.mockReset();

  const keeper = useSessionKeeper({ store: ctx.store, channel: ctx.channel });

  return { ...ctx, keeper };
}

function activity() {
  window.dispatchEvent(new Event('pointerdown'));
}

beforeEach(() => {
  vi.useFakeTimers();
  setActivePinia(createPinia());
  refresh.mockReset();
  window.localStorage.clear();
  window.sessionStorage.clear();
});

afterEach(() => {
  vi.useRealTimers();
});

describe('useSessionKeeper', () => {
  it('does not refresh without user activity (polling never counts)', async () => {
    const { keeper } = await start();

    await vi.advanceTimersByTimeAsync(29 * MIN);

    expect(refresh).not.toHaveBeenCalled();
    keeper.stop();
  });

  it('refreshes once after activity when less than 20 minutes remain', async () => {
    const { keeper } = await start();

    activity();
    refresh.mockImplementation(async () => times(30, 24 * 60));
    await vi.advanceTimersByTimeAsync(9 * MIN);
    expect(refresh).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(2 * MIN);
    expect(refresh).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(5 * MIN);
    expect(refresh).toHaveBeenCalledTimes(1);
    keeper.stop();
  });

  it('warns at 2 minutes idle and Continue refreshes', async () => {
    const { keeper } = await start();

    await vi.advanceTimersByTimeAsync(27 * MIN + 59_000);
    expect(keeper.warning.value).toBe('none');
    await vi.advanceTimersByTimeAsync(2000);
    expect(keeper.warning.value).toBe('idle');
    expect(keeper.remainingIdleMs.value).toBeLessThanOrEqual(2 * MIN);

    refresh.mockResolvedValueOnce(times(30, 24 * 60));
    await keeper.extend();
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(keeper.warning.value).toBe('none');
    keeper.stop();
  });

  it('shows the max warning 5 minutes before the cap', async () => {
    const { keeper, store } = await start(30, 40);

    // a late refresh moved the idle deadline up to the cap
    store.applyRemoteTimes(Date.now() + 40 * MIN, Date.now() + 40 * MIN);
    await vi.advanceTimersByTimeAsync(34 * MIN);
    expect(keeper.warning.value).toBe('none');
    await vi.advanceTimersByTimeAsync(2 * MIN);
    expect(keeper.warning.value).toBe('max');
    keeper.stop();
  });

  it('is not fooled by a client clock 10 minutes ahead', async () => {
    const { keeper } = await start(30, 24 * 60, 10 * MIN);

    await vi.advanceTimersByTimeAsync(1000);
    expect(keeper.remainingIdleMs.value).toBeGreaterThan(29 * MIN);
    expect(keeper.remainingIdleMs.value).toBeLessThanOrEqual(30 * MIN);
    expect(keeper.warning.value).toBe('none');
    keeper.stop();
  });

  it('expires with reason idle when refresh answers 401', async () => {
    const { keeper, expire } = await start();

    activity();
    refresh.mockRejectedValue(httpError(401));
    await vi.advanceTimersByTimeAsync(11 * MIN);
    expect(expire).toHaveBeenCalledWith('idle');
    keeper.stop();
  });

  it('retries a network error 3 times after 10 s without logging out', async () => {
    const { keeper, expire } = await start();

    activity();
    refresh.mockRejectedValue(new Error('network'));
    await vi.advanceTimersByTimeAsync(10 * MIN + 2000);
    expect(refresh).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(refresh).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(refresh).toHaveBeenCalledTimes(3);
    await vi.advanceTimersByTimeAsync(10_000);
    expect(refresh).toHaveBeenCalledTimes(4);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(refresh).toHaveBeenCalledTimes(4);
    expect(expire).not.toHaveBeenCalled();
    keeper.stop();
  });

  it('expires with reason max when the cap passes', async () => {
    const { keeper, expire } = await start(30, 10);

    await vi.advanceTimersByTimeAsync(10 * MIN + 2000);
    expect(expire).toHaveBeenCalledWith('max');
    keeper.stop();
  });

  it('adopts a deadline from another tab and signs out on its logout', async () => {
    const { keeper, store, listeners, expire } = await start();
    const far = Date.now() + 60 * MIN;

    listeners.forEach((l) =>
      l({ type: 'refreshed', expiresAt: far, maxExpiresAt: far + MIN }),
    );
    expect(store.expiresAt).toBe(far);

    listeners.forEach((l) => l({ type: 'logout' }));
    expect(expire).toHaveBeenCalledWith('signedOut');
    keeper.stop();
  });

  it('broadcasts a refresh to other tabs', async () => {
    const { store } = setup();
    const spy = vi.fn();
    const { getSessionChannel } =
      await import('@/utils/helpers/SessionChannel');

    getSessionChannel().subscribe(spy);
    await store.refreshSession();
    // own posts never come back to this tab
    expect(spy).not.toHaveBeenCalled();
  });
});
