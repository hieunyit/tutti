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

import { I18nLocale, type I18nMessages } from '@clyso/clyso-ui-kit';

export default <I18nMessages>{
  [I18nLocale.EN]: {
    sinceLabel: 'Which objects to copy',
    sinceAll: 'All objects',
    sinceRecent: 'Only recent objects',
    sinceDate: 'Since a date',
    sinceAmount: 'Last',
    sinceUnit: 'Unit',
    sinceDateTime: 'Date and time',
    unitDays: 'days',
    unitHours: 'hours',
    sinceComputed: 'Objects modified since {date}',
    sinceHint: 'Older objects are not copied.',
    sinceAmountError: 'Enter a whole number of 1 or more.',
    sinceFutureError: 'Choose a moment that is not in the future.',
    sinceTooOldError: 'Choose a date in the year 2000 or later.',
    sinceDateMissing: 'Choose a date and time.',
    reviewObjects: 'Objects',
    reviewObjectsAll: 'All objects',
    reviewObjectsSince: 'Modified since {date}',
    listSince: 'Since {date}',
    detailSince: 'Objects copied',
    detailSinceValue: 'Modified since {date}',
  },
  [I18nLocale.DE]: {
    sinceLabel: 'Welche Objekte kopiert werden',
    sinceAll: 'Alle Objekte',
    sinceRecent: 'Nur neuere Objekte',
    sinceDate: 'Ab einem Datum',
    sinceAmount: 'Letzte',
    sinceUnit: 'Einheit',
    sinceDateTime: 'Datum und Uhrzeit',
    unitDays: 'Tage',
    unitHours: 'Stunden',
    sinceComputed: 'Objekte, die seit {date} geändert wurden',
    sinceHint: 'Ältere Objekte werden nicht kopiert.',
    sinceAmountError: 'Geben Sie eine ganze Zahl ab 1 ein.',
    sinceFutureError:
      'Wählen Sie einen Zeitpunkt, der nicht in der Zukunft liegt.',
    sinceTooOldError: 'Wählen Sie ein Datum ab dem Jahr 2000.',
    sinceDateMissing: 'Wählen Sie Datum und Uhrzeit.',
    reviewObjects: 'Objekte',
    reviewObjectsAll: 'Alle Objekte',
    reviewObjectsSince: 'Geändert seit {date}',
    listSince: 'Ab {date}',
    detailSince: 'Kopierte Objekte',
    detailSinceValue: 'Geändert seit {date}',
  },
  vi: {
    sinceLabel: 'Object nào được sao chép',
    sinceAll: 'Tất cả object',
    sinceRecent: 'Chỉ object gần đây',
    sinceDate: 'Từ một ngày',
    sinceAmount: 'Trong',
    sinceUnit: 'Đơn vị',
    sinceDateTime: 'Ngày và giờ',
    unitDays: 'ngày',
    unitHours: 'giờ',
    sinceComputed: 'Object được sửa đổi từ {date}',
    sinceHint: 'Các object cũ hơn không được sao chép.',
    sinceAmountError: 'Nhập một số nguyên từ 1 trở lên.',
    sinceFutureError: 'Hãy chọn thời điểm không nằm trong tương lai.',
    sinceTooOldError: 'Hãy chọn ngày từ năm 2000 trở đi.',
    sinceDateMissing: 'Hãy chọn ngày và giờ.',
    reviewObjects: 'Object',
    reviewObjectsAll: 'Tất cả object',
    reviewObjectsSince: 'Sửa đổi từ {date}',
    listSince: 'Từ {date}',
    detailSince: 'Object được sao chép',
    detailSinceValue: 'Sửa đổi từ {date}',
  },
};
