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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ActivityTracker } from '../ActivityTracker';

describe('ActivityTracker', () => {
  let tracker: ActivityTracker;

  beforeEach(() => {
    vi.useFakeTimers();
    tracker = new ActivityTracker();
  });

  afterEach(() => {
    tracker.stop();
    vi.useRealTimers();
  });

  it.each(['pointerdown', 'keydown', 'wheel', 'touchstart'])(
    'reports %s',
    (name) => {
      const spy = vi.fn();

      tracker.start(spy);
      window.dispatchEvent(new Event(name));

      expect(spy).toHaveBeenCalledTimes(1);
    },
  );

  it('throttles to one call per 5 seconds', () => {
    const spy = vi.fn();

    tracker.start(spy);
    window.dispatchEvent(new Event('keydown'));
    vi.advanceTimersByTime(4000);
    window.dispatchEvent(new Event('keydown'));
    expect(spy).toHaveBeenCalledTimes(1);
    vi.advanceTimersByTime(1000);
    window.dispatchEvent(new Event('keydown'));
    expect(spy).toHaveBeenCalledTimes(2);
  });

  it('ignores other events and stops listening', () => {
    const spy = vi.fn();

    tracker.start(spy);
    window.dispatchEvent(new Event('scroll'));
    window.dispatchEvent(new Event('mousemove'));
    expect(spy).not.toHaveBeenCalled();
    tracker.stop();
    window.dispatchEvent(new Event('keydown'));
    expect(spy).not.toHaveBeenCalled();
  });
});
