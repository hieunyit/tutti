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

import { createI18n, type I18nOptions } from 'vue-i18n';
import { watch } from 'vue';
import { i18n } from '@/i18n';

type LocaleMessages = NonNullable<I18nOptions['messages']>;

/**
 * Translation function for Pinia stores with their own messages.
 *
 * Stores are set up inside the component that first uses them, so calling
 * `useI18n({ messages })` in a store registers a second local scope on that
 * component (intlify warns "Duplicate `useI18n` calling by local scope").
 * This creates a standalone composer instead and keeps its locale in sync
 * with the global one.
 */
export function useStoreI18n({ messages }: { messages: LocaleMessages }) {
  const globalLocale = i18n.global.locale;
  const { global: composer } = createI18n({
    legacy: false,
    locale: globalLocale.value,
    fallbackLocale: i18n.global.fallbackLocale.value,
    messages,
  });

  watch(globalLocale, (locale) => {
    composer.locale.value = locale;
  });

  return { t: composer.t };
}
