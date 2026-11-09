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

import type { RouteLocationRaw } from 'vue-router';

export abstract class AuthHelper {
  /**
   * Accepts only same-origin absolute paths from the `redirect` query param
   * (no protocol-relative `//host`, no external URLs, never back to /login).
   */
  static getSafeRedirect(raw: unknown): RouteLocationRaw | null {
    const value = Array.isArray(raw) ? raw[0] : raw;

    if (
      typeof value !== 'string' ||
      !value.startsWith('/') ||
      value.startsWith('//') ||
      value.startsWith('/\\') ||
      value.startsWith('/login')
    ) {
      return null;
    }

    return value;
  }

  /** "45 s", "2 min" - rounds up so the user never retries too early. */
  static formatRetryAfter(
    seconds: number,
    format: {
      seconds: (n: number) => string;
      minutes: (n: number) => string;
    },
  ): string {
    const safe = Math.max(1, Math.ceil(seconds));

    return safe < 60
      ? format.seconds(safe)
      : format.minutes(Math.ceil(safe / 60));
  }
}
