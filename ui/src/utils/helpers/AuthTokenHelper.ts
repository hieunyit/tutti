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

const TOKEN_KEY = 'tutti.auth.token';

/**
 * Token persistence.
 *
 * Trade-off: sessionStorage (default) is scoped to one browser tab and is wiped
 * when the tab closes, which limits the lifetime of a stolen token but forces a
 * new login per tab/session. localStorage ("Keep me signed in") survives restarts
 * and is shared by all tabs, which is convenient but leaves the bearer token on
 * disk and readable by any script running on this origin (XSS) until it expires
 * or is revoked via logout. Both stores are script-readable; neither is as safe as
 * an HttpOnly cookie, so only opt into localStorage on trusted personal machines.
 *
 * Storage access can throw (private mode, blocked site data), so every call is
 * guarded and degrades to "no persisted token".
 */
export abstract class AuthTokenHelper {
  static get(): string | null {
    try {
      return (
        window.sessionStorage.getItem(TOKEN_KEY) ??
        window.localStorage.getItem(TOKEN_KEY)
      );
    } catch {
      return null;
    }
  }

  static set(token: string, persistent: boolean): void {
    AuthTokenHelper.clear();

    try {
      (persistent ? window.localStorage : window.sessionStorage).setItem(
        TOKEN_KEY,
        token,
      );
    } catch {
      // storage unavailable: the token only lives in memory for this page load
    }
  }

  static clear(): void {
    try {
      window.sessionStorage.removeItem(TOKEN_KEY);
      window.localStorage.removeItem(TOKEN_KEY);
    } catch {
      // ignore
    }
  }
}
