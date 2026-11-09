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

export interface SearchableItem<T> {
  item: T;
  /** translated visible name */
  label: string;
  /** translated group name, also matched */
  group?: string;
}

/**
 * Jump-to-page search: items whose name (or group) contains every word of the
 * query, case-insensitive. Name matches rank before group matches, names that
 * start with the query before the rest. An empty query returns everything.
 */
export function filterSearchItems<T>(
  query: string,
  entries: SearchableItem<T>[],
): SearchableItem<T>[] {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);

  if (words.length === 0) return entries;

  const scored: { entry: SearchableItem<T>; score: number; index: number }[] =
    [];

  entries.forEach((entry, index) => {
    const label = entry.label.toLowerCase();
    const haystack = `${label} ${(entry.group ?? '').toLowerCase()}`;

    if (!words.every((word) => haystack.includes(word))) return;

    const inLabel = words.every((word) => label.includes(word));
    const startsWith = label.startsWith(words[0] ?? '');

    scored.push({
      entry,
      score: (inLabel ? 0 : 2) + (startsWith ? 0 : 1),
      index,
    });
  });

  return scored
    .sort((a, b) => a.score - b.score || a.index - b.index)
    .map((s) => s.entry);
}
