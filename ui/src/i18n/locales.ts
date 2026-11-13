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

import { I18nLocale } from '@clyso/clyso-ui-kit';

/**
 * Languages the app offers. The ui-kit `I18nLocale` enum only knows en and
 * de, so the app owns the wider list and maps back for ui-kit components.
 */
export type AppLocale = 'en' | 'de' | 'vi';

export const APP_LOCALES: AppLocale[] = ['en', 'de', 'vi'];

export const APP_LOCALE_LABELS: Record<AppLocale, string> = {
  en: 'English',
  de: 'Deutsch',
  vi: 'Tiếng Việt',
};

/** The locale ui-kit components should use (vi falls back to en). */
export function toKitLocale(locale: AppLocale): I18nLocale {
  return locale === 'de' ? I18nLocale.DE : I18nLocale.EN;
}
