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

/** upper bound of pages `loadNewer` walks to close a gap */
const MAX_CATCH_UP_PAGES = 10;

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

  async function run<R = AuditCursorPage<T>>(
    fetcher: () => Promise<R>,
    apply: (result: R) => void,
    restart = false,
    discardOnError = false,
  ) {
    if (restart) generation += 1;

    const mine = generation;

    inFlight += 1;
    loading.value = true;
    error.value = '';

    try {
      const result = await fetcher();

      if (mine === generation) apply(result);
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
      () => fetchPage(),
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

    const from = nextCursor.value;

    await run(
      () => fetchPage(from),
      (page) => {
        items.value = [...items.value, ...dedupe(page.items, items.value)];
        nextCursor.value = page.next;
      },
    );
  }

  /**
   * Fetches from the newest page and walks `next` until the previously
   * newest entry shows up (bounded by MAX_CATCH_UP_PAGES), so more than one
   * page of new entries cannot be skipped. `closed` is false when the bound
   * was hit before reaching a known entry.
   */
  async function collectNewer(knownKey: string) {
    const { keyOf } = options;
    const fresh: T[] = [];
    let cursor: string | undefined;
    let next: string | undefined;

    for (let i = 0; i < MAX_CATCH_UP_PAGES; i += 1) {
      const page = await fetchPage(cursor);
      const hit = page.items.findIndex((item) => keyOf?.(item) === knownKey);

      if (hit >= 0) {
        fresh.push(...page.items.slice(0, hit));

        return { fresh, closed: true, next };
      }

      fresh.push(...page.items);
      next = page.next;

      if (page.next === undefined) return { fresh, closed: true, next };

      cursor = page.next;
    }

    return { fresh, closed: false, next };
  }

  async function loadNewer() {
    if (loading.value) return;

    const { keyOf } = options;
    const newest = items.value[0];

    // the backend gave us a cursor for newer entries, or there is nothing to
    // anchor a catch-up on: single-page behaviour
    if (prevCursor.value !== undefined || !keyOf || newest === undefined) {
      const from = prevCursor.value;

      await run(
        () => fetchPage(from),
        (page) => {
          items.value = [...dedupe(page.items, items.value), ...items.value];

          if (page.prev !== undefined) prevCursor.value = page.prev;
        },
      );

      return;
    }

    await run(
      () => collectNewer(keyOf(newest)),
      ({ fresh, closed, next }) => {
        if (closed) {
          items.value = [...dedupe(fresh, items.value), ...items.value];

          return;
        }

        // gap too large to close: show the newest entries and continue
        // older from where the walk stopped, so nothing is silently skipped
        items.value = fresh;
        nextCursor.value = next;
      },
    );
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
