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

import axios from 'axios';
import { getCurrentScope, onScopeDispose, ref, watch } from 'vue';
import { useAuthStore } from '@/stores/authStore';
import { ActivityTracker } from '@/utils/helpers/ActivityTracker';
import {
  getSessionChannel,
  type SessionChannelLike,
} from '@/utils/helpers/SessionChannel';

export type SessionWarning = 'none' | 'idle' | 'max';

const MINUTE = 60_000;
const TICK_MS = 1000;
const IDLE_WARNING_MS = 2 * MINUTE;
const MAX_WARNING_MS = 5 * MINUTE;
/** refresh quietly once less than 2/3 of the 30 minute idle window remains */
const QUIET_REFRESH_BELOW_MS = 20 * MINUTE;
const RETRY_DELAY_MS = 10_000;
const MAX_RETRIES = 3;

export interface SessionKeeperDeps {
  store?: ReturnType<typeof useAuthStore>;
  tracker?: Pick<ActivityTracker, 'start' | 'stop'>;
  channel?: SessionChannelLike;
}

/**
 * Keeps a signed-in session alive while the user works: quiet refresh after
 * real activity, expiry warnings, expiry handling and tab sync. All deadlines
 * are server-clock values, read as `Date.now() + serverOffsetMs`.
 */
export function useSessionKeeper(deps: SessionKeeperDeps = {}) {
  const store = deps.store ?? useAuthStore();
  const tracker = deps.tracker ?? new ActivityTracker();
  const channel = deps.channel ?? getSessionChannel();

  const remainingIdleMs = ref(Number.POSITIVE_INFINITY);
  const remainingMaxMs = ref(Number.POSITIVE_INFINITY);
  const warning = ref<SessionWarning>('none');

  let activeSinceRefresh = false;
  let inFlight: Promise<void> | null = null;
  let retryTimer: ReturnType<typeof setTimeout> | null = null;
  let retries = 0;
  let giveUp = false;
  let expiring = false;
  let stopped = false;

  function serverNow(): number {
    return Date.now() + store.serverOffsetMs;
  }

  function recompute() {
    if (!store.token || store.expiresAt === null) {
      remainingIdleMs.value = Number.POSITIVE_INFINITY;
      remainingMaxMs.value = Number.POSITIVE_INFINITY;
      warning.value = 'none';

      return;
    }

    const now = serverNow();

    remainingIdleMs.value = store.expiresAt - now;
    remainingMaxMs.value =
      store.maxExpiresAt === null
        ? Number.POSITIVE_INFINITY
        : store.maxExpiresAt - now;

    // an extension is only possible while the cap is further away than the idle deadline
    const canExtend = remainingMaxMs.value - remainingIdleMs.value > TICK_MS;

    if (remainingIdleMs.value <= IDLE_WARNING_MS && canExtend) {
      warning.value = 'idle';
    } else if (remainingMaxMs.value <= MAX_WARNING_MS) {
      warning.value = 'max';
    } else {
      warning.value = 'none';
    }
  }

  function expire(reason: 'idle' | 'max') {
    if (expiring) return;

    expiring = true;
    void store.expire(reason).finally(() => {
      expiring = false;
    });
  }

  function clearRetry() {
    if (retryTimer !== null) clearTimeout(retryTimer);

    retryTimer = null;
  }

  async function attempt(): Promise<void> {
    try {
      await store.refreshSession();
      retries = 0;
      giveUp = false;
      recompute();
    } catch (e) {
      if (axios.isAxiosError(e) && e.response?.status === 401) {
        expire(remainingMaxMs.value <= MAX_WARNING_MS ? 'max' : 'idle');

        return;
      }

      // network error, 429 or 5xx: never log out, try again shortly
      if (retries < MAX_RETRIES && !stopped) {
        retries += 1;
        retryTimer = setTimeout(() => {
          retryTimer = null;
          void doRefresh(true);
        }, RETRY_DELAY_MS);
      } else {
        giveUp = true;
      }

      throw e;
    }
  }

  function doRefresh(isRetry = false): Promise<void> {
    if (inFlight) return inFlight;

    if (!isRetry) {
      clearRetry();
      retries = 0;
    }

    activeSinceRefresh = false;
    inFlight = attempt()
      .catch(() => undefined)
      .finally(() => {
        inFlight = null;
      });

    return inFlight;
  }

  /** "Continue" in the warning dialog: refresh now, whatever the activity. */
  function extend(): Promise<void> {
    giveUp = false;

    return doRefresh();
  }

  function tick() {
    if (!store.token) {
      recompute();

      return;
    }

    recompute();

    if (store.expiresAt === null) return;

    if (remainingMaxMs.value <= 0) {
      expire('max');

      return;
    }

    if (remainingIdleMs.value <= 0) {
      expire('idle');

      return;
    }

    if (
      activeSinceRefresh &&
      !giveUp &&
      retryTimer === null &&
      !inFlight &&
      remainingIdleMs.value < QUIET_REFRESH_BELOW_MS &&
      remainingMaxMs.value - remainingIdleMs.value > TICK_MS
    ) {
      void doRefresh();
    }
  }

  tracker.start(() => {
    activeSinceRefresh = true;
    giveUp = false;
  });

  const unsubscribe = channel.subscribe((message) => {
    if (message.type === 'refreshed') {
      store.applyRemoteTimes(message.expiresAt, message.maxExpiresAt);
      activeSinceRefresh = false;
      recompute();
    } else if (store.token) {
      void store.expire('signedOut');
    }
  });

  const timer = setInterval(tick, TICK_MS);
  const stopWatch = watch(
    () => [store.token, store.expiresAt, store.maxExpiresAt],
    recompute,
    { immediate: true },
  );

  function stop() {
    stopped = true;
    clearInterval(timer);
    clearRetry();
    tracker.stop();
    unsubscribe();
    stopWatch();
  }

  if (getCurrentScope()) onScopeDispose(stop);

  return { remainingIdleMs, remainingMaxMs, warning, extend, stop };
}
