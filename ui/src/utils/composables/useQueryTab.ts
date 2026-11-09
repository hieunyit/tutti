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

import { computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';

/** Active tab kept in the "tab" query parameter, so it survives reloads and links. */
export function useQueryTab<T extends string>(
  allowed: readonly T[],
  fallback: T,
) {
  const route = useRoute();
  const router = useRouter();

  return computed<T>({
    get() {
      const value = route.query.tab;

      return typeof value === 'string' &&
        (allowed as readonly string[]).includes(value)
        ? (value as T)
        : fallback;
    },
    set(tab) {
      router.replace({
        query: { ...route.query, tab: tab === fallback ? undefined : tab },
      });
    },
  });
}
