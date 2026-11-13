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
 * Change detector for polled list responses: serialises the raw response once
 * and compares it with the previous one, so an unchanged poll never touches
 * reactive state (no deep-reactive churn, no re-render of big tables).
 */
export function createChangeDetector() {
  let last: string | null = null;

  return {
    /** true when `value` differs from the previous call (first call: true) */
    hasChanged(value: unknown): boolean {
      let next: string;

      try {
        next = JSON.stringify(value) ?? '';
      } catch {
        last = null;

        return true;
      }

      if (next === last) return false;

      last = next;

      return true;
    },
    reset() {
      last = null;
    },
  };
}
