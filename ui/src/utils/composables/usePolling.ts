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

/**
 * Visibility-aware polling loop (setTimeout chain, so ticks never overlap).
 *
 * - `start(fn, ms)` runs `fn` once immediately and resolves/rejects with that
 *   first run; later ticks are scheduled `ms` after the previous one settles
 *   and their errors are swallowed (the loop keeps going).
 * - While `document.hidden` no tick is scheduled/executed; when the tab
 *   becomes visible again one refresh runs at once and the cadence resumes.
 * - `stop()` cancels the loop and resolves once any in-flight tick settled.
 * - Adaptive back-off: a tick slower than `slowMs` (default 1 s) doubles the
 *   delay up to `maxMs` (default 60 s); fast ticks halve it back toward `ms`.
 */
export interface PollingOptions {
  /** a tick taking longer than this doubles the interval */
  slowMs?: number;
  /** upper bound of the backed-off interval */
  maxMs?: number;
}

export function usePolling() {
  let generation = 0;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let inflight: Promise<unknown> | null = null;
  let current: {
    fn: () => Promise<unknown> | unknown;
    ms: number;
    delay: number;
    slowMs: number;
    maxMs: number;
  } | null = null;
  let listening = false;

  const isHidden = () => typeof document !== 'undefined' && document.hidden;

  // calls fn synchronously; sync throws become rejections
  function invoke(fn: () => Promise<unknown> | unknown): Promise<unknown> {
    return new Promise((resolve) => {
      resolve(fn());
    });
  }

  function clearTimer() {
    if (timer !== null) {
      clearTimeout(timer);
      timer = null;
    }
  }

  function schedule(gen: number) {
    clearTimer();

    if (gen !== generation || !current || isHidden()) {
      return;
    }

    timer = setTimeout(() => {
      timer = null;
      void run(gen).catch(() => undefined);
    }, current.delay);
  }

  function adapt(duration: number) {
    if (!current) return;

    const { ms, slowMs, maxMs } = current;

    current.delay =
      duration > slowMs
        ? Math.min(Math.max(current.delay, ms) * 2, Math.max(maxMs, ms))
        : Math.max(ms, Math.floor(current.delay / 2));
  }

  async function run(gen: number) {
    if (gen !== generation || !current || inflight) {
      return;
    }

    if (isHidden()) {
      // paused: the visibilitychange handler resumes
      return;
    }

    const { fn } = current;
    const startedAt = Date.now();
    const request = invoke(fn);

    inflight = request;

    try {
      await request;
    } finally {
      if (inflight === request) {
        inflight = null;
      }

      if (gen === generation) adapt(Date.now() - startedAt);

      schedule(gen);
    }
  }

  function onVisibilityChange() {
    if (isHidden() || !current || inflight) {
      return;
    }

    clearTimer();
    void run(generation).catch(() => undefined);
  }

  function listen() {
    if (!listening && typeof document !== 'undefined') {
      document.addEventListener('visibilitychange', onVisibilityChange);
      listening = true;
    }
  }

  function unlisten() {
    if (listening) {
      document.removeEventListener('visibilitychange', onVisibilityChange);
      listening = false;
    }
  }

  async function stop(): Promise<void> {
    generation++;
    current = null;
    clearTimer();
    unlisten();

    if (inflight) {
      try {
        await inflight;
      } catch {
        // errors of an in-flight tick are reported to the start() caller
      }
    }
  }

  async function start(
    fn: () => Promise<unknown> | unknown,
    ms: number,
    options: PollingOptions = {},
  ): Promise<void> {
    const previous = stop();
    const gen = generation;

    current = {
      fn,
      ms,
      delay: ms,
      slowMs: options.slowMs ?? 1000,
      maxMs: options.maxMs ?? 60_000,
    };
    listen();

    // wait for a tick of the previous loop that is still running
    if (inflight) {
      await previous;

      if (gen !== generation) {
        return;
      }
    }

    // the first run is executed even if the tab is hidden (explicit request)
    const startedAt = Date.now();
    const request = invoke(fn);

    inflight = request;

    try {
      await request;
    } finally {
      if (inflight === request) {
        inflight = null;
      }

      if (gen === generation) adapt(Date.now() - startedAt);

      schedule(gen);
    }
  }

  return {
    start,
    stop,
    isRunning: () => current !== null,
    /** current delay between ticks (backed off when ticks are slow) */
    currentDelay: () => current?.delay ?? null,
  };
}
