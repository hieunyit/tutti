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

const EVENTS = ['pointerdown', 'keydown', 'wheel', 'touchstart'] as const;
const THROTTLE_MS = 5000;

/**
 * Reports real user input (never programmatic requests such as polling) to
 * `onActivity`, at most once per 5 seconds.
 */
export class ActivityTracker {
  private handler: (() => void) | null = null;

  private lastAt = Number.NEGATIVE_INFINITY;

  start(onActivity: (at: number) => void): void {
    this.stop();
    this.lastAt = Number.NEGATIVE_INFINITY;

    this.handler = () => {
      const at = Date.now();

      if (at - this.lastAt < THROTTLE_MS) return;

      this.lastAt = at;
      onActivity(at);
    };

    EVENTS.forEach((name) =>
      window.addEventListener(name, this.handler as () => void, {
        passive: true,
        capture: true,
      }),
    );
  }

  stop(): void {
    const { handler } = this;

    if (!handler) return;

    EVENTS.forEach((name) =>
      window.removeEventListener(name, handler, { capture: true }),
    );
    this.handler = null;
  }
}
