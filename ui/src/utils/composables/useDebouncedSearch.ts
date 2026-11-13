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

import { onScopeDispose, ref, watch } from 'vue';
import { GeneralHelper } from '@/utils/helpers/GeneralHelper';

/**
 * Raw `search` follows every keystroke (the input stays responsive);
 * `debounced` lags by `ms` (default 250) and clearing applies at once. Use
 * `debounced` in the computed that filters the rows.
 */
export function useDebouncedSearch(ms = 250) {
  const search = ref('');
  const debounced = ref('');
  const apply = GeneralHelper.debounce((value: string) => {
    debounced.value = value;
  }, ms);

  watch(search, (value) => {
    if (value === '') {
      apply.cancel();
      debounced.value = '';
    } else {
      apply(value);
    }
  });
  onScopeDispose(() => apply.cancel());

  return { search, debounced };
}
