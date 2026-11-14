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

import type { I18nMessages } from '@clyso/clyso-ui-kit';

/**
 * Merges message sets per locale. Use it when one component needs several message files: a second
 * `useI18n({ messages })` call in the same component returns the first composer and drops its messages.
 */
export function mergeMessages(
  ...sets: Array<Record<string, object>>
): I18nMessages {
  const merged: Record<string, Record<string, unknown>> = {};

  sets.forEach((set) => {
    Object.entries(set).forEach(([locale, messages]) => {
      merged[locale] = { ...merged[locale], ...(messages as object) };
    });
  });

  return merged as I18nMessages;
}
