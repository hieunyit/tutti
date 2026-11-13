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

import {
  computed,
  getCurrentScope,
  onScopeDispose,
  ref,
  watch,
  type Ref,
} from 'vue';
import { useRoute, useRouter } from 'vue-router';

/**
 * Active tab kept in a query parameter (default "tab"), so it survives
 * reloads and links. Generalises utils/composables/useQueryTab to any key and
 * any value; the fallback is never written to the URL.
 */
export function useQueryTab(key = 'tab', fallback: string): Ref<string> {
  const route = useRoute();
  const router = useRouter();

  return computed<string>({
    get() {
      const value = route.query[key];

      return typeof value === 'string' && value !== '' ? value : fallback;
    },
    set(tab) {
      router.replace({
        query: { ...route.query, [key]: tab === fallback ? undefined : tab },
      });
    },
  });
}

export type TableDensity = 'comfortable' | 'compact';

const DENSITY_KEY = 'tutti-table-density';
const density = ref<TableDensity>(readDensity());

function readDensity(): TableDensity {
  try {
    return window.localStorage.getItem(DENSITY_KEY) === 'compact'
      ? 'compact'
      : 'comfortable';
  } catch {
    return 'comfortable';
  }
}

/** Table row density shared by every DataTable, saved in localStorage. */
export function useTableDensity(): Ref<TableDensity> {
  return computed<TableDensity>({
    get: () => density.value,
    set(value) {
      density.value = value;

      try {
        window.localStorage.setItem(DENSITY_KEY, value);
      } catch {
        // storage unavailable (private mode): keep the in-memory value
      }
    },
  });
}

const DRAFT_PREFIX = 'tutti-wizard-draft:';
const draftResetters = new Map<string, Set<() => void>>();

/** sessionStorage key of a wizard draft; `part` separates step from data. */
export function wizardDraftStorageKey(draftKey: string, part = 'data') {
  return `${DRAFT_PREFIX}${draftKey}:${part}`;
}

function readJson<T>(key: string): T | undefined {
  try {
    const raw = window.sessionStorage.getItem(key);

    return raw === null ? undefined : (JSON.parse(raw) as T);
  } catch {
    return undefined;
  }
}

/**
 * Form data of a wizard that survives navigating away and back (kept in
 * sessionStorage for this tab only). Never put secrets in a draft.
 * `clearWizardDraft(draftKey)` (WizardShell's "Start over") resets it.
 */
export function useWizardDraft<T extends object>(
  draftKey: string,
  initial: () => T,
): Ref<T> {
  const storageKey = wizardDraftStorageKey(draftKey);
  const draft = ref(readJson<T>(storageKey) ?? initial()) as Ref<T>;

  watch(
    draft,
    (value) => {
      try {
        window.sessionStorage.setItem(storageKey, JSON.stringify(value));
      } catch {
        // storage full or unavailable: the draft just won't survive
      }
    },
    { deep: true },
  );

  const reset = () => {
    draft.value = initial();
  };
  const set = draftResetters.get(draftKey) ?? new Set();

  set.add(reset);
  draftResetters.set(draftKey, set);

  if (getCurrentScope()) onScopeDispose(() => set.delete(reset));

  return draft;
}

/** Drops a wizard draft (data and current step) and resets live drafts. */
export function clearWizardDraft(draftKey: string) {
  try {
    window.sessionStorage.removeItem(wizardDraftStorageKey(draftKey));
    window.sessionStorage.removeItem(wizardDraftStorageKey(draftKey, 'step'));
  } catch {
    // nothing stored
  }

  draftResetters.get(draftKey)?.forEach((reset) => reset());
}

export function readWizardStep(draftKey: string): number {
  const step = readJson<number>(wizardDraftStorageKey(draftKey, 'step'));

  return typeof step === 'number' && step >= 0 ? step : 0;
}

export function writeWizardStep(draftKey: string, step: number) {
  try {
    window.sessionStorage.setItem(
      wizardDraftStorageKey(draftKey, 'step'),
      JSON.stringify(step),
    );
  } catch {
    // ignore
  }
}
