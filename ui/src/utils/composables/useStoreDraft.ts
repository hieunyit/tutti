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

import { watch, type Ref, type WatchSource } from 'vue';
import { useWizardDraft } from '@/components/kit';

interface StoreDraftOptions<T extends object> {
  draftKey: string;
  initial: () => T;
  /** the form values currently held by the (pinia) store */
  snapshot: () => T;
  /** write a saved draft back into the store (ignore values that no longer exist) */
  apply: (draft: T) => void;
  /** true once the store has loaded the data a draft refers to */
  ready: WatchSource<boolean>;
}

/**
 * Keeps a wizard whose form lives in a pinia store alive across navigation:
 * once the store is ready the saved draft is applied to it, and every later
 * change of the store values is saved as the new draft. A reload of the store
 * data (ready false, then true again, e.g. "Start over") applies the draft
 * again; "Start over" empties the draft first, so the store keeps its defaults.
 */
export function useStoreDraft<T extends object>(
  options: StoreDraftOptions<T>,
): Ref<T> {
  const draft = useWizardDraft(options.draftKey, options.initial);
  let applied = false;

  watch(
    options.ready,
    (isReady) => {
      if (!isReady) {
        applied = false;

        return;
      }

      if (applied) return;

      applied = true;
      options.apply(draft.value);
    },
    { immediate: true },
  );

  watch(
    options.snapshot,
    (value) => {
      if (applied) draft.value = value;
    },
    { deep: true },
  );

  return draft;
}
