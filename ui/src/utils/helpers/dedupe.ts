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

/** Default time a successful response is reused. */
export const DEDUPE_TTL_MS = 3000;

interface Entry {
  promise: Promise<unknown>;
  /** set once the request resolved successfully */
  expiresAt: number | null;
}

const entries = new Map<string, Entry>();

/**
 * Shares one request between concurrent callers (in-flight de-duplication)
 * and reuses a successful response for `ttlMs`. Failures are never cached.
 * Use `invalidate` after mutations of the same resource.
 */
export function dedupe<T>(
  key: string,
  fetcher: () => Promise<T>,
  ttlMs: number = DEDUPE_TTL_MS,
): Promise<T> {
  const existing = entries.get(key);

  if (
    existing &&
    (existing.expiresAt === null || existing.expiresAt > Date.now())
  ) {
    return existing.promise as Promise<T>;
  }

  const entry: Entry = { promise: undefined as never, expiresAt: null };

  entry.promise = fetcher().then(
    (value) => {
      // an invalidate() while in flight replaced/removed this entry
      if (entries.get(key) === entry) {
        entry.expiresAt = Date.now() + ttlMs;
      }

      return value;
    },
    (error) => {
      if (entries.get(key) === entry) {
        entries.delete(key);
      }

      throw error;
    },
  );
  entries.set(key, entry);

  return entry.promise as Promise<T>;
}

/** Drops cached and in-flight entries so the next call hits the network. */
export function invalidate(...keys: string[]): void {
  keys.forEach((key) => entries.delete(key));
}

export function clearDedupeCache(): void {
  entries.clear();
}

export const DedupeKey = {
  PROXIES: 'tutti:proxies',
  USERS: 'tutti:users',
  STORAGES: 'chorus:storages',
} as const;
