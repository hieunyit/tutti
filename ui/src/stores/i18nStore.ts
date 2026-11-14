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

import { computed } from 'vue';
import { defineStore } from 'pinia';
import { I18N_DEFAULT_LOCALE } from '@clyso/clyso-ui-kit';
import { i18n } from '@/i18n';
import { APP_LOCALES, type AppLocale } from '@/i18n/locales';
import LocalStorageHelper from '@/utils/helpers/LocalStorageHelper';
import { LocalStorageItem } from '@/utils/types/localStorage';

const LEGACY_LOCALE_KEY = 'chorus-locale';

export const useI18nStore = defineStore('i18n', () => {
  const locale = computed<AppLocale>(
    () => i18n.global.locale.value as AppLocale,
  );
  const localeList = computed<AppLocale[]>(() => APP_LOCALES);

  function updateLocaleValue(newLocale: AppLocale) {
    (i18n.global.locale as { value: string }).value = newLocale;
  }

  function setLocale(newLocale: AppLocale) {
    updateLocaleValue(newLocale);
    LocalStorageHelper.set(LocalStorageItem.I18N_LOCALE, newLocale);
  }

  function initLocale() {
    // Legacy key from before the rename: read once and migrate to the new key.
    const legacyLocale = LocalStorageHelper.get<AppLocale>(LEGACY_LOCALE_KEY);

    if (legacyLocale) {
      if (!LocalStorageHelper.get<AppLocale>(LocalStorageItem.I18N_LOCALE)) {
        LocalStorageHelper.set(LocalStorageItem.I18N_LOCALE, legacyLocale);
      }

      window.localStorage.removeItem(LEGACY_LOCALE_KEY);
    }

    const restoredLocale: AppLocale =
      LocalStorageHelper.get<AppLocale>(LocalStorageItem.I18N_LOCALE) ||
      (navigator.language.slice(0, 2) as AppLocale);

    updateLocaleValue(
      localeList.value.includes(restoredLocale)
        ? restoredLocale
        : (I18N_DEFAULT_LOCALE as AppLocale),
    );
  }

  return {
    locale,
    localeList,
    setLocale,
    initLocale,
  };
});
