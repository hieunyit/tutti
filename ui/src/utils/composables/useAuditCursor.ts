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

import { computed, ref, type Ref } from 'vue';

export interface AuditCursorPage<T> {
  items: T[];
  /** cursor for the next-older page; undefined when there is no more */
  next?: string;
  /** cursor for newer entries; undefined = start from the newest */
  prev?: string;
}

export interface AuditCursorOptions<T> {
  /** identity of an item, used to skip entries that are already loaded */
  keyOf?: (item: T) => string;
}

function messageOf(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

/**
 * Cursor paging that accumulates: `loadOlder` appends the next-older page,
 * `loadNewer` prepends entries that appeared since, `refresh` restarts from
 * the newest. A failed page keeps everything loaded so far and sets `error`.
 * Responses that arrive after a newer `refresh` are discarded.
 */
export function useAuditCursor<T>(
  fetchPage: (cursor?: string) => Promise<AuditCursorPage<T>>,
  options: AuditCursorOptions<T> = {},
) {
  const items = ref([]) as Ref<T[]>;
  const nextCursor = ref<string>();
  const prevCursor = ref<string>();
  const loading = ref(false);
  const error = ref('');
  const loaded = ref(false);
  let generation = 0;
  let inFlight = 0;

  const hasOlder = computed(() => nextCursor.value !== undefined);

  function dedupe(incoming: T[], existing: T[]): T[] {
    const { keyOf } = options;

    if (!keyOf) return incoming;

    const known = new Set(existing.map(keyOf));

    return incoming.filter((item) => !known.has(keyOf(item)));
  }

  async function run(
    cursor: string | undefined,
    apply: (page: AuditCursorPage<T>) => void,
    restart = false,
    discardOnError = false,
  ) {
    if (restart) generation += 1;

    const mine = generation;

    inFlight += 1;
    loading.value = true;
    error.value = '';

    try {
      const page = await fetchPage(cursor);

      if (mine === generation) apply(page);
    } catch (e) {
      if (mine === generation) {
        error.value = messageOf(e);

        // rows of a previous query must not stay under a new query's filters
        if (discardOnError) {
          items.value = [];
          nextCursor.value = undefined;
          prevCursor.value = undefined;
        }
      }
    } finally {
      inFlight -= 1;

      loading.value = inFlight > 0;
    }
  }

  /** `discardOnError`: a failed restart drops the old rows (new query). */
  async function refresh(refreshOptions: { discardOnError?: boolean } = {}) {
    await run(
      undefined,
      (page) => {
        items.value = page.items;
        nextCursor.value = page.next;
        prevCursor.value = page.prev;
        loaded.value = true;
      },
      true,
      refreshOptions.discardOnError,
    );
  }

  async function loadOlder() {
    if (loading.value || nextCursor.value === undefined) return;

    await run(nextCursor.value, (page) => {
      items.value = [...items.value, ...dedupe(page.items, items.value)];
      nextCursor.value = page.next;
    });
  }

  async function loadNewer() {
    if (loading.value) return;

    await run(prevCursor.value, (page) => {
      items.value = [...dedupe(page.items, items.value), ...items.value];

      if (page.prev !== undefined) prevCursor.value = page.prev;
    });
  }

  return {
    items,
    loading,
    error,
    loaded,
    hasOlder,
    refresh,
    loadOlder,
    loadNewer,
  };
}
