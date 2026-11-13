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

import { createI18n } from 'vue-i18n';

let patched = false;

/**
 * jsdom's cssstyle throws on shorthands such as `border: var(--n-border)`
 * that Naive UI injects when a component first mounts. Call this once per
 * spec file before mounting kit (or other Naive UI) components.
 */
export function tolerateNaiveStyles() {
  if (patched) return;

  patched = true;

  const proto = window.CSSStyleDeclaration.prototype;
  const setProperty = proto.setProperty;

  proto.setProperty = function (
    this: CSSStyleDeclaration,
    ...args: Parameters<CSSStyleDeclaration['setProperty']>
  ) {
    try {
      setProperty.apply(this, args);
    } catch {
      // unsupported by jsdom; irrelevant for behaviour tests
    }
  };
}

/** A fresh vue-i18n instance for mounting kit components in tests. */
export function kitI18n(locale = 'en') {
  return createI18n({ legacy: false, locale, messages: {} });
}
