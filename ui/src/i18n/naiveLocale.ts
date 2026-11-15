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

import {
  dateDeDE,
  dateEnUS,
  dateViVN,
  deDE,
  enUS,
  viVN,
  type NDateLocale,
  type NLocale,
} from 'naive-ui';
import type { AppLocale } from '@/i18n/locales';

/**
 * Naive UI strings per app language. Every language gets a locale (en and de
 * used to fall back to Naive's English defaults). Input-like controls get NO
 * default placeholder: fields carry visible labels, so "Please Input" /
 * "Please Select" would only repeat the label. Pages that need a format hint
 * pass their own `placeholder`. English Naive strings are put in sentence case.
 */
const BLANK = { placeholder: '' };

function withoutDefaultPlaceholders(base: NLocale): NLocale {
  return {
    ...base,
    Input: { ...base.Input, ...BLANK },
    InputNumber: { ...base.InputNumber, ...BLANK },
    Select: { ...base.Select, ...BLANK },
    Cascader: { ...base.Cascader, ...BLANK },
  };
}

const en: NLocale = {
  ...withoutDefaultPlaceholders(enUS),
  Empty: { description: 'No data' },
  Pagination: { goto: 'Go to', selectionSuffix: 'page' },
  TimePicker: {
    ...enUS.TimePicker,
    placeholder: 'Select time',
    positiveText: 'Apply',
  },
  DatePicker: {
    ...enUS.DatePicker,
    selectDate: 'Select date',
    datePlaceholder: 'Select date',
    datetimePlaceholder: 'Select date and time',
    monthPlaceholder: 'Select month',
    yearPlaceholder: 'Select year',
    quarterPlaceholder: 'Select quarter',
    weekPlaceholder: 'Select week',
    selectTime: 'Select time',
    startDatePlaceholder: 'Start date',
    endDatePlaceholder: 'End date',
    startDatetimePlaceholder: 'Start date and time',
    endDatetimePlaceholder: 'End date and time',
  },
};

export const NAIVE_LOCALES: Record<
  AppLocale,
  { locale: NLocale; dateLocale: NDateLocale }
> = {
  en: { locale: en, dateLocale: dateEnUS },
  de: { locale: withoutDefaultPlaceholders(deDE), dateLocale: dateDeDE },
  vi: { locale: withoutDefaultPlaceholders(viVN), dateLocale: dateViVN },
};
